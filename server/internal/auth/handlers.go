package auth

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/ratelimit"
)

type Handlers struct {
	DB      *gorm.DB
	OTP     *OTP
	Tokens  *Tokens
	Limiter *ratelimit.Limiter
	// Google verifies Google ID tokens; nil when no OAuth client is configured.
	Google GoogleVerifier
}

func (h *Handlers) Register(g *echo.Group) {
	ip := func(name string, n int, w time.Duration) echo.MiddlewareFunc {
		return h.Limiter.Middleware(ratelimit.Rule{Name: name, Limit: n, Window: w, Key: ratelimit.ByIP, FailClosed: true})
	}
	g.POST("/auth/otp/request", h.requestOTP)
	g.POST("/auth/otp/verify", h.verifyOTP, ip("otp_verify", 20, 10*time.Minute))
	g.POST("/auth/email/signup", h.emailSignup, ip("email_signup", 10, time.Hour))
	g.POST("/auth/email/login", h.emailLogin, ip("email_login", 30, 15*time.Minute))
	g.POST("/auth/google", h.google, ip("google_login", 30, 15*time.Minute))
	g.POST("/auth/refresh", h.refresh, ip("refresh", 30, time.Minute))
	g.POST("/auth/logout", h.logout, ip("logout", 30, time.Minute))
}

type otpRequest struct {
	Phone string `json:"phone" validate:"required,max=20"`
}

func (h *Handlers) requestOTP(c echo.Context) error {
	req, err := httpx.Bind[otpRequest](c)
	if err != nil {
		return err
	}
	if err := h.OTP.Request(c.Request().Context(), req.Phone, c.RealIP()); err != nil {
		return err
	}
	return c.JSON(http.StatusAccepted, map[string]int{"expires_in": int(OTPTTL.Seconds()), "code_length": 6})
}

type deviceInfo struct {
	Platform   model.Platform `json:"platform" validate:"required,oneof=ios android"`
	AppVersion string         `json:"app_version" validate:"max=16"`
}

type verifyRequest struct {
	Phone  string     `json:"phone" validate:"required,max=20"`
	Code   string     `json:"code" validate:"required,len=6,numeric"`
	Device deviceInfo `json:"device" validate:"required"`
}

type verifyResponse struct {
	Session   *Session `json:"session"`
	IsNewUser bool     `json:"is_new_user"`
}

func (h *Handlers) verifyOTP(c echo.Context) error {
	req, err := httpx.Bind[verifyRequest](c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	u, isNew, err := h.OTP.Verify(ctx, req.Phone, req.Code, c.RealIP())
	if err != nil {
		return err
	}
	now := time.Now()
	dev := model.Device{UserID: u.ID, Platform: req.Device.Platform, AppVersion: req.Device.AppVersion, LastSeenAt: now}
	if err := h.DB.WithContext(ctx).Create(&dev).Error; err != nil {
		return err
	}
	h.DB.WithContext(ctx).Model(&model.User{}).Where("id = ?", u.ID).Update("last_active_at", now)
	s, err := h.Tokens.Issue(ctx, u.ID, &dev.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, verifyResponse{Session: s, IsNewUser: isNew})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,max=128"`
}

func (h *Handlers) refresh(c echo.Context) error {
	req, err := httpx.Bind[refreshRequest](c)
	if err != nil {
		return err
	}
	s, err := h.Tokens.Rotate(c.Request().Context(), req.RefreshToken, c.RealIP())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, s)
}

func (h *Handlers) logout(c echo.Context) error {
	req, err := httpx.Bind[refreshRequest](c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	var rt model.RefreshToken
	if err := h.DB.WithContext(ctx).Select("family_id").Where("token_hash = ?", hashToken(req.RefreshToken)).First(&rt).Error; err == nil {
		if err := h.Tokens.RevokeFamily(ctx, rt.FamilyID); err != nil {
			return err
		}
	}
	return c.NoContent(http.StatusNoContent)
}

func SessionFamily(c echo.Context) uuid.UUID {
	f, _ := c.Get(familyKey).(uuid.UUID)
	return f
}
