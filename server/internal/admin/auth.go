package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/secure"
)

const (
	cookieName     = "mb_admin"
	sessionTTL     = 8 * time.Hour
	maxFailed      = 5
	lockFor        = 15 * time.Minute
	adminAudience  = "metro-buddy-admin"
	argonTime      = 3
	argonMemoryKiB = 64 * 1024
	argonThreads   = 2
)

var ErrLogin = httpx.E(http.StatusUnauthorized, "login_failed", "wrong email, password or code")

func HashPassword(pw string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemoryKiB, argonThreads, 32)
	return fmt.Sprintf("argon2id$%d$%d$%d$%s$%s", argonTime, argonMemoryKiB, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

func checkPassword(encoded, pw string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "argon2id" {
		return false
	}
	var t, m uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[1]+" "+parts[2]+" "+parts[3], "%d %d %d", &t, &m, &p); err != nil {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[4])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// Burns comparable time for unknown emails so response timing doesn't reveal which admins exist.
var dummyHash = HashPassword("not-a-real-password")

type Principal struct {
	ID    uuid.UUID
	Email string
	Role  model.AdminRole
}

const principalKey = "admin_principal"

func PrincipalFrom(c echo.Context) *Principal {
	p, _ := c.Get(principalKey).(*Principal)
	return p
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email,max=120"`
	Password string `json:"password" validate:"required,max=200"`
	Code     string `json:"code" validate:"required,len=6,numeric"`
}

func (h *Handlers) login(c echo.Context) error {
	req, err := httpx.Bind[loginRequest](c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	now := h.now()
	var a model.AdminUser
	err = h.DB.WithContext(ctx).Where("email = ?", strings.ToLower(req.Email)).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		checkPassword(dummyHash, req.Password)
		h.Audit.Security(ctx, model.SecAdminLoginFail, nil, c.RealIP(), model.JSONMap{"reason": "unknown"})
		return ErrLogin
	}
	if err != nil {
		return err
	}
	if !a.Active {
		return ErrLogin
	}
	if a.LockedUntil != nil && now.Before(*a.LockedUntil) {
		return httpx.E(http.StatusTooManyRequests, "locked", "too many attempts, try again later")
	}
	ok := checkPassword(a.PasswordHash, req.Password) && a.TOTPConfirmed && h.validTOTP(&a, req.Code, now)
	if !ok {
		fails := a.FailedLogins + 1
		updates := map[string]any{"failed_logins": fails}
		if fails >= maxFailed {
			updates["locked_until"] = now.Add(lockFor)
			updates["failed_logins"] = 0
		}
		h.DB.WithContext(ctx).Model(&a).Updates(updates)
		h.Audit.Security(ctx, model.SecAdminLoginFail, nil, c.RealIP(), model.JSONMap{"admin_id": a.ID.String()})
		return ErrLogin
	}
	h.DB.WithContext(ctx).Model(&a).Updates(map[string]any{"failed_logins": 0, "locked_until": nil})
	tok, err := h.sign(a, now)
	if err != nil {
		return err
	}
	c.SetCookie(&http.Cookie{
		Name: cookieName, Value: tok, Path: "/admin", HttpOnly: true, Secure: h.SecureCookie,
		SameSite: http.SameSiteStrictMode, Expires: now.Add(sessionTTL),
	})
	h.Audit.Log(h.DB.WithContext(ctx), model.ActorAdmin, &a.ID, "admin.login", "admin_user", &a.ID, nil)
	return c.JSON(http.StatusOK, meDTO(a))
}

func (h *Handlers) validTOTP(a *model.AdminUser, code string, now time.Time) bool {
	secret, err := h.Box.OpenString(a.TOTPSecretEnc)
	if err != nil {
		return false
	}
	ok, _ := totp.ValidateCustom(code, secret, now, totpOpts)
	return ok
}

func (h *Handlers) logout(c echo.Context) error {
	c.SetCookie(&http.Cookie{Name: cookieName, Value: "", Path: "/admin", HttpOnly: true, Secure: h.SecureCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	return c.NoContent(http.StatusNoContent)
}

type adminClaims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

func (h *Handlers) key() []byte { return secure.HMAC(h.Secret, "admin-session") }

func (h *Handlers) sign(a model.AdminUser, now time.Time) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, adminClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: a.ID.String(), Audience: jwt.ClaimStrings{adminAudience},
			IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(sessionTTL)),
		},
		Role: string(a.Role),
	}).SignedString(h.key())
}

// authenticate re-reads the admin row each request so deactivation and role changes apply at once.
func (h *Handlers) authenticate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ck, err := c.Cookie(cookieName)
		if err != nil {
			return httpx.Unauthorized()
		}
		var cl adminClaims
		_, err = jwt.ParseWithClaims(ck.Value, &cl, func(*jwt.Token) (any, error) { return h.key(), nil },
			jwt.WithValidMethods([]string{"HS256"}), jwt.WithAudience(adminAudience), jwt.WithExpirationRequired(),
			jwt.WithTimeFunc(h.now))
		if err != nil {
			return httpx.Unauthorized()
		}
		id, err := uuid.Parse(cl.Subject)
		if err != nil {
			return httpx.Unauthorized()
		}
		var a model.AdminUser
		if err := h.DB.WithContext(c.Request().Context()).First(&a, "id = ? AND active = ?", id, true).Error; err != nil {
			return httpx.Unauthorized()
		}
		// SameSite=Strict blocks cross-site cookies; requiring JSON on writes blocks simple-form CSRF too.
		if c.Request().Method != http.MethodGet && !strings.HasPrefix(c.Request().Header.Get(echo.HeaderContentType), echo.MIMEApplicationJSON) {
			return httpx.E(http.StatusUnsupportedMediaType, "json_required", "send JSON")
		}
		c.Set(principalKey, &Principal{ID: a.ID, Email: a.Email, Role: a.Role})
		return next(c)
	}
}

// RequireRole allows the given role and anything above it.
func (h *Handlers) RequireRole(min model.AdminRole) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			p := PrincipalFrom(c)
			if p == nil || model.AdminRoleRank[p.Role] < model.AdminRoleRank[min] {
				h.Audit.Security(c.Request().Context(), model.SecPermissionDenied, nil, c.RealIP(),
					model.JSONMap{"route": c.Path(), "role": string(p.Role)})
				return httpx.Forbidden("your role can't do that")
			}
			return next(c)
		}
	}
}
