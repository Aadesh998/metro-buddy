// Package realtime is the WebSocket gateway. Each API instance keeps its own sockets and
// subscribes to a Redis channel per connected persona, so events published on any
// instance reach the persona wherever they are connected.
package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/secure"
)

const (
	ticketTTL     = 30 * time.Second
	pingEvery     = 25 * time.Second
	sessionCheck  = time.Minute
	writeTimeout  = 10 * time.Second
	maxInbound    = 16 << 10
	PresenceTTL   = 70 * time.Second
	channelPrefix = "rt:p:"
)

type Event struct {
	Type string    `json:"type"`
	Data any       `json:"data,omitempty"`
	TS   time.Time `json:"ts"`
}

// Inbound is a client-sent frame (typing, read receipts, pings).
type Inbound struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type client struct {
	persona uuid.UUID
	user    uuid.UUID
	family  uuid.UUID
	conn    *websocket.Conn
	send    chan []byte
}

type Hub struct {
	Redis  *redis.Client
	Log    *slog.Logger
	Origin []string
	// SessionActive lets the hub drop sockets once a session is revoked (logout, ban, token theft).
	SessionActive func(ctx context.Context, family uuid.UUID) (bool, error)
	// OnInbound handles typing / read / delivered frames from clients.
	OnInbound func(ctx context.Context, persona uuid.UUID, in Inbound)
	// OnPresence reports when a persona's first socket connects or last socket closes.
	OnPresence func(persona uuid.UUID, online bool)
	// SessionCheckEvery defaults to a minute; tests shorten it.
	SessionCheckEvery time.Duration

	mu      sync.Mutex
	clients map[uuid.UUID]map[*client]struct{}
	pubsub  *redis.PubSub
	once    sync.Once
}

func (h *Hub) start(ctx context.Context) {
	h.once.Do(func() {
		h.clients = map[uuid.UUID]map[*client]struct{}{}
		// Subscribe to a placeholder so the PubSub connection exists before the first real channel.
		h.pubsub = h.Redis.Subscribe(ctx, channelPrefix+"_")
		go h.fanIn(ctx)
	})
}

func (h *Hub) fanIn(ctx context.Context) {
	ch := h.pubsub.Channel(redis.WithChannelSize(1024))
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			id, err := uuid.Parse(strings.TrimPrefix(msg.Channel, channelPrefix))
			if err != nil {
				continue
			}
			h.deliverLocal(id, []byte(msg.Payload))
		}
	}
}

func (h *Hub) deliverLocal(persona uuid.UUID, payload []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients[persona] {
		select {
		case c.send <- payload:
		default:
			// A client that can't keep up is closed rather than allowed to stall fan-out.
			go c.conn.Close(websocket.StatusPolicyViolation, "too slow")
		}
	}
}

// Publish sends an event to every socket of the given personas across all instances.
func (h *Hub) Publish(ctx context.Context, personas []uuid.UUID, typ string, data any) {
	if h == nil || len(personas) == 0 {
		return
	}
	b, err := json.Marshal(Event{Type: typ, Data: data, TS: time.Now().UTC()})
	if err != nil {
		return
	}
	pipe := h.Redis.Pipeline()
	seen := map[uuid.UUID]bool{}
	for _, p := range personas {
		if !seen[p] {
			seen[p] = true
			pipe.Publish(ctx, channelPrefix+p.String(), b)
		}
	}
	if _, err := pipe.Exec(ctx); err != nil && h.Log != nil {
		h.Log.Warn("realtime publish failed", "err", err.Error())
	}
}

