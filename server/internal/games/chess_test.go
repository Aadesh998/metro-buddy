package games

import (
	"encoding/json"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/google/uuid"
)

var chessT0 = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func chessCtx(at time.Time) Ctx {
	return Ctx{Now: at, Rand: rand.New(rand.NewPCG(1, 2)), Mask: func(s string) string { return s }}
}

// chessReload round-trips the state through JSON, as the service does between calls.
func chessReload(t *testing.T, st State) *chessState {
	t.Helper()
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	out, err := chessEngine{}.Load(b)
	if err != nil {
		t.Fatal(err)
	}
	return out.(*chessState)
}

func chessNew(t *testing.T) *chessState {
	t.Helper()
	st, err := chessEngine{}.Start(chessCtx(chessT0), []uuid.UUID{uuid.New(), uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	return chessReload(t, st)
}

func chessPlay(t *testing.T, s *chessState, by uuid.UUID, at time.Time, move map[string]any) (*chessState, error) {
	t.Helper()
	raw, _ := json.Marshal(move)
	_, err := s.Move(chessCtx(at), by, raw)
	return chessReload(t, s), err
}

func TestChessLegalAndIllegal(t *testing.T) {
	s := chessNew(t)
	s, err := chessPlay(t, s, s.White, chessT0.Add(time.Second), map[string]any{"type": "move", "uci": "e2e5"})
	if m, ok := IsRefused(err); !ok || m.Msg != "That move isn't legal" {
		t.Fatalf("illegal move: %v", err)
	}
	s, err = chessPlay(t, s, s.White, chessT0.Add(3*time.Second), map[string]any{"type": "move", "uci": "e2e4"})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.SAN) != 1 || s.SAN[0] != "e4" || s.turn() != "black" {
		t.Fatalf("after e4: %v %s", s.SAN, s.turn())
	}
	if s.WhiteMs != chessClock.Milliseconds()-3000 {
		t.Fatalf("white clock %d", s.WhiteMs)
	}
	// White can't move twice.
	if _, err = chessPlay(t, s, s.White, chessT0.Add(4*time.Second), map[string]any{"type": "move", "uci": "d2d4"}); err == nil {
		t.Fatal("white moved out of turn")
	}
	if _, err = chessPlay(t, s, uuid.New(), chessT0.Add(4*time.Second), map[string]any{"type": "move", "uci": "e7e5"}); err == nil {
		t.Fatal("outsider moved")
	}
}

func TestChessFoolsMate(t *testing.T) {
	s := chessNew(t)
	var err error
	for i, m := range []string{"f2f3", "e7e5", "g2g4", "d8h4"} {
		by := s.White
		if i%2 == 1 {
			by = s.Black
		}
		if s, err = chessPlay(t, s, by, chessT0.Add(time.Duration(i+1)*time.Second), map[string]any{"type": "move", "uci": m}); err != nil {
			t.Fatal(m, err)
		}
	}
	if !s.Over() || s.Result != "black" || s.Reason != "checkmate" {
		t.Fatalf("result %q %q", s.Result, s.Reason)
	}
	if sc := s.Scores(); sc[s.Black] != chessWin || sc[s.White] != 0 {
		t.Fatalf("scores %v", sc)
	}
	if s.Deadline() != nil {
		t.Fatal("deadline after the end")
	}
	if v := s.View(s.White, false).(map[string]any); v["check"] != true || v["last"].([]string)[1] != "h4" {
		t.Fatalf("view %v", v)
	}
}

func TestChessResignAndDraw(t *testing.T) {
	s := chessNew(t)
	if _, err := chessPlay(t, s, s.White, chessT0, map[string]any{"type": "draw_accept"}); err == nil {
		t.Fatal("accepted a missing offer")
	}
	s, _ = chessPlay(t, s, s.White, chessT0, map[string]any{"type": "draw_offer"})
	if s.DrawOffer != "white" {
		t.Fatal("offer not recorded")
	}
	s, _ = chessPlay(t, s, s.Black, chessT0, map[string]any{"type": "draw_decline"})
	if s.DrawOffer != "" || s.Over() {
		t.Fatal("decline")
	}
	s, err := chessPlay(t, s, s.Black, chessT0, map[string]any{"type": "resign"})
	if err != nil || s.Result != "white" || s.Reason != "resign" {
		t.Fatalf("resign: %v %q", err, s.Result)
	}

	s = chessNew(t)
	s, _ = chessPlay(t, s, s.Black, chessT0, map[string]any{"type": "draw_offer"})
	s, _ = chessPlay(t, s, s.White, chessT0, map[string]any{"type": "draw_accept"})
	if sc := s.Scores(); s.Result != "draw" || sc[s.White] != chessDraw || sc[s.Black] != chessDraw {
		t.Fatalf("draw: %q %v", s.Result, sc)
	}

	s = chessNew(t)
	if !s.Leave(chessCtx(chessT0), s.White) || s.Result != "black" {
		t.Fatal("leaving should resign")
	}
}

func TestChessFlag(t *testing.T) {
	s := chessNew(t)
	if d := s.Deadline(); d == nil || !d.Equal(chessT0.Add(chessClock)) {
		t.Fatalf("deadline %v", d)
	}
	if s.Tick(chessCtx(chessT0.Add(chessClock - time.Second))) {
		t.Fatal("ticked early")
	}
	if !s.Tick(chessCtx(chessT0.Add(chessClock))) {
		t.Fatal("no flag")
	}
	s = chessReload(t, s)
	if s.Result != "black" || s.Reason != "flag" || s.WhiteMs != 0 {
		t.Fatalf("flag: %q %q %d", s.Result, s.Reason, s.WhiteMs)
	}
}

func TestChessViewLegalOnlyForMover(t *testing.T) {
	s := chessNew(t)
	vw := s.View(s.White, false).(map[string]any)
	vb := s.View(s.Black, false).(map[string]any)
	vs := s.View(uuid.New(), true).(map[string]any)
	if n := len(vw["legal"].([]string)); n != 20 {
		t.Fatalf("white has %d legal moves", n)
	}
	if len(vb["legal"].([]string)) != 0 || len(vs["legal"].([]string)) != 0 {
		t.Fatal("legal moves leaked to the side not moving")
	}
	if vw["you"] != "white" || vb["you"] != "black" || vs["you"] != "spectator" {
		t.Fatal("colours")
	}
}
