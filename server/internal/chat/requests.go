package chat

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/ratelimit"
	"metro-buddy/server/internal/safety"
)

const RequestTTL = 7 * 24 * time.Hour

var (
	ErrNoDMs          = httpx.Forbidden("this person isn't accepting message requests")
	ErrAlreadyPending = httpx.Conflict("you already sent a request")
	ErrAlreadyChat    = httpx.Conflict("you already have a chat with this person")
)

func (s *Service) registerRequests(g *echo.Group) {
	// New accounts get 5 requests a day; the limit doubles with each trust tier.
	daily := s.Limiter.Middleware(ratelimit.Rule{Name: "dm_requests", Limit: 5, Window: 24 * time.Hour, ByTier: true})
	g.POST("/dm-requests", s.createRequest, daily)
	g.GET("/dm-requests", s.listRequests)
	g.POST("/dm-requests/:id/accept", s.acceptRequest)
	g.POST("/dm-requests/:id/ignore", s.ignoreRequest)
	g.DELETE("/dm-requests/:id", s.withdrawRequest)
}

type createRequestBody struct {
	ToPersonaID uuid.UUID       `json:"to_persona_id" validate:"required"`
	Intro       string          `json:"intro" validate:"required,min=2,max=200"`
	ContextType model.DMContext `json:"context_type" validate:"required,oneof=post comment room profile episode"`
	ContextID   *uuid.UUID      `json:"context_id"`
}

type RequestDTO struct {
	ID             uuid.UUID           `json:"id"`
	From           identity.PersonaRef `json:"from"`
	To             identity.PersonaRef `json:"to"`
	Intro          string              `json:"intro"`
	ContextType    model.DMContext     `json:"context_type"`
	ContextID      *uuid.UUID          `json:"context_id"`
	Status         model.RequestStatus `json:"status"`
	ConversationID *uuid.UUID          `json:"conversation_id"`
	CreatedAt      time.Time           `json:"created_at"`
	ExpiresAt      time.Time           `json:"expires_at"`
}

func (s *Service) requestDTOs(tx *gorm.DB, rs []model.DMRequest) ([]RequestDTO, error) {
	var ids []uuid.UUID
	for _, r := range rs {
		ids = append(ids, r.FromPersonaID, r.ToPersonaID)
	}
	refs, err := identity.LoadRefs(tx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]RequestDTO, len(rs))
	for i, r := range rs {
		out[i] = RequestDTO{ID: r.ID, From: refs[r.FromPersonaID], To: refs[r.ToPersonaID], Intro: r.IntroMasked, ContextType: r.ContextType,
			ContextID: r.ContextID, Status: r.Status, ConversationID: r.ConversationID, CreatedAt: r.CreatedAt, ExpiresAt: r.CreatedAt.Add(RequestTTL)}
	}
	return out, nil
}

// existingDM finds a live 1:1 chat between two personas.
func existingDM(tx *gorm.DB, a, b uuid.UUID) (*uuid.UUID, error) {
	var ids []uuid.UUID
	err := tx.Table("conversations").Select("conversations.id").
		Joins("JOIN conversation_members ma ON ma.conversation_id = conversations.id AND ma.persona_id = ? AND ma.left_at IS NULL", a).
		Joins("JOIN conversation_members mb ON mb.conversation_id = conversations.id AND mb.persona_id = ? AND mb.left_at IS NULL", b).
		Where("conversations.kind = ? AND conversations.deleted_at IS NULL", model.ConvDM).Limit(1).Pluck("conversations.id", &ids).Error
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	return &ids[0], nil
}

func (s *Service) createRequest(c echo.Context) error {
	req, err := httpx.Bind[createRequestBody](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	if req.ToPersonaID == p.PersonaID {
		return httpx.BadRequest("that's you")
	}
	ctx := c.Request().Context()
	var out *model.DMRequest
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var to model.Persona
		if err := tx.First(&to, "id = ?", req.ToPersonaID).Error; err != nil {
			return httpx.NotFound()
		}
		if b, err := safety.IsBlocked(ctx, tx, p.PersonaID, to.ID); err != nil || b {
			// Same answer as "doesn't exist" so a blocked person learns nothing.
			return httpx.NotFound()
		}
		var settings model.UserSettings
		if tx.Where("user_id = ?", to.UserID).Limit(1).Find(&settings).RowsAffected == 1 && settings.DMFrom == model.DMNobody {
			return ErrNoDMs
		}
		if id, err := existingDM(tx, p.PersonaID, to.ID); err != nil {
			return err
		} else if id != nil {
			return ErrAlreadyChat
		}
		var pending int64
		if err := tx.Model(&model.DMRequest{}).Where("status = ? AND ((from_persona_id = ? AND to_persona_id = ?) OR (from_persona_id = ? AND to_persona_id = ?))",
			model.RequestPending, p.PersonaID, to.ID, to.ID, p.PersonaID).Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return ErrAlreadyPending
		}
		r := model.DMRequest{FromPersonaID: p.PersonaID, ToPersonaID: to.ID, ContextType: req.ContextType, ContextID: req.ContextID, Status: model.RequestPending}
		r.ID = model.NewID()
		res, err := s.PII.Process(tx, model.CTConversation, r.ID, req.Intro, pii.Options{})
		if err != nil {
			return err
		}
		r.IntroMasked = res.Text
		out = &r
		return tx.Create(&r).Error
	})
	if err != nil {
		return err
	}
	dtos, err := s.requestDTOs(s.DB.WithContext(ctx), []model.DMRequest{*out})
	if err != nil {
		return err
	}
	s.Hub.Publish(ctx, []uuid.UUID{out.ToPersonaID}, "dm_request.new", dtos[0])
	if s.OnRequest != nil {
		s.OnRequest(ctx, out.ToPersonaID, out.ID)
	}
	return c.JSON(http.StatusCreated, dtos[0])
}

