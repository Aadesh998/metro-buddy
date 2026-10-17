package realtime_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/app"
)

func dial(t *testing.T, srv *httptest.Server, env *apitest.Env, token string) *websocket.Conn {
	t.Helper()
	var tk struct{ Ticket string }
	env.Do(http.MethodPost, "/v1/ws/ticket", token, nil).Want(http.StatusOK).JSON(&tk)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws?ticket="+tk.Ticket, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	read(t, conn, "hello")
	return conn
}

func read(t *testing.T, conn *websocket.Conn, want string) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		_, b, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("waiting for %s: %v", want, err)
		}
		var m map[string]any
		json.Unmarshal(b, &m)
		if m["type"] == want {
			return m
		}
	}
}

func TestEventCrossesInstancesWithin300ms(t *testing.T) {
	env := apitest.New(t)
	u := env.NewUser("9876543210", env.City("DEL"))

	// A second API instance on the same Postgres-equivalent DB and Redis.
	other, err := app.New(env.App.Deps)
	if err != nil {
		t.Fatal(err)
	}
	srvA := httptest.NewServer(env.App.Echo)
	defer srvA.Close()
	conn := dial(t, srvA, env, u.AccessToken)

	deadline := time.Now().Add(2 * time.Second)
	for env.App.Hub.LocalCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	other.Hub.Publish(context.Background(), []uuid.UUID{u.PersonaID}, "message.new", map[string]string{"body": "hi"})
	m := read(t, conn, "message.new")
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("delivery took %v", elapsed)
	}
	if m["data"].(map[string]any)["body"] != "hi" {
		t.Fatalf("%v", m)
	}
}

func TestTicketsAreSingleUseAndRequired(t *testing.T) {
	env := apitest.New(t)
	u := env.NewUser("9876543210", env.City("DEL"))
	srv := httptest.NewServer(env.App.Echo)
	defer srv.Close()
	var tk struct{ Ticket string }
	env.Do(http.MethodPost, "/v1/ws/ticket", u.AccessToken, nil).Want(http.StatusOK).JSON(&tk)
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?ticket="
	ctx := context.Background()
	c1, _, err := websocket.Dial(ctx, url+tk.Ticket, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c1.CloseNow()
	if _, _, err := websocket.Dial(ctx, url+tk.Ticket, nil); err == nil {
		t.Fatal("ticket reused")
	}
	if _, _, err := websocket.Dial(ctx, url+"forged", nil); err == nil {
		t.Fatal("forged ticket accepted")
	}
	env.Do(http.MethodPost, "/v1/ws/ticket", "", nil).Want(http.StatusUnauthorized)
}

func TestSocketClosesWhenSessionIsRevoked(t *testing.T) {
	env := apitest.New(t)
	env.App.Hub.SessionCheckEvery = 50 * time.Millisecond
	u := env.NewUser("9876543210", env.City("DEL"))
	srv := httptest.NewServer(env.App.Echo)
	defer srv.Close()
	conn := dial(t, srv, env, u.AccessToken)
	env.Do(http.MethodPost, "/v1/auth/logout", "", map[string]string{"refresh_token": u.RefreshToken}).Want(http.StatusNoContent)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
				t.Fatalf("expected policy close after logout, got %v", err)
			}
			return
		}
	}
}
