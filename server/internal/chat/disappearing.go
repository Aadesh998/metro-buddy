package chat

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
)

// Disappearing messages (F26): a per-conversation timer applied to messages sent after it is
// set. Expired messages are hidden from reads at once and hard-deleted by PurgeExpired.
var DisappearOptions = []int{0, 3600, 86400, 7 * 86400}

type disappearRequest struct {
	Seconds *int `json:"seconds" validate:"required"`
}

func disappearLabel(sec int) string {
	switch sec {
	case 3600:
		return "1 hour"
	case 86400:
		return "24 hours"
	default:
		return "7 days"
	}
}

func (s *Service) setDisappearing(c echo.Context) error {
	req, err := httpx.Bind[disappearRequest](c)
	if err != nil {
		return err
	}
	if !slices.Contains(DisappearOptions, *req.Seconds) {
		return httpx.Unprocessable("invalid_timer", "choose off, 1 hour, 24 hours or 7 days")
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return ErrNotMember
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	tx := s.DB.WithContext(ctx)
	conv, me, err := member(tx, id, p.PersonaID)
	if err != nil {
		return err
	}
	switch conv.Kind {
	case model.ConvDiscussion:
		return httpx.Forbidden("rooms already close on their own")
	case model.ConvGroup:
		if me.Role == model.ConvMember {
			return httpx.Forbidden("only group admins can change this")
		}
	}
	var val any
	if *req.Seconds > 0 {
		val = *req.Seconds
	}
	if err := tx.Model(&model.Conversation{}).Where("id = ?", conv.ID).Update("disappear_after_sec", val).Error; err != nil {
		return err
	}
	refs, err := identity.LoadRefs(tx, []uuid.UUID{p.PersonaID})
	if err != nil {
		return err
	}
	text := refs[p.PersonaID].Alias + " turned off disappearing messages"
	if *req.Seconds > 0 {
		text = refs[p.PersonaID].Alias + " turned on disappearing messages. New messages vanish after " + disappearLabel(*req.Seconds) + "."
	}
	if err := s.PostSystem(ctx, conv.ID, model.MsgSystem, text); err != nil {
		return err
	}
	return s.get(c)
}

// PurgeExpired hard-deletes expired messages and their images. Messages under an open
// moderation case are kept as evidence until the case closes.
func (s *Service) PurgeExpired(ctx context.Context) error {
	tx := s.DB.WithContext(ctx)
	for {
		var msgs []model.Message
		if err := tx.Unscoped().Where("expires_at IS NOT NULL AND expires_at < ?", s.now()).
			Where("id NOT IN (SELECT target_id FROM moderation_cases WHERE target_type = ? AND status IN ?)", model.CTMessage, []model.CaseStatus{model.CaseOpen, model.CaseInReview}).
			Limit(500).Find(&msgs).Error; err != nil {
			return err
		}
		if len(msgs) == 0 {
			return nil
		}
		ids := make([]uuid.UUID, len(msgs))
		for i, m := range msgs {
			ids[i] = m.ID
			for _, im := range m.Media {
				if s.Media != nil && im.Key != "" {
					_ = s.Media.Store.Delete(ctx, im.Key)
				}
			}
		}
		// Read pointers may reference purged rows. Step them back to the newest surviving message at or before
		// that point, so unread counts stay right; with nothing left before it the pointer clears and every
		// surviving message (all newer) counts from the join time, which is also right.
		for _, col := range []string{"last_read_message_id", "last_delivered_message_id"} {
			tx.Model(&model.ConversationMember{}).Where(col+" IN ?", ids).
				Update(col, gorm.Expr(`(SELECT m2.id FROM messages m2 WHERE m2.conversation_id = conversation_members.conversation_id AND m2.id NOT IN ?
					AND m2.created_at <= (SELECT p.created_at FROM messages p WHERE p.id = conversation_members.`+col+`) ORDER BY m2.created_at DESC LIMIT 1)`, ids))
		}
		if err := tx.Unscoped().Where("id IN ?", ids).Delete(&model.Message{}).Error; err != nil {
			return err
		}
		if len(msgs) < 500 {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
}
