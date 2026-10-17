package chat

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/safety"
)

// Reveal ladder (F08): none → first_name → photo → contact, one-to-one chats only.
// Each person's member row holds the level they have offered; what either side actually
// sees is the lower of the two, so every step needs both people. Either can step back down
// at any time, which hides what was shared and re-masks contact details in new messages.
var ladder = []model.RevealLevel{model.RevealNone, model.RevealFirstName, model.RevealPhoto, model.RevealContact}

var (
	ErrRevealKind    = httpx.Forbidden("reveal works in one-to-one chats only")
	ErrRevealProfile = httpx.Unprocessable("reveal_profile_missing", "add this to your private reveal card first")
)

func (s *Service) registerReveal(g *echo.Group) {
	g.GET("/me/reveal-card", s.getRevealCard)
	g.PUT("/me/reveal-card", s.putRevealCard)
	g.GET("/conversations/:id/reveal", s.getReveal)
	g.POST("/conversations/:id/reveal", s.stepReveal)
}

type RevealCardDTO struct {
	FirstName string  `json:"first_name"`
	PhotoURL  *string `json:"photo_url"`
	Contact   string  `json:"contact"`
}

func (s *Service) card(ctx context.Context, tx *gorm.DB, userID uuid.UUID) (RevealCardDTO, *model.RevealProfile) {
	var rp model.RevealProfile
	if tx.Where("user_id = ?", userID).Limit(1).Find(&rp).RowsAffected == 0 {
		return RevealCardDTO{}, nil
	}
	d := RevealCardDTO{}
	if len(rp.FirstNameEnc) > 0 {
		d.FirstName, _ = s.Box.OpenString(rp.FirstNameEnc)
	}
	if len(rp.ContactEnc) > 0 {
		d.Contact, _ = s.Box.OpenString(rp.ContactEnc)
	}
	if rp.PhotoKey != nil && s.Media != nil {
		if ms := s.Media.DTOs(ctx, model.MediaList{{Key: *rp.PhotoKey, Cleared: true}}); len(ms) == 1 && ms[0].URL != "" {
			d.PhotoURL = &ms[0].URL
		}
	}
	return d, &rp
}

func (s *Service) getRevealCard(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	d, _ := s.card(c.Request().Context(), s.DB.WithContext(c.Request().Context()), p.UserID)
	return c.JSON(http.StatusOK, d)
}

type revealCardRequest struct {
	FirstName   *string `json:"first_name" validate:"omitempty,max=30"`
	Contact     *string `json:"contact" validate:"omitempty,max=80"`
	PhotoKey    *string `json:"photo_key" validate:"omitempty,max=200"`
	RemovePhoto bool    `json:"remove_photo"`
}

// putRevealCard stores real details, encrypted. They are never shown anywhere until a
// conversation partner and the user have both stepped up to that level.
func (s *Service) putRevealCard(c echo.Context) error {
	req, err := httpx.Bind[revealCardRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	tx := s.DB.WithContext(ctx)
	_, existing := s.card(ctx, tx, p.UserID)
	rp := model.RevealProfile{UserID: p.UserID}
	if existing != nil {
		rp = *existing
	}
	if req.FirstName != nil {
		rp.FirstNameEnc = nil
		if v := strings.TrimSpace(*req.FirstName); v != "" {
			rp.FirstNameEnc = s.Box.SealString(v)
		}
	}
	if req.Contact != nil {
		rp.ContactEnc = nil
		if v := strings.TrimSpace(*req.Contact); v != "" {
			rp.ContactEnc = s.Box.SealString(v)
		}
	}
	if req.PhotoKey != nil {
		m, verdict, err := s.Media.Finalize(ctx, p.PersonaID, *req.PhotoKey)
		if err != nil {
			return err
		}
		if !m.Cleared {
			_ = s.Media.Store.Delete(ctx, m.Key)
			return httpx.Unprocessable("photo_rejected", "that photo can't be used. Try a different one")
		}
		_ = verdict
		rp.PhotoKey = &m.Key
	}
	if req.RemovePhoto {
		rp.PhotoKey = nil
	}
	rp.UpdatedAt = s.now()
	if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&rp).Error; err != nil {
		return err
	}
	d, _ := s.card(ctx, tx, p.UserID)
	return c.JSON(http.StatusOK, d)
}

