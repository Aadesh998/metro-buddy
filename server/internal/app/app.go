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

	"metro-buddy/server/internal/admin"
	"metro-buddy/server/internal/ai"
	"metro-buddy/server/internal/analytics"
	"metro-buddy/server/internal/audit"
	"metro-buddy/server/internal/auth"
	"metro-buddy/server/internal/bowls"
	"metro-buddy/server/internal/chat"
	"metro-buddy/server/internal/config"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/jobs"
	"metro-buddy/server/internal/media"
	"metro-buddy/server/internal/metro"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/notify"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/privacy"
	"metro-buddy/server/internal/ratelimit"
	"metro-buddy/server/internal/realtime"
	"metro-buddy/server/internal/safety"
	"metro-buddy/server/internal/secure"
	"metro-buddy/server/internal/sms"
	"metro-buddy/server/internal/web"
)

type Deps struct {
	Cfg   *config.Config
	DB    *gorm.DB
	Redis *redis.Client
	Log   *slog.Logger
	// Overridable in tests.
	SMS    sms.Sender
	Pusher notify.Pusher
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
	AI      *ai.Client
	Metro   *metro.Service
	Safety  *safety.Handlers
	Bowls   *bowls.Handlers
	Media   *media.Service
	Privacy *privacy.Service
	Admin   *admin.Handlers
	Hub     *realtime.Hub
	Chat    *chat.Service
	Notify  *notify.Service
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
	a.AI = &ai.Client{APIKey: d.Cfg.AnthropicKey, DB: d.DB, Log: d.Log}
	a.PII = &pii.Pipeline{DB: d.DB, Box: box, Audit: a.Audit,
		Second: &pii.SecondPass{DB: d.DB, Redis: d.Redis, AI: a.AI, Log: d.Log}}
	a.Metro = &metro.Service{DB: d.DB}
	a.Media = newMedia(d.Cfg)

	a.Jobs = &jobs.Runner{Redis: d.Redis, Log: d.Log}
	a.Hub = &realtime.Hub{Redis: d.Redis, Log: d.Log, SessionActive: a.Tokens.FamilyActive}
	if d.Cfg.IsDev() {
		a.Hub.Origin = []string{"localhost:8081", "localhost:19006", "localhost:4173"}
	}
	a.Notify = &notify.Service{DB: d.DB, Hub: a.Hub, Log: d.Log, Pusher: d.Pusher}
	if a.Notify.Pusher == nil {
		a.Notify.Pusher = notify.Expo{AccessToken: d.Cfg.ExpoPushToken}
	}
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

func newMedia(cfg *config.Config) *media.Service {
	var store media.Store = media.NewMemory(cfg.PublicBaseURL)
	if cfg.R2Bucket != "" {
		store = media.NewR2(cfg.R2AccountID, cfg.R2AccessKey, cfg.R2SecretKey, cfg.R2Bucket)
	}
	var scanner media.Scanner = media.ReviewAll{}
	switch {
	case cfg.NSFWEndpoint != "":
		scanner = media.HTTPScanner{Endpoint: cfg.NSFWEndpoint}
	case !cfg.IsProd():
		scanner = media.ClearAll{}
	}
	return &media.Service{Store: store, Scanner: scanner}
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

	if a.Cfg.IsDev() {
		// Lets the Expo web preview call the local API; native apps don't use CORS.
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{"http://localhost:8081", "http://localhost:19006", "http://localhost:4173"},
			AllowHeaders: []string{echo.HeaderAuthorization, echo.HeaderContentType},
		}))
	}

	e.GET("/healthz", a.health)

	if err := (&web.Handlers{DB: a.DB, Box: a.Box, Limiter: a.Limiter, Base: a.Cfg.PublicBaseURL}).Register(e); err != nil {
		panic(err)
	}

	a.Admin = &admin.Handlers{DB: a.DB, Box: a.Box, Audit: a.Audit, Limiter: a.Limiter, PII: a.PII,
		Secret: a.Cfg.JWTSecret, SecureCookie: a.Cfg.IsProd(),
		RevokeSessions: func(tx *gorm.DB, userID uuid.UUID) error {
			return a.Tokens.RevokeAllForUser(context.Background(), tx, userID)
		}}
	a.Admin.Register(e)

	v1 := e.Group("/v1", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Robots-Tag", "noindex")
			return next(c)
		}
	})
	(&auth.Handlers{DB: a.DB, OTP: a.OTP, Tokens: a.Tokens, Limiter: a.Limiter}).Register(v1)

	(&metro.Handlers{Svc: a.Metro}).Register(v1.Group("", a.Limiter.Middleware(ratelimit.Rule{Name: "metro", Limit: 120, Window: time.Minute, Key: ratelimit.ByIP})))

	authed := v1.Group("", a.Tokens.Middleware())
	(&identity.Handlers{DB: a.DB, PII: a.PII, Visible: safety.Visible(a.DB)}).Register(authed, auth.RequirePersona)
	a.Privacy = &privacy.Service{DB: a.DB, Box: a.Box, Store: a.Media.Store, Tokens: a.Tokens, Audit: a.Audit, Log: a.Log}
	a.Privacy.Register(authed)
	(&safety.Appeals{DB: a.DB, Box: a.Box, Limiter: a.Limiter}).Register(authed)
	social := authed.Group("", auth.RequirePersona)
	a.Safety = &safety.Handlers{DB: a.DB, Audit: a.Audit, Limiter: a.Limiter}
	a.Safety.Register(social)
	a.Bowls = &bowls.Handlers{DB: a.DB, PII: a.PII, Media: a.Media, Limiter: a.Limiter}
	a.Bowls.Register(social)
	social.POST("/uploads", a.Media.CreateUpload, a.Limiter.Middleware(ratelimit.Rule{Name: "uploads", Limit: 30, Window: time.Hour, ByTier: true}))
	a.Media.DevRoutes(e)
	a.Chat = &chat.Service{DB: a.DB, PII: a.PII, Media: a.Media, Hub: a.Hub, Limiter: a.Limiter}
	a.Chat.Register(social)
	a.Notify.Register(authed)
	authed.POST("/events", (&analytics.Handlers{DB: a.DB, Pepper: a.Cfg.IPPepper}).Track,
		a.Limiter.Middleware(ratelimit.Rule{Name: "events", Limit: 120, Window: time.Minute}))
	a.wireNotifications()
	receipts := &chat.Receipts{Service: a.Chat, Redis: a.Redis}
	a.Hub.OnInbound = receipts.HandleInbound
	a.Hub.OnPresence = receipts.OnPresence
	a.PII.Second.OnUpdated = a.Chat.OnMaskUpdated
	social.POST("/ws/ticket", func(c echo.Context) error { return a.Hub.IssueTicket(c, auth.SessionFamily(c)) },
		a.Limiter.Middleware(ratelimit.Rule{Name: "ws_ticket", Limit: 60, Window: time.Minute}))
	e.GET("/ws", a.Hub.Serve(context.Background()))
	social.POST("/pii/preview", pii.PreviewHandler, a.Limiter.Middleware(ratelimit.Rule{Name: "pii_preview", Limit: 120, Window: time.Minute}))

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
	go a.PII.Second.Run(ctx)
	a.Jobs.Every(ctx, "hot_rank", 5*time.Minute, a.Bowls.RefreshHotRanks)
	a.Jobs.Every(ctx, "daily_prompts", 30*time.Minute, a.Bowls.PostDailyPrompts)
	a.Jobs.Every(ctx, "exports", time.Minute, a.Privacy.RunExports)
	a.Jobs.Every(ctx, "account_purge", time.Hour, a.Privacy.PurgeDeleted)
	a.Jobs.Every(ctx, "grievance_overdue", 15*time.Minute, a.Admin.AlertOverdue)
	a.Jobs.Every(ctx, "dm_request_expiry", time.Hour, a.Chat.ExpireRequests)
	a.Jobs.Every(ctx, "room_close", time.Minute, a.Chat.CloseExpiredRooms)
	a.Jobs.Every(ctx, "inbox_purge", 6*time.Hour, a.Notify.Purge)
}
