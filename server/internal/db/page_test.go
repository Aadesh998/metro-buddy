package db_test

import (
	"testing"
	"time"

	"metro-buddy/server/internal/db"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/testutil"
)

type widget struct {
	model.Base
	N int
}

func TestPageDescWalksAllRowsWithoutGapsOrDuplicates(t *testing.T) {
	gdb := testutil.DB(t)
	if err := gdb.AutoMigrate(&widget{}); err != nil {
		t.Fatal(err)
	}
	same := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 25 {
		w := widget{N: i}
		// Several rows share a timestamp so the id tie-breaker is exercised.
		w.CreatedAt = same.Add(time.Duration(i/3) * time.Millisecond)
		if err := gdb.Create(&w).Error; err != nil {
			t.Fatal(err)
		}
	}

	seen := map[int]bool{}
	cursor, pages := "", 0
	for {
		q, limit, err := db.PageDesc(gdb.Model(&widget{}), "", cursor, 7)
		if err != nil {
			t.Fatal(err)
		}
		var rows []widget
		if err := q.Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		rows, next := db.Trim(rows, limit, func(w widget) db.Cursor { return db.Cursor{CreatedAt: w.CreatedAt, ID: w.ID} })
		pages++
		for i, r := range rows {
			if seen[r.N] {
				t.Fatalf("duplicate row %d", r.N)
			}
			seen[r.N] = true
			if i > 0 && r.CreatedAt.After(rows[i-1].CreatedAt) {
				t.Fatalf("rows not in descending order")
			}
		}
		if next == "" {
			break
		}
		cursor = next
	}
	if len(seen) != 25 || pages != 4 {
		t.Fatalf("saw %d rows over %d pages, want 25 over 4", len(seen), pages)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	for _, s := range []string{"%%%", "bm90LWEtY3Vyc29y", db.Cursor{}.Encode()[:4]} {
		if _, err := db.DecodeCursor(s); err == nil {
			t.Errorf("DecodeCursor(%q) succeeded", s)
		}
	}
}

func TestClampLimit(t *testing.T) {
	cases := map[int]int{0: db.DefaultLimit, -5: db.DefaultLimit, 10: 10, 1000: db.MaxLimit}
	for in, want := range cases {
		if got := db.ClampLimit(in); got != want {
			t.Errorf("ClampLimit(%d) = %d, want %d", in, got, want)
		}
	}
}
