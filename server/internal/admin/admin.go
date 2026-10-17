// Package admin is the moderator console API under /admin/api; the console SPA is served at /admin.
package admin

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"

	"metro-buddy/server/internal/audit"
	"metro-buddy/server/internal/cache"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/ratelimit"
	"metro-buddy/server/internal/secure"
)

//go:embed all:dist
var distFS embed.FS

var totpOpts = totp.ValidateOpts{Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1}

type Handlers struct {
	DB           *gorm.DB
	Box          *secure.Box
	Audit        *audit.Recorder
	Limiter      *ratelimit.Limiter
	PII          *pii.Pipeline
	Secret       []byte
	SecureCookie bool
	Now          func() time.Time
	// Hooks the moderation actions need from other domains.
	RevokeSessions func(tx *gorm.DB, userID uuid.UUID) error
	NotifyPersona  func(persona uuid.UUID, kind model.NotificationKind, payload model.JSONMap)
	OnHostApproved func(persona uuid.UUID)
	EndEpisode     func(ctx context.Context, id uuid.UUID) error
	MediaStats     func() cache.Stats
	Triage         func(caseID uuid.UUID)
	// OnCaseClosed lets domains refresh what clients see (e.g. a held image was released).
	OnCaseClosed func(caseID uuid.UUID)
	OnOverdue    func(n int)
}

func (h *Handlers) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now().UTC()
}

func (h *Handlers) Register(e *echo.Echo) {
	api := e.Group("/admin/api", noStore)
	api.POST("/login", h.login, h.Limiter.Middleware(ratelimit.Rule{Name: "admin_login", Limit: 10, Window: 15 * time.Minute, Key: ratelimit.ByIP, FailClosed: true}))
	api.POST("/logout", h.logout)

	authed := api.Group("", h.authenticate)
	authed.GET("/me", h.me)

	lead := h.RequireRole(model.AdminTSLead)
	boss := h.RequireRole(model.AdminAdmin)

	authed.GET("/admins", h.listAdmins, boss)
	authed.POST("/admins", h.createAdmin, boss)
	authed.PATCH("/admins/:id", h.updateAdmin, boss)

	authed.GET("/bowls", h.listBowls)
	authed.POST("/bowls", h.createBowl, boss)
	authed.PATCH("/bowls/:id", h.updateBowl, boss)
	authed.DELETE("/bowls/:id", h.deleteBowl, boss)

	authed.POST("/vault/read", h.readVault, lead)

	h.registerModeration(authed, lead)
	h.registerGrievance(authed, lead)
	h.registerStudio(authed)
	h.registerMetrics(authed)

	h.spa(e)
}

func noStore(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		c.Response().Header().Set("X-Robots-Tag", "noindex")
		return next(c)
	}
}

// spa serves the embedded console; unknown paths under /admin fall back to index.html.
func (h *Handlers) spa(e *echo.Echo) {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		return
	}
	files := http.FileServer(http.FS(dist))
	handler := func(c echo.Context) error {
		p := strings.TrimPrefix(c.Request().URL.Path, "/admin")
		h := c.Response().Header()
		h.Set("X-Robots-Tag", "noindex")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob: https:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		if p == "" || p == "/" || !strings.Contains(p, ".") {
			idx, err := fs.ReadFile(dist, "index.html")
			if err != nil {
				return c.String(http.StatusNotFound, "admin console not built")
			}
			h.Set("Cache-Control", "no-cache")
			return c.HTMLBlob(http.StatusOK, idx)
		}
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
		r := c.Request().Clone(c.Request().Context())
		r.URL.Path = p
		files.ServeHTTP(c.Response(), r)
		return nil
	}
	e.GET("/admin", handler)
	e.GET("/admin/*", handler)
}

type AdminDTO struct {
	ID            uuid.UUID       `json:"id"`
	Email         string          `json:"email"`
	Role          model.AdminRole `json:"role"`
	Active        bool            `json:"active"`
	TOTPConfirmed bool            `json:"totp_confirmed"`
	CreatedAt     time.Time       `json:"created_at"`
}

