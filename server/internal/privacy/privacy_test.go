package privacy_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/bowls"
	"metro-buddy/server/internal/media"
	"metro-buddy/server/internal/model"
)

func TestExportContainsMyDataButNoSecrets(t *testing.T) {
	env := apitest.New(t)
	city := env.Seeded()
	u := env.NewUser("9876543210", city)
	env.Do(http.MethodPost, "/v1/me/export", u.AccessToken, nil).Want(http.StatusOK)
	if err := env.App.Privacy.RunExports(t.Context()); err != nil {
		t.Fatal(err)
	}
	var st struct {
		Export struct{ Status, URL string }
	}
	env.Do(http.MethodGet, "/v1/me/export", u.AccessToken, nil).Want(http.StatusOK).JSON(&st)
	if st.Export.Status != "ready" || st.Export.URL == "" {
		t.Fatalf("%+v", st)
	}
	mem := env.App.Media.Store.(*media.Memory)
	var dump string
	for k, b := range mem.Objects {
		if strings.HasPrefix(k, "exports/") {
			dump = string(b)
		}
	}
	for _, want := range []string{"+919876543210", u.Alias, "core_service"} {
		if !strings.Contains(dump, want) {
			t.Errorf("export missing %q", want)
		}
	}
	for _, leak := range []string{"PhoneHash", "PhoneEnc", "TokenHash"} {
		if strings.Contains(dump, leak) {
			t.Errorf("export leaks %s", leak)
		}
	}
}

func TestDeletionTombstonesPersonaAllowsRestoreThenPurges(t *testing.T) {
	env := apitest.New(t)
	city := env.Seeded()
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	var list struct{ Items []bowls.BowlDTO }
	env.Do(http.MethodGet, "/v1/bowls?kind=topic", a.AccessToken, nil).Want(http.StatusOK).JSON(&list)
	bowl := list.Items[0].ID.String()
	env.Do(http.MethodPost, "/v1/bowls/"+bowl+"/join", a.AccessToken, nil).Want(http.StatusOK)
	var p bowls.PostDTO
	env.Do(http.MethodPost, "/v1/posts", a.AccessToken, map[string]any{"bowl_id": bowl, "type": "text", "title": "bye"}).Want(http.StatusCreated).JSON(&p)

	env.Do(http.MethodDelete, "/v1/me", a.AccessToken, nil).Want(http.StatusAccepted)
	env.Do(http.MethodGet, "/v1/me", a.AccessToken, nil).Want(http.StatusUnauthorized)
	env.Do(http.MethodGet, "/v1/posts/"+p.ID.String(), b.AccessToken, nil).Want(http.StatusOK).JSON(&p)
	if p.Author.Alias != "[deleted]" {
		t.Fatalf("author shows %q", p.Author.Alias)
	}

	again := env.Login("9000000001")
	var me struct {
		HasPersona          bool       `json:"has_persona"`
		DeletionScheduledAt *time.Time `json:"deletion_scheduled_at"`
	}
	env.Do(http.MethodGet, "/v1/me", again.AccessToken, nil).Want(http.StatusOK).JSON(&me)
	if me.HasPersona || me.DeletionScheduledAt == nil {
		t.Fatalf("%+v", me)
	}
	env.Do(http.MethodPost, "/v1/me/restore", again.AccessToken, nil).Want(http.StatusNoContent)
	env.Do(http.MethodGet, "/v1/posts/"+p.ID.String(), b.AccessToken, nil).Want(http.StatusOK).JSON(&p)
	if p.Author.Alias != a.Alias {
		t.Fatalf("restore didn't bring the persona back: %q", p.Author.Alias)
	}

	env.Do(http.MethodDelete, "/v1/me", again.AccessToken, nil).Want(http.StatusAccepted)
	env.App.DB.Model(&model.User{}).Where("id IN (SELECT user_id FROM personas WHERE id = ?)", a.PersonaID).
		Update("deletion_requested_at", time.Now().UTC().Add(-31*24*time.Hour))
	if err := env.App.Privacy.PurgeDeleted(t.Context()); err != nil {
		t.Fatal(err)
	}
	var u model.User
	env.App.DB.Where("id IN (SELECT user_id FROM personas WHERE id = ?)", a.PersonaID).First(&u)
	if u.Status != model.UserDeleted || len(u.PhoneEnc) != 0 || u.BirthYear != nil {
		t.Fatalf("not purged: %+v", u)
	}
	fresh := env.Login("9000000001")
	env.Do(http.MethodGet, "/v1/me", fresh.AccessToken, nil).Want(http.StatusOK).JSON(&me)
	if me.HasPersona || me.DeletionScheduledAt != nil {
		t.Fatal("same number should start a brand-new account after purge")
	}
}
