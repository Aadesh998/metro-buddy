package apitest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"github.com/livekit/protocol/livekit"
	"google.golang.org/protobuf/encoding/protojson"
)

type Grant struct {
	Room, Identity string
	Publish, Admin bool
}

// Live records what the studio asks of LiveKit. Webhooks are accepted unsigned.
type Live struct {
	mu       sync.Mutex
	Tokens   []Grant
	Publish  map[string]bool
	Removed  []string
	Stopped  []string
	Closed   []string
	FailNext bool
}

func (l *Live) URL() string { return "ws://livekit.test" }

func (l *Live) Token(room, identity, _ string, publish, admin bool) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Tokens = append(l.Tokens, Grant{room, identity, publish, admin})
	b, _ := json.Marshal(Grant{room, identity, publish, admin})
	return string(b), nil
}

func (l *Live) SetPublish(_ context.Context, _, identity string, publish bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.FailNext {
		l.FailNext = false
		return io.ErrUnexpectedEOF
	}
	l.Publish[identity] = publish
	return nil
}

func (l *Live) CanPublish(identity string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Publish[identity]
}

func (l *Live) Remove(_ context.Context, _, identity string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Removed = append(l.Removed, identity)
	l.Publish[identity] = false
	return nil
}

func (l *Live) CloseRoom(_ context.Context, room string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Closed = append(l.Closed, room)
	return nil
}

func (l *Live) StopEgress(_ context.Context, id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Stopped = append(l.Stopped, id)
	return nil
}

func (l *Live) Webhook(r *http.Request) (*livekit.WebhookEvent, error) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	var ev livekit.WebhookEvent
	return &ev, protojson.Unmarshal(b, &ev)
}
