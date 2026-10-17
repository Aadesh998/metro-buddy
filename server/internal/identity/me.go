package identity

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
)

type Handlers struct {
	DB *gorm.DB
}

func (h *Handlers) Register(authed *echo.Group) {
	authed.GET("/me", h.me)
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
	return c.JSON(http.StatusOK, meResponse{Status: u.Status, HasPersona: p.PersonaID != [16]byte{}, Locale: u.Locale})
}
