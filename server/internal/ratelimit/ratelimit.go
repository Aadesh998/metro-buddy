package ratelimit

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
)

type KeyFunc func(c echo.Context) string

func ByIP(c echo.Context) string { return "ip:" + c.RealIP() }

func ByUser(c echo.Context) string {
	if p := httpx.PrincipalFrom(c); p != nil {
		return "u:" + p.UserID.String()
	}
	return ByIP(c)
}

type Rule struct {
	Name   string
	Limit  int
	Window time.Duration
	Key    KeyFunc
	// Scales Limit by the caller's trust tier; new accounts get the base limit.
	ByTier bool
	// Auth/OTP rules deny when Redis is unavailable instead of letting traffic through.
	FailClosed bool
}

var tierMultiplier = []float64{1, 2, 4, 8}

func (r Rule) limitFor(c echo.Context) int {
	if !r.ByTier {
		return r.Limit
	}
	tier := 0
	p := httpx.PrincipalFrom(c)
	if p != nil {
		tier = max(0, min(p.TrustTier, len(tierMultiplier)-1))
	}
	// Failed attestation never blocks, but drops to half the lowest tier's budget.
	if p != nil && p.Restricted {
		return max(1, r.Limit/2)
	}
	return int(math.Round(float64(r.Limit) * tierMultiplier[tier]))
}

type Limiter struct {
	rdb       *redis.Client
	log       *slog.Logger
	now       func() time.Time
	OnLimited func(c echo.Context, rule string)
}

func New(rdb *redis.Client, log *slog.Logger) *Limiter {
	return &Limiter{rdb: rdb, log: log, now: time.Now}
}

// Sliding-window log: one sorted-set member per request, scored by time.
var slidingWindow = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)
local count = redis.call('ZCARD', key)
if count < limit then
  redis.call('ZADD', key, now, member)
  redis.call('PEXPIRE', key, window)
  return {1, 0}
end
local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
return {0, tonumber(oldest[2]) + window - now}
`)

type Result struct {
	Allowed    bool
	RetryAfter time.Duration
}

func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (Result, error) {
	now := l.now().UnixMilli()
	res, err := slidingWindow.Run(ctx, l.rdb, []string{"rl:" + key}, now, window.Milliseconds(), limit, model.NewID().String()).Int64Slice()
	if err != nil {
		return Result{}, err
	}
	return Result{Allowed: res[0] == 1, RetryAfter: time.Duration(res[1]) * time.Millisecond}, nil
}

func (l *Limiter) Middleware(r Rule) echo.MiddlewareFunc {
	key := r.Key
	if key == nil {
		key = ByUser
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			res, err := l.Allow(c.Request().Context(), r.Name+":"+key(c), r.limitFor(c), r.Window)
			if err != nil {
				l.log.Error("rate limiter unavailable", "rule", r.Name, "err", err.Error())
				if r.FailClosed {
					return httpx.E(http.StatusServiceUnavailable, "unavailable", "please try again shortly")
				}
				return next(c)
			}
			if !res.Allowed {
				if l.OnLimited != nil {
					l.OnLimited(c, r.Name)
				}
				secs := int(math.Ceil(res.RetryAfter.Seconds()))
				c.Response().Header().Set("Retry-After", strconv.Itoa(max(secs, 1)))
				return httpx.TooMany()
			}
			return next(c)
		}
	}
}
