package web_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/model"
)

func form(env *apitest.Env, vals url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/grievance", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	env.App.Echo.ServeHTTP(rec, req)
	return rec
}

func TestPublicPagesAreServerRenderedAndIndexable(t *testing.T) {
	env := apitest.New(t)
	for _, p := range []string{"/", "/privacy", "/terms", "/guidelines", "/grievance"} {
		r := env.Do(http.MethodGet, p, "", nil).Want(http.StatusOK)
		body := string(r.Body)
		for _, want := range []string{`<link rel="canonical" href="https://metrobuddy.test` + p, `property="og:title"`, `<meta name="description"`, "/static/htmx.min.js"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s missing %q", p, want)
			}
		}
		if strings.Contains(body, "noindex") {
			t.Errorf("%s is noindex", p)
		}
	}
	rec := httptest.NewRecorder()
	env.App.Echo.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
		t.Fatalf("csp %q", csp)
	}
	sm := env.Do(http.MethodGet, "/sitemap.xml", "", nil).Want(http.StatusOK)
	if !bytes.Contains(sm.Body, []byte("<loc>https://metrobuddy.test/privacy</loc>")) {
		t.Fatalf("%s", sm.Body)
	}
	if r := env.Do(http.MethodGet, "/robots.txt", "", nil); !bytes.Contains(r.Body, []byte("Disallow: /v1")) {
		t.Fatalf("%s", r.Body)
	}
	env.Do(http.MethodGet, "/static/og.png", "", nil).Want(http.StatusOK)
}

func TestGrievanceFormCreatesEncryptedTicketWithSLA(t *testing.T) {
	env := apitest.New(t)
	bad := form(env, url.Values{"category": {"content"}, "body": {"short"}})
	if bad.Code != http.StatusUnprocessableEntity || bad.Header().Get("HX-Retarget") != "#grievance-result" {
		t.Fatalf("%d %v", bad.Code, bad.Header())
	}
	ok := form(env, url.Values{"category": {"ncii"}, "email": {"me@example.com"}, "body": {"Someone posted my private photo in the Yellow Line bowl."}})
	if ok.Code != http.StatusCreated || !strings.Contains(ok.Body.String(), "GRV-") {
		t.Fatalf("%d %s", ok.Code, ok.Body)
	}
	var tk model.GrievanceTicket
	env.App.DB.First(&tk)
	if bytes.Contains(tk.BodyEnc, []byte("private photo")) || bytes.Contains(tk.ContactEmailEnc, []byte("example.com")) {
		t.Fatal("ticket stored in plaintext")
	}
	if tk.ResolveDueAt.Sub(tk.CreatedAt).Hours() > 24.1 {
		t.Fatalf("NCII must resolve within 24h, got %v", tk.ResolveDueAt.Sub(tk.CreatedAt))
	}
}
