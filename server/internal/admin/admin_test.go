package admin_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"metro-buddy/server/internal/admin"
	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
)

type console struct {
	env *apitest.Env
}

type creds struct {
	email, pw, secret string
}

func (c console) newAdmin(t *testing.T, email string, role model.AdminRole) creds {
	_, pw, url, err := admin.NewAdmin(c.env.App.DB, c.env.App.Box, email, role)
	if err != nil {
		t.Fatal(err)
	}
	k, _ := otp.NewKeyFromURL(url)
	return creds{email, pw, k.Secret()}
}

func (c console) do(method, path, cookie string, body string, ct string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "10.0.0.1:1"
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	rec := httptest.NewRecorder()
	c.env.App.Echo.ServeHTTP(rec, req)
	return rec
}

func (c console) login(t *testing.T, cr creds, code string) *httptest.ResponseRecorder {
	if code == "" {
		code, _ = totp.GenerateCode(cr.secret, time.Now())
	}
	return c.do(http.MethodPost, "/admin/api/login", "", `{"email":"`+cr.email+`","password":"`+cr.pw+`","code":"`+code+`"}`, "application/json")
}

func cookieOf(rec *httptest.ResponseRecorder) string {
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "mb_admin" {
			if !ck.HttpOnly || ck.SameSite != http.SameSiteStrictMode {
				panic("admin cookie must be HttpOnly + SameSite=Strict")
			}
			return ck.Name + "=" + ck.Value
		}
	}
	return ""
}

func TestAdminLoginNeedsPasswordAndTOTPAndLocksOut(t *testing.T) {
	c := console{apitest.New(t)}
	cr := c.newAdmin(t, "mod@metrobuddy.in", model.AdminModerator)
	if rec := c.login(t, creds{cr.email, cr.pw, cr.secret}, "000000"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong TOTP accepted: %d", rec.Code)
	}
	if rec := c.login(t, creds{cr.email, "nope", cr.secret}, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password accepted: %d", rec.Code)
	}
	ok := c.login(t, cr, "")
	if ok.Code != http.StatusOK || cookieOf(ok) == "" {
		t.Fatalf("login failed: %d %s", ok.Code, ok.Body)
	}
	for range 5 {
		c.login(t, creds{cr.email, "bad", cr.secret}, "")
	}
	if rec := c.login(t, cr, ""); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("not locked after 5 failures: %d", rec.Code)
	}
	var n int64
	c.env.App.DB.Model(&model.SecurityEvent{}).Where("kind = ?", model.SecAdminLoginFail).Count(&n)
	if n < 7 {
		t.Fatalf("admin login failures not logged: %d", n)
	}
}

func TestModeratorsCannotReadVaultButLeadsCan(t *testing.T) {
	c := console{apitest.New(t)}
	mod := cookieOf(c.login(t, c.newAdmin(t, "mod@m.in", model.AdminModerator), ""))
	lead := cookieOf(c.login(t, c.newAdmin(t, "lead@m.in", model.AdminTSLead), ""))
	id := model.NewID()
	c.env.App.PII.Process(c.env.App.DB, model.CTPost, id, "call 9876543210", pii.Options{})
	body := `{"content_type":"post","content_id":"` + id.String() + `","reason":"police request FIR 123/2026"}`
	if rec := c.do(http.MethodPost, "/admin/api/vault/read", mod, body, "application/json"); rec.Code != http.StatusForbidden {
		t.Fatalf("moderator read the vault: %d", rec.Code)
	}
	rec := c.do(http.MethodPost, "/admin/api/vault/read", lead, body, "application/json")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "9876543210") {
		t.Fatalf("lead vault read: %d %s", rec.Code, rec.Body)
	}
	if rec := c.do(http.MethodPost, "/admin/api/bowls", lead, `{}`, "application/json"); rec.Code != http.StatusForbidden {
		t.Fatalf("lead created a bowl: %d", rec.Code)
	}
}

func TestDeactivationAndCSRFGuard(t *testing.T) {
	c := console{apitest.New(t)}
	boss := cookieOf(c.login(t, c.newAdmin(t, "boss@m.in", model.AdminAdmin), ""))
	mod := cookieOf(c.login(t, c.newAdmin(t, "mod@m.in", model.AdminModerator), ""))
	if rec := c.do(http.MethodGet, "/admin/api/me", mod, "", ""); rec.Code != http.StatusOK {
		t.Fatalf("%d", rec.Code)
	}
	if rec := c.do(http.MethodPost, "/admin/api/logout", boss, "", "application/x-www-form-urlencoded"); rec.Code != http.StatusNoContent {
		t.Fatalf("logout %d", rec.Code)
	}
	if rec := c.do(http.MethodPost, "/admin/api/bowls", boss, "kind=topic", "application/x-www-form-urlencoded"); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("form post not rejected: %d", rec.Code)
	}
	var m model.AdminUser
	c.env.App.DB.First(&m, "email = ?", "mod@m.in")
	rec := c.do(http.MethodPatch, "/admin/api/admins/"+m.ID.String(), boss, `{"active":false}`, "application/json")
	if rec.Code != http.StatusOK {
		t.Fatalf("deactivate %d %s", rec.Code, rec.Body)
	}
	if rec := c.do(http.MethodGet, "/admin/api/me", mod, "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("deactivated admin still in: %d", rec.Code)
	}
}

func TestMetricsAreAggregateOnly(t *testing.T) {
	env := apitest.New(t)
	c := console{env}
	env.NewUser("9000000001", env.City("DEL"))
	mod := cookieOf(c.login(t, c.newAdmin(t, "mod@m.in", model.AdminModerator), ""))
	rec := c.do(http.MethodGet, "/admin/api/metrics?days=7", mod, "", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"signups":[{"day":`) || strings.Contains(rec.Body.String(), "persona") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
