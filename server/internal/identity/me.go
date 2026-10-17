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
	return time.Now()
}

func (h *Handlers) Register(authed *echo.Group, persona echo.MiddlewareFunc) {
	authed.GET("/me", h.me)
	authed.POST("/me/persona/suggestions", h.suggest)
	authed.POST("/me/persona", h.createPersona)
	authed.GET("/me/persona", h.getPersona, persona)
	authed.PATCH("/me/persona", h.patchPersona, persona)
	authed.PUT("/me/persona/alias", h.changeAlias, persona)
	authed.GET("/personas/:id", h.getPublicPersona, persona)
}

type meResponse struct {
	Status     model.UserStatus `json:"status"`
	HasPersona bool             `json:"has_persona"`
	Locale     string           `json:"locale"`
}

func (h *Handlers) me(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	var u model.User
	if err := h.DB.WithContext(c.Request().Context()).First(&u, "id = ?", p.UserID).Error; err != nil {
		return err
	}
	return c.JSON(http.StatusOK, meResponse{Status: u.Status, HasPersona: p.PersonaID != uuid.Nil, Locale: u.Locale})
}