func (s *Service) listRequests(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	q := tx.Model(&model.DMRequest{}).Where("status = ?", model.RequestPending).Order("created_at DESC").Limit(100)
	if c.QueryParam("box") == "outgoing" {
		q = q.Where("from_persona_id = ?", p.PersonaID)
	} else {
		q = q.Where("to_persona_id = ?", p.PersonaID).Scopes(safety.ExcludeBlocked(p.PersonaID, "from_persona_id"))
	}
	var rows []model.DMRequest
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	out, err := s.requestDTOs(tx, rows)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (s *Service) loadIncoming(tx *gorm.DB, idParam string, to uuid.UUID) (*model.DMRequest, error) {
	id, err := uuid.Parse(idParam)
	if err != nil {
		return nil, httpx.NotFound()
	}
	var r model.DMRequest
	if err := tx.First(&r, "id = ? AND to_persona_id = ?", id, to).Error; err != nil {
		return nil, err
	}
	if r.Status != model.RequestPending {
		return nil, httpx.Conflict("this request is no longer pending")
	}
	return &r, nil
}

// acceptRequest is the consent moment: only now does a DM conversation exist.
func (s *Service) acceptRequest(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var r *model.DMRequest
	var conv model.Conversation
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		if r, err = s.loadIncoming(tx, c.Param("id"), p.PersonaID); err != nil {
			return err
		}
		if b, err := safety.IsBlocked(ctx, tx, r.FromPersonaID, r.ToPersonaID); err != nil || b {
			return httpx.NotFound()
		}
		now := s.now()
		conv = model.Conversation{Kind: model.ConvDM, MemberCap: 2}
		if err := Create(tx, now, &conv, map[uuid.UUID]model.ConvRole{r.FromPersonaID: model.ConvMember, r.ToPersonaID: model.ConvMember}); err != nil {
			return err
		}
		first := model.Message{ID: model.NewID(), ConversationID: conv.ID, SenderPersonaID: &r.FromPersonaID, Kind: model.MsgText,
			BodyMasked: r.IntroMasked, Status: model.ContentVisible, CreatedAt: now}
		if err := tx.Create(&first).Error; err != nil {
			return err
		}
		return tx.Model(r).Updates(map[string]any{"status": model.RequestAccepted, "conversation_id": conv.ID}).Error
	})
	if err != nil {
		return err
	}
	s.Hub.Publish(ctx, []uuid.UUID{r.FromPersonaID, r.ToPersonaID}, "conversation.new", map[string]any{"id": conv.ID, "kind": conv.Kind})
	if s.OnAccepted != nil {
		s.OnAccepted(ctx, r.FromPersonaID, conv.ID)
	}
	c.SetParamNames("id")
	c.SetParamValues(conv.ID.String())
	return s.get(c)
}

func (s *Service) ignoreRequest(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	err := s.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		r, err := s.loadIncoming(tx, c.Param("id"), p.PersonaID)
		if err != nil {
			return err
		}
		return tx.Model(r).Update("status", model.RequestIgnored).Error
	})
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Service) withdrawRequest(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	p := httpx.PrincipalFrom(c)
	res := s.DB.WithContext(c.Request().Context()).Where("id = ? AND from_persona_id = ? AND status = ?", id, p.PersonaID, model.RequestPending).Delete(&model.DMRequest{})
	if res.Error != nil {
		return res.Error
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Service) ExpireRequests(ctx context.Context) error {
	return s.DB.WithContext(ctx).Model(&model.DMRequest{}).Where("status = ? AND created_at < ?", model.RequestPending, s.now().Add(-RequestTTL)).
		Update("status", model.RequestExpired).Error
}
