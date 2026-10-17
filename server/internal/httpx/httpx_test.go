package httpx_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/logx"
)

func newEcho(logs *bytes.Buffer) *echo.Echo {
	log := logx.New(logs, slog.LevelInfo)
	e := echo.New()
	e.Validator = httpx.NewValidator()
	e.HTTPErrorHandler = httpx.ErrorHandler(log)
	e.Use(httpx.RequestIDMiddleware(), httpx.AccessLog(log), httpx.Recover(log))
	return e
}

func TestPanicReturnsGenericEnvelopeWithoutStack(t *testing.T) {
	var logs bytes.Buffer
	e := newEcho(&logs)
	e.GET("/boom", func(echo.Context) error { panic("db password=hunter2 for 9876543210") })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, leak := range []string{"hunter2", "goroutine", ".go:", "9876543210"} {
		if strings.Contains(body, leak) {
			t.Fatalf("response leaks %q: %s", leak, body)
		}
	}
	var env struct {
		Error     struct{ Code string } `json:"error"`
		RequestID string                `json:"request_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != "internal" || env.RequestID == "" {
		t.Fatalf("bad envelope: %s", body)
	}
	if strings.Contains(logs.String(), "9876543210") {
		t.Fatalf("log leaks phone number: %s", logs.String())
	}
}

func TestValidationErrorsUseJSONFieldNames(t *testing.T) {
	type req struct {
		Phone string `json:"phone_e164" validate:"required,e164"`
	}
	e := newEcho(&bytes.Buffer{})
	e.POST("/v", func(c echo.Context) error {
		_, err := httpx.Bind[req](c)
		return err
	})
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v", strings.NewReader(`{"phone_e164":"abc"}`))
	r.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(rec, r)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"phone_e164":"e164"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestLoggerRedactsDeniedKeysAndPII(t *testing.T) {
	var buf bytes.Buffer
	log := logx.New(&buf, slog.LevelInfo)
	log.Info("x", "otp", "123456", "token", "abc", "note", "call me at +91 98765 43210 or a@b.com", "route", "/v1/posts")
	out := buf.String()
	for _, leak := range []string{"123456", "abc", "98765", "a@b.com"} {
		if strings.Contains(out, leak) {
			t.Fatalf("log leaks %q: %s", leak, out)
		}
	}
	if !strings.Contains(out, "/v1/posts") {
		t.Fatalf("log dropped safe field: %s", out)
	}
}
