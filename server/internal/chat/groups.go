package chat

import (
	"crypto/rand"
	"errors"
	"math/big"
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

const GroupCap = 50

var (
	ErrGroupFull = httpx.Conflict("this group is full")
	ErrNotAdmin  = httpx.Forbidden("only group admins can do that")
	ErrBadInvite = httpx.NotFound()
)

func (s *Service) registerGroups(g *echo.Group) {
	g.POST("/groups", s.createGroup, s.Limiter.Middleware(ratelimit.Rule{Name: "groups", Limit: 5, Window: 24 * time.Hour, ByTier: true}))
	g.POST("/groups/join", s.joinGroup, s.Limiter.Middleware(ratelimit.Rule{Name: "group_join", Limit: 20, Window: time.Hour}))
	g.POST("/conversations/:id/invite", s.rotateInvite)
	g.PATCH("/conversations/:id/members/:persona_id", s.setRole)
	g.DELETE("/conversations/:id/members/:persona_id", s.removeMember)
}

// Invite codes avoid look-alike characters so they survive being read out loud on a platform.
const inviteAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func newInvite() string {
	b := make([]byte, 8)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(inviteAlphabet))))
		b[i] = inviteAlphabet[n.Int64()]
	}
	return string(b)
}

type createGroupRequest struct {
	Title string `json:"title" validate:"required,min=2,max=60"`
}

func (s *Service) createGroup(c echo.Context) error {
	req, err := httpx.Bind[createGroupRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var conv model.Conversation
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		code := newInvite()
		title := pii.Mask(req.Title, pii.Options{}).Text
		conv = model.Conversation{Kind: model.ConvGroup, Title: &title, MemberCap: GroupCap, InviteCode: &code, CreatedByPersonaID: &p.PersonaID}
		return Create(tx, s.now(), &conv, map[uuid.UUID]model.ConvRole{p.PersonaID: model.ConvOwner})
	})
	if err != nil {
		return err
	}
	c.SetParamNames("id")
	c.SetParamValues(conv.ID.String())
	return s.get(c)
}

type joinRequest struct {
	Code string `json:"code" validate:"required,len=8,alphanum"`
}

func (s *Service) joinGroup(c echo.Context) error {
	req, err := httpx.Bind[joinRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var conv model.Conversation
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("invite_code = ? AND kind = ?", req.Code, model.ConvGroup).First(&conv).Error; err != nil {
			return ErrBadInvite
		}
		members, err := memberIDs(tx, conv.ID)
		if err != nil {
			return err
		}
		for _, m := range members {
			if m == p.PersonaID {
				return nil
			}
		}
		if len(members) >= GroupCap {
			return ErrGroupFull
		}
		var prev model.ConversationMember
		if tx.Where("conversation_id = ? AND persona_id = ?", conv.ID, p.PersonaID).Limit(1).Find(&prev).RowsAffected == 1 {
			return tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND persona_id = ?", conv.ID, p.PersonaID).
				Updates(map[string]any{"left_at": nil, "joined_at": s.now(), "role": model.ConvMember}).Error
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.ConversationMember{ConversationID: conv.ID, PersonaID: p.PersonaID, Role: model.ConvMember, JoinedAt: s.now()}).Error
	})
	if err != nil {
		return err
	}
	others, _ := memberIDs(s.DB.WithContext(ctx), conv.ID)
	s.Hub.Publish(ctx, others, "conversation.member_joined", map[string]any{"conversation_id": conv.ID, "persona_id": p.PersonaID})
	c.SetParamNames("id")
	c.SetParamValues(conv.ID.String())
	return s.get(c)
}

func (s *Service) requireAdmin(tx *gorm.DB, convIDParam string, persona uuid.UUID) (*model.Conversation, *model.ConversationMember, error) {
	id, err := uuid.Parse(convIDParam)
	if err != nil {
		return nil, nil, ErrNotMember
	}
	conv, me, err := member(tx, id, persona)
	if err != nil {
		return nil, nil, err
	}
	if conv.Kind != model.ConvGroup && conv.Kind != model.ConvCrew && conv.Kind != model.ConvDiscussion {
		return nil, nil, httpx.BadRequest("not a group")
	}
	if me.Role == model.ConvMember {
		return nil, nil, ErrNotAdmin
	}
	return conv, me, nil
}

func (s *Service) rotateInvite(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	err := s.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		conv, _, err := s.requireAdmin(tx, c.Param("id"), p.PersonaID)
		if err != nil {
			return err
		}
		return tx.Model(conv).Update("invite_code", newInvite()).Error
	})
	if err != nil {
		return err
	}
	return s.get(c)
}

type roleRequest struct {
	Role model.ConvRole `json:"role" validate:"required,oneof=member admin"`
}

func (s *Service) setRole(c echo.Context) error {
	req, err := httpx.Bind[roleRequest](c)
	if err != nil {
		return err
	}
	target, err := uuid.Parse(c.Param("persona_id"))
	if err != nil {
		return httpx.NotFound()
	}
	p := httpx.PrincipalFrom(c)
	err = s.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		conv, me, err := s.requireAdmin(tx, c.Param("id"), p.PersonaID)
		if err != nil {
			return err
		}
		if me.Role != model.ConvOwner {
			return httpx.Forbidden("only the owner can change roles")
		}
		res := tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND persona_id = ? AND left_at IS NULL AND role <> ?", conv.ID, target, model.ConvOwner).
			Update("role", req.Role)
		if res.Error == nil && res.RowsAffected == 0 {
			return httpx.NotFound()
		}
		return res.Error
	})
	if err != nil {
		return err
	}
	return s.get(c)
}

// removeMember: admins remove members; the owner can remove admins; nobody removes the owner.
func (s *Service) removeMember(c echo.Context) error {
	target, err := uuid.Parse(c.Param("persona_id"))
	if err != nil {
		return httpx.NotFound()
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var convID uuid.UUID
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		conv, me, err := s.requireAdmin(tx, c.Param("id"), p.PersonaID)
		if err != nil {
			return err
		}
		convID = conv.ID
		var t model.ConversationMember
		if err := tx.Where("conversation_id = ? AND persona_id = ? AND left_at IS NULL", conv.ID, target).First(&t).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return httpx.NotFound()
			}
			return err
		}
		if t.Role == model.ConvOwner || (t.Role == model.ConvAdmin && me.Role != model.ConvOwner) {
			return httpx.Forbidden("you can't remove this member")
		}
		return tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND persona_id = ?", conv.ID, target).Update("left_at", s.now()).Error
	})
	if err != nil {
		return err
	}
	others, _ := memberIDs(s.DB.WithContext(ctx), convID)
	s.Hub.Publish(ctx, append(others, target), "conversation.member_removed", map[string]any{"conversation_id": convID, "persona_id": target})
	return c.NoContent(http.StatusNoContent)
}
