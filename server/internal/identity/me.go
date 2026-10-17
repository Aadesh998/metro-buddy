package identity

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
)

type Handlers struct {
	DB  *gorm.DB
	PII *pii.Pipeline
	// Visible reports whether viewer may see target; wired to the block check by the safety domain.
	Visible func(ctx context.Context, viewer, target uuid.UUID) (bool, error)
	Now     func() time.Time
}

func (h *Handlers) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now().UTC()
}

func (h *Handlers) Register(authed *echo.Group, persona echo.MiddlewareFunc) {
	authed.GET("/me", h.me)
	authed.PATCH("/me", h.patchMe)
	authed.PUT("/me/age", h.setAge)
	authed.GET("/me/settings", h.getSettings)
	authed.PATCH("/me/settings", h.patchSettings)
	authed.GET("/me/consents", h.getConsents)
	authed.POST("/me/consents", h.setConsents)
	authed.POST("/me/persona/suggestions", h.suggest, h.requireOnboarded)
	authed.POST("/me/persona", h.createPersona, h.requireOnboarded)
	authed.GET("/me/persona", h.getPersona, persona)
	authed.PATCH("/me/persona", h.patchPersona, persona)
	authed.PUT("/me/persona/alias", h.changeAlias, persona)
	authed.GET("/personas/:id", h.getPublicPersona, persona)
	authed.GET("/vibes", h.vibeCatalog)
	authed.GET("/me/vibes", h.myVibes, persona)
	authed.PUT("/me/vibes", h.setVibes, persona)
	authed.PUT("/me/intents", h.setIntents, persona)
}

type meResponse struct {
	Status              model.UserStatus `json:"status"`
	HasPersona          bool             `json:"has_persona"`
	Locale              string           `json:"locale"`
	Gender              *model.Gender    `json:"gender"`
	AgeVerified         bool             `json:"age_verified"`
	Onboarded           bool             `json:"onboarded"`
	CoreConsent         bool             `json:"core_consent"`
	Verified            bool             `json:"verified"`
	PolicyVersion       string           `json:"policy_version"`
	DeletionScheduledAt *time.Time       `json:"deletion_scheduled_at"`
}

const DeletionGrace = 30 * 24 * time.Hour

func (h *Handlers) me(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	var u model.User
	if err := h.DB.WithContext(c.Request().Context()).First(&u, "id = ?", p.UserID).Error; err != nil {
		return err
	}
	core, err := HasConsent(h.DB.WithContext(c.Request().Context()), u.ID.String(), model.ConsentCore)
	if err != nil {
		return err
	}
	var del *time.Time
	if u.DeletionRequestedAt != nil {
		t := u.DeletionRequestedAt.Add(DeletionGrace)
		del = &t
	}
	return c.JSON(http.StatusOK, meResponse{
		Status: u.Status, HasPersona: p.PersonaID != uuid.Nil, Locale: u.Locale, Gender: u.Gender,
		AgeVerified: u.BirthYear != nil, Onboarded: u.OnboardedAt != nil, CoreConsent: core,
		Verified: u.VerifiedAt != nil, PolicyVersion: PolicyVersion, DeletionScheduledAt: del,
	})
}
