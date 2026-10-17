package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"

	"metro-buddy/server/internal/app"
	"metro-buddy/server/internal/auth"
	"metro-buddy/server/internal/config"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/notify"
	"metro-buddy/server/internal/seed"
	"metro-buddy/server/internal/testutil"
)

type SMS struct {
	mu   sync.Mutex
	OTPs map[string]string
	SOS  []string
}

func (s *SMS) SendOTP(_ context.Context, e164, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.OTPs[e164] = code
	return nil
}

func (s *SMS) SendSOS(_ context.Context, e164, _, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SOS = append(s.SOS, e164)
	return nil
}

func (s *SMS) Code(e164 string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.OTPs[e164]
}

type Pushes struct {
	mu   sync.Mutex
	Sent []notify.PushMessage
	// Tokens listed here report DeviceNotRegistered.
	Dead map[string]bool
}

func (p *Pushes) Send(_ context.Context, msgs []notify.PushMessage) ([]notify.PushTicket, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]notify.PushTicket, len(msgs))
	for i, m := range msgs {
		p.Sent = append(p.Sent, m)
		out[i].Status = "ok"
		if p.Dead[m.To] {
			out[i].Status = "error"
			out[i].Details.Error = "DeviceNotRegistered"
		}
	}
	return out, nil
}

func (p *Pushes) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.Sent)
}

type Env struct {
	T      *testing.T
	App    *app.App
	SMS    *SMS
	Pushes *Pushes
	Redis  *miniredis.Miniredis
	ipSeq  int
}

func New(t *testing.T) *Env {
	t.Helper()
	cfg := &config.Config{
		Env:           "test",
		JWTSecret:     []byte("test-jwt-secret-test-jwt-secret!!"),
		PhonePepper:   []byte("test-pepper"),
		IPPepper:      []byte("test-ip-pepper"),
		DataKey:       []byte("0123456789abcdef0123456789abcdef"),
		PublicBaseURL: "https://metrobuddy.test",
	}
	rdb, mr := testutil.Redis(t)
	s := &SMS{OTPs: map[string]string{}}
	pushes := &Pushes{Dead: map[string]bool{}}
	a, err := app.New(&app.Deps{Cfg: cfg, DB: testutil.DB(t), Redis: rdb, Log: slog.New(slog.NewTextHandler(tWriter{t}, &slog.HandlerOptions{Level: slog.LevelWarn})), SMS: s, Pusher: pushes})
	if err != nil {
		t.Fatal(err)
	}
	return &Env{T: t, App: a, SMS: s, Redis: mr, Pushes: pushes}
}

type Resp struct {
	Code int
	Body []byte
	T    *testing.T
}

func (r *Resp) JSON(v any) {
	r.T.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		r.T.Fatalf("decode %s: %v", r.Body, err)
	}
}

func (r *Resp) Map() map[string]any {
	var m map[string]any
	r.JSON(&m)
	return m
}

func (r *Resp) ErrCode() string {
	var e struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(r.Body, &e)
	return e.Error.Code
}

func (r *Resp) Want(code int) *Resp {
	r.T.Helper()
	if r.Code != code {
		r.T.Fatalf("status %d, want %d: %s", r.Code, code, r.Body)
	}
	return r
}

func (e *Env) Do(method, path, token string, body any, headers ...string) *Resp {
	e.T.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rd)
	// Requests arrive via a private proxy hop, as on Railway, so X-Forwarded-For is honoured.
	req.RemoteAddr = "10.0.0.1:40000"
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.App.Echo.ServeHTTP(rec, req)
	return &Resp{Code: rec.Code, Body: rec.Body.Bytes(), T: e.T}
}

type Session struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Each login uses its own client IP so per-IP OTP throttles don't couple tests together.
func (e *Env) Login(phone string) Session {
	e.T.Helper()
	e.ipSeq++
	ip := "203.0.113." + itoa(e.ipSeq)
	e.Do(http.MethodPost, "/v1/auth/otp/request", "", map[string]string{"phone": phone}, "X-Forwarded-For", ip).Want(http.StatusAccepted)
	var out struct{ Session Session }
	e.Do(http.MethodPost, "/v1/auth/otp/verify", "", map[string]any{
		"phone": phone, "code": e.SMS.Code(mustE164(phone)),
		"device": map[string]string{"platform": "android", "app_version": "1.0.0"},
	}, "X-Forwarded-For", ip).Want(http.StatusOK).JSON(&out)
	return out.Session
}

func mustE164(phone string) string {
	p, err := auth.NormalizePhone(phone)
	if err != nil {
		panic(err)
	}
	return p
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// Seeded loads all reference data (Delhi metro, vibes, bowls) and returns the Delhi city id.
func (e *Env) Seeded() uuid.UUID {
	e.T.Helper()
	if err := seed.All(e.App.DB); err != nil {
		e.T.Fatal(err)
	}
	var c model.City
	e.App.DB.First(&c, "code = ?", "DEL")
	return c.ID
}

func (e *Env) StationID(name string) uuid.UUID {
	e.T.Helper()
	var s model.Station
	if err := e.App.DB.First(&s, "name = ?", name).Error; err != nil {
		e.T.Fatalf("station %q: %v", name, err)
	}
	return s.ID
}

func (e *Env) City(code string) uuid.UUID {
	e.T.Helper()
	c := model.City{Name: "City " + code, Code: code, IsLive: true}
	if err := e.App.DB.Create(&c).Error; err != nil {
		e.T.Fatal(err)
	}
	return c.ID
}

type User struct {
	Session
	PersonaID uuid.UUID
	Alias     string
}

// NewUser logs in a fresh phone number and creates a persona in cityID.
// Onboard confirms age 18+ and grants every consent.
func (e *Env) Onboard(s Session) {
	e.T.Helper()
	e.Do(http.MethodPut, "/v1/me/age", s.AccessToken, map[string]string{"birth_date": "1995-06-15"}).Want(http.StatusOK)
	items := []map[string]any{}
	for _, p := range model.ConsentPurposes {
		items = append(items, map[string]any{"purpose": p, "granted": true})
	}
	e.Do(http.MethodPost, "/v1/me/consents", s.AccessToken, map[string]any{"policy_version": identity.PolicyVersion, "items": items}).Want(http.StatusOK)
}

// LoginOnboarded logs in and completes the age gate and consents.
func (e *Env) LoginOnboarded(phone string) Session {
	e.T.Helper()
	s := e.Login(phone)
	e.Onboard(s)
	return s
}

func (e *Env) NewUser(phone string, cityID uuid.UUID) User {
	e.T.Helper()
	s := e.LoginOnboarded(phone)
	var sug struct {
		Suggestions []struct {
			Alias      string `json:"alias"`
			AvatarSeed string `json:"avatar_seed"`
		}
	}
	e.Do(http.MethodPost, "/v1/me/persona/suggestions", s.AccessToken, map[string]any{"city_id": cityID}).Want(http.StatusOK).JSON(&sug)
	var p struct {
		ID    uuid.UUID
		Alias string
	}
	e.Do(http.MethodPost, "/v1/me/persona", s.AccessToken, map[string]any{
		"city_id": cityID, "alias": sug.Suggestions[0].Alias, "avatar_seed": sug.Suggestions[0].AvatarSeed,
	}).Want(http.StatusOK).JSON(&p)
	return User{Session: s, PersonaID: p.ID, Alias: p.Alias}
}

type tWriter struct{ t *testing.T }

func (w tWriter) Write(p []byte) (int, error) {
	w.t.Log(string(bytes.TrimRight(p, "\n")))
	return len(p), nil
}
