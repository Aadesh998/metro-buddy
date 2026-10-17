package auth_test

import (
	"fmt"
	"net/http"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/auth"
	"metro-buddy/server/internal/model"
)

func TestNormalizePhone(t *testing.T) {
	ok := map[string]string{
		"9876543210": "+919876543210", "+91 98765 43210": "+919876543210",
		"098765-43210": "+919876543210", "919876543210": "+919876543210",
	}
	for in, want := range ok {
		if got, err := auth.NormalizePhone(in); err != nil || got != want {
			t.Errorf("NormalizePhone(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"12345", "5876543210", "+1 415 555 0100", ""} {
		if _, err := auth.NormalizePhone(in); err == nil {
			t.Errorf("NormalizePhone(%q) accepted", in)
		}
	}
}

func TestOTPLoginCreatesUserWithHashedAndEncryptedPhone(t *testing.T) {
	env := apitest.New(t)
	s := env.Login("9876543210")
	if s.AccessToken == "" || s.RefreshToken == "" {
		t.Fatal("missing tokens")
	}
	var u model.User
	env.App.DB.First(&u)
	if string(u.PhoneEnc) == "+919876543210" || len(u.PhoneHash) != 32 {
		t.Fatal("phone stored in plaintext")
	}
	if got, _ := env.App.Box.OpenString(u.PhoneEnc); got != "+919876543210" {
		t.Fatalf("phone_enc decrypts to %q", got)
	}
	var n int64
	env.App.DB.Model(&model.User{}).Count(&n)
	env.Login("+91 98765 43210")
	env.App.DB.Model(&model.User{}).Count(&n)
	if n != 1 {
		t.Fatalf("second login created another user: %d", n)
	}
}

func TestOTPLocksAfterFiveWrongAttempts(t *testing.T) {
	env := apitest.New(t)
	env.Do(http.MethodPost, "/v1/auth/otp/request", "", map[string]string{"phone": "9876543210"}).Want(http.StatusAccepted)
	verify := func(code string) *apitest.Resp {
		return env.Do(http.MethodPost, "/v1/auth/otp/verify", "", map[string]any{
			"phone": "9876543210", "code": code, "device": map[string]string{"platform": "ios"},
		})
	}
	wrong := "000000"
	if env.SMS.Code("+919876543210") == wrong {
		wrong = "111111"
	}
	for i := 1; i <= 4; i++ {
		if r := verify(wrong); r.ErrCode() != "otp_invalid" {
			t.Fatalf("attempt %d: %s", i, r.Body)
		}
	}
	if r := verify(wrong); r.ErrCode() != "otp_locked" {
		t.Fatalf("5th attempt: %s", r.Body)
	}
	if r := verify(env.SMS.Code("+919876543210")); r.ErrCode() != "otp_expired" {
		t.Fatalf("correct code accepted after lockout: %s", r.Body)
	}
	var fails int64
	env.App.DB.Model(&model.SecurityEvent{}).Where("kind = ?", model.SecOTPFail).Count(&fails)
	if fails != 5 {
		t.Fatalf("security events = %d, want 5", fails)
	}
}

func TestOTPExpiresAfterFiveMinutes(t *testing.T) {
	env := apitest.New(t)
	env.Do(http.MethodPost, "/v1/auth/otp/request", "", map[string]string{"phone": "9876543210"}).Want(http.StatusAccepted)
	env.Redis.FastForward(auth.OTPTTL + 1)
	r := env.Do(http.MethodPost, "/v1/auth/otp/verify", "", map[string]any{
		"phone": "9876543210", "code": env.SMS.Code("+919876543210"), "device": map[string]string{"platform": "ios"},
	})
	if r.ErrCode() != "otp_expired" {
		t.Fatalf("got %s", r.Body)
	}
}

func TestOTPThrottlesPerNumberAndPerIP(t *testing.T) {
	env := apitest.New(t)
	req := func(phone, ip string) *apitest.Resp {
		return env.Do(http.MethodPost, "/v1/auth/otp/request", "", map[string]string{"phone": phone}, "X-Forwarded-For", ip)
	}
	for i := range 3 {
		req("9876543210", fmt.Sprintf("198.51.100.%d", i+1)).Want(http.StatusAccepted)
	}
	if r := req("9876543210", "198.51.100.9"); r.ErrCode() != "otp_throttled" {
		t.Fatalf("4th request for same number: %s", r.Body)
	}
	for i := range 10 {
		req(fmt.Sprintf("90000000%02d", i), "192.0.2.1").Want(http.StatusAccepted)
	}
	if r := req("9111111111", "192.0.2.1"); r.ErrCode() != "otp_throttled" {
		t.Fatalf("11th request from same IP: %s", r.Body)
	}
}

func TestRefreshRotatesAndReuseRevokesFamily(t *testing.T) {
	env := apitest.New(t)
	s1 := env.Login("9876543210")

	var s2 apitest.Session
	env.Do(http.MethodPost, "/v1/auth/refresh", "", map[string]string{"refresh_token": s1.RefreshToken}).Want(http.StatusOK).JSON(&s2)
	if s2.RefreshToken == s1.RefreshToken {
		t.Fatal("refresh token not rotated")
	}
	var s3 apitest.Session
	env.Do(http.MethodPost, "/v1/auth/refresh", "", map[string]string{"refresh_token": s2.RefreshToken}).Want(http.StatusOK).JSON(&s3)

	// Attacker replays the first, already-rotated token.
	env.Do(http.MethodPost, "/v1/auth/refresh", "", map[string]string{"refresh_token": s1.RefreshToken}).Want(http.StatusUnauthorized)

	// The legitimate latest token is now dead too, and so is its access token.
	env.Do(http.MethodPost, "/v1/auth/refresh", "", map[string]string{"refresh_token": s3.RefreshToken}).Want(http.StatusUnauthorized)
	env.Do(http.MethodGet, "/v1/me", s3.AccessToken, nil).Want(http.StatusUnauthorized)

	var n int64
	env.App.DB.Model(&model.SecurityEvent{}).Where("kind = ?", model.SecRefreshReuse).Count(&n)
	if n == 0 {
		t.Fatal("no refresh_reuse security event")
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	env := apitest.New(t)
	s := env.Login("9876543210")
	other := env.Login("9876543210")
	env.Do(http.MethodPost, "/v1/auth/logout", "", map[string]string{"refresh_token": s.RefreshToken}).Want(http.StatusNoContent)
	env.Do(http.MethodPost, "/v1/auth/refresh", "", map[string]string{"refresh_token": s.RefreshToken}).Want(http.StatusUnauthorized)
	env.Do(http.MethodPost, "/v1/auth/refresh", "", map[string]string{"refresh_token": other.RefreshToken}).Want(http.StatusOK)
}

func TestProtectedRouteRejectsBadTokens(t *testing.T) {
	env := apitest.New(t)
	for _, tok := range []string{"", "garbage", "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ4In0."} {
		env.Do(http.MethodGet, "/v1/me", tok, nil).Want(http.StatusUnauthorized)
	}
}
