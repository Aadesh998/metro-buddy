package identity_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/cache"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
)

func TestRefCacheServesHitsAndAnyPersonaWriteInvalidates(t *testing.T) {
	env := apitest.New(t)
	u := env.NewUser("9000000001", env.City("DEL"))
	bus := &cache.Bus{}
	t.Cleanup(identity.DisableRefCache)
	if err := identity.EnableRefCache(env.App.DB, bus, 100, time.Minute); err != nil {
		t.Fatal(err)
	}
	db := env.App.DB
	first, _ := identity.LoadRefs(db, []uuid.UUID{u.PersonaID})
	_ = first
	before := identity.RefCacheStats()
	identity.LoadRefs(db, []uuid.UUID{u.PersonaID})
	if identity.RefCacheStats().Hits != before.Hits+1 {
		t.Fatal("second lookup should be a cache hit")
	}
	db.Model(&model.Persona{}).Where("id = ?", u.PersonaID).Update("is_host", true)
	refs, _ := identity.LoadRefs(db, []uuid.UUID{u.PersonaID})
	if !refs[u.PersonaID].IsHost {
		t.Fatal("stale ref served after a persona write")
	}
	db.Model(&model.User{}).Where("id = (SELECT user_id FROM personas WHERE id = ?)", u.PersonaID).Update("verified_at", time.Now().UTC())
	refs, _ = identity.LoadRefs(db, []uuid.UUID{u.PersonaID})
	if !refs[u.PersonaID].Verified {
		t.Fatal("stale ref served after verification")
	}
}
