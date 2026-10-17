package auth_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/auth"
	"metro-buddy/server/internal/model"
)

var dev = map[string]string{"platform": "android", "app_version": "1.0.0"}

func TestEmailSignupLoginAndLockout(t *testing.T) {
	env := apitest.New(t)
	body := map[string]any{"email": "Riya@Example.com ", "password": "metro1234", "device": dev}
	env.Do(http.MethodPost, "/v1/auth/email/signup", "", map[string]any{"email": "x@example.com", "password": "short", "device": dev}).Want(http.StatusUnprocessableEntity)
	var r struct {
		Session   auth.Session `json:"session"`
		IsNewUser bool         `json:"is_new_user"`
	}
	env.Do(http.MethodPost, "/v1/auth/email/signup", "", body).Want(http.StatusOK).JSON(&r)
	if !r.IsNewUser || r.Session.AccessToken == "" {
		t.Fatalf("%+v", r)
	}
	env.Do(http.MethodGet, "/v1/me", r.Session.AccessToken, nil).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/auth/email/signup", "", map[string]any{"email": "riya@example.com", "password": "other12345", "device": dev}).Want(http.StatusConflict)

	var u model.User
	env.App.DB.First(&u)
	if len(u.PhoneHash) != 0 || u.PasswordHash == nil || *u.PasswordHash == "metro1234" || string(u.EmailEnc) == "riya@example.com" {
		t.Fatal("email or password stored in the clear")
	}

	env.Do(http.MethodPost, "/v1/auth/email/login", "", map[string]any{"email": "RIYA@example.com", "password": "metro1234", "device": dev}).Want(http.StatusOK).JSON(&r)
	if r.IsNewUser {
		t.Fatal("login reported a new user")
	}
	env.Do(http.MethodPost, "/v1/auth/email/login", "", map[string]any{"email": "nobody@example.com", "password": "metro1234", "device": dev}).Want(http.StatusUnauthorized)
	for range 10 {
		env.Do(http.MethodPost, "/v1/auth/email/login", "", map[string]any{"email": "riya@example.com", "password": "wrongpass1", "device": dev}, "X-Forwarded-For", "203.0.113.9")
	}
	// Locked even with the right password, from any IP, until the window passes.
	env.Do(http.MethodPost, "/v1/auth/email/login", "", map[string]any{"email": "riya@example.com", "password": "metro1234", "device": dev}, "X-Forwarded-For", "203.0.113.10").Want(http.StatusTooManyRequests)
}

func TestGoogleSignInCreatesVerifiedAccountAndTakesOverUnverifiedEmail(t *testing.T) {
	env := apitest.New(t)
	h := env.App.AuthHandlers
	h.Google = func(_ context.Context, tok string) (*auth.GoogleIdentity, error) {
		switch tok {
		case "riya":
			return &auth.GoogleIdentity{Sub: "g-riya", Email: "riya@example.com"}, nil
		case "kabir":
			return &auth.GoogleIdentity{Sub: "g-kabir", Email: "kabir@example.com"}, nil
		}
		return nil, errors.New("bad")
	}
	env.Do(http.MethodPost, "/v1/auth/google", "", map[string]any{"id_token": "forged", "device": dev}).Want(http.StatusUnauthorized)

	var r struct {
		IsNewUser bool `json:"is_new_user"`
	}
	env.Do(http.MethodPost, "/v1/auth/google", "", map[string]any{"id_token": "kabir", "device": dev}).Want(http.StatusOK).JSON(&r)
	if !r.IsNewUser {
		t.Fatal("first Google sign-in should create the account")
	}
	env.Do(http.MethodPost, "/v1/auth/google", "", map[string]any{"id_token": "kabir", "device": dev}).Want(http.StatusOK).JSON(&r)
	if r.IsNewUser {
		t.Fatal("second Google sign-in should log in")
	}

	// Someone registered riya@example.com with a password first; the real owner then uses Google.
	env.Do(http.MethodPost, "/v1/auth/email/signup", "", map[string]any{"email": "riya@example.com", "password": "squatter123", "device": dev}).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/auth/google", "", map[string]any{"id_token": "riya", "device": dev}).Want(http.StatusOK).JSON(&r)
	if r.IsNewUser {
		t.Fatal("should link to the existing account")
	}
	env.Do(http.MethodPost, "/v1/auth/email/login", "", map[string]any{"email": "riya@example.com", "password": "squatter123", "device": dev}).Want(http.StatusUnauthorized)
	var n int64
	env.App.DB.Model(&model.User{}).Where("google_sub IS NOT NULL AND email_verified_at IS NOT NULL").Count(&n)
	if n != 2 {
		t.Fatalf("verified google users %d", n)
	}
}
