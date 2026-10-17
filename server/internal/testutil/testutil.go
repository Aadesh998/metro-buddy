package testutil

import (
	"fmt"
	"net/url"
	"os"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"metro-buddy/server/internal/db"
)

var dbSeq atomic.Int64

// DB returns a migrated database for one test: in-memory SQLite by default, or a throwaway
// Postgres database when TEST_POSTGRES_URL points at a server (catches Postgres-only errors).
func DB(t testing.TB) *gorm.DB {
	t.Helper()
	if admin := os.Getenv("TEST_POSTGRES_URL"); admin != "" {
		return pgDB(t, admin)
	}
	url := fmt.Sprintf("file:test%d?mode=memory&cache=shared&_pragma=foreign_keys(1)", dbSeq.Add(1))
	gdb, err := db.Open(url)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(gdb); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if s, err := gdb.DB(); err == nil {
			s.Close()
		}
	})
	return gdb
}

func Redis(t testing.TB) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return rdb, mr
}

func pgDB(t testing.TB, admin string) *gorm.DB {
	t.Helper()
	root, err := db.Open(admin)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("mb_test_%d_%d", os.Getpid(), dbSeq.Add(1))
	if err := root.Exec("CREATE DATABASE " + name).Error; err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(admin)
	u.Path = "/" + name
	gdb, err := db.Open(u.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(gdb); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if s, err := gdb.DB(); err == nil {
			s.Close()
		}
		root.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
		if s, err := root.DB(); err == nil {
			s.Close()
		}
	})
	return gdb
}