func meDTO(a model.AdminUser) AdminDTO {
	return AdminDTO{ID: a.ID, Email: a.Email, Role: a.Role, Active: a.Active, TOTPConfirmed: a.TOTPConfirmed, CreatedAt: a.CreatedAt}
}

func (h *Handlers) me(c echo.Context) error {
	var a model.AdminUser
	if err := h.DB.WithContext(c.Request().Context()).First(&a, "id = ?", PrincipalFrom(c).ID).Error; err != nil {
		return err
	}
	return c.JSON(http.StatusOK, meDTO(a))
}

func (h *Handlers) listAdmins(c echo.Context) error {
	var rows []model.AdminUser
	if err := h.DB.WithContext(c.Request().Context()).Order("created_at").Find(&rows).Error; err != nil {
		return err
	}
	out := make([]AdminDTO, len(rows))
	for i, r := range rows {
		out[i] = meDTO(r)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

type createAdminRequest struct {
	Email string          `json:"email" validate:"required,email,max=120"`
	Role  model.AdminRole `json:"role" validate:"required,oneof=moderator trust_safety_lead admin"`
}

// NewAdmin creates an account with a random password and TOTP secret, returned once for handover.
func NewAdmin(db *gorm.DB, box *secure.Box, email string, role model.AdminRole) (*model.AdminUser, string, string, error) {
	pw := secure.Token(18)
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Metro Buddy Admin", AccountName: email})
	if err != nil {
		return nil, "", "", err
	}
	a := model.AdminUser{
		Email: strings.ToLower(email), Role: role, PasswordHash: HashPassword(pw),
		TOTPSecretEnc: box.SealString(key.Secret()), TOTPConfirmed: true, Active: true,
	}
	return &a, pw, key.URL(), db.Create(&a).Error
}

func (h *Handlers) createAdmin(c echo.Context) error {
	req, err := httpx.Bind[createAdminRequest](c)
	if err != nil {
		return err
	}
	a, pw, url, err := NewAdmin(h.DB.WithContext(c.Request().Context()), h.Box, req.Email, req.Role)
	if err != nil {
		return err
	}
	actor := PrincipalFrom(c).ID
	h.Audit.Log(h.DB.WithContext(c.Request().Context()), model.ActorAdmin, &actor, "admin.create", "admin_user", &a.ID, model.JSONMap{"role": string(a.Role)})
	return c.JSON(http.StatusCreated, map[string]any{"admin": meDTO(*a), "temp_password": pw, "totp_url": url})
}

type updateAdminRequest struct {
	Role   *model.AdminRole `json:"role" validate:"omitempty,oneof=moderator trust_safety_lead admin"`
	Active *bool            `json:"active"`
}

func (h *Handlers) updateAdmin(c echo.Context) error {
	req, err := httpx.Bind[updateAdminRequest](c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	actor := PrincipalFrom(c).ID
	if id == actor {
		return httpx.BadRequest("you can't change your own role or status")
	}
	updates := map[string]any{}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}
	tx := h.DB.WithContext(c.Request().Context())
	if err := tx.Model(&model.AdminUser{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	h.Audit.Log(tx, model.ActorAdmin, &actor, "admin.update", "admin_user", &id, model.JSONMap(updates))
	return h.listAdmins(c)
}

type vaultRequest struct {
	ContentType model.ContentType `json:"content_type" validate:"required"`
	ContentID   uuid.UUID         `json:"content_id" validate:"required"`
	Reason      string            `json:"reason" validate:"required,min=10,max=300"`
}

func (h *Handlers) readVault(c echo.Context) error {
	req, err := httpx.Bind[vaultRequest](c)
	if err != nil {
		return err
	}
	out, err := h.PII.ReadVault(c.Request().Context(), PrincipalFrom(c).ID, c.RealIP(), req.ContentType, req.ContentID, req.Reason)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}
