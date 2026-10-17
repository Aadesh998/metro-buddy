package identity

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"metro-buddy/server/internal/cache"
)

// Persona refs (alias, avatar, badges) are read on almost every request and change rarely,
// so they sit in an LRU. Any write to personas, or to a user's verification or status,
// purges it on every instance; the TTL only bounds staleness if a purge is ever missed.
var refCache *cache.LRU[uuid.UUID, PersonaRef]

const refCacheName = "persona_refs"

func RefCacheStats() cache.Stats { return refCache.Stats() }

// EnableRefCache turns the cache on and hooks the database so writes invalidate it.
func EnableRefCache(db *gorm.DB, bus *cache.Bus, capacity int, ttl time.Duration) error {
	refCache = cache.New[uuid.UUID, PersonaRef](capacity, ttl)
	bus.On(refCacheName, refCache.Purge)
	purge := func(tx *gorm.DB) {
		if tx.Error != nil || tx.Statement == nil {
			return
		}
		switch tx.Statement.Table {
		case "personas":
		case "users":
			if !touchesUserRef(tx.Statement) {
				return
			}
		default:
			return
		}
		ctx := tx.Statement.Context
		if ctx == nil {
			ctx = context.Background()
		}
		bus.Purge(ctx, refCacheName)
	}
	cb := db.Callback()
	if err := cb.Update().After("gorm:update").Register("refcache:update", purge); err != nil {
		return err
	}
	if err := cb.Delete().After("gorm:delete").Register("refcache:delete", purge); err != nil {
		return err
	}
	return cb.Create().After("gorm:create").Register("refcache:create", purge)
}

// Only verification and account status show up in a ref; trust-score updates and the like don't.
func touchesUserRef(st *gorm.Statement) bool {
	if st.ReflectValue.IsValid() && st.Schema == nil {
		return true
	}
	if m, ok := st.Dest.(map[string]any); ok {
		_, v := m["verified_at"]
		_, s := m["status"]
		_, d := m["deleted_at"]
		return v || s || d
	}
	return true
}

func cachedRefs(ids []uuid.UUID) (map[uuid.UUID]PersonaRef, []uuid.UUID) {
	out := make(map[uuid.UUID]PersonaRef, len(ids))
	if refCache == nil {
		return out, ids
	}
	var missing []uuid.UUID
	for _, id := range ids {
		if r, ok := refCache.Get(id); ok {
			out[id] = r
		} else {
			missing = append(missing, id)
		}
	}
	return out, missing
}

// DisableRefCache turns the cache off (tests that enable it restore this afterwards).
func DisableRefCache() { refCache = nil }
