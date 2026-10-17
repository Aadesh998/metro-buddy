package identity_test

import (
	"net/http"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/identity"
)

func TestVibesEnforceThreeToTen(t *testing.T) {
	env := apitest.New(t)
	if err := identity.SeedVibes(env.App.DB); err != nil {
		t.Fatal(err)
	}
	u := env.NewUser("9876543210", env.City("DEL"))
	set := func(slugs ...string) *apitest.Resp {
		return env.Do(http.MethodPut, "/v1/me/vibes", u.AccessToken, map[string]any{"slugs": slugs})
	}
	set("chai", "cricket").Want(http.StatusUnprocessableEntity)
	set("chai", "cricket", "books", "gym", "yoga", "anime", "tech", "upsc", "coffee", "gaming", "memes").Want(http.StatusUnprocessableEntity)
	set("chai", "cricket", "not-a-vibe").Want(http.StatusUnprocessableEntity)
	set("chai", "chai", "books").Want(http.StatusUnprocessableEntity)
	set("chai", "cricket", "books").Want(http.StatusOK)
	set("indie-music", "gym", "yoga", "anime").Want(http.StatusOK)

	var got struct{ Items []identity.VibeDTO }
	env.Do(http.MethodGet, "/v1/me/vibes", u.AccessToken, nil).Want(http.StatusOK).JSON(&got)
	if len(got.Items) != 4 {
		t.Fatalf("%+v", got)
	}
	var cat struct{ Items []identity.VibeDTO }
	env.Do(http.MethodGet, "/v1/vibes", u.AccessToken, nil).Want(http.StatusOK).JSON(&cat)
	if len(cat.Items) < 40 || cat.Items[0].LabelHI == "" {
		t.Fatalf("catalog %d", len(cat.Items))
	}
	env.Do(http.MethodPut, "/v1/me/intents", u.AccessToken, map[string]any{"intents": []string{"vent", "dating"}}).Want(http.StatusUnprocessableEntity)
	env.Do(http.MethodPut, "/v1/me/intents", u.AccessToken, map[string]any{"intents": []string{"vent", "travel_buddy"}}).Want(http.StatusOK)
}
