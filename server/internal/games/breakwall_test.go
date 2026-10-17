package games

import (
	"encoding/json"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

// The app's port (apps/mobile/src/features/games/breakwall/layout.ts) must give the same walls:
//
//	mulberry32(1) first three outputs: 2693262067, 11749833, 2265367787
//	breakwallLayout(12345, 0) -> 5 rows, 8 cols, 34 bricks:
//	  [0 1 2 3 4 5 6 7 9 10 11 12 13 14 16 18 19 20 21 23 24 25 26 27 28 29 30 31 32 33 35 36 38 39]
//	breakwallLayout(12345, 2) -> 6 rows, 7 cols, 31 bricks:
//	  [0 1 2 3 4 5 6 8 9 10 11 12 14 16 17 18 20 21 22 23 24 25 26 27 28 29 31 33 34 37 39]
func TestBreakwallLayoutIsStable(t *testing.T) {
	next := breakwallMulberry32(1)
	if got := []uint32{next(), next(), next()}; !slices.Equal(got, []uint32{2693262067, 11749833, 2265367787}) {
		t.Fatalf("mulberry32: %v", got)
	}
	rows, cols, cells := breakwallLayout(12345, 0)
	want := []int{0, 1, 2, 3, 4, 5, 6, 7, 9, 10, 11, 12, 13, 14, 16, 18, 19, 20, 21, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 35, 36, 38, 39}
	if rows != 5 || cols != 8 || !slices.Equal(cells, want) {
		t.Fatalf("layout 0: %d×%d %v", rows, cols, cells)
	}
	rows, cols, cells = breakwallLayout(12345, 2)
	want = []int{0, 1, 2, 3, 4, 5, 6, 8, 9, 10, 11, 12, 14, 16, 17, 18, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 31, 33, 34, 37, 39}
	if rows != 6 || cols != 7 || !slices.Equal(cells, want) {
		t.Fatalf("layout 2: %d×%d %v", rows, cols, cells)
	}
}

func breakwallCtx(now time.Time) Ctx {
	return Ctx{Now: now, Rand: rand.New(rand.NewPCG(1, 2)), Mask: func(s string) string { return s }}
}

// breakwallRoundTrip mimics the service: state is stored as JSON between calls.
func breakwallRoundTrip(t *testing.T, st State) *breakwallState {
	t.Helper()
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	out, err := breakwallEngine{}.Load(b)
	if err != nil {
		t.Fatal(err)
	}
	return out.(*breakwallState)
}

func breakwallMoveJSON(round, bricks, score int, final bool) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"type": "score", "round": round, "bricks": bricks, "score": score, "final": final})
	return b
}

