package bowls

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metro-buddy/server/internal/db"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/media"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/ratelimit"
)

type Handlers struct {
	DB      *gorm.DB
	PII     *pii.Pipeline
	Media   *media.Service
	Limiter *ratelimit.Limiter
	// OnUGC lets the safety domain watch new text for distress signals; never punitive.
	OnUGC func(ctx context.Context, persona uuid.UUID, text string)
	// OnReply notifies the persona whose post or comment got a reply.
	OnReply func(ctx context.Context, to, postID, commentID uuid.UUID)
	// Publish fans out realtime events (wired by the realtime gateway); nil-safe.
	Publish func(personaIDs []uuid.UUID, event string, payload any)
	Now     func() time.Time
}

func (h *Handlers) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now().UTC()
}

func (h *Handlers) Register(g *echo.Group) {
	g.GET("/feed", h.home)
	g.GET("/bowls", h.list)
	g.GET("/bowls/joined", h.joined)
	g.GET("/bowls/:id", h.get)
	g.POST("/bowls/:id/join", h.join)
	g.DELETE("/bowls/:id/join", h.leave)
	h.registerPosts(g)
}

type LineRef struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Color string    `json:"color"`
}

type StationRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type BowlDTO struct {
	ID          uuid.UUID      `json:"id"`
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Kind        model.BowlKind `json:"kind"`
	Visibility  string         `json:"visibility"`
	CityID      *uuid.UUID     `json:"city_id"`
	Line        *LineRef       `json:"line"`
	Station     *StationRef    `json:"station"`
	MemberCount int            `json:"member_count"`
	Joined      bool           `json:"joined"`
	Role        *string        `json:"role"`
	RulesMD     string         `json:"rules_md,omitempty"`
	Reason      string         `json:"reason,omitempty"`
}

func (h *Handlers) dtos(tx *gorm.DB, viewer uuid.UUID, bowls []model.Bowl, withRules bool) ([]BowlDTO, error) {
	var lineIDs, stationIDs, bowlIDs []uuid.UUID
	for _, b := range bowls {
		bowlIDs = append(bowlIDs, b.ID)
		if b.LineID != nil {
			lineIDs = append(lineIDs, *b.LineID)
		}
		if b.StationID != nil {
			stationIDs = append(stationIDs, *b.StationID)
		}
	}
	lines := map[uuid.UUID]model.MetroLine{}
	if len(lineIDs) > 0 {
		var ls []model.MetroLine
		if err := tx.Where("id IN ?", lineIDs).Find(&ls).Error; err != nil {
			return nil, err
		}
		for _, l := range ls {
			lines[l.ID] = l
		}
	}
	stations := map[uuid.UUID]model.Station{}
	if len(stationIDs) > 0 {
		var ss []model.Station
		if err := tx.Where("id IN ?", stationIDs).Find(&ss).Error; err != nil {
			return nil, err
		}
		for _, s := range ss {
			stations[s.ID] = s
		}
	}
	roles := map[uuid.UUID]string{}
	if len(bowlIDs) > 0 {
		var ms []model.BowlMember
		if err := tx.Where("persona_id = ? AND bowl_id IN ?", viewer, bowlIDs).Find(&ms).Error; err != nil {
			return nil, err
		}
		for _, m := range ms {
			roles[m.BowlID] = string(m.Role)
		}
	}
	out := make([]BowlDTO, len(bowls))
	for i, b := range bowls {
		d := BowlDTO{ID: b.ID, Slug: b.Slug, Name: b.Name, Description: b.Description, Kind: b.Kind,
			Visibility: string(b.Visibility), CityID: b.CityID, MemberCount: b.MemberCount}
		if withRules {
			d.RulesMD = b.RulesMD
		}
		if b.LineID != nil {
			if l, ok := lines[*b.LineID]; ok {
				d.Line = &LineRef{ID: l.ID, Name: l.Name, Color: l.ColorHex}
			}
		}
		if b.StationID != nil {
			if s, ok := stations[*b.StationID]; ok {
				d.Station = &StationRef{ID: s.ID, Name: s.Name}
			}
		}
		if r, ok := roles[b.ID]; ok {
			d.Joined = true
			d.Role = &r
		}
		out[i] = d
	}
	return out, nil
}

type listQuery struct {
	Suggested bool   `query:"suggested"`
	Q         string `query:"q" validate:"max=60"`
	Kind      string `query:"kind" validate:"omitempty,oneof=city line station office_hub college topic"`
	Cursor    string `query:"cursor"`
	Limit     int    `query:"limit"`
}

