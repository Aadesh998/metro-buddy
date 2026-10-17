package ratelimit

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/testutil"
)

func TestSlidingWindowBlocksPastLimitThenRecovers(t *testing.T) {
	rdb, _ := testutil.Redis(t)
	l := New(rdb, slog.Default())
	now := time.Unix(1_700_000_000, 0)
	l.now = func() time.Time { return now }

	for i := range 3 {
		if res, err := l.Allow(t.Context(), "k", 3, time.Minute); err != nil || !res.Allowed {
			t.Fatalf("request %d blocked: %v", i, err)
		}
		now = now.Add(10 * time.Second)
	}
	res, _ := l.Allow(t.Context(), "k", 3, time.Minute)
	if res.Allowed {
		t.Fatal("4th request allowed")
	}
	if res.RetryAfter != 30*time.Second {
		t.Fatalf("retry after = %v, want 30s", res.RetryAfter)
	}
	now = now.Add(31 * time.Second)
	if res, _ := l.Allow(t.Context(), "k", 3, time.Minute); !res.Allowed {
		t.Fatal("not allowed after oldest request left the window")
	}
}

func TestMiddlewareReturns429AndScalesByTier(t *testing.T) {
	rdb, _ := testutil.Redis(t)
	l := New(rdb, slog.Default())
	var limited []string
	l.OnLimited = func(_ echo.Context, rule string) { limited = append(limited, rule) }

	e := echo.New()
	e.HTTPErrorHandler = httpx.ErrorHandler(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	tier := 0
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			httpx.SetPrincipal(c, &httpx.Principal{UserID: uuid.MustParse("00000000-0000-0000-0000-00000000000" + string(rune('1'+tier))), TrustTier: tier})
			return next(c)
		}
	})
	e.GET("/posts", func(c echo.Context) error { return c.NoContent(http.StatusOK) },
		l.Middleware(Rule{Name: "posts", Limit: 2, Window: time.Hour, ByTier: true}))

	count := func() int {
		ok := 0
		for range 10 {
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/posts", nil))
			if rec.Code == http.StatusOK {
				ok++
			} else if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
				t.Fatalf("unexpected %d", rec.Code)
			}
		}
		return ok
	}
	if got := count(); got != 2 {
		t.Fatalf("tier 0 allowed %d, want 2", got)
	}
	tier = 2
	if got := count(); got != 8 {
		t.Fatalf("tier 2 allowed %d, want 8", got)
	}
	if len(limited) == 0 || limited[0] != "posts" {
		t.Fatalf("OnLimited not called: %v", limited)
	}
}

func TestFailClosedWhenRedisDown(t *testing.T) {
	rdb, mr := testutil.Redis(t)
	mr.Close()
	l := New(rdb, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	e := echo.New()
	e.HTTPErrorHandler = httpx.ErrorHandler(slog.Default())
	ok := func(c echo.Context) error { return c.NoContent(http.StatusOK) }
	e.GET("/otp", ok, l.Middleware(Rule{Name: "otp", Limit: 1, Window: time.Minute, Key: ByIP, FailClosed: true}))
	e.GET("/feed", ok, l.Middleware(Rule{Name: "feed", Limit: 1, Window: time.Minute, Key: ByIP}))

	for path, want := range map[string]int{"/otp": http.StatusServiceUnavailable, "/feed": http.StatusOK} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("%s = %d, want %d", path, rec.Code, want)
		}
	}
}