func TestBreakwallScoring(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	a, b := uuid.New(), uuid.New()
	st0, _ := breakwallEngine{}.Start(breakwallCtx(t0), []uuid.UUID{a, b})
	s := breakwallRoundTrip(t, st0)
	total := s.cur().Total

	// Countdown: too early.
	if _, err := s.Move(breakwallCtx(t0.Add(2*time.Second)), a, breakwallMoveJSON(1, 0, 0, false)); err == nil {
		t.Fatal("score during countdown accepted")
	}
	play := t0.Add(breakwallCountdown)
	if !s.Tick(breakwallCtx(play)) || s.Phase != breakwallPlayPhase {
		t.Fatalf("phase %s", s.Phase)
	}
	at := play.Add(5 * time.Second)
	if _, err := s.Move(breakwallCtx(at), a, breakwallMoveJSON(1, 6, 60, false)); err != nil {
		t.Fatalf("valid score refused: %v", err)
	}
	s = breakwallRoundTrip(t, s)
	for name, m := range map[string]json.RawMessage{
		"wrong formula":  breakwallMoveJSON(1, 7, 75, false),
		"too fast":       breakwallMoveJSON(1, 40, 400, false),
		"too many":       breakwallMoveJSON(1, total+1, (total+1)*10, false),
		"wrong round":    breakwallMoveJSON(2, 7, 70, false),
		"backwards":      breakwallMoveJSON(1, 5, 50, false),
		"bonus too high": breakwallMoveJSON(1, total, total*10+50+200, false),
	} {
		if _, err := s.Move(breakwallCtx(at), a, m); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// A clear late in the round: the time bonus is capped by the time that was left.
	late := play.Add(40 * time.Second)
	if _, err := s.Move(breakwallCtx(late), a, breakwallMoveJSON(1, total, total*10+50+20, true)); err == nil {
		t.Fatal("time bonus beyond the clock accepted")
	}
	if _, err := s.Move(breakwallCtx(late), a, breakwallMoveJSON(1, total, total*10+50+4, true)); err != nil {
		t.Fatalf("clear refused: %v", err)
	}
	if _, err := s.Move(breakwallCtx(late), a, breakwallMoveJSON(1, total, total*10+50+4, true)); err == nil {
		t.Fatal("final score changed")
	}
	// Outside the window (past the grace period).
	if _, err := s.Move(breakwallCtx(play.Add(breakwallPlay+breakwallGrace+time.Second)), b, breakwallMoveJSON(1, 3, 30, true)); err == nil {
		t.Fatal("score after the round accepted")
	}
	// Within the grace period, and it ends the round early since everyone is in.
	live, err := s.Move(breakwallCtx(play.Add(breakwallPlay+time.Second)), b, breakwallMoveJSON(1, 3, 30, true))
	if err != nil || live == nil || live.Type != "score" {
		t.Fatalf("grace score: %v", err)
	}
	if s.Phase != breakwallResultsPhase {
		t.Fatalf("phase %s after everyone finished", s.Phase)
	}

	// Two more rounds; the totals add up across them.
	r1a := total*10 + 54
	now := play.Add(breakwallPlay + time.Second)
	for round := 2; round <= breakwallRounds; round++ {
		now = now.Add(breakwallResults)
		s.Tick(breakwallCtx(now))
		s = breakwallRoundTrip(t, s)
		if len(s.Rounds) != round || s.Phase != breakwallCountdownPhase {
			t.Fatalf("round %d phase %s", len(s.Rounds), s.Phase)
		}
		now = s.cur().PlayStarts.Add(10 * time.Second)
		s.Tick(breakwallCtx(now))
		if _, err := s.Move(breakwallCtx(now), a, breakwallMoveJSON(round, 2, 20, false)); err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		now = s.cur().PlayEnds
		s.Tick(breakwallCtx(now))
	}
	now = now.Add(breakwallResults)
	s.Tick(breakwallCtx(now))
	if !s.Over() || s.Deadline() != nil {
		t.Fatal("game should be over")
	}
	sc := s.Scores()
	if sc[a] != r1a+40 || sc[b] != 30 {
		t.Fatalf("totals %v", sc)
	}
	if _, err := s.Move(breakwallCtx(now), a, breakwallMoveJSON(3, 3, 30, false)); err == nil {
		t.Fatal("move after the game accepted")
	}
}

func TestBreakwallView(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	a := uuid.New()
	st, _ := breakwallEngine{}.Start(breakwallCtx(t0), []uuid.UUID{a})
	s := breakwallRoundTrip(t, st)
	v := s.View(uuid.Nil, true).(map[string]any)
	if v["phase"] != breakwallCountdownPhase || v["rows"] == nil || v["seed"] == nil || v["round"] != 1 {
		t.Fatalf("view %v", v)
	}
	// A slow ticker catches up through several phases at once.
	if !s.Tick(breakwallCtx(t0.Add(breakwallCountdown + breakwallPlay + breakwallResults + time.Second))) {
		t.Fatal("tick did nothing")
	}
	if len(s.Rounds) != 2 || s.Phase != breakwallCountdownPhase {
		t.Fatalf("round %d phase %s", len(s.Rounds), s.Phase)
	}
	if s.Leave(breakwallCtx(t0), a) {
		t.Fatal("leave should change nothing")
	}
}