func (h *Handlers) list(c echo.Context) error {
	req, err := httpx.Bind[listQuery](c)
	if err != nil {
		return err
	}
	if req.Suggested {
		return h.suggest(c)
	}
	p := httpx.PrincipalFrom(c)
	tx := h.DB.WithContext(c.Request().Context())
	var persona model.Persona
	if err := tx.First(&persona, "id = ?", p.PersonaID).Error; err != nil {
		return err
	}
	q := tx.Model(&model.Bowl{}).Where("city_id = ? OR city_id IS NULL", persona.CityID)
	if req.Kind != "" {
		q = q.Where("kind = ?", req.Kind)
	}
	if req.Q != "" {
		q = q.Where("LOWER(name) LIKE ? ESCAPE '\\'", "%"+escapeLike(strings.ToLower(req.Q))+"%")
	}
	q, limit, err := db.PageDesc(q, "", req.Cursor, req.Limit)
	if err != nil {
		return err
	}
	var rows []model.Bowl
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	rows, next := db.Trim(rows, limit, func(b model.Bowl) db.Cursor { return db.Cursor{CreatedAt: b.CreatedAt, ID: b.ID} })
	out, err := h.dtos(tx, p.PersonaID, rows, false)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out, "next_cursor": next})
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// Suggestions: your stations, the lines through them, hubs and colleges there, your city, then topics.
func (h *Handlers) suggest(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	var out []model.Bowl
	reasons := map[uuid.UUID]string{}
	err := h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var persona model.Persona
		if err := tx.First(&persona, "id = ?", p.PersonaID).Error; err != nil {
			return err
		}
		seen := map[uuid.UUID]bool{}
		add := func(bs []model.Bowl, why string) {
			for _, b := range bs {
				if !seen[b.ID] {
					seen[b.ID] = true
					reasons[b.ID] = why
					out = append(out, b)
				}
			}
		}
		for _, sid := range []*uuid.UUID{persona.HomeStationID, persona.WorkStationID} {
			if sid == nil {
				continue
			}
			b, err := EnsureStationBowl(tx, *sid)
			if err != nil {
				return err
			}
			add([]model.Bowl{*b}, "Your station")
			var lineIDs []uuid.UUID
			if err := tx.Model(&model.LineStation{}).Where("station_id = ?", *sid).Pluck("line_id", &lineIDs).Error; err != nil {
				return err
			}
			var lineBowls []model.Bowl
			if len(lineIDs) > 0 {
				if err := tx.Where("kind = ? AND line_id IN ?", model.BowlLine, lineIDs).Find(&lineBowls).Error; err != nil {
					return err
				}
			}
			add(lineBowls, "Your line")
			var hubBowls []model.Bowl
			if err := tx.Where("kind IN ? AND station_id = ?", []model.BowlKind{model.BowlOfficeHub, model.BowlCollege}, *sid).Find(&hubBowls).Error; err != nil {
				return err
			}
			add(hubBowls, "Near your station")
		}
		var city []model.Bowl
		if err := tx.Where("kind = ? AND city_id = ?", model.BowlCity, persona.CityID).Find(&city).Error; err != nil {
			return err
		}
		add(city, "Your city")
		var hubs []model.Bowl
		if err := tx.Where("kind IN ? AND city_id = ?", []model.BowlKind{model.BowlOfficeHub, model.BowlCollege}, persona.CityID).
			Order("member_count DESC").Limit(6).Find(&hubs).Error; err != nil {
			return err
		}
		add(hubs, "Popular in your city")
		var topics []model.Bowl
		if err := tx.Where("kind = ?", model.BowlTopic).Order("member_count DESC, name").Limit(8).Find(&topics).Error; err != nil {
			return err
		}
		add(topics, "Popular topic")
		return nil
	})
	if err != nil {
		return err
	}
	dtos, err := h.dtos(h.DB.WithContext(c.Request().Context()), p.PersonaID, out, false)
	if err != nil {
		return err
	}
	for i := range dtos {
		dtos[i].Reason = reasons[dtos[i].ID]
	}
	return c.JSON(http.StatusOK, map[string]any{"items": dtos})
}

func (h *Handlers) joined(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	tx := h.DB.WithContext(c.Request().Context())
	var rows []model.Bowl
	if err := tx.Joins("JOIN bowl_members ON bowl_members.bowl_id = bowls.id").
		Where("bowl_members.persona_id = ?", p.PersonaID).Order("bowls.name").Limit(200).Find(&rows).Error; err != nil {
		return err
	}
	out, err := h.dtos(tx, p.PersonaID, rows, false)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (h *Handlers) loadBowl(c echo.Context, tx *gorm.DB) (*model.Bowl, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return nil, httpx.NotFound()
	}
	var b model.Bowl
	if err := tx.First(&b, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (h *Handlers) get(c echo.Context) error {
	tx := h.DB.WithContext(c.Request().Context())
	b, err := h.loadBowl(c, tx)
	if err != nil {
		return err
	}
	out, err := h.dtos(tx, httpx.PrincipalFrom(c).PersonaID, []model.Bowl{*b}, true)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out[0])
}

func (h *Handlers) join(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	err := h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		b, err := h.loadBowl(c, tx)
		if err != nil {
			return err
		}
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.BowlMember{BowlID: b.ID, PersonaID: p.PersonaID, Role: model.BowlMemberRole, JoinedAt: h.now()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 1 {
			return tx.Model(&model.Bowl{}).Where("id = ?", b.ID).Update("member_count", gorm.Expr("member_count + 1")).Error
		}
		return nil
	})
	if err != nil {
		return err
	}
	return h.get(c)
}

func (h *Handlers) leave(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	err := h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		b, err := h.loadBowl(c, tx)
		if err != nil {
			return err
		}
		res := tx.Where("bowl_id = ? AND persona_id = ?", b.ID, p.PersonaID).Delete(&model.BowlMember{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 1 {
			return tx.Model(&model.Bowl{}).Where("id = ? AND member_count > 0", b.ID).Update("member_count", gorm.Expr("member_count - 1")).Error
		}
		return nil
	})
	if err != nil {
		return err
	}
	return h.get(c)
}

var ErrNotMember = httpx.Forbidden("join this bowl first")

// CanView implements scopeBowlVisible: members-only bowls hide content from non-members.
func CanView(tx *gorm.DB, b *model.Bowl, persona uuid.UUID) (bool, error) {
	if b.Visibility == model.BowlPublic {
		return true, nil
	}
	var n int64
	err := tx.Model(&model.BowlMember{}).Where("bowl_id = ? AND persona_id = ?", b.ID, persona).Count(&n).Error
	return n > 0, err
}

func membership(tx *gorm.DB, bowlID, persona uuid.UUID) (*model.BowlMember, error) {
	var m model.BowlMember
	err := tx.Where("bowl_id = ? AND persona_id = ?", bowlID, persona).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &m, err
}
