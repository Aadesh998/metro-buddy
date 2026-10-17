package cache

import (
	"testing"
	"time"
)

func TestLRUEvictsLeastRecentlyUsedAndExpires(t *testing.T) {
	c := New[string, int](2, time.Minute)
	now := time.Unix(0, 0)
	c.now = func() time.Time { return now }
	c.Set("a", 1)
	c.Set("b", 2)
	c.Get("a")
	c.Set("c", 3)
	if _, ok := c.Get("b"); ok {
		t.Fatal("b should have been evicted as least recently used")
	}
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatal("a should survive")
	}
	now = now.Add(2 * time.Minute)
	if _, ok := c.Get("a"); ok {
		t.Fatal("expired entry served")
	}
	s := c.Stats()
	if s.Evictions != 1 || s.Hits != 2 || s.Misses != 2 {
		t.Fatalf("%+v", s)
	}
	c.Set("x", 9)
	c.Purge()
	if _, ok := c.Get("x"); ok || c.Stats().Size != 0 {
		t.Fatal("purge")
	}
}
