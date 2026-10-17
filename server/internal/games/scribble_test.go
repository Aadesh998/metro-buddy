package games

import (
	"encoding/json"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func scribbleCtx(now time.Time) Ctx {
	return Ctx{Now: now, Rand: rand.New(rand.NewPCG(1, 2)), Mask: func(s string) string { return s }}
}

// scribbleRoundTrip mimics the service: state is reloaded from JSON before every call.
func scribbleRoundTrip(t *testing.T, st State) *scribbleState {
	t.Helper()
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	out, err := scribbleEngine{}.Load(b)
	if err != nil {
		t.Fatal(err)
	}
	return out.(*scribbleState)
}

func scribbleStart(t *testing.T, n int) (*scribbleState, []uuid.UUID, time.Time) {
	t.Helper()
	ps := make([]uuid.UUID, n)
	for i := range ps {
		ps[i] = uuid.New()
	}
	now := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	st, err := scribbleEngine{}.Start(scribbleCtx(now), ps)
	if err != nil {
		t.Fatal(err)
	}
	return scribbleRoundTrip(t, st), ps, now
}

func scribblePlay(t *testing.T, s *scribbleState, c Ctx, by uuid.UUID, m map[string]any) (*Live, error) {
	t.Helper()
	b, _ := json.Marshal(m)
	return s.Move(c, by, b)
}

func scribbleViewJSON(s *scribbleState, viewer uuid.UUID, spectator bool) string {
	b, _ := json.Marshal(s.View(viewer, spectator))
	return string(b)
}

func TestScribbleChoosingAutoPick(t *testing.T) {
	s, ps, now := scribbleStart(t, 3)
	if s.Phase != scribbleChoosing || s.Drawer != ps[0] || len(s.Choices) != 3 || s.Turns != 6 {
		t.Fatalf("bad start: %+v", s)
	}
	choices := s.Choices
	if strings.Contains(scribbleViewJSON(s, ps[1], false), choices[0]) {
		t.Fatal("guesser sees word choices")
	}
	if s.Tick(scribbleCtx(now.Add(5 * time.Second))) {
		t.Fatal("ticked before the choosing deadline")
	}
	if !s.Tick(scribbleCtx(now.Add(scribbleChooseTime))) {
		t.Fatal("no auto-pick")
	}
	if s.Phase != scribbleDrawing || s.Word == "" || !strings.Contains(strings.Join(choices, "|"), s.Word) {
		t.Fatalf("auto-pick: phase %s word %q of %v", s.Phase, s.Word, choices)
	}
}

func TestScribbleOnlyDrawerDraws(t *testing.T) {
	s, ps, now := scribbleStart(t, 3)
	c := scribbleCtx(now)
	stroke := map[string]any{"type": "stroke", "points": [][2]int{{10, 10}, {500, 2000}}, "color": "#e53935", "width": 5}
	if _, err := scribblePlay(t, s, c, ps[0], stroke); err == nil {
		t.Fatal("drew while choosing")
	}
	if _, err := scribblePlay(t, s, c, ps[1], map[string]any{"type": "choose", "index": 0}); err == nil {
		t.Fatal("non-drawer chose")
	}
	if _, err := scribblePlay(t, s, c, ps[0], map[string]any{"type": "choose", "index": 1}); err != nil {
		t.Fatal(err)
	}
	s = scribbleRoundTrip(t, s)
	if _, err := scribblePlay(t, s, c, ps[1], stroke); err == nil {
		t.Fatal("guesser drew")
	}
	live, err := scribblePlay(t, s, c, ps[0], stroke)
	if err != nil || live == nil || live.Type != "draw" {
		t.Fatalf("drawer stroke: %v %v", live, err)
	}
	if len(s.Strokes) != 1 || s.Strokes[0].Points[1] != [2]int{500, 1000} || s.Strokes[0].Color != "#E53935" {
		t.Fatalf("stroke stored wrong: %+v", s.Strokes)
	}
	bad := map[string]any{"type": "stroke", "points": [][2]int{{1, 1}}, "color": "#123456", "width": 5}
	if _, err := scribblePlay(t, s, c, ps[0], bad); err == nil {
		t.Fatal("off-palette colour accepted")
	}
	// A drag sent in pieces is undone as one.
	cont := map[string]any{"type": "stroke", "points": [][2]int{{1, 1}, {2, 2}}, "color": "#111111", "width": 2}
	scribblePlay(t, s, c, ps[0], cont)
	cont["cont"] = true
	scribblePlay(t, s, c, ps[0], cont)
	if _, err := scribblePlay(t, s, c, ps[0], map[string]any{"type": "undo"}); err != nil || len(s.Strokes) != 1 {
		t.Fatalf("undo left %d strokes (%v)", len(s.Strokes), err)
	}
	if _, err := scribblePlay(t, s, c, ps[0], map[string]any{"type": "clear"}); err != nil || len(s.Strokes) != 0 {
		t.Fatal("clear failed")
	}
}

func TestScribbleGuessScoringAndSecrecy(t *testing.T) {
	s, ps, now := scribbleStart(t, 3)
	c := scribbleCtx(now)
	scribblePlay(t, s, c, ps[0], map[string]any{"type": "choose", "index": 0})
	s = scribbleRoundTrip(t, s)
	word := s.Word
	if !strings.Contains(scribbleViewJSON(s, ps[0], false), `"word":"`+word+`"`) {
		t.Fatal("drawer can't see the word")
	}
	for _, v := range []string{scribbleViewJSON(s, ps[1], false), scribbleViewJSON(s, ps[2], false), scribbleViewJSON(s, uuid.New(), true)} {
		if strings.Contains(v, `"word"`) {
			t.Fatal("word leaked before guessing")
		}
	}

	// A wrong guess is public; drawers can't guess.
	if _, err := scribblePlay(t, s, c, ps[0], map[string]any{"type": "guess", "text": word}); err == nil {
		t.Fatal("drawer guessed")
	}
	scribblePlay(t, s, c, ps[2], map[string]any{"type": "guess", "text": "zzzz"})
	if !strings.Contains(scribbleViewJSON(s, ps[1], false), "zzzz") {
		t.Fatal("wrong guess not in the feed")
	}

	// Correct 20s in: 50 + 100*60/80 = 125; the drawer gets 25.
	c2 := scribbleCtx(now.Add(20 * time.Second))
	if _, err := scribblePlay(t, s, c2, ps[1], map[string]any{"type": "guess", "text": "  " + strings.ToUpper(word) + "!"}); err != nil {
		t.Fatal(err)
	}
	s = scribbleRoundTrip(t, s)
	if s.Score[ps[1]] != 125 || s.Score[ps[0]] != 25 || s.Phase != scribbleDrawing {
		t.Fatalf("scores %v phase %s", s.Score, s.Phase)
	}
	other := scribbleViewJSON(s, ps[2], false)
	if strings.Contains(other, `"word"`) || strings.Contains(other, strings.ToUpper(word)) {
		t.Fatal("the word leaked to another guesser")
	}
	if !strings.Contains(other, `"kind":"correct"`) {
		t.Fatal("no 'guessed it' line")
	}
	if _, err := scribblePlay(t, s, c2, ps[1], map[string]any{"type": "guess", "text": "it was " + word}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(scribbleViewJSON(s, ps[2], false), "it was") || !strings.Contains(scribbleViewJSON(s, ps[0], false), "it was") {
		t.Fatal("chat after guessing should reach only those who know the word")
	}

	// Last guesser gets it: the turn ends and everyone sees the word.
	scribblePlay(t, s, c2, ps[2], map[string]any{"type": "guess", "text": word})
	if s.Phase != scribbleReveal || !strings.Contains(scribbleViewJSON(s, uuid.New(), true), `"word":"`+word+`"`) {
		t.Fatalf("turn didn't end into reveal: %s", s.Phase)
	}
}

func TestScribbleCloseGuess(t *testing.T) {
	s, ps, now := scribbleStart(t, 3)
	c := scribbleCtx(now)
	s.choose(c, "samosa")
	scribblePlay(t, s, c, ps[1], map[string]any{"type": "guess", "text": "samosas"})
	if !strings.Contains(scribbleViewJSON(s, ps[1], false), `"kind":"close"`) || strings.Contains(scribbleViewJSON(s, ps[2], false), `"kind":"close"`) {
		t.Fatal("'so close' should go only to the guesser")
	}
}

func TestScribbleHintsTurnsAndGameOver(t *testing.T) {
	s, ps, now := scribbleStart(t, 2)
	c := scribbleCtx(now)
	s.choose(c, "elephant")
	if d := s.Deadline(); !d.Equal(now.Add(40 * time.Second)) {
		t.Fatalf("first deadline should be the first hint, got %v", d)
	}
	s.Tick(scribbleCtx(now.Add(61 * time.Second)))
	if strings.Count(s.hint(), "_") != 6 {
		t.Fatalf("want two letters revealed, hint %q", s.hint())
	}
	at := now.Add(scribbleDrawTime)
	s.Tick(scribbleCtx(at))
	if s.Phase != scribbleReveal {
		t.Fatalf("time up should reveal, got %s", s.Phase)
	}
	at = at.Add(scribbleRevealTime)
	s.Tick(scribbleCtx(at))
	if s.Phase != scribbleChoosing || s.Drawer != ps[1] || s.Turn != 1 || len(s.Strokes) != 0 {
		t.Fatalf("turn didn't advance: %s turn %d", s.Phase, s.Turn)
	}
	// Two players, two cycles: four turns in all, then over.
	for s.Phase != scribbleOver {
		at = at.Add(time.Minute)
		s.Tick(scribbleCtx(at))
		s = scribbleRoundTrip(t, s)
	}
	if s.Turn != 4 || s.Deadline() != nil || len(s.Used) != 4 {
		t.Fatalf("game over at turn %d", s.Turn)
	}
	if _, err := scribblePlay(t, s, c, ps[0], map[string]any{"type": "guess", "text": "x"}); err == nil {
		t.Fatal("move after game over")
	}
}

func TestScribbleDrawerLeaves(t *testing.T) {
	s, ps, now := scribbleStart(t, 3)
	c := scribbleCtx(now)
	scribblePlay(t, s, c, ps[0], map[string]any{"type": "choose", "index": 0})
	if !s.Leave(c, ps[0]) || s.Phase != scribbleReveal {
		t.Fatal("drawer leaving should end the turn")
	}
	s.Tick(scribbleCtx(now.Add(scribbleRevealTime)))
	if s.Drawer != ps[1] {
		t.Fatal("next drawer wrong")
	}
	s.Leave(c, ps[2])
	if !s.Over() {
		t.Fatal("one player left should end the game")
	}
}
