package chat

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/ratelimit"
)

const (
	RoomCap       = 300
	MinRoomTier   = 1
	roomJoinEarly = 15 * time.Minute
)

func (s *Service) registerRooms(g *echo.Group) {
	g.POST("/rooms", s.createRoom, s.Limiter.Middleware(ratelimit.Rule{Name: "rooms", Limit: 3, Window: 24 * time.Hour}))
	g.GET("/rooms/live", s.liveRooms)
	g.GET("/rooms", s.upcomingRooms)
	g.POST("/rooms/:id/join", s.joinRoom)
	g.POST("/rooms/:id/close", s.closeRoom)
	g.PATCH("/rooms/:id", s.patchRoom)
}

type createRoomRequest struct {
	BowlID      uuid.UUID `json:"bowl_id" validate:"required"`
	Title       string    `json:"title" validate:"required,min=3,max=60"`
	Topic       string    `json:"topic" validate:"max=200"`
	StartsAt    time.Time `json:"starts_at" validate:"required"`
	DurationMin int       `json:"duration_min" validate:"required,min=15,max=120"`
	SlowModeSec int16     `json:"slow_mode_sec" validate:"min=0,max=300"`
}

// Hosting a room is earned: it needs trust tier 1 and membership of the bowl it belongs to.
func (s *Service) createRoom(c echo.Context) error {
	req, err := httpx.Bind[createRoomRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	if p.TrustTier < MinRoomTier {
		return httpx.Forbidden("hosting rooms unlocks once you've been around a little longer")
	}
	now := s.now()
	if req.StartsAt.Before(now.Add(-time.Minute)) || req.StartsAt.After(now.Add(14*24*time.Hour)) {
		return httpx.Unprocessable("invalid_start", "start within the next two weeks")
	}
	ctx := c.Request().Context()
	var conv model.Conversation
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&model.BowlMember{}).Where("bowl_id = ? AND persona_id = ?", req.BowlID, p.PersonaID).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return httpx.Forbidden("join the bowl first")
		}
		title := pii.Mask(req.Title, pii.Options{}).Text
		topic := pii.Mask(req.Topic, pii.Options{}).Text
		start := req.StartsAt.UTC()
		end := start.Add(time.Duration(req.DurationMin) * time.Minute)
		conv = model.Conversation{Kind: model.ConvDiscussion, Title: &title, Topic: &topic, BowlID: &req.BowlID, StartsAt: &start, EndsAt: &end,
			SlowModeSec: req.SlowModeSec, MemberCap: RoomCap, CreatedByPersonaID: &p.PersonaID}
		return Create(tx, now, &conv, map[uuid.UUID]model.ConvRole{p.PersonaID: model.ConvOwner})
	})
	if err != nil {
		return err
	}
	c.SetParamNames("id")
	c.SetParamValues(conv.ID.String())
	return s.get(c)
}

type RoomDTO struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Topic       string     `json:"topic"`
	BowlID      *uuid.UUID `json:"bowl_id"`
	StartsAt    time.Time  `json:"starts_at"`
	EndsAt      time.Time  `json:"ends_at"`
	Live        bool       `json:"live"`
	MemberCount int64      `json:"member_count"`
	Joined      bool       `json:"joined"`
	SlowModeSec int16      `json:"slow_mode_sec"`
}

func (s *Service) roomDTOs(tx *gorm.DB, viewer uuid.UUID, rooms []model.Conversation) []RoomDTO {
	now := s.now()
	out := make([]RoomDTO, len(rooms))
	for i, r := range rooms {
		d := RoomDTO{ID: r.ID, BowlID: r.BowlID, StartsAt: *r.StartsAt, EndsAt: *r.EndsAt, SlowModeSec: r.SlowModeSec,
			Live: r.ClosedAt == nil && !now.Before(*r.StartsAt) && now.Before(*r.EndsAt)}
		if r.Title != nil {
			d.Title = *r.Title
		}
		if r.Topic != nil {
			d.Topic = *r.Topic
		}
		tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND left_at IS NULL", r.ID).Count(&d.MemberCount)
		var n int64
		tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND persona_id = ? AND left_at IS NULL", r.ID, viewer).Count(&n)
		d.Joined = n > 0
		out[i] = d
	}
	return out
}

