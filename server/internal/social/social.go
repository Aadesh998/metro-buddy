// Package social holds follows (one-way, no consent) and connections (mutual, consent-gated).
package social

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/ratelimit"
	"metro-buddy/server/internal/safety"
)

type Publisher interface {
	Publish(ctx context.Context, personas []uuid.UUID, event string, data any)
}

type Service struct {
	DB      *gorm.DB
	PII     *pii.Pipeline
	Hub     Publisher
	Limiter *ratelimit.Limiter
	// OnFollow and OnConnection let push notifications fire; nil-safe.
	OnFollow     func(ctx context.Context, follower, followee uuid.UUID)
	OnConnection func(ctx context.Context, to, from uuid.UUID, accepted bool)
	Now          func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

func (s *Service) Register(g *echo.Group) {
	follows := s.Limiter.Middleware(ratelimit.Rule{Name: "follows", Limit: 100, Window: 24 * time.Hour, ByTier: true})
	connects := s.Limiter.Middleware(ratelimit.Rule{Name: "connections", Limit: 20, Window: 24 * time.Hour, ByTier: true})
	g.GET("/personas/:id/relationship", s.relationship)
	g.POST("/personas/:id/follow", s.follow, follows)
	g.DELETE("/personas/:id/follow", s.unfollow)
	g.GET("/me/followers", s.followers)
	g.GET("/me/following", s.following)
	g.POST("/connections", s.connect, connects)
	g.GET("/connections", s.listConnections)
	g.POST("/connections/:id/accept", s.accept)
	g.POST("/connections/:id/decline", s.decline)
	g.DELETE("/connections/:id", s.remove)
}

// ConnState is the connection as seen by the viewer.
type ConnState string

const (
	ConnNone     ConnState = "none"
	ConnOutgoing ConnState = "outgoing"
	ConnIncoming ConnState = "incoming"
	ConnDone     ConnState = "connected"
)

type RelationshipDTO struct {
	PersonaID    uuid.UUID  `json:"persona_id"`
	Following    bool       `json:"following"`
	FollowsYou   bool       `json:"follows_you"`
	Connection   ConnState  `json:"connection"`
	ConnectionID *uuid.UUID `json:"connection_id"`
	Followers    int64      `json:"followers"`
	FollowingN   int64      `json:"following_count"`
	Connections  int64      `json:"connections"`
}

type ConnectionDTO struct {
	ID         uuid.UUID           `json:"id"`
	Persona    identity.PersonaRef `json:"persona"`
	State      ConnState           `json:"state"`
	Note       string              `json:"note"`
	CreatedAt  time.Time           `json:"created_at"`
	AcceptedAt *time.Time          `json:"accepted_at"`
}

// target loads another live persona the viewer may interact with. Blocked looks the same as missing.
func (s *Service) target(c echo.Context, raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, httpx.NotFound()
	}
	p := httpx.PrincipalFrom(c)
	if id == p.PersonaID {
		return uuid.Nil, httpx.BadRequest("that's you")
	}
	ctx := c.Request().Context()
	var n int64
	if err := s.DB.WithContext(ctx).Model(&model.Persona{}).Where("id = ?", id).Count(&n).Error; err != nil {
		return uuid.Nil, err
	}
	if n == 0 {
		return uuid.Nil, httpx.NotFound()
	}
	if b, err := safety.IsBlocked(ctx, s.DB, p.PersonaID, id); err != nil || b {
		return uuid.Nil, httpx.NotFound()
	}
	return id, nil
}

// stateFor maps a connection row to the viewer's side of it. A declined request
// still reads as outgoing to its sender so they never learn it was declined.
func stateFor(c *model.Connection, viewer uuid.UUID) ConnState {
	switch {
	case c == nil:
		return ConnNone
	case c.Status == model.ConnAccepted:
		return ConnDone
	case c.FromPersonaID == viewer:
		return ConnOutgoing
	case c.Status == model.ConnDeclined:
		return ConnNone
	default:
		return ConnIncoming
	}
}

func loadPair(tx *gorm.DB, a, b uuid.UUID) (*model.Connection, error) {
	var c model.Connection
	err := tx.Where("pair_key = ?", model.PairKey(a, b)).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &c, err
}

// Connected reports whether two personas have an accepted connection.
func Connected(tx *gorm.DB, a, b uuid.UUID) (bool, error) {
	var n int64
	err := tx.Model(&model.Connection{}).Where("pair_key = ? AND status = ?", model.PairKey(a, b), model.ConnAccepted).Count(&n).Error
	return n > 0, err
}

func (s *Service) counts(tx *gorm.DB, id uuid.UUID, r *RelationshipDTO) error {
	if err := tx.Model(&model.Follow{}).Where("followee_persona_id = ?", id).Count(&r.Followers).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.Follow{}).Where("follower_persona_id = ?", id).Count(&r.FollowingN).Error; err != nil {
		return err
	}
	return tx.Model(&model.Connection{}).Where("(from_persona_id = ? OR to_persona_id = ?) AND status = ?", id, id, model.ConnAccepted).Count(&r.Connections).Error
}

