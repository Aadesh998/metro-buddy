package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"metro-buddy/server/internal/audit"
	"metro-buddy/server/internal/auth"
	"metro-buddy/server/internal/config"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/jobs"
	"metro-buddy/server/internal/metro"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/ratelimit"
	"metro-buddy/server/internal/secure"
	"metro-buddy/server/internal/sms"
)

type Deps struct {
	Cfg   *config.Config
	DB    *gorm.DB
	Redis *redis.Client
	Log   *slog.Logger
	// Overridable in tests.
	SMS sms.Sender
}

type App struct {
	*Deps
	Echo    *echo.Echo
	Box     *secure.Box
	Audit   *audit.Recorder
	Limiter *ratelimit.Limiter
	Tokens  *auth.Tokens
	OTP     *auth.OTP
	PII     *pii.Pipeline
	Metro   *metro.Service
	Jobs    *jobs.Runner
}

func New(d *Deps) (*App, error) {
	box, err := secure.NewBox(d.Cfg.DataKey)
	if err != nil {
		return nil, err
	}
	a := &App{Deps: d, Box: box}
	a.Audit = audit.New(d.DB, d.Log, d.Cfg.IPPepper)
	a.Limiter = ratelimit.New(d.Redis, d.Log)
	a.Limiter.OnLimited = func(c echo.Context, rule string) {
		var uid *uuid.UUID
		if p := httpx.PrincipalFrom(c); p != nil {
			uid = &p.UserID
		}
		a.Audit.Security(c.Request().Context(), model.SecRateLimited, uid, c.RealIP(), model.JSONMap{"rule": rule, "route": c.Path()})
	}
	if d.SMS == nil {
		d.SMS = newSMS(d.Cfg, d.Log)
	}
	a.Tokens = &auth.Tokens{DB: d.DB, Secret: d.Cfg.JWTSecret, Audit: a.Audit}
	a.OTP = &auth.OTP{
		DB: d.DB, Redis: d.Redis, Limiter: a.Limiter, SMS: d.SMS, Box: box, Audit: a.Audit,
		Pepper: d.Cfg.PhonePepper, CodeKey: d.Cfg.JWTSecret,
		DevCode: d.Cfg.OTPDevCode, AllowDev: d.Cfg.IsDev(),
	}
	a.PII = &pii.Pipeline{DB: d.DB, Box: box, Audit: a.Audit}
	a.Metro = &metro.Service{DB: d.DB}
	a.Jobs = &jobs.Runner{Redis: d.Redis, Log: d.Log}
	a.Echo = a.routes()
	return a, nil
}

func newSMS(cfg *config.Config, log *slog.Logger) sms.Sender {
	if cfg.SMSProvider != "msg91" {
		return sms.Dev{Log: log}
	}
	var s sms.Sender = sms.MSG91{AuthKey: cfg.MSG91AuthKey, OTPTemplateID: cfg.MSG91Template, SOSFlowID: cfg.MSG91SOSFlow}
	if cfg.TwilioSID != "" {
		s = sms.Fallback{Primary: s, Secondary: sms.Twilio{AccountSID: cfg.TwilioSID, AuthToken: cfg.TwilioToken, From: cfg.TwilioFrom}, Log: log}
	}
	return s
}

func (a *App) routes() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.IPExtractor = echo.ExtractIPFromXFFHeader()
	e.Validator = httpx.NewValidator()
	e.HTTPErrorHandler = httpx.ErrorHandler(a.Log)
	e.Use(
		httpx.RequestIDMiddleware(),
		httpx.AccessLog(a.Log),
		httpx.Recover(a.Log),
		middleware.BodyLimit("1M"),
		middleware.SecureWithConfig(middleware.SecureConfig{
			XSSProtection:      "0",
			ContentTypeNosniff: "nosniff",
			XFrameOptions:      "DENY",
			HSTSMaxAge:         31536000,
			ReferrerPolicy:     "no-referrer",
		}),
	)

	e.GET("/healthz", a.health)

	v1 := e.Group("/v1")
	(&auth.Handlers{DB: a.DB, OTP: a.OTP, Tokens: a.Tokens, Limiter: a.Limiter}).Register(v1)

	(&metro.Handlers{Svc: a.Metro}).Register(v1.Group("", a.Limiter.Middleware(ratelimit.Rule{Name: "metro", Limit: 120, Window: time.Minute, Key: ratelimit.ByIP})))

	authed := v1.Group("", a.Tokens.Middleware())
	(&identity.Handlers{DB: a.DB, PII: a.PII}).Register(authed, auth.RequirePersona)

	return e
}

func (a *App) health(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
	defer cancel()

	status := map[string]string{"db": "ok", "redis": "ok"}
	code := http.StatusOK
	if sqlDB, err := a.DB.DB(); err != nil || sqlDB.PingContext(ctx) != nil {
		status["db"] = "down"
		code = http.StatusServiceUnavailable
	}
	if a.Redis.Ping(ctx).Err() != nil {
		status["redis"] = "down"
		code = http.StatusServiceUnavailable
	}
	return c.JSON(code, status)
}

// StartJobs runs background work; kept out of New so tests drive jobs explicitly.
func (a *App) StartJobs(ctx context.Context) {
	a.Jobs.Every(ctx, "pii_vault_purge", time.Hour, a.PII.Purge)
}
