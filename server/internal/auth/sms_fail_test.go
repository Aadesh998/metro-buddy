package auth_test

import (
	"context"
	"net/http"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/sms"
)

func TestOTPRequestReportsUnsentSMS(t *testing.T) {
	env := apitest.New(t)
	env.App.OTP.SMS = sms.MSG91{}
	r := env.Do(http.MethodPost, "/v1/auth/otp/request", "", map[string]string{"phone": "9876543210"})
	if r.Code != http.StatusServiceUnavailable || r.ErrCode() != "sms_unavailable" {
		t.Fatalf("%d %s", r.Code, r.Body)
	}
	if err := (sms.MSG91{}).SendSOS(context.Background(), "+919876543210", "a", "l"); err != sms.ErrNotConfigured {
		t.Fatal(err)
	}
}