func (s *Service) relationshipOf(tx *gorm.DB, viewer, id uuid.UUID) (RelationshipDTO, error) {
	r := RelationshipDTO{PersonaID: id, Connection: ConnNone}
	var n int64
	if err := tx.Model(&model.Follow{}).Where("follower_persona_id = ? AND followee_persona_id = ?", viewer, id).Count(&n).Error; err != nil {
		return r, err
	}
	r.Following = n > 0
	if err := tx.Model(&model.Follow{}).Where("follower_persona_id = ? AND followee_persona_id = ?", id, viewer).Count(&n).Error; err != nil {
		return r, err
	}
	r.FollowsYou = n > 0
	conn, err := loadPair(tx, viewer, id)
	if err != nil {
		return r, err
	}
	if r.Connection = stateFor(conn, viewer); r.Connection != ConnNone {
		r.ConnectionID = &conn.ID
	}
	return r, s.counts(tx, id, &r)
}

func (s *Service) relationship(c echo.Context) error {
	id, err := s.target(c, c.Param("id"))
	if err != nil {
		return err
	}
	r, err := s.relationshipOf(s.DB.WithContext(c.Request().Context()), httpx.PrincipalFrom(c).PersonaID, id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, r)
}

func (s *Service) follow(c echo.Context) error {
	id, err := s.target(c, c.Param("id"))
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	res := s.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model.Follow{FollowerPersonaID: p.PersonaID, FolloweePersonaID: id, CreatedAt: s.now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 1 && s.OnFollow != nil {
		s.OnFollow(ctx, p.PersonaID, id)
	}
	return s.relationship(c)
}

func (s *Service) unfollow(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	p := httpx.PrincipalFrom(c)
	if err := s.DB.WithContext(c.Request().Context()).
		Where("follower_persona_id = ? AND followee_persona_id = ?", p.PersonaID, id).Delete(&model.Follow{}).Error; err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// Follower lists are visible only to their owner.
func (s *Service) listFollows(c echo.Context, mine, theirs string) error {
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	var rows []model.Follow
	if err := tx.Where(mine+" = ?", p.PersonaID).Scopes(safety.ExcludeBlocked(p.PersonaID, theirs)).
		Order("created_at DESC").Limit(500).Find(&rows).Error; err != nil {
		return err
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.FolloweePersonaID
		if theirs == "follower_persona_id" {
			ids[i] = r.FollowerPersonaID
		}
	}
	refs, err := identity.LoadRefs(tx, ids)
	if err != nil {
		return err
	}
	out := make([]identity.PersonaRef, 0, len(ids))
	for _, id := range ids {
		if ref := refs[id]; ref.Alias != identity.DeletedAlias {
			out = append(out, ref)
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (s *Service) followers(c echo.Context) error {
	return s.listFollows(c, "followee_persona_id", "follower_persona_id")
}

func (s *Service) following(c echo.Context) error {
	return s.listFollows(c, "follower_persona_id", "followee_persona_id")
}

type connectBody struct {
	ToPersonaID uuid.UUID `json:"to_persona_id" validate:"required"`
	Note        string    `json:"note" validate:"max=140"`
}

func (s *Service) connect(c echo.Context) error {
	req, err := httpx.Bind[connectBody](c)
	if err != nil {
		return err
	}
	to, err := s.target(c, req.ToPersonaID.String())
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var conn model.Connection
	notifyTo, accepted := to, false
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := loadPair(tx, p.PersonaID, to)
		if err != nil {
			return err
		}
		now := s.now()
		note := ""
		if req.Note != "" {
			res, err := s.PII.Process(tx, model.CTConversation, model.NewID(), req.Note, pii.Options{})
			if err != nil {
				return err
			}
			note = res.Text
		}
		switch {
		case existing == nil:
			conn = model.Connection{FromPersonaID: p.PersonaID, ToPersonaID: to, PairKey: model.PairKey(p.PersonaID, to), NoteMasked: note, Status: model.ConnPending}
			return tx.Create(&conn).Error
		case existing.Status == model.ConnAccepted:
			conn = *existing
			notifyTo = uuid.Nil
			return nil
		case existing.FromPersonaID == p.PersonaID:
			// Already sent (or quietly declined): nothing changes.
			conn = *existing
			notifyTo = uuid.Nil
			return nil
		case existing.Status == model.ConnPending:
			// They asked first: asking back accepts.
			conn = *existing
			accepted, notifyTo = true, existing.FromPersonaID
			return tx.Model(&conn).Updates(map[string]any{"status": model.ConnAccepted, "responded_at": now}).Error
		default:
			// The viewer declined them earlier and now asks themselves: start over in this direction.
			conn = *existing
			return tx.Model(&conn).Updates(map[string]any{"from_persona_id": p.PersonaID, "to_persona_id": to,
				"status": model.ConnPending, "note_masked": note, "responded_at": nil, "created_at": now}).Error
		}
	})
	if err != nil {
		return err
	}
	if notifyTo != uuid.Nil {
		s.Hub.Publish(ctx, []uuid.UUID{notifyTo}, "connection.updated", map[string]any{"id": conn.ID})
		if s.OnConnection != nil {
			s.OnConnection(ctx, notifyTo, p.PersonaID, accepted)
		}
	}
	status := http.StatusCreated
	if accepted || notifyTo == uuid.Nil {
		status = http.StatusOK
	}
	r, err := s.relationshipOf(s.DB.WithContext(ctx), p.PersonaID, to)
	if err != nil {
		return err
	}
	return c.JSON(status, r)
}

func (s *Service) listConnections(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	q := tx.Model(&model.Connection{}).Order("updated_at DESC").Limit(500)
	switch c.QueryParam("box") {
	case "incoming":
		q = q.Where("to_persona_id = ? AND status = ?", p.PersonaID, model.ConnPending).Scopes(safety.ExcludeBlocked(p.PersonaID, "from_persona_id"))
	case "outgoing":
		q = q.Where("from_persona_id = ? AND status IN ?", p.PersonaID, []model.ConnectionStatus{model.ConnPending, model.ConnDeclined})
	default:
		q = q.Where("(from_persona_id = ? OR to_persona_id = ?) AND status = ?", p.PersonaID, p.PersonaID, model.ConnAccepted)
	}
	var rows []model.Connection
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	other := func(r model.Connection) uuid.UUID {
		if r.FromPersonaID == p.PersonaID {
			return r.ToPersonaID
		}
		return r.FromPersonaID
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = other(r)
	}
	refs, err := identity.LoadRefs(tx, ids)
	if err != nil {
		return err
	}
	blocked, err := safety.BlockedSet(c.Request().Context(), tx, p.PersonaID)
	if err != nil {
		return err
	}
	out := make([]ConnectionDTO, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		id := other(*r)
		if blocked[id] || refs[id].Alias == identity.DeletedAlias {
			continue
		}
		dto := ConnectionDTO{ID: r.ID, Persona: refs[id], State: stateFor(r, p.PersonaID), CreatedAt: r.CreatedAt}
		if r.ToPersonaID == p.PersonaID {
			dto.Note = r.NoteMasked
		}
		if r.Status == model.ConnAccepted {
			dto.AcceptedAt = r.RespondedAt
		}
		out = append(out, dto)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (s *Service) incoming(tx *gorm.DB, raw string, viewer uuid.UUID) (*model.Connection, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, httpx.NotFound()
	}
	var conn model.Connection
	if err := tx.First(&conn, "id = ? AND to_persona_id = ? AND status = ?", id, viewer, model.ConnPending).Error; err != nil {
		return nil, httpx.NotFound()
	}
	return &conn, nil
}

func (s *Service) accept(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var conn *model.Connection
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		if conn, err = s.incoming(tx, c.Param("id"), p.PersonaID); err != nil {
			return err
		}
		if b, err := safety.IsBlocked(ctx, tx, conn.FromPersonaID, conn.ToPersonaID); err != nil || b {
			return httpx.NotFound()
		}
		return tx.Model(conn).Updates(map[string]any{"status": model.ConnAccepted, "responded_at": s.now()}).Error
	})
	if err != nil {
		return err
	}
	s.Hub.Publish(ctx, []uuid.UUID{conn.FromPersonaID}, "connection.updated", map[string]any{"id": conn.ID})
	if s.OnConnection != nil {
		s.OnConnection(ctx, conn.FromPersonaID, p.PersonaID, true)
	}
	r, err := s.relationshipOf(s.DB.WithContext(ctx), p.PersonaID, conn.FromPersonaID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, r)
}

func (s *Service) decline(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	err := s.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		conn, err := s.incoming(tx, c.Param("id"), p.PersonaID)
		if err != nil {
			return err
		}
		return tx.Model(conn).Updates(map[string]any{"status": model.ConnDeclined, "responded_at": s.now()}).Error
	})
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// remove withdraws a request the viewer sent, or ends a connection from either side.
func (s *Service) remove(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	p := httpx.PrincipalFrom(c)
	res := s.DB.WithContext(c.Request().Context()).
		Where("id = ? AND ((from_persona_id = ? AND status IN ?) OR ((from_persona_id = ? OR to_persona_id = ?) AND status = ?))",
			id, p.PersonaID, []model.ConnectionStatus{model.ConnPending, model.ConnDeclined}, p.PersonaID, p.PersonaID, model.ConnAccepted).
		Delete(&model.Connection{})
	if res.Error != nil {
		return res.Error
	}
	return c.NoContent(http.StatusNoContent)
}
