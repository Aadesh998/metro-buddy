package identity

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"metro-buddy/server/internal/bloom"
	"metro-buddy/server/internal/model"
)

// AliasIndex keeps a Bloom filter of every alias in use (per city) and every retired alias,
// so suggestions skip names that are probably taken without a database round trip.
type AliasIndex struct {
	Filter *bloom.Filter
}

func cityKey(city uuid.UUID, alias string) string { return city.String() + ":" + AliasKey(alias) }
func retiredKey(alias string) string              { return "retired:" + AliasKey(alias) }

func (x *AliasIndex) mightBeTaken(ctx context.Context, city uuid.UUID, alias string) bool {
	if x == nil || x.Filter == nil {
		return false
	}
	for _, k := range []string{cityKey(city, alias), retiredKey(alias)} {
		if ok, err := x.Filter.MightContain(ctx, k); err == nil && ok {
			return true
		}
	}
	return false
}

func (x *AliasIndex) add(ctx context.Context, keys ...string) {
	if x != nil && x.Filter != nil {
		_ = x.Filter.Add(ctx, keys...)
	}
}

// Warm builds the filter from the database the first time a deployment starts.
func (x *AliasIndex) Warm(ctx context.Context, db *gorm.DB) error {
	if x == nil || x.Filter == nil || x.Filter.Ready(ctx) {
		return nil
	}
	return x.Filter.Rebuild(ctx, func(yield func([]string) error) error {
		var rows []struct {
			CityID   uuid.UUID
			AliasKey string
		}
		err := db.WithContext(ctx).Unscoped().Model(&model.Persona{}).Select("city_id, alias_key").
			FindInBatches(&rows, 1000, func(_ *gorm.DB, _ int) error {
				keys := make([]string, len(rows))
				for i, r := range rows {
					keys[i] = r.CityID.String() + ":" + r.AliasKey
				}
				return yield(keys)
			}).Error
		if err != nil {
			return err
		}
		var retired []string
		if err := db.WithContext(ctx).Model(&model.PersonaAliasHistory{}).Pluck("alias", &retired).Error; err != nil {
			return err
		}
		keys := make([]string, len(retired))
		for i, a := range retired {
			keys[i] = retiredKey(a)
		}
		return yield(keys)
	})
}
