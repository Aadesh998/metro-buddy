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
	"metro-buddy/server/internal/media"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
)

// Stickers is the fixed sticker set; clients render each id from their bundled pack.
var Stickers = []string{
	"chai", "late-again", "rush-hour", "window-seat", "mind-the-gap", "on-my-way", "same", "hug",
	"lol", "facepalm", "party", "thank-you", "good-morning", "good-night", "monsoon", "dosa",
}

const EditWindow = 15 * time.Minute

var (
	ErrEditWindow = httpx.Conflict("messages can only be edited for 15 minutes")
	ErrViewedOnce = httpx.E(http.StatusGone, "already_viewed", "this photo can only be opened once")
)

func viewOncePlaceholders(ms model.MediaList) []media.MediaDTO {
	out := make([]media.MediaDTO, len(ms))
	for i, m := range ms {
		out[i] = media.MediaDTO{Width: m.Width, Height: m.Height, Pending: !m.Cleared}
	}
	return out
}

func (s *Service) viewedByMe(tx *gorm.DB, viewer uuid.UUID, msgs []model.Message) map[uuid.UUID]bool {
	out := map[uuid.UUID]bool{}
	if viewer == uuid.Nil {
		return out
	}
	var ids []uuid.UUID
	for _, m := range msgs {
		if m.ViewOnce {
			ids = append(ids, m.ID)
		}
	}
	if len(ids) == 0 {
		return out
	}
	var seen []uuid.UUID
	tx.Model(&model.MessageView{}).Where("persona_id = ? AND message_id IN ?", viewer, ids).Pluck("message_id", &seen)
	for _, id := range seen {
		out[id] = true
	}
	return out
}

type editRequest struct {
	Body string `json:"body" validate:"required,min=1,max=4000"`
}

// editMessage lets the sender fix a text message briefly; the new text is masked like any other.
func (s *Service) editMessage(c echo.Context) error {
	req, err := httpx.Bind[editRequest](c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var msg model.Message
	var members []uuid.UUID
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&msg, "id = ? AND sender_persona_id = ?", id, p.PersonaID).Error; err != nil {
			return err
		}
		if msg.Kind != model.MsgText || msg.Status != model.ContentVisible {
			return httpx.Conflict("this message can't be edited")
		}
		if s.now().Sub(msg.CreatedAt) > EditWindow {
			return ErrEditWindow
		}
		opts := pii.Options{}
		if s.MaskOptions != nil {
			opts = s.MaskOptions(tx, msg.ConversationID)
		}
		r, err := s.PII.Process(tx, model.CTMessage, msg.ID, req.Body, opts)
		if err != nil {
			return err
		}
		now := s.now()
		msg.BodyMasked, msg.PIIFlags, msg.EditedAt = r.Text, r.Flags, &now
		if err := tx.Model(&msg).Select("body_masked", "pii_flags", "edited_at").Updates(&msg).Error; err != nil {
			return err
		}
		members, err = memberIDs(tx, msg.ConversationID)
		return err
	})
	if err != nil {
		return err
	}
	s.PII.AfterCommit(ctx, model.CTMessage, msg.ID)
	s.Hub.Publish(ctx, members, "message.updated", map[string]any{
		"id": msg.ID, "conversation_id": msg.ConversationID, "body": msg.BodyMasked, "pii_hidden": msg.PIIFlags != 0, "edited_at": msg.EditedAt,
	})
	out, err := s.messageDTOs(ctx, s.DB.WithContext(ctx), p.PersonaID, []model.Message{msg})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out[0])
}

// viewOnce hands a recipient a short-lived link to a view-once photo, exactly once.
func (s *Service) viewOnce(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var msg model.Message
	var url string
	var allSeen bool
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&msg, "id = ? AND view_once = ?", id, true).Error; err != nil {
			return err
		}
		if _, _, err := member(tx, msg.ConversationID, p.PersonaID); err != nil {
			return err
		}
		if msg.SenderPersonaID != nil && *msg.SenderPersonaID == p.PersonaID {
			return httpx.Forbidden("you sent this photo")
		}
		if msg.Status != model.ContentVisible || len(msg.Media) == 0 || !msg.Media[0].Cleared {
			return ErrViewedOnce
		}
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.MessageView{MessageID: msg.ID, PersonaID: p.PersonaID, ViewedAt: s.now()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrViewedOnce
		}
		u, err := s.Media.Store.PresignGet(ctx, msg.Media[0].Key, time.Minute)
		if err != nil {
			return err
		}
		url = u
		var recipients, views int64
		tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND left_at IS NULL AND persona_id <> ?", msg.ConversationID, msg.SenderPersonaID).Count(&recipients)
		tx.Model(&model.MessageView{}).Where("message_id = ?", msg.ID).Count(&views)
		allSeen = views >= recipients
		return nil
	})
	if err != nil {
		return err
	}
	if allSeen {
		// Everyone has seen it: the file goes, shortly after the last viewer's link was issued.
		key := msg.Media[0].Key
		time.AfterFunc(2*time.Minute, func() { _ = s.Media.Store.Delete(context.Background(), key) })
		s.DB.WithContext(ctx).Model(&model.Message{}).Where("id = ?", msg.ID).Update("media", model.MediaList{})
	}
	return c.JSON(http.StatusOK, map[string]any{"url": url, "expires_in": 60})
}
