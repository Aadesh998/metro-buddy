// Package bloom is a Bloom filter stored as a Redis bitmap, so every API instance shares one
// filter and it survives restarts. It answers "definitely not present" or "maybe present";
// callers still treat the database as the source of truth.
package bloom

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"

	"github.com/redis/go-redis/v9"
)

type Filter struct {
	rdb  *redis.Client
	key  string
	bits uint64
	k    int
}

// New sizes the filter for n expected items at false-positive rate p.
func New(rdb *redis.Client, key string, n int, p float64) *Filter {
	m := math.Ceil(-float64(n) * math.Log(p) / (math.Ln2 * math.Ln2))
	k := int(math.Round(m / float64(n) * math.Ln2))
	return &Filter{rdb: rdb, key: key, bits: uint64(m), k: max(1, k)}
}

// positions uses double hashing (Kirsch–Mitzenmacher) over one SHA-256.
func (f *Filter) positions(item string) []int64 {
	sum := sha256.Sum256([]byte(item))
	h1 := binary.BigEndian.Uint64(sum[0:8])
	h2 := binary.BigEndian.Uint64(sum[8:16]) | 1
	out := make([]int64, f.k)
	for i := range out {
		out[i] = int64((h1 + uint64(i)*h2) % f.bits)
	}
	return out
}

func (f *Filter) Add(ctx context.Context, items ...string) error {
	if len(items) == 0 {
		return nil
	}
	pipe := f.rdb.Pipeline()
	for _, it := range items {
		for _, pos := range f.positions(it) {
			pipe.SetBit(ctx, f.key, pos, 1)
		}
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (f *Filter) MightContain(ctx context.Context, item string) (bool, error) {
	pipe := f.rdb.Pipeline()
	cmds := make([]*redis.IntCmd, 0, f.k)
	for _, pos := range f.positions(item) {
		cmds = append(cmds, pipe.GetBit(ctx, f.key, pos))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	for _, c := range cmds {
		if c.Val() == 0 {
			return false, nil
		}
	}
	return true, nil
}

// Ready reports whether the filter has been built; Rebuild marks it once loaded.
func (f *Filter) Ready(ctx context.Context) bool {
	n, err := f.rdb.Exists(ctx, f.key+":ready").Result()
	return err == nil && n == 1
}

// Rebuild loads every item from load (called in batches) and marks the filter ready.
func (f *Filter) Rebuild(ctx context.Context, load func(yield func(items []string) error) error) error {
	if err := f.rdb.Del(ctx, f.key).Err(); err != nil {
		return err
	}
	if err := load(func(items []string) error { return f.Add(ctx, items...) }); err != nil {
		return err
	}
	return f.rdb.Set(ctx, f.key+":ready", 1, 0).Err()
}
