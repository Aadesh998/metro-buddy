package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/realtime"
)

const presencePrefix = "presence:"

// Receipts are separate from the core Service so the realtime hub can call into them.
type Receipts struct {
	*Service
	Redis *redis.Client
}

func (s *Service) registerReceipts(g *echo.Group) {
	g.POST("/conversations/:id/read", s.markReadHTTP)
}

type readRequest struct {
	MessageID uuid.UUID `json:"message_id" validate:"required"`
}

func (s *Service) markReadHTTP(c echo.Context) error {
	req, err := httpx.Bind[readRequest](c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return ErrNotMember
	}
	if err := s.MarkRead(c.Request().Context(), httpx.PrincipalFrom(c).PersonaID, id, req.MessageID, "read"); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// MarkRead moves the member's read (or delivered) pointer forward only, then tells the others.
func (s *Service) MarkRead(ctx context.Context, persona, convID, msgID uuid.UUID, kind string) error {
	col := "last_read_message_id"
	if kind == "delivered" {
		col = "last_delivered_message_id"
	}
	var members []uuid.UUID
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, me, err := member(tx, convID, persona)
		if err != nil {
			return err
		}
		var msg model.Message
		if err := tx.First(&msg, "id = ? AND conversation_id = ?", msgID, convID).Error; err != nil {
			return httpx.NotFound()
		}
		cur := me.LastReadMessageID
		if kind == "delivered" {
			cur = me.LastDeliveredMessageID
		}
		if cur != nil {
			var prev model.Message
			if tx.Select("created_at").First(&prev, "id = ?", *cur).Error == nil && !msg.CreatedAt.After(prev.CreatedAt) {
				return nil
			}
		}
		if err := tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND persona_id = ?", convID, persona).Update(col, msgID).Error; err != nil {
			return err
		}
		members, err = memberIDs(tx, convID)
		return err
	})
	if err != nil || len(members) == 0 {
		return err
	}
	others := members[:0]
	for _, m := range members {
		if m != persona {
			others = append(others, m)
		}
	}
	s.Hub.Publish(ctx, others, "receipt", map[string]any{"conversation_id": convID, "persona_id": persona, "message_id": msgID, "kind": kind})
	return nil
}

type convFrame struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	MessageID      uuid.UUID `json:"message_id"`
}

// HandleInbound processes client socket frames: typing indicators and delivery/read receipts.
func (r *Receipts) HandleInbound(ctx context.Context, persona uuid.UUID, in realtime.Inbound) {
	var f convFrame
	if json.Unmarshal(in.Data, &f) != nil || f.ConversationID == uuid.Nil {
		return
	}
	switch in.Type {
	case "typing":
		// Throttled so a held-down key doesn't flood the room.
		ok, err := r.Redis.SetNX(ctx, "typing:"+f.ConversationID.String()+":"+persona.String(), 1, 2*time.Second).Result()
		if err != nil || !ok {
			return
		}
		tx := r.DB.WithContext(ctx)
		if _, _, err := member(tx, f.ConversationID, persona); err != nil {
			return
		}
		members, err := memberIDs(tx, f.ConversationID)
		if err != nil {
			return
		}
		others := members[:0]
		for _, m := range members {
			if m != persona {
				others = append(others, m)
			}
		}
		r.Hub.Publish(ctx, others, "typing", map[string]any{"conversation_id": f.ConversationID, "persona_id": persona})
	case "read", "delivered":
		if f.MessageID != uuid.Nil {
			_ = r.MarkRead(ctx, persona, f.ConversationID, f.MessageID, in.Type)
		}
	}
}

// Presence is visible only to people you already chat 1:1 with (DM or accepted travel match).
func (r *Receipts) OnPresence(persona uuid.UUID, online bool) {
	ctx := context.Background()
	key := presencePrefix + persona.String()
	if online {
		was, _ := r.Redis.Exists(ctx, key).Result()
		r.Redis.Set(ctx, key, 1, realtime.PresenceTTL)
		if was == 1 {
			return
		}
	} else {
		r.Redis.Del(ctx, key)
	}
	var partners []uuid.UUID
	r.DB.WithContext(ctx).Table("conversation_members o").
		Joins("JOIN conversation_members me ON me.conversation_id = o.conversation_id AND me.persona_id = ? AND me.left_at IS NULL", persona).
		Joins("JOIN conversations c ON c.id = o.conversation_id AND c.kind IN ?", []model.ConversationKind{model.ConvDM, model.ConvTravel}).
		Where("o.persona_id <> ? AND o.left_at IS NULL", persona).Distinct().Pluck("o.persona_id", &partners)
	r.Hub.Publish(ctx, partners, "presence", map[string]any{"persona_id": persona, "online": online})
}

func (r *Receipts) Online(ctx context.Context, persona uuid.UUID) bool {
	n, _ := r.Redis.Exists(ctx, presencePrefix+persona.String()).Result()
	return n == 1
}