func (h *Hub) register(ctx context.Context, c *client) {
	h.mu.Lock()
	first := len(h.clients[c.persona]) == 0
	if first {
		h.clients[c.persona] = map[*client]struct{}{}
	}
	h.clients[c.persona][c] = struct{}{}
	h.mu.Unlock()
	if first {
		if err := h.pubsub.Subscribe(ctx, channelPrefix+c.persona.String()); err != nil && h.Log != nil {
			h.Log.Warn("realtime subscribe failed", "err", err.Error())
		}
		if h.OnPresence != nil {
			h.OnPresence(c.persona, true)
		}
	}
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	delete(h.clients[c.persona], c)
	last := len(h.clients[c.persona]) == 0
	if last {
		delete(h.clients, c.persona)
	}
	h.mu.Unlock()
	if last {
		_ = h.pubsub.Unsubscribe(context.Background(), channelPrefix+c.persona.String())
		if h.OnPresence != nil {
			h.OnPresence(c.persona, false)
		}
	}
}

// LocalCount reports sockets on this instance; used by tests and metrics.
func (h *Hub) LocalCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, s := range h.clients {
		n += len(s)
	}
	return n
}

type ticketValue struct {
	Persona uuid.UUID `json:"p"`
	User    uuid.UUID `json:"u"`
	Family  uuid.UUID `json:"f"`
}

// IssueTicket returns a single-use, 30-second ticket so the access token never goes in a URL.
func (h *Hub) IssueTicket(c echo.Context, family uuid.UUID) error {
	p := httpx.PrincipalFrom(c)
	t := secure.Token(24)
	v, _ := json.Marshal(ticketValue{Persona: p.PersonaID, User: p.UserID, Family: family})
	if err := h.Redis.Set(c.Request().Context(), "ws:ticket:"+t, v, ticketTTL).Err(); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"ticket": t, "expires_in": int(ticketTTL.Seconds())})
}

func (h *Hub) Serve(ctx context.Context) echo.HandlerFunc {
	return func(c echo.Context) error {
		h.start(ctx)
		t := c.QueryParam("ticket")
		if t == "" {
			return httpx.Unauthorized()
		}
		raw, err := h.Redis.GetDel(c.Request().Context(), "ws:ticket:"+t).Result()
		if err != nil {
			return httpx.Unauthorized()
		}
		var tv ticketValue
		if json.Unmarshal([]byte(raw), &tv) != nil {
			return httpx.Unauthorized()
		}
		conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{OriginPatterns: h.Origin})
		if err != nil {
			return nil
		}
		conn.SetReadLimit(maxInbound)
		cl := &client{persona: tv.Persona, user: tv.User, family: tv.Family, conn: conn, send: make(chan []byte, 64)}
		h.register(ctx, cl)
		defer h.unregister(cl)
		h.run(ctx, cl)
		return nil
	}
}

func (h *Hub) run(parent context.Context, c *client) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	defer c.conn.CloseNow()

	go func() {
		defer cancel()
		for {
			_, b, err := c.conn.Read(ctx)
			if err != nil {
				return
			}
			var in Inbound
			if json.Unmarshal(b, &in) != nil {
				continue
			}
			if in.Type == "ping" {
				h.write(ctx, c, []byte(`{"type":"pong"}`))
				continue
			}
			if h.OnInbound != nil {
				h.OnInbound(ctx, c.persona, in)
			}
		}
	}()

	hello, _ := json.Marshal(Event{Type: "hello", TS: time.Now().UTC()})
	if !h.write(ctx, c, hello) {
		return
	}
	ping := time.NewTicker(pingEvery)
	defer ping.Stop()
	every := h.SessionCheckEvery
	if every == 0 {
		every = sessionCheck
	}
	check := time.NewTicker(every)
	defer check.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-c.send:
			if !h.write(ctx, c, b) {
				return
			}
		case <-ping.C:
			pctx, pcancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Ping(pctx)
			pcancel()
			if err != nil {
				return
			}
			if h.OnPresence != nil {
				h.OnPresence(c.persona, true)
			}
		case <-check.C:
			if h.SessionActive != nil {
				if ok, err := h.SessionActive(ctx, c.family); err == nil && !ok {
					c.conn.Close(websocket.StatusPolicyViolation, "session ended")
					return
				}
			}
		}
	}
}

func (h *Hub) write(ctx context.Context, c *client, b []byte) bool {
	wctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return c.conn.Write(wctx, websocket.MessageText, b) == nil
}
