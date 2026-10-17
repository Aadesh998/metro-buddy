package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(url string) (*gorm.DB, error) {
	var dial gorm.Dialector
	switch {
	case strings.HasPrefix(url, "postgres://"), strings.HasPrefix(url, "postgresql://"):
		dial = postgres.Open(url)
	case url == "":
		dial = sqlite.Open("file:metro-buddy.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	case strings.HasPrefix(url, "file:"), url == ":memory:":
		dial = sqlite.Open(url)
	default:
		return nil, fmt.Errorf("unsupported DATABASE_URL scheme")
	}

	gdb, err := gorm.Open(dial, &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		TranslateError:         true,
		SkipDefaultTransaction: true,
		NowFunc:                func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	if IsPostgres(gdb) {
		sqlDB.SetMaxOpenConns(20)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(30 * time.Minute)
	} else {
		// SQLite allows one writer; a single connection avoids "database is locked".
		sqlDB.SetMaxOpenConns(1)
	}
	return gdb, nil
}

func IsPostgres(gdb *gorm.DB) bool {
	return gdb.Dialector.Name() == "postgres"
}
