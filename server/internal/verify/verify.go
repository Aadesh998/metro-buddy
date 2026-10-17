// Package verify runs selfie liveness checks (F07) through a vendor. Only the outcome is kept:
// users.verified_at. No face image, video or vendor score is stored, and audit images the vendor
// returns are dropped in memory.
package verify

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	rtypes "github.com/aws/aws-sdk-go-v2/service/rekognition/types"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"metro-buddy/server/internal/audit"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/ratelimit"
	"metro-buddy/server/internal/trust"
)

const (
	MinConfidence = 90
	sessionTTL    = 10 * time.Minute
)

type Status string

const (
	Pending Status = "pending"
	Passed  Status = "passed"
	Failed  Status = "failed"
)

type Liveness interface {
	Start(ctx context.Context, ref string) (string, error)
	Result(ctx context.Context, sessionID string) (Status, error)
}

// Rekognition uses AWS Face Liveness. The mobile app streams the selfie video straight to AWS
// with the session id; our servers never see it.
type Rekognition struct {
	c *rekognition.Client
}

func NewRekognition(region, key, secret string) *Rekognition {
	cfg := aws.Config{Region: region, Credentials: credentials.NewStaticCredentialsProvider(key, secret, "")}
	return &Rekognition{c: rekognition.NewFromConfig(cfg)}
}

func (r *Rekognition) Start(ctx context.Context, ref string) (string, error) {
	// No OutputConfig: AWS keeps no audit images in our account, and AuditImagesLimit 0 asks for none back.
	out, err := r.c.CreateFaceLivenessSession(ctx, &rekognition.CreateFaceLivenessSessionInput{
		ClientRequestToken: aws.String(ref),
		Settings:           &rtypes.CreateFaceLivenessSessionRequestSettings{AuditImagesLimit: aws.Int32(0)},
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.SessionId), nil
}

func (r *Rekognition) Result(ctx context.Context, id string) (Status, error) {
	out, err := r.c.GetFaceLivenessSessionResults(ctx, &rekognition.GetFaceLivenessSessionResultsInput{SessionId: aws.String(id)})
	if err != nil {
		return Failed, err
	}
	switch out.Status {
	case rtypes.LivenessSessionStatusSucceeded:
		if out.Confidence != nil && *out.Confidence >= MinConfidence {
			return Passed, nil
		}
		return Failed, nil
	case rtypes.LivenessSessionStatusCreated, rtypes.LivenessSessionStatusInProgress:
		return Pending, nil
	}
	return Failed, nil
}

// Dev passes every session; it is wired only outside production.
type Dev struct{}

func (Dev) Start(context.Context, string) (string, error) { return "dev-" + uuid.NewString(), nil }
func (Dev) Result(_ context.Context, id string) (Status, error) {
	if strings.HasPrefix(id, "dev-fail") {
		return Failed, nil
	}
	return Passed, nil
}

type Service struct {
	DB       *gorm.DB
	Redis    *redis.Client
	Liveness Liveness
	Provider string
	Region   string
	Audit    *audit.Recorder
	Limiter  *ratelimit.Limiter
}

func (s *Service) Register(g *echo.Group) {
	g.POST("/me/verification", s.start, s.Limiter.Middleware(ratelimit.Rule{Name: "verify", Limit: 3, Window: 24 * time.Hour, FailClosed: true}))
	g.POST("/me/verification/check", s.check)
}

func key(user uuid.UUID) string { return "verify:" + user.String() }

func (s *Service) start(c echo.Context) error {
	if s.Liveness == nil {
		return httpx.E(http.StatusServiceUnavailable, "verification_unavailable", "verification isn't available right now")
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var u model.User
	if err := s.DB.WithContext(ctx).Select("verified_at").First(&u, "id = ?", p.UserID).Error; err != nil {
		return err
	}
	if u.VerifiedAt != nil {
		return httpx.Conflict("you're already verified")
	}
	id, err := s.Liveness.Start(ctx, uuid.NewString())
	if err != nil {
		return httpx.E(http.StatusBadGateway, "verification_unavailable", "couldn't start verification, try again later")
	}
	if err := s.Redis.Set(ctx, key(p.UserID), id, sessionTTL).Err(); err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, map[string]any{"session_id": id, "provider": s.Provider, "region": s.Region, "expires_in": int(sessionTTL.Seconds())})
}

type checkRequest struct {
	SessionID string `json:"session_id" validate:"required,max=100"`
}

var errSession = httpx.Unprocessable("invalid_session", "that verification session has expired, start again")

func (s *Service) check(c echo.Context) error {
	req, err := httpx.Bind[checkRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	// A session only counts for the user who started it.
	want, err := s.Redis.Get(ctx, key(p.UserID)).Result()
	if errors.Is(err, redis.Nil) || want != req.SessionID {
		return errSession
	}
	if err != nil {
		return err
	}
	st, err := s.Liveness.Result(ctx, req.SessionID)
	if err != nil {
		return httpx.E(http.StatusBadGateway, "verification_unavailable", "couldn't read the result, try again")
	}
	if st == Pending {
		return c.JSON(http.StatusOK, map[string]any{"status": st})
	}
	s.Redis.Del(ctx, key(p.UserID))
	if st == Passed {
		err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&model.User{}).Where("id = ? AND verified_at IS NULL", p.UserID).Update("verified_at", time.Now().UTC()).Error; err != nil {
				return err
			}
			return trust.Record(tx, p.UserID, model.TrustVerified, 0, nil)
		})
		if err != nil {
			return err
		}
	}
	s.Audit.Log(s.DB.WithContext(ctx), model.ActorUser, &p.UserID, "verify."+string(st), "user", &p.UserID, nil)
	return c.JSON(http.StatusOK, map[string]any{"status": st})
}
