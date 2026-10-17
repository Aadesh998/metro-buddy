package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"metro-buddy/server/internal/audit"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/secure"
)

const (
	AccessTTL  = 15 * time.Minute
	RefreshTTL = 30 * 24 * time.Hour
	issuer     = "metro-buddy"
	audience   = "metro-buddy-app"
)

var ErrRefreshInvalid = httpx.E(http.StatusUnauthorized, "refresh_invalid", "session expired, please log in again")

type Tokens struct {
	DB     *gorm.DB
	Secret []byte
	Audit  *audit.Recorder
	Now    func() time.Time
}

type Session struct {
	AccessToken      string    `json:"access_token"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshToken     string    `json:"refresh_token"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

type claims struct {
	jwt.RegisteredClaims
	Family string `json:"fam"`
}

func (t *Tokens) now() time.Time {
	if t.Now != nil {
		return t.Now()
	}
	return time.Now()
}

// Every login starts a new family, so a token stolen before login can never be rotated into this session.
func (t *Tokens) Issue(ctx context.Context, userID uuid.UUID, deviceID *uuid.UUID) (*Session, error) {
	return t.issue(t.DB.WithContext(ctx), userID, deviceID, model.NewID())
}

func (t *Tokens) issue(tx *gorm.DB, userID uuid.UUID, deviceID *uuid.UUID, family uuid.UUID) (*Session, error) {
	now := t.now()
	raw := secure.Token(32)
	rt := model.RefreshToken{
		UserID:    userID,
		DeviceID:  deviceID,
		FamilyID:  family,
		TokenHash: hashToken(raw),
		ExpiresAt: now.Add(RefreshTTL),
	}
	if err := tx.Create(&rt).Error; err != nil {
		return nil, err
	}
	access, err := t.sign(userID, family, now)
	if err != nil {
		return nil, err
	}
	return &Session{AccessToken: access, AccessExpiresAt: now.Add(AccessTTL), RefreshToken: raw, RefreshExpiresAt: rt.ExpiresAt}, nil
}

func (t *Tokens) sign(userID, family uuid.UUID, now time.Time) (string, error) {
	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL)),
			ID:        model.NewID().String(),
		},
		Family: family.String(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(t.Secret)
}

type AccessClaims struct {
	UserID uuid.UUID
	Family uuid.UUID
}

func (t *Tokens) Parse(token string) (*AccessClaims, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return t.Secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer), jwt.WithAudience(audience), jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil {
		return nil, err
	}
	uid, err := uuid.Parse(c.Subject)
	if err != nil {
		return nil, err
	}
	fam, err := uuid.Parse(c.Family)
	if err != nil {
		return nil, err
	}
	return &AccessClaims{UserID: uid, Family: fam}, nil
}

func (t *Tokens) Rotate(ctx context.Context, raw, ip string) (*Session, error) {
	var out *Session
	var reused *model.RefreshToken
	err := t.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rt model.RefreshToken
		if err := tx.Where("token_hash = ?", hashToken(raw)).First(&rt).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRefreshInvalid
			}
			return err
		}
		if rt.RevokedAt != nil {
			reused = &rt
			return ErrRefreshInvalid
		}
		if t.now().After(rt.ExpiresAt) {
			return ErrRefreshInvalid
		}
		var u model.User
		if err := tx.Select("id", "status").First(&u, "id = ?", rt.UserID).Error; err != nil {
			return ErrRefreshInvalid
		}
		if u.Status == model.UserBanned || u.Status == model.UserDeleted {
			return ErrAccountBlocked
		}

		s, err := t.issue(tx, rt.UserID, rt.DeviceID, rt.FamilyID)
		if err != nil {
			return err
		}
		var next model.RefreshToken
		if err := tx.Select("id").Where("token_hash = ?", hashToken(s.RefreshToken)).First(&next).Error; err != nil {
			return err
		}
		// Conditional update: if a concurrent rotation already revoked this row, we lose the race and fail.
		res := tx.Model(&model.RefreshToken{}).Where("id = ? AND revoked_at IS NULL", rt.ID).
			Updates(map[string]any{"revoked_at": t.now(), "replaced_by": next.ID})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrRefreshInvalid
		}
		out = s
		return nil
	})
	if reused != nil {
		// A revoked token being presented means it was copied: kill the whole family.
		t.RevokeFamily(ctx, reused.FamilyID)
		t.Audit.Security(ctx, model.SecRefreshReuse, &reused.UserID, ip, model.JSONMap{"family": reused.FamilyID.String()})
	}
	return out, err
}

func (t *Tokens) RevokeFamily(ctx context.Context, family uuid.UUID) error {
	return t.DB.WithContext(ctx).Model(&model.RefreshToken{}).
		Where("family_id = ? AND revoked_at IS NULL", family).Update("revoked_at", t.now()).Error
}

func (t *Tokens) RevokeAllForUser(ctx context.Context, tx *gorm.DB, userID uuid.UUID) error {
	if tx == nil {
		tx = t.DB.WithContext(ctx)
	}
	return tx.Model(&model.RefreshToken{}).Where("user_id = ? AND revoked_at IS NULL", userID).Update("revoked_at", t.now()).Error
}

func (t *Tokens) FamilyActive(ctx context.Context, family uuid.UUID) (bool, error) {
	var n int64
	err := t.DB.WithContext(ctx).Model(&model.RefreshToken{}).
		Where("family_id = ? AND revoked_at IS NULL AND expires_at > ?", family, t.now()).Count(&n).Error
	return n > 0, err
}

func hashToken(raw string) []byte { return secure.SHA256([]byte(raw)) }
