package identity_test

import (
	"net/http"
	"regexp"
	"testing"
	"time"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/bloom"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
)

func TestAliasFilterLearnsNewAndRetiredAliasesAndSuggestionsAreClean(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	u := env.NewUser("9000000001", city)
	f := env.App.Aliases.Filter
	if err := env.App.Aliases.Warm(t.Context(), env.App.DB); err != nil {
		t.Fatal(err)
	}
	var p model.Persona
	env.App.DB.First(&p, "id = ?", u.PersonaID)
	if ok, _ := f.MightContain(t.Context(), city.String()+":"+p.AliasKey); !ok {
		t.Fatal("new alias missing from filter")
	}

	var sug struct{ Suggestions []struct{ Alias string } }
	env.Do(http.MethodPost, "/v1/me/persona/suggestions", u.AccessToken, map[string]any{"city_id": city}).Want(http.StatusOK).JSON(&sug)
	clean := regexp.MustCompile(`^[A-Z][a-z]+ [A-Z][a-z]+$`)
	for _, s := range sug.Suggestions {
		if !clean.MatchString(s.Alias) {
			t.Fatalf("expected a plain two-word alias first, got %q", s.Alias)
		}
	}

	env.App.DB.Model(&model.Persona{}).Where("id = ?", u.PersonaID).Update("alias_changed_at", time.Now().UTC().AddDate(0, -2, 0))
	old := p.Alias
	env.Do(http.MethodPut, "/v1/me/persona/alias", u.AccessToken, map[string]any{"alias": sug.Suggestions[0].Alias}).Want(http.StatusOK)
	if ok, _ := f.MightContain(t.Context(), "retired:"+identityKey(old)); !ok {
		t.Fatal("retired alias missing from filter")
	}
	// A filter rebuilt from scratch knows both names too.
	g := &identity.AliasIndex{Filter: bloom.New(env.App.Redis, "bf:rebuilt", 1000, 0.01)}
	if err := g.Warm(t.Context(), env.App.DB); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"retired:" + identityKey(old), city.String() + ":" + identityKey(sug.Suggestions[0].Alias)} {
		if ok, _ := g.Filter.MightContain(t.Context(), k); !ok {
			t.Fatalf("rebuilt filter missing %s", k)
		}
	}
}

func identityKey(a string) string {
	out := []rune{}
	for _, r := range a {
		if r != ' ' {
			if r >= 'A' && r <= 'Z' {
				r += 'a' - 'A'
			}
			out = append(out, r)
		}
	}
	return string(out)
}
