package verify_test

import (
	"net/http"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/model"
)

func TestLivenessSetsOnlyTheVerifiedFlag(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	u, other := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	var s struct {
		SessionID string `json:"session_id"`
	}
	env.Do(http.MethodPost, "/v1/me/verification", u.AccessToken, nil).Want(http.StatusCreated).JSON(&s)
	env.Do(http.MethodPost, "/v1/me/verification/check", other.AccessToken, map[string]any{"session_id": s.SessionID}).Want(http.StatusUnprocessableEntity)
	var r struct{ Status string }
	env.Do(http.MethodPost, "/v1/me/verification/check", u.AccessToken, map[string]any{"session_id": s.SessionID}).Want(http.StatusOK).JSON(&r)
	if r.Status != "passed" {
		t.Fatalf("%+v", r)
	}
	env.Do(http.MethodPost, "/v1/me/verification/check", u.AccessToken, map[string]any{"session_id": s.SessionID}).Want(http.StatusUnprocessableEntity)
	var me struct{ Verified bool }
	env.Do(http.MethodGet, "/v1/me", u.AccessToken, nil).Want(http.StatusOK).JSON(&me)
	if !me.Verified {
		t.Fatal("not verified")
	}
	var user model.User
	env.App.DB.First(&user, "id = (SELECT user_id FROM personas WHERE id = ?)", u.PersonaID)
	if user.TrustScore != 60 {
		t.Fatalf("verified bonus missing: %d", user.TrustScore)
	}
	env.Do(http.MethodPost, "/v1/me/verification", u.AccessToken, nil).Want(http.StatusConflict)
}
