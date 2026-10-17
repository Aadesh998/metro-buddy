package bloom

import (
	"context"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestNoFalseNegativesAndLowFalsePositives(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	f := New(rdb, "bf:test", 2000, 0.01)
	var items []string
	for i := range 2000 {
		items = append(items, fmt.Sprintf("del:alias%d", i))
	}
	if err := f.Add(ctx, items...); err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if ok, _ := f.MightContain(ctx, it); !ok {
			t.Fatalf("false negative for %s", it)
		}
	}
	fp := 0
	for i := range 2000 {
		if ok, _ := f.MightContain(ctx, fmt.Sprintf("del:other%d", i)); ok {
			fp++
		}
	}
	if fp > 60 {
		t.Fatalf("false positive rate too high: %d/2000", fp)
	}
}
