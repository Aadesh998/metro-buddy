package auth

import (
	"context"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"metro-buddy/server/internal/audit"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/ratelimit"
	"metro-buddy/server/internal/secure"
	"metro-buddy/server/internal/sms"
)

const (
	OTPTTL         = 5 * time.Minute
	OTPMaxAttempts = 5
)

var (
	ErrBadPhone       = httpx.Unprocessable("invalid_phone", "enter a valid Indian mobile number")
	ErrOTPInvalid     = httpx.E(http.StatusUnauthorized, "otp_invalid", "that code is incorrect")
	ErrOTPExpired     = httpx.E(http.StatusUnauthorized, "otp_expired", "that code has expired, request a new one")
	ErrOTPLocked      = httpx.E(http.StatusTooManyRequests, "otp_locked", "too many wrong attempts, request a new code")
	ErrOTPThrottled   = httpx.E(http.StatusTooManyRequests, "otp_throttled", "too many codes requested, try again later")
	ErrAccountBlocked = httpx.E(http.StatusForbidden, "account_disabled", "this account is not available")
)

var phoneDigits = regexp.MustCompile(`\D`)

// Accepts 10-digit Indian mobiles with optional +91/91/0 prefix and returns E.164.
func NormalizePhone(in string) (string, error) {
	d := phoneDigits.ReplaceAllString(in, "")
	switch {
	case len(d) == 12 && strings.HasPrefix(d, "91"):
		d = d[2:]
	case len(d) == 11 && strings.HasPrefix(d, "0"):
		d = d[1:]
	}
	if len(d) != 10 || d[0] < '6' {
		return "", ErrBadPhone
	}
	return "+91" + d, nil
}

type OTP struct {
	DB       *gorm.DB
	Redis    *redis.Client
	Limiter  *ratelimit.Limiter
	SMS      sms.Sender
	Log      *slog.Logger
	Box      *secure.Box
	Audit    *audit.Recorder
	Pepper   []byte
	CodeKey  []byte
	DevCode  string
	AllowDev bool
}

func (o *OTP) PhoneHash(e164 string) []byte { return secure.HMAC(o.Pepper, e164) }

func (o *OTP) key(e164 string) string { return "otp:" + hex.EncodeToString(o.PhoneHash(e164)) }

type throttle struct {
	name   string
	limit  int
	window time.Duration
}

type throttleCheck struct {
	key string
	t   throttle
}

var (
	perNumber = []throttle{{"num10m", 3, 10 * time.Minute}, {"numday", 10, 24 * time.Hour}}
	perIP     = []throttle{{"ip1h", 10, time.Hour}, {"ipday", 30, 24 * time.Hour}}
)

func (o *OTP) Request(ctx context.Context, phone, ip string) error {
	e164, err := NormalizePhone(phone)
	if err != nil {
		return err
	}
	ph := hex.EncodeToString(o.PhoneHash(e164))
	var checks []throttleCheck
	for _, t := range perNumber {
		checks = append(checks, throttleCheck{"otp:" + t.name + ":" + ph, t})
	}
	for _, t := range perIP {
		checks = append(checks, throttleCheck{"otp:" + t.name + ":" + ip, t})
	}
	for _, c := range checks {
		res, err := o.Limiter.Allow(ctx, c.key, c.t.limit, c.t.window)
		if err != nil {
			return httpx.E(http.StatusServiceUnavailable, "unavailable", "please try again shortly")
		}
		if !res.Allowed {
			o.Audit.Security(ctx, model.SecOTPThrottle, nil, ip, model.JSONMap{"throttle": c.t.name})
			return ErrOTPThrottled
		}
	}

	code := secure.Digits(6)
	if o.AllowDev && o.DevCode != "" {
		code = o.DevCode
	}
	pipe := o.Redis.TxPipeline()
	pipe.Del(ctx, o.key(e164))
	pipe.HSet(ctx, o.key(e164), "code", hex.EncodeToString(secure.HMAC(o.CodeKey, e164, code)), "attempts", 0)
	pipe.Expire(ctx, o.key(e164), OTPTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	if err := o.SMS.SendOTP(ctx, e164, code); err != nil {
		if o.Log != nil {
			o.Log.Error("otp sms failed", "err", err.Error())
		}
		return ErrSMSUnavailable
	}
	return nil
}

var ErrSMSUnavailable = httpx.E(http.StatusServiceUnavailable, "sms_unavailable", "We couldn't send the code right now. Try again in a few minutes.")

var verifyScript = redis.NewScript(`
local code = redis.call('HGET', KEYS[1], 'code')
if not code then return {-1, ''} end
local attempts = redis.call('HINCRBY', KEYS[1], 'attempts', 1)
return {attempts, code}
`)

// Returns the user, creating it on first login.
func (o *OTP) Verify(ctx context.Context, phone, code, ip string) (*model.User, bool, error) {
	e164, err := NormalizePhone(phone)
	if err != nil {
		return nil, false, err
	}
	key := o.key(e164)
	res, err := verifyScript.Run(ctx, o.Redis, []string{key}).Slice()
	if err != nil {
		return nil, false, err
	}
	attempts, _ := res[0].(int64)
	stored, _ := res[1].(string)
	if attempts == -1 {
		return nil, false, ErrOTPExpired
	}
	if attempts > OTPMaxAttempts {
		o.Redis.Del(ctx, key)
		o.Audit.Security(ctx, model.SecOTPFail, nil, ip, model.JSONMap{"reason": "locked"})
		return nil, false, ErrOTPLocked
	}
	want, _ := hex.DecodeString(stored)
	if !secure.Equal(want, secure.HMAC(o.CodeKey, e164, code)) {
		o.Audit.Security(ctx, model.SecOTPFail, nil, ip, model.JSONMap{"attempt": strconv.FormatInt(attempts, 10)})
		if attempts == OTPMaxAttempts {
			o.Redis.Del(ctx, key)
			return nil, false, ErrOTPLocked
		}
		return nil, false, ErrOTPInvalid
	}
	o.Redis.Del(ctx, key)

	hash := o.PhoneHash(e164)
	var u model.User
	err = o.DB.WithContext(ctx).Where("phone_hash = ?", hash).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		u = model.User{PhoneHash: hash, PhoneEnc: o.Box.SealString(e164), Status: model.UserActive}
		if err := o.DB.WithContext(ctx).Create(&u).Error; err != nil {
			return nil, false, err
		}
		return &u, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	if u.Status == model.UserBanned || u.Status == model.UserDeleted {
		return nil, false, ErrAccountBlocked
	}
	return &u, false, nil
}
