package auth

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
)

var (
	ErrPersonaRequired = httpx.E(http.StatusForbidden, "persona_required", "create your persona first")
	ErrSuspended       = httpx.E(http.StatusForbidden, "account_suspended", "your account is suspended")
)

type principalRow struct {
	ID             uuid.UUID
	Status         model.UserStatus
	TrustTier      int
	SuspendedUntil *time.Time
	PersonaID      *uuid.UUID
}

func (t *Tokens) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			h := c.Request().Header.Get(echo.HeaderAuthorization)
			raw, ok := strings.CutPrefix(h, "Bearer ")
			if !ok || raw == "" {
				return httpx.Unauthorized()
			}
			ac, err := t.Parse(raw)
			if err != nil {
				return httpx.Unauthorized()
			}
			ctx := c.Request().Context()
			// Checked per request so logout and refresh-reuse revocation cut access immediately, not after 15 minutes.
			if active, err := t.FamilyActive(ctx, ac.Family); err != nil {
				return err
			} else if !active {
				return httpx.Unauthorized()
			}
			var row principalRow
			err = t.DB.WithContext(ctx).Table("users").
				Select("users.id, users.status, users.trust_tier, users.suspended_until, personas.id AS persona_id").
				Joins("LEFT JOIN personas ON personas.user_id = users.id AND personas.deleted_at IS NULL").
				Where("users.id = ? AND users.deleted_at IS NULL", ac.UserID).Take(&row).Error
			if err != nil {
				return httpx.Unauthorized()
			}
			switch row.Status {
			case model.UserBanned, model.UserDeleted:
				return ErrAccountBlocked
			case model.UserSuspended:
				if row.SuspendedUntil == nil || t.now().Before(*row.SuspendedUntil) {
					if c.Request().Method != http.MethodGet {
						return ErrSuspended
					}
				}
			}
			p := &httpx.Principal{UserID: row.ID, TrustTier: row.TrustTier}
			if row.PersonaID != nil {
				p.PersonaID = *row.PersonaID
			}
			httpx.SetPrincipal(c, p)
			c.Set(familyKey, ac.Family)
			return next(c)
		}
	}
}

const familyKey = "session_family"

func RequirePersona(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if p := httpx.PrincipalFrom(c); p == nil || p.PersonaID == uuid.Nil {
			return ErrPersonaRequired
		}
		return next(c)
	}
}
