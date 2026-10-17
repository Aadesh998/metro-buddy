// Package trust turns TrustEvents into a 0-100 score and a tier that scales rate limits
// and unlocks features. Scores are private: no API shows them to other users.
package trust

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"metro-buddy/server/internal/model"
)

const (
	Base           = 50
	maxAgeBonus    = 15
	verifiedBonus  = 10
	maxPositive    = 25
	eventRetention = 2 * 365 * 24 * time.Hour
)

type Inputs struct {
	AgeDays   int
	Verified  bool
	Positive  int // sum of positive deltas
	Negative  int // sum of negative deltas (<= 0)
	Dismissed int // reports this user filed that moderators dismissed, last 90 days
	Upheld    int
}

func Score(in Inputs) int {
	s := Base + min(maxAgeBonus, in.AgeDays/7) + min(maxPositive, in.Positive) + in.Negative
	if in.Verified {
		s += verifiedBonus
	}
	// Serial false reporting costs trust; a few misjudged reports don't.
	if excess := in.Dismissed - in.Upheld - 3; excess > 0 {
		s -= 2 * excess
	}
	return max(0, min(100, s))
}

// Tier gates features and multiplies rate limits (1x, 2x, 4x, 8x).
func Tier(score, ageDays int, verified bool) int16 {
	switch {
	case score >= 80 && ageDays >= 30 && verified:
		return 3
	case score >= 60 && ageDays >= 14:
		return 2
	case score >= 40 && ageDays >= 3:
		return 1
	}
	return 0
}

func now() time.Time { return time.Now().UTC() }

// Record stores an event and refreshes the user's score in the same transaction.
func Record(tx *gorm.DB, userID uuid.UUID, kind model.TrustEventKind, delta int16, ref *uuid.UUID) error {
	if err := tx.Create(&model.TrustEvent{UserID: userID, Kind: kind, Delta: delta, RefID: ref}).Error; err != nil {
		return err
	}
	return Recompute(tx, userID)
}

func Recompute(tx *gorm.DB, userID uuid.UUID) error {
	var u model.User
	if err := tx.Unscoped().Select("id, created_at, verified_at").First(&u, "id = ?", userID).Error; err != nil {
		return err
	}
	t := now()
	in := Inputs{AgeDays: int(t.Sub(u.CreatedAt).Hours() / 24), Verified: u.VerifiedAt != nil}
	var sums struct{ Pos, Neg int }
	tx.Model(&model.TrustEvent{}).Where("user_id = ? AND created_at > ?", userID, t.Add(-eventRetention)).
		Select("COALESCE(SUM(CASE WHEN delta > 0 THEN delta ELSE 0 END), 0) AS pos, COALESCE(SUM(CASE WHEN delta < 0 THEN delta ELSE 0 END), 0) AS neg").Scan(&sums)
	in.Positive, in.Negative = sums.Pos, sums.Neg
	var counts []struct {
		Kind model.TrustEventKind
		N    int
	}
	tx.Model(&model.TrustEvent{}).Where("user_id = ? AND kind IN ? AND created_at > ?", userID,
		[]model.TrustEventKind{model.TrustReportDismissed, model.TrustReportUpheld}, t.AddDate(0, 0, -90)).
		Select("kind, COUNT(*) AS n").Group("kind").Scan(&counts)
	for _, c := range counts {
		if c.Kind == model.TrustReportDismissed {
			in.Dismissed = c.N
		} else {
			in.Upheld = c.N
		}
	}
	score := Score(in)
	return tx.Model(&model.User{}).Where("id = ?", userID).
		Updates(map[string]any{"trust_score": score, "trust_tier": Tier(score, in.AgeDays, in.Verified)}).Error
}

type Service struct {
	DB *gorm.DB
}

// RecomputeAll runs nightly so account-age bonuses and expiring events take effect.
func (s *Service) RecomputeAll(ctx context.Context) error {
	tx := s.DB.WithContext(ctx)
	last := uuid.Nil
	for {
		var ids []uuid.UUID
		if err := tx.Model(&model.User{}).Where("id > ? AND status IN ?", last, []model.UserStatus{model.UserActive, model.UserSuspended}).
			Order("id").Limit(500).Pluck("id", &ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			if err := Recompute(tx, id); err != nil {
				return err
			}
		}
		if len(ids) < 500 {
			return nil
		}
		last = ids[len(ids)-1]
	}
}
