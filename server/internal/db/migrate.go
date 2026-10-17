package db

import (
	"embed"
	"io/fs"
	"sort"
	"time"

	"gorm.io/gorm"

	"metro-buddy/server/internal/model"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type schemaMigration struct {
	Name      string `gorm:"primaryKey;size:128"`
	AppliedAt time.Time
}

// AutoMigrate covers additive changes; hand-written SQL in migrations/ covers
// Postgres-only features and anything destructive.
func Migrate(gdb *gorm.DB) error {
	if err := gdb.AutoMigrate(append([]any{&schemaMigration{}}, model.All()...)...); err != nil {
		return err
	}
	if !IsPostgres(gdb) {
		return nil
	}
	names, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, n := range names {
		var count int64
		if err := gdb.Model(&schemaMigration{}).Where("name = ?", n).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		sqlBytes, err := migrationsFS.ReadFile(n)
		if err != nil {
			return err
		}
		if err := gdb.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(string(sqlBytes)).Error; err != nil {
				return err
			}
			return tx.Create(&schemaMigration{Name: n, AppliedAt: time.Now().UTC()}).Error
		}); err != nil {
			return err
		}
	}
	return nil
}
