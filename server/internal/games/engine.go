// Package games runs multiplayer game rooms. The server holds the only real state:
// clients send moves, engines validate them, and each player sees only their own view.
package games

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
)

// Ctx is what an engine may use: the clock, randomness, and text masking for anything players type.
type Ctx struct {
	Now  time.Time
	Rand *rand.Rand
	Mask func(string) string
}

// Engine is one kind of game.
type Engine interface {
	Name() string
	// Players is the allowed player count; rooms pick a max within it.
	Players() (min, max int)
	Start(c Ctx, players []uuid.UUID) (State, error)
	Load(data []byte) (State, error)
}

// State is one running game. It is stored as JSON between moves.
type State interface {
	// Move applies a player's move or returns a MoveError explaining why it was refused.
	Move(c Ctx, by uuid.UUID, move json.RawMessage) (*Live, error)
	// Tick advances timers; it reports whether anything changed.
	Tick(c Ctx) bool
	// Deadline is the next moment Tick has work to do, if any.
	Deadline() *time.Time
	// View is what one viewer may see. Spectators get the public view.
	View(viewer uuid.UUID, spectator bool) any
	Scores() map[uuid.UUID]int
	Over() bool
	// Leave handles a player quitting mid-game; it reports whether anything changed.
	Leave(c Ctx, who uuid.UUID) bool
}

// Live is a small event pushed to the room right away (a drawing stroke), on top of
// the usual "state changed" ping. To limits it to some members; nil means everyone.
type Live struct {
	Type string      `json:"type"`
	Data any         `json:"data"`
	To   []uuid.UUID `json:"-"`
}

// MoveError is a refused move; its message is shown to the player.
type MoveError struct{ Msg string }

func (e *MoveError) Error() string { return e.Msg }

func Refuse(msg string) error { return &MoveError{Msg: msg} }

func IsRefused(err error) (*MoveError, bool) {
	var m *MoveError
	ok := errors.As(err, &m)
	return m, ok
}

var registry = map[string]Engine{}

func Register(e Engine) { registry[e.Name()] = e }

func Lookup(name string) (Engine, bool) {
	e, ok := registry[name]
	return e, ok
}

func Names() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	return out
}

// Decode reads a move into T, refusing malformed input.
func Decode[T any](raw json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, Refuse("that move doesn't make sense")
	}
	return v, nil
}
