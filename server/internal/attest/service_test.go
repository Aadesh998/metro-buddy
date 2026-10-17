package attest_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/model"
)

type fakeAndroid struct{ ok bool }

func (f fakeAndroid) Verify(context.Context, string, string) error {
	if f.ok {
		return nil
	}
	return errors.New("device integrity not met")
}

func attestOnce(env *apitest.Env, u apitest.User) *bool {
	env.Do(http.MethodPost, "/v1/me/attest/challenge", u.AccessToken, nil).Want(http.StatusOK)
	var r struct{ OK *bool }
	env.Do(http.MethodPost, "/v1/me/attest", u.AccessToken, map[string]any{"platform": "android", "token": "x"}).Want(http.StatusOK).JSON(&r)
	return r.OK
}

func TestFailedAttestationTightensLimitsWithoutBlocking(t *testing.T) {
	env := apitest.New(t)
	u := env.NewUser("9000000001", env.City("DEL"))
	if attestOnce(env, u) != nil {
		t.Fatal("unconfigured verifier must not judge")
	}
	env.Do(http.MethodPost, "/v1/me/attest", u.AccessToken, map[string]any{"platform": "android", "token": "x"}).Want(http.StatusUnprocessableEntity)

	env.App.Attest.Android = fakeAndroid{false}
	if ok := attestOnce(env, u); ok == nil || *ok {
		t.Fatal("expected failed verdict")
	}
	var d model.Device
	env.App.DB.First(&d)
	if d.AttestationOK == nil || *d.AttestationOK {
		t.Fatalf("device not marked: %+v", d)
	}
	// Not blocked, but the 30/hour safety_write budget halves to 15.
	for range 15 {
		env.Do(http.MethodPost, "/v1/blocks", u.AccessToken, map[string]any{"persona_id": model.NewID()}).Want(http.StatusNotFound)
	}
	env.Do(http.MethodPost, "/v1/blocks", u.AccessToken, map[string]any{"persona_id": model.NewID()}).Want(http.StatusTooManyRequests)
	env.Do(http.MethodGet, "/v1/me", u.AccessToken, nil).Want(http.StatusOK)

	env.App.Attest.Android = fakeAndroid{true}
	if ok := attestOnce(env, u); ok == nil || !*ok {
		t.Fatal("expected pass")
	}
}
