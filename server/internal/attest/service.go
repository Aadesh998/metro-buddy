// Package attest checks that requests come from a genuine app on a genuine device (F06):
// Play Integrity on Android, App Attest on iOS. A failed or missing check never blocks
// signup; the account just gets tighter rate limits until a later check passes.
package attest

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"metro-buddy/server/internal/audit"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
)

const challengeTTL = 5 * time.Minute

type Android interface {
	Verify(ctx context.Context, token, nonce string) error
}

type Service struct {
	DB      *gorm.DB
	Redis   *redis.Client
	Audit   *audit.Recorder
	Android Android
	// AppleAppID is "<TeamID>.<bundle id>"; empty disables iOS checks.
	AppleAppID string
	AllowDev   bool
	// Family returns the session family of the request, to find the device behind it.
	Family func(c echo.Context) uuid.UUID
	Now    func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

func (s *Service) Register(g *echo.Group) {
	g.POST("/me/attest/challenge", s.challenge)
	g.POST("/me/attest", s.verify)
}

func ckey(u uuid.UUID) string { return "attest:" + u.String() }

func (s *Service) challenge(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	nonce := base64.RawURLEncoding.EncodeToString(b)
	if err := s.Redis.Set(c.Request().Context(), ckey(p.UserID), nonce, challengeTTL).Err(); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"challenge": nonce})
}

type verifyRequest struct {
	Platform    string `json:"platform" validate:"required,oneof=android ios"`
	Token       string `json:"token" validate:"required,max=20000"`
	KeyID       string `json:"key_id" validate:"max=200"`
	Attestation string `json:"attestation" validate:"max=20000"`
}

func (s *Service) verify(c echo.Context) error {
	req, err := httpx.Bind[verifyRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	nonce, err := s.Redis.GetDel(ctx, ckey(p.UserID)).Result()
	if errors.Is(err, redis.Nil) {
		return httpx.Unprocessable("invalid_challenge", "the check expired, try again")
	}
	if err != nil {
		return err
	}
	var verr error
	switch req.Platform {
	case "android":
		if s.Android == nil {
			return c.JSON(http.StatusOK, map[string]any{"ok": nil})
		}
		verr = s.Android.Verify(ctx, req.Token, nonce)
	case "ios":
		if s.AppleAppID == "" {
			return c.JSON(http.StatusOK, map[string]any{"ok": nil})
		}
		att, err1 := base64.StdEncoding.DecodeString(req.Attestation)
		kid, err2 := base64.StdEncoding.DecodeString(req.KeyID)
		if err1 != nil || err2 != nil {
			verr = errors.New("bad encoding")
		} else {
			verr = VerifyAppAttest(att, kid, []byte(nonce), s.AppleAppID, s.AllowDev, s.now())
		}
	}
	ok := verr == nil
	if err := s.record(ctx, c, p.UserID, ok); err != nil {
		return err
	}
	if !ok {
		s.Audit.Security(ctx, model.SecAttestationFailed, &p.UserID, c.RealIP(), model.JSONMap{"platform": req.Platform})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": ok})
}

// record stores the verdict on the device behind this session.
func (s *Service) record(ctx context.Context, c echo.Context, user uuid.UUID, ok bool) error {
	tx := s.DB.WithContext(ctx)
	var ids []uuid.UUID
	if s.Family != nil {
		tx.Model(&model.RefreshToken{}).Where("family_id = ? AND device_id IS NOT NULL", s.Family(c)).Order("created_at DESC").Limit(1).Pluck("device_id", &ids)
	}
	if len(ids) == 0 {
		return nil
	}
	return tx.Model(&model.Device{}).Where("id = ? AND user_id = ?", ids[0], user).Updates(map[string]any{"attestation_ok": ok, "last_seen_at": s.now()}).Error
}