func levelIndex(l model.RevealLevel) int { return model.RevealOrder[l] }

func minLevel(a, b model.RevealLevel) model.RevealLevel {
	if levelIndex(a) < levelIndex(b) {
		return a
	}
	return b
}

type RevealDTO struct {
	Mine   model.RevealLevel `json:"mine"`
	Theirs model.RevealLevel `json:"theirs"`
	Mutual model.RevealLevel `json:"mutual"`
	// What the other person shared, up to the mutual level.
	Revealed RevealCardDTO `json:"revealed"`
	// Which levels my own card can support.
	CanOffer []model.RevealLevel `json:"can_offer"`
}

func (s *Service) pair(tx *gorm.DB, convID, viewer uuid.UUID) (*model.Conversation, *model.ConversationMember, *model.ConversationMember, error) {
	conv, me, err := member(tx, convID, viewer)
	if err != nil {
		return nil, nil, nil, err
	}
	if conv.Kind != model.ConvDM && conv.Kind != model.ConvTravel {
		return nil, nil, nil, ErrRevealKind
	}
	var other model.ConversationMember
	if err := tx.Where("conversation_id = ? AND persona_id <> ?", conv.ID, viewer).First(&other).Error; err != nil {
		return nil, nil, nil, ErrRevealKind
	}
	return conv, me, &other, nil
}

func (s *Service) revealDTO(ctx context.Context, tx *gorm.DB, viewerUser uuid.UUID, me, other *model.ConversationMember) RevealDTO {
	mutual := minLevel(me.RevealLevel, other.RevealLevel)
	d := RevealDTO{Mine: me.RevealLevel, Theirs: other.RevealLevel, Mutual: mutual, CanOffer: []model.RevealLevel{}}
	// Someone who left or is blocked shares nothing, whatever the stored levels say.
	if other.LeftAt != nil || me.LeftAt != nil {
		d.Mutual = model.RevealNone
	} else if blocked, _ := safety.IsBlocked(ctx, tx, me.PersonaID, other.PersonaID); blocked {
		d.Mutual = model.RevealNone
	}
	if d.Mutual != model.RevealNone {
		var per model.Persona
		if tx.Unscoped().Select("user_id").First(&per, "id = ?", other.PersonaID).Error == nil {
			full, _ := s.card(ctx, tx, per.UserID)
			n := levelIndex(d.Mutual)
			if n >= 1 {
				d.Revealed.FirstName = full.FirstName
			}
			if n >= 2 {
				d.Revealed.PhotoURL = full.PhotoURL
			}
			if n >= 3 {
				d.Revealed.Contact = full.Contact
			}
		}
	}
	mine, _ := s.card(ctx, tx, viewerUser)
	if mine.FirstName != "" {
		d.CanOffer = append(d.CanOffer, model.RevealFirstName)
		if mine.PhotoURL != nil {
			d.CanOffer = append(d.CanOffer, model.RevealPhoto)
			if mine.Contact != "" {
				d.CanOffer = append(d.CanOffer, model.RevealContact)
			}
		}
	}
	return d
}

func (s *Service) getReveal(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return ErrNotMember
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	tx := s.DB.WithContext(ctx)
	_, me, other, err := s.pair(tx, id, p.PersonaID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, s.revealDTO(ctx, tx, p.UserID, me, other))
}

type stepRequest struct {
	Level model.RevealLevel `json:"level" validate:"required,oneof=none first_name photo contact"`
}

