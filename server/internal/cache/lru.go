// Package cache is a small, bounded in-process LRU with per-entry expiry, plus a Redis
// pub/sub bus so every API instance drops stale entries when the underlying rows change.
package cache

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"
)

type entry[K comparable, V any] struct {
	key     K
	val     V
	expires time.Time
}

// LRU evicts the least recently used entry once it holds Cap items. Safe for concurrent use.
type LRU[K comparable, V any] struct {
	mu    sync.Mutex
	cap   int
	ttl   time.Duration
	ll    *list.List
	items map[K]*list.Element
	now   func() time.Time

	hits, misses, evictions atomic.Int64
}

func New[K comparable, V any](capacity int, ttl time.Duration) *LRU[K, V] {
	return &LRU[K, V]{cap: max(1, capacity), ttl: ttl, ll: list.New(), items: make(map[K]*list.Element, capacity), now: time.Now}
}

func (c *LRU[K, V]) Get(k K) (V, bool) {
	var zero V
	if c == nil {
		return zero, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[k]
	if !ok {
		c.misses.Add(1)
		return zero, false
	}
	e := el.Value.(*entry[K, V])
	if c.ttl > 0 && c.now().After(e.expires) {
		c.ll.Remove(el)
		delete(c.items, k)
		c.misses.Add(1)
		return zero, false
	}
	c.ll.MoveToFront(el)
	c.hits.Add(1)
	return e.val, true
}

func (c *LRU[K, V]) Set(k K, v V) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	exp := c.now().Add(c.ttl)
	if el, ok := c.items[k]; ok {
		e := el.Value.(*entry[K, V])
		e.val, e.expires = v, exp
		c.ll.MoveToFront(el)
		return
	}
	c.items[k] = c.ll.PushFront(&entry[K, V]{key: k, val: v, expires: exp})
	for c.ll.Len() > c.cap {
		last := c.ll.Back()
		c.ll.Remove(last)
		delete(c.items, last.Value.(*entry[K, V]).key)
		c.evictions.Add(1)
	}
}

func (c *LRU[K, V]) Delete(keys ...K) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range keys {
		if el, ok := c.items[k]; ok {
			c.ll.Remove(el)
			delete(c.items, k)
		}
	}
}

func (c *LRU[K, V]) Purge() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ll.Init()
	clear(c.items)
}

type Stats struct {
	Size      int   `json:"size"`
	Capacity  int   `json:"capacity"`
	Hits      int64 `json:"hits"`
	Misses    int64 `json:"misses"`
	Evictions int64 `json:"evictions"`
}

func (c *LRU[K, V]) Stats() Stats {
	if c == nil {
		return Stats{}
	}
	c.mu.Lock()
	n := c.ll.Len()
	c.mu.Unlock()
	return Stats{Size: n, Capacity: c.cap, Hits: c.hits.Load(), Misses: c.misses.Load(), Evictions: c.evictions.Load()}
}
