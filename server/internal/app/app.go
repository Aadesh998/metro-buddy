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
	"metro-buddy/server/internal/attest"
	"metro-buddy/server/internal/audit"
	"metro-buddy/server/internal/auth"
	"metro-buddy/server/internal/bloom"
	"metro-buddy/server/internal/bowls"
	"metro-buddy/server/internal/cache"
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
	"metro-buddy/server/internal/studio"
	"metro-buddy/server/internal/travel"
	"metro-buddy/server/internal/trust"
	"metro-buddy/server/internal/verify"
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
	Live   studio.Live
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
	Travel  *travel.Service
	Studio  *studio.Service
	Triage  *safety.Triage
	Attest  *attest.Service
	Aliases *identity.AliasIndex
	Cache   *cache.Bus
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
		DB: d.DB, Redis: d.Redis, Limiter: a.Limiter, SMS: d.SMS, Box: box, Audit: a.Audit, Log: d.Log,
		Pepper: d.Cfg.PhonePepper, CodeKey: d.Cfg.JWTSecret,
		DevCode: d.Cfg.OTPDevCode, AllowDev: d.Cfg.IsDev(),
	}
	a.AI = &ai.Client{APIKey: d.Cfg.AnthropicKey, DB: d.DB, Log: d.Log}
	a.PII = &pii.Pipeline{DB: d.DB, Box: box, Audit: a.Audit,
		Second: &pii.SecondPass{DB: d.DB, Redis: d.Redis, AI: a.AI, Log: d.Log}}
	a.Metro = &metro.Service{DB: d.DB}
	a.Cache = &cache.Bus{Redis: d.Redis, Log: d.Log}
	// Tests change rows directly and assert on the result, so they read through the database.
	if !d.Cfg.IsTest() {
		if err := identity.EnableRefCache(d.DB, a.Cache, 50_000, time.Minute); err != nil {
			return nil, err
		}
	}
	// ~2.4 MB of Redis covers two million aliases at a 1% false-positive rate.
	a.Aliases = &identity.AliasIndex{Filter: bloom.New(d.Redis, "bf:aliases", 2_000_000, 0.01)}
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
	if cfg.SMSProvider == "gateway" {
		return sms.Gateway{URL: cfg.SMSGatewayURL, User: cfg.SMSGatewayUser, Password: cfg.SMSGatewayPassword}
	}
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
	switch {
	case cfg.S3Endpoint != "":
		s3 := media.NewS3(cfg.S3Endpoint, cfg.S3Region, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := s3.EnsureBucket(ctx); err != nil {
			slog.Default().Warn("s3 bucket check failed", "err", err.Error())
		}
		cancel()
		store = s3
	case cfg.R2Bucket != "":
		store = media.NewR2(cfg.R2AccountID, cfg.R2AccessKey, cfg.R2SecretKey, cfg.R2Bucket)
	}
	var scanner media.Scanner = media.ReviewAll{}
	switch {
	case cfg.NSFWEndpoint != "":
		scanner = media.HTTPScanner{Endpoint: cfg.NSFWEndpoint, Token: cfg.NSFWToken}
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
		},
		EndEpisode: func(ctx context.Context, id uuid.UUID) error { return a.Studio.ForceEnd(ctx, id) },
		MediaStats: a.Media.URLCacheStats}
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
	(&identity.Handlers{DB: a.DB, PII: a.PII, Visible: safety.Visible(a.DB), Aliases: a.Aliases}).Register(authed, auth.RequirePersona)
	a.Privacy = &privacy.Service{DB: a.DB, Box: a.Box, Store: a.Media.Store, Tokens: a.Tokens, Audit: a.Audit, Log: a.Log}
	a.Privacy.Register(authed)
	(&safety.Appeals{DB: a.DB, Box: a.Box, Limiter: a.Limiter}).Register(authed)
	social := authed.Group("", auth.RequirePersona)
	a.Triage = &safety.Triage{DB: a.DB, Redis: a.Redis, AI: a.AI, Log: a.Log}
	a.Safety = &safety.Handlers{DB: a.DB, Audit: a.Audit, Limiter: a.Limiter, OnCase: a.Triage.Enqueue}
	a.Admin.Triage = a.Triage.Enqueue
	a.Safety.Register(social)
	a.Bowls = &bowls.Handlers{DB: a.DB, PII: a.PII, Media: a.Media, Limiter: a.Limiter}
	a.Bowls.Register(social)
	social.POST("/uploads", a.Media.CreateUpload, a.Limiter.Middleware(ratelimit.Rule{Name: "uploads", Limit: 30, Window: time.Hour, ByTier: true}))
	a.Media.DevRoutes(e)
	a.Chat = &chat.Service{DB: a.DB, PII: a.PII, Media: a.Media, Hub: a.Hub, Limiter: a.Limiter, Box: a.Box, MaskOptions: chat.RevealMaskOptions,
		Audit: func(tx *gorm.DB, userID uuid.UUID, verb string, conv uuid.UUID, meta model.JSONMap) {
			a.Audit.Log(tx, model.ActorUser, &userID, verb, "conversation", &conv, meta)
		}}
	a.Chat.Register(social)
	a.Notify.Register(authed)
	a.Travel = &travel.Service{DB: a.DB, Redis: a.Redis, Metro: a.Metro, Chat: a.Chat, Notify: a.Notify, Limiter: a.Limiter,
		Audit: a.Audit, Box: a.Box, SMS: a.SMS, BaseURL: a.Cfg.PublicBaseURL, AI: a.AI, Log: a.Log}
	if !a.Cfg.IsTest() {
		a.Travel.Async = func(f func()) { go f() }
	}
	a.Travel.Register(social, authed)
	e.GET("/trip/:token", a.Travel.SharePage)
	if a.Live == nil {
		a.Live = studio.NewLiveKit(a.Cfg.LiveKitURL, a.Cfg.LiveKitAPIKey, a.Cfg.LiveKitAPISecret)
	}
	a.Studio = &studio.Service{DB: a.DB, Live: a.Live, Hub: a.Hub, Notify: a.Notify, PII: a.PII, Audit: a.Audit, Limiter: a.Limiter, AI: a.AI, Redis: a.Redis, Log: a.Log}
	a.Studio.Register(social, e)
	a.Safety.OnReport = a.Studio.OnReport
	v := &verify.Service{DB: a.DB, Redis: a.Redis, Audit: a.Audit, Limiter: a.Limiter, Provider: a.Cfg.LivenessProvider, Region: a.Cfg.AWSRegion}
	switch {
	case a.Cfg.LivenessProvider == "rekognition":
		v.Liveness = verify.NewRekognition(a.Cfg.AWSRegion, a.Cfg.AWSAccessKey, a.Cfg.AWSSecretKey)
	case !a.Cfg.IsProd():
		v.Liveness, v.Provider = verify.Dev{}, "dev"
	}
	v.Register(authed)
	at := &attest.Service{DB: a.DB, Redis: a.Redis, Audit: a.Audit, AppleAppID: a.Cfg.AppleAppID, AllowDev: !a.Cfg.IsProd(), Family: auth.SessionFamily}
	if a.Cfg.PlayIntegrityKey != "" {
		at.Android = &attest.PlayIntegrity{Package: a.Cfg.PlayIntegrityPkg, Credentials: []byte(a.Cfg.PlayIntegrityKey)}
	}
	at.Register(authed)
	a.Attest = at
	authed.POST("/events", (&analytics.Handlers{DB: a.DB, Pepper: a.Cfg.IPPepper}).Track,
		a.Limiter.Middleware(ratelimit.Rule{Name: "events", Limit: 120, Window: time.Minute}))
	a.wireNotifications()
	receipts := &chat.Receipts{Service: a.Chat, Redis: a.Redis}
	a.Hub.OnInbound = receipts.HandleInbound
	a.Hub.OnPresence = receipts.OnPresence
	a.PII.Second.OnUpdated = a.Chat.OnMaskUpdated
	a.Admin.OnCaseClosed = a.Chat.OnCaseClosed
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
	go a.Cache.Run(ctx)
	go func() {
		if err := a.Aliases.Warm(ctx, a.DB); err != nil {
			a.Log.Warn("alias filter warm-up failed", "err", err.Error())
		}
	}()
	go a.Triage.Run(ctx)
	a.Jobs.Every(ctx, "hot_rank", 5*time.Minute, a.Bowls.RefreshHotRanks)
	a.Jobs.Every(ctx, "daily_prompts", 30*time.Minute, a.Bowls.PostDailyPrompts)
	a.Jobs.Every(ctx, "exports", time.Minute, a.Privacy.RunExports)
	a.Jobs.Every(ctx, "account_purge", time.Hour, a.Privacy.PurgeDeleted)
	a.Jobs.Every(ctx, "grievance_overdue", 15*time.Minute, a.Admin.AlertOverdue)
	a.Jobs.Every(ctx, "dm_request_expiry", time.Hour, a.Chat.ExpireRequests)
	a.Jobs.Every(ctx, "room_close", time.Minute, a.Chat.CloseExpiredRooms)
	a.Jobs.Every(ctx, "message_expiry", time.Minute, a.Chat.PurgeExpired)
	a.Jobs.Every(ctx, "inbox_purge", 6*time.Hour, a.Notify.Purge)
	a.Jobs.Every(ctx, "plan_intents", time.Minute, a.Travel.SpawnPlanIntents)
	a.Jobs.Every(ctx, "match_expiry", 5*time.Minute, a.Travel.ExpireMatches)
	a.Jobs.Every(ctx, "trip_lifecycle", time.Minute, a.Travel.TripLifecycle)
	a.Jobs.Every(ctx, "crew_reminders", time.Minute, a.Travel.CrewReminders)
	a.Jobs.Every(ctx, "trust_recompute", 24*time.Hour, (&trust.Service{DB: a.DB}).RecomputeAll)
	a.Jobs.Every(ctx, "episode_reminders", time.Minute, a.Studio.SendReminders)
	a.Jobs.Every(ctx, "on_air_cap", 2*time.Second, a.Studio.EnforceOnAirCap)
	a.Jobs.Every(ctx, "stale_episodes", 10*time.Minute, a.Studio.EndStale)
}