// stepReveal moves my offer one rung up, or down to any lower rung (revoking).
func (s *Service) stepReveal(c echo.Context) error {
	req, err := httpx.Bind[stepRequest](c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return ErrNotMember
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var me, other *model.ConversationMember
	var before model.RevealLevel
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		if _, me, other, err = s.pair(tx, id, p.PersonaID); err != nil {
			return err
		}
		if blocked, _ := safety.IsBlocked(ctx, tx, p.PersonaID, other.PersonaID); blocked {
			return ErrNotMember
		}
		before = minLevel(me.RevealLevel, other.RevealLevel)
		cur, next := levelIndex(me.RevealLevel), levelIndex(req.Level)
		if next == cur {
			return nil
		}
		if next > cur {
			if next != cur+1 {
				return httpx.Unprocessable("one_step", "reveal one step at a time")
			}
			d := s.revealDTO(ctx, tx, p.UserID, me, other)
			ok := false
			for _, l := range d.CanOffer {
				ok = ok || l == req.Level
			}
			if !ok {
				return ErrRevealProfile
			}
		}
		action := model.RevealGrant
		if next < cur {
			action = model.RevealRevoke
		}
		if err := tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND persona_id = ?", id, p.PersonaID).Update("reveal_level", req.Level).Error; err != nil {
			return err
		}
		me.RevealLevel = req.Level
		return tx.Create(&model.RevealEvent{ConversationID: id, PersonaID: p.PersonaID, Level: req.Level, Action: action}).Error
	})
	if err != nil {
		return err
	}
	tx := s.DB.WithContext(ctx)
	after := minLevel(me.RevealLevel, other.RevealLevel)
	refs, _ := identity.LoadRefs(tx, []uuid.UUID{p.PersonaID})
	alias := refs[p.PersonaID].Alias
	var note string
	switch {
	case levelIndex(after) > levelIndex(before):
		note = "You both chose to share " + levelLabel(after) + "."
		s.audit(tx, p.UserID, "reveal.mutual", id, model.JSONMap{"level": string(after)})
	case levelIndex(after) < levelIndex(before):
		note = alias + " stopped sharing " + levelLabel(before) + ". New messages are masked again."
		if after != model.RevealNone {
			note = alias + " stepped back to sharing " + levelLabel(after) + " only."
		}
		s.audit(tx, p.UserID, "reveal.revoke", id, model.JSONMap{"level": string(after)})
	case levelIndex(me.RevealLevel) > levelIndex(other.RevealLevel):
		note = alias + " is ready to share " + levelLabel(me.RevealLevel) + ". Nothing is shown until you both agree."
	}
	if note != "" {
		_ = s.PostSystem(ctx, id, model.MsgSystem, note)
	}
	var mine model.ConversationMember
	tx.Where("conversation_id = ? AND persona_id = ?", id, p.PersonaID).First(&mine)
	return c.JSON(http.StatusOK, s.revealDTO(ctx, tx, p.UserID, &mine, other))
}

func levelLabel(l model.RevealLevel) string {
	switch l {
	case model.RevealFirstName:
		return "first names"
	case model.RevealPhoto:
		return "photos"
	case model.RevealContact:
		return "contact details"
	}
	return "nothing"
}

// RevealMaskOptions lifts phone and handle masking only at a mutual contact-level reveal.
func RevealMaskOptions(tx *gorm.DB, convID uuid.UUID) pii.Options {
	var levels []model.RevealLevel
	tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND left_at IS NULL", convID).Pluck("reveal_level", &levels)
	if len(levels) != 2 || levels[0] != model.RevealContact || levels[1] != model.RevealContact {
		return pii.Options{}
	}
	return pii.Options{Allow: map[model.PIIKind]bool{model.PIIPhone: true, model.PIIHandle: true}}
}

func (s *Service) audit(tx *gorm.DB, userID uuid.UUID, verb string, conv uuid.UUID, meta model.JSONMap) {
	if s.Audit != nil {
		s.Audit(tx, userID, verb, conv, meta)
	}
}
