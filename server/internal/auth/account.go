package auth

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"google.golang.org/api/idtoken"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/secure"
)

var (
	ErrEmailTaken    = httpx.Conflict("an account with this email already exists. Log in instead")
	ErrLoginFailed   = httpx.E(http.StatusUnauthorized, "login_failed", "wrong email or password")
	ErrWeakPassword  = httpx.Unprocessable("weak_password", "use at least 8 characters, with letters and numbers")
	ErrGoogleInvalid = httpx.E(http.StatusUnauthorized, "google_failed", "couldn't verify your Google account. Try again")
	ErrGoogleOff     = httpx.E(http.StatusServiceUnavailable, "google_unavailable", "Google sign-in isn't available right now")
	ErrTooManyTries  = httpx.E(http.StatusTooManyRequests, "too_many_attempts", "too many attempts. Try again in 15 minutes")
)

const (
	loginFailLimit  = 10
	loginFailWindow = 15 * time.Minute
)

// GoogleIdentity is what we keep from a verified Google ID token.
type GoogleIdentity struct {
	Sub, Email string
}

// GoogleVerifier checks an ID token's signature, issuer, expiry and audience.
type GoogleVerifier func(ctx context.Context, token string) (*GoogleIdentity, error)

// NewGoogleVerifier accepts tokens issued to any of our OAuth client IDs, and only for
// Google accounts whose email Google has verified.
func NewGoogleVerifier(clientIDs []string) GoogleVerifier {
	return func(ctx context.Context, token string) (*GoogleIdentity, error) {
		for _, aud := range clientIDs {
			p, err := idtoken.Validate(ctx, token, aud)
			if err != nil {
				continue
			}
			email, _ := p.Claims["email"].(string)
			verified, _ := p.Claims["email_verified"].(bool)
			if p.Subject == "" || email == "" || !verified {
				return nil, errors.New("email not verified")
			}
			return &GoogleIdentity{Sub: p.Subject, Email: email}, nil
		}
		return nil, errors.New("invalid token")
	}
}

func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

var ErrBadEmail = httpx.Unprocessable("invalid_email", "enter a valid email address")

func validEmail(email string) bool {
	a, err := mail.ParseAddress(email)
	return err == nil && a.Address == email && strings.Contains(email[strings.LastIndex(email, "@"):], ".")
}

func (h *Handlers) emailHash(email string) []byte {
	return secure.HMAC(h.OTP.Pepper, "email:"+email)
}

func strongEnough(pw string) bool {
	if len(pw) < 8 || len(pw) > 128 {
		return false
	}
	hasLetter, hasDigit := false, false
	for _, r := range pw {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		}
	}
	return hasLetter && hasDigit
}

// startSession registers the device and issues tokens, the same way for every sign-in method.
func (h *Handlers) startSession(c echo.Context, u *model.User, dev deviceInfo, isNew bool) error {
	ctx := c.Request().Context()
	if u.Status == model.UserBanned || u.Status == model.UserDeleted {
		return ErrAccountBlocked
	}
	now := time.Now()
	d := model.Device{UserID: u.ID, Platform: dev.Platform, AppVersion: dev.AppVersion, LastSeenAt: now}
	if err := h.DB.WithContext(ctx).Create(&d).Error; err != nil {
		return err
	}
	h.DB.WithContext(ctx).Model(&model.User{}).Where("id = ?", u.ID).Update("last_active_at", now)
	s, err := h.Tokens.Issue(ctx, u.ID, &d.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, verifyResponse{Session: s, IsNewUser: isNew})
}

type signupRequest struct {
	Email    string     `json:"email" validate:"required,max=254"`
	Password string     `json:"password" validate:"required,max=128"`
	Device   deviceInfo `json:"device" validate:"required"`
}

func (h *Handlers) emailSignup(c echo.Context) error {
	req, err := httpx.Bind[signupRequest](c)
	if err != nil {
		return err
	}
	email := normalizeEmail(req.Email)
	if !validEmail(email) {
		return ErrBadEmail
	}
	if !strongEnough(req.Password) {
		return ErrWeakPassword
	}
	hash := secure.HashPassword(req.Password)
	u := model.User{EmailHash: h.emailHash(email), EmailEnc: h.OTP.Box.SealString(email), PasswordHash: &hash, Status: model.UserActive}
	if err := h.DB.WithContext(c.Request().Context()).Create(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrEmailTaken
		}
		return err
	}
	return h.startSession(c, &u, req.Device, true)
}

func (h *Handlers) failKey(email string) string {
	return "login_fail:" + string(secure.SHA256([]byte(email)))
}

func (h *Handlers) emailLogin(c echo.Context) error {
	req, err := httpx.Bind[signupRequest](c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	email := normalizeEmail(req.Email)
	key := h.failKey(email)
	if n, _ := h.OTP.Redis.Get(ctx, key).Int(); n >= loginFailLimit {
		return ErrTooManyTries
	}
	var u model.User
	found := h.DB.WithContext(ctx).Where("email_hash = ?", h.emailHash(email)).Limit(1).Find(&u).RowsAffected == 1
	// Unknown emails and Google-only accounts take as long as a wrong password.
	encoded := secure.DummyHash
	if found && u.PasswordHash != nil {
		encoded = *u.PasswordHash
	}
	if !secure.CheckPassword(encoded, req.Password) || !found || u.PasswordHash == nil {
		pipe := h.OTP.Redis.TxPipeline()
		pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, loginFailWindow)
		_, _ = pipe.Exec(ctx)
		h.OTP.Audit.Security(ctx, model.SecLoginFail, nil, c.RealIP(), nil)
		return ErrLoginFailed
	}
	h.OTP.Redis.Del(ctx, key)
	return h.startSession(c, &u, req.Device, false)
}

type googleRequest struct {
	IDToken string     `json:"id_token" validate:"required,max=4096"`
	Device  deviceInfo `json:"device" validate:"required"`
}

func (h *Handlers) google(c echo.Context) error {
	req, err := httpx.Bind[googleRequest](c)
	if err != nil {
		return err
	}
	if h.Google == nil {
		return ErrGoogleOff
	}
	ctx := c.Request().Context()
	id, err := h.Google(ctx, req.IDToken)
	if err != nil {
		h.OTP.Audit.Security(ctx, model.SecLoginFail, nil, c.RealIP(), model.JSONMap{"method": "google"})
		return ErrGoogleInvalid
	}
	email := normalizeEmail(id.Email)
	now := time.Now().UTC()
	var u model.User
	isNew := false
	err = h.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Where("google_sub = ?", id.Sub).Limit(1).Find(&u).RowsAffected == 1 {
			return nil
		}
		if tx.Where("email_hash = ?", h.emailHash(email)).Limit(1).Find(&u).RowsAffected == 1 {
			updates := map[string]any{"google_sub": id.Sub, "email_verified_at": now}
			// Google proves who owns this address. If nobody had verified it, a password set by
			// whoever registered it first could belong to someone else, so it stops working.
			if u.EmailVerifiedAt == nil {
				updates["password_hash"] = nil
			}
			return tx.Model(&u).Updates(updates).Error
		}
		sub := id.Sub
		u = model.User{EmailHash: h.emailHash(email), EmailEnc: h.OTP.Box.SealString(email), EmailVerifiedAt: &now, GoogleSub: &sub, Status: model.UserActive}
		isNew = true
		return tx.Create(&u).Error
	})
	if err != nil {
		return err
	}
	return h.startSession(c, &u, req.Device, isNew)
}
