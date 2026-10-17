package seed

import (
	"gorm.io/gorm"

	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/metro"
)

// All loads reference data; every step is idempotent and safe on each boot.
func All(db *gorm.DB) error {
	if err := metro.SeedAll(db); err != nil {
		return err
	}
	return identity.SeedVibes(db)
}