func (s *Service) liveRooms(c echo.Context) error {
	now := s.now()
	tx := s.DB.WithContext(c.Request().Context())
	q := tx.Where("kind = ? AND closed_at IS NULL AND starts_at <= ? AND ends_at > ?", model.ConvDiscussion, now, now)
	if b := c.QueryParam("bowl_id"); b != "" {
		q = q.Where("bowl_id = ?", b)
	}
	var rooms []model.Conversation
	if err := q.Order("starts_at").Limit(50).Find(&rooms).Error; err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": s.roomDTOs(tx, httpx.PrincipalFrom(c).PersonaID, rooms)})
}

func (s *Service) upcomingRooms(c echo.Context) error {
	now := s.now()
	tx := s.DB.WithContext(c.Request().Context())
	q := tx.Where("kind = ? AND closed_at IS NULL AND ends_at > ?", model.ConvDiscussion, now)
	if b := c.QueryParam("bowl_id"); b != "" {
		q = q.Where("bowl_id = ?", b)
	}
	var rooms []model.Conversation
	if err := q.Order("starts_at").Limit(50).Find(&rooms).Error; err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": s.roomDTOs(tx, httpx.PrincipalFrom(c).PersonaID, rooms)})
}

// Rooms are public: joining is the consent to receive messages from everyone in it.
func (s *Service) joinRoom(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	p := httpx.PrincipalFrom(c)
	now := s.now()
	err = s.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var conv model.Conversation
		if err := tx.First(&conv, "id = ? AND kind = ?", id, model.ConvDiscussion).Error; err != nil {
			return httpx.NotFound()
		}
		if conv.ClosedAt != nil || now.After(*conv.EndsAt) {
			return ErrClosed
		}
		if now.Before(conv.StartsAt.Add(-roomJoinEarly)) {
			return httpx.Conflict("this room opens 15 minutes before it starts")
		}
		var n int64
		tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND left_at IS NULL", id).Count(&n)
		if n >= int64(conv.MemberCap) {
			return httpx.Conflict("this room is full")
		}
		var prev model.ConversationMember
		if tx.Where("conversation_id = ? AND persona_id = ?", id, p.PersonaID).Limit(1).Find(&prev).RowsAffected == 1 {
			return tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND persona_id = ?", id, p.PersonaID).Update("left_at", nil).Error
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.ConversationMember{ConversationID: id, PersonaID: p.PersonaID, Role: model.ConvMember, JoinedAt: now}).Error
	})
	if err != nil {
		return err
	}
	return s.get(c)
}

type roomPatch struct {
	SlowModeSec *int16 `json:"slow_mode_sec" validate:"omitempty,min=0,max=300"`
}

func (s *Service) patchRoom(c echo.Context) error {
	req, err := httpx.Bind[roomPatch](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	err = s.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		conv, _, err := s.requireAdmin(tx, c.Param("id"), p.PersonaID)
		if err != nil {
			return err
		}
		if req.SlowModeSec != nil {
			return tx.Model(conv).Update("slow_mode_sec", *req.SlowModeSec).Error
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.get(c)
}

func (s *Service) closeRoom(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	var conv *model.Conversation
	err := s.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var err error
		conv, _, err = s.requireAdmin(tx, c.Param("id"), p.PersonaID)
		return err
	})
	if err != nil {
		return err
	}
	if err := s.close(c.Request().Context(), conv.ID, "The host closed this room."); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Service) close(ctx context.Context, id uuid.UUID, notice string) error {
	var members []uuid.UUID
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Conversation{}).Where("id = ? AND closed_at IS NULL", id).Update("closed_at", s.now())
		if res.Error != nil || res.RowsAffected == 0 {
			return res.Error
		}
		if _, err := s.SystemMessage(ctx, tx, id, model.MsgSystem, notice); err != nil {
			return err
		}
		var err error
		members, err = memberIDs(tx, id)
		return err
	})
	if err != nil {
		return err
	}
	s.Hub.Publish(ctx, members, "room.closed", map[string]any{"conversation_id": id})
	return nil
}

// CloseExpiredRooms runs every minute; a room's end time is a hard stop.
func (s *Service) CloseExpiredRooms(ctx context.Context) error {
	var ids []uuid.UUID
	if err := s.DB.WithContext(ctx).Model(&model.Conversation{}).
		Where("kind = ? AND closed_at IS NULL AND ends_at <= ?", model.ConvDiscussion, s.now()).Pluck("id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.close(ctx, id, "This room has ended. Thanks for talking."); err != nil {
			return err
		}
	}
	return nil
}
