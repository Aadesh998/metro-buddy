package testutil

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"metro-buddy/server/internal/db"
)

var dbSeq atomic.Int64

func DB(t testing.TB) *gorm.DB {
	t.Helper()
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
