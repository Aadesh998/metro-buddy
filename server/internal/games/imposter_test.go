package games

import (
	"encoding/json"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

var imposterT0 = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

func imposterCtx(at time.Time) Ctx {
	return Ctx{Now: at, Rand: rand.New(rand.NewPCG(1, 2)), Mask: func(s string) string { return s }}
}

// imposterGame starts a game and makes the second clue giver the imposter, so tests know who is who.
func imposterGame(t *testing.T, n int) (*imposterState, []uuid.UUID) {
	t.Helper()
	ps := make([]uuid.UUID, n)
	for i := range ps {
		ps[i] = uuid.New()
	}
	st, err := imposterEngine{}.Start(imposterCtx(imposterT0), ps)
	if err != nil {
		t.Fatal(err)
	}
	s := st.(*imposterState)
	s.Imposters = []uuid.UUID{s.Order[1]}
	return reloadImposter(t, s), ps
}

// reloadImposter round-trips through JSON, as the service does between calls.
func reloadImposter(t *testing.T, s *imposterState) *imposterState {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	st, err := imposterEngine{}.Load(b)
	if err != nil {
		t.Fatal(err)
	}
	return st.(*imposterState)
}

func imposterMv(t *testing.T, s *imposterState, at time.Time, by uuid.UUID, m map[string]any) error {
	t.Helper()
	b, _ := json.Marshal(m)
	_, err := s.Move(imposterCtx(at), by, b)
	return err
}

func imposterViewJSON(t *testing.T, s *imposterState, viewer uuid.UUID, spectator bool) (string, map[string]any) {
	t.Helper()
	b, err := json.Marshal(s.View(viewer, spectator))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	return string(b), m
}

// imposterToVote skips the clue and discussion phases by letting their timers run out.
func imposterToVote(t *testing.T, s *imposterState) time.Time {
	t.Helper()
	at := s.Due
	for s.Phase != imposterPhaseVote {
		if !s.Tick(imposterCtx(at)) {
			t.Fatalf("tick did nothing in %s", s.Phase)
		}
		at = s.Due
	}
	return at.Add(-time.Second)
}

func TestImposterViewHidesRoles(t *testing.T) {
	s, _ := imposterGame(t, 4)
	imp, crew := s.Order[1], s.Order[0]
	word := `"` + s.Word + `"`

	raw, v := imposterViewJSON(t, s, imp, false)
	if v["role"] != "imposter" || strings.Contains(raw, word) || v["imposters"] != nil {
		t.Fatalf("imposter view leaks: %s", raw)
	}
	if v["category"] != s.Category {
		t.Fatal("imposter should see the category")
	}
	raw, v = imposterViewJSON(t, s, crew, false)
	if v["role"] != "crew" || v["word"] != s.Word {
		t.Fatalf("crew view wrong: %s", raw)
	}
	if _, ok := v["imposters"]; ok {
		t.Fatal("crew can see who the imposter is")
	}
	for _, spectator := range []bool{true, false} {
		// A spectator, or someone not in the game, sees only public info.
		raw, v = imposterViewJSON(t, s, uuid.New(), spectator)
		if strings.Contains(raw, word) || v["role"] != nil || v["imposters"] != nil {
			t.Fatalf("spectator view leaks: %s", raw)
		}
	}
	// Even a player marked as spectating gets nothing private.
	if _, v = imposterViewJSON(t, s, crew, true); v["word"] != nil {
		t.Fatal("spectator flag ignored")
	}
}

func TestImposterClueEqualToWordRefused(t *testing.T) {
	s, _ := imposterGame(t, 4)
	at := imposterT0.Add(time.Second)
	crew, imp := s.Order[0], s.Order[1]
	if err := imposterMv(t, s, at, imp, map[string]any{"type": "clue", "text": "hm"}); err == nil {
		t.Fatal("clue out of turn accepted")
	}
	for _, text := range []string{strings.ToUpper(s.Word), " " + s.Word + "!"} {
		if err := imposterMv(t, s, at, crew, map[string]any{"type": "clue", "text": text}); err == nil {
			t.Fatalf("clue %q equal to the word accepted", text)
		}
	}
	if err := imposterMv(t, s, at, crew, map[string]any{"type": "clue", "text": strings.Repeat("x", 31)}); err == nil {
		t.Fatal("long clue accepted")
	}
	if err := imposterMv(t, s, at, crew, map[string]any{"type": "clue", "text": "tasty"}); err != nil {
		t.Fatal(err)
	}
	if len(s.Clues) != 1 || s.Order[s.Turn] != imp || !s.Due.Equal(at.Add(imposterClueTime)) {
		t.Fatalf("turn didn't pass: %+v", s)
	}
	// The imposter typing the word is not refused: a refusal would tell them the word.
	if err := imposterMv(t, s, at, imp, map[string]any{"type": "clue", "text": s.Word}); err != nil {
		t.Fatal(err)
	}
}

func TestImposterCaughtAndFailedGuess(t *testing.T) {
	s, ps := imposterGame(t, 4)
	imp := s.Order[1]
	at := imposterToVote(t, s)
	s = reloadImposter(t, s)
	if err := imposterMv(t, s, at, ps[0], map[string]any{"type": "vote", "target": ps[0].String()}); err == nil {
		t.Fatal("self vote accepted")
	}
	for _, p := range ps {
		target := imp
		if p == imp {
			target = s.Order[0]
		}
		if err := imposterMv(t, s, at, p, map[string]any{"type": "vote", "target": target.String()}); err != nil && p != ps[len(ps)-1] {
			t.Fatal(err)
		}
		if p != ps[len(ps)-1] {
			// Who voted is public, but not for whom.
			_, v := imposterViewJSON(t, s, uuid.New(), true)
			if v["votes"] != nil || len(v["voted"].([]any)) == 0 {
				t.Fatalf("vote view wrong: %v", v)
			}
		}
	}
	// Everyone voted, so the vote ended early and the caught imposter gets a last guess.
	if s.Phase != imposterPhaseGuess || *s.Result.Out != imp {
		t.Fatalf("phase %s", s.Phase)
	}
	raw, _ := imposterViewJSON(t, s, imp, false)
	if strings.Contains(raw, `"`+s.Word+`"`) {
		t.Fatal("word shown to the imposter before their guess")
	}
	if err := imposterMv(t, s, at, s.Order[0], map[string]any{"type": "guess", "text": "x"}); err == nil {
		t.Fatal("crew allowed to guess")
	}
	if err := imposterMv(t, s, at, imp, map[string]any{"type": "guess", "text": "definitely not it"}); err != nil {
		t.Fatal(err)
	}
	if s.Phase != imposterPhaseReveal || s.Result.Winner != "crew" {
		t.Fatalf("got %s %+v", s.Phase, s.Result)
	}
	for _, p := range ps {
		want := imposterCrewPoints
		if p == imp {
			want = 0
		}
		if s.Score[p] != want {
			t.Errorf("score %d, want %d", s.Score[p], want)
		}
	}
	_, v := imposterViewJSON(t, s, uuid.New(), true)
	if v["word"] != s.Word || v["votes"] == nil || v["imposters"] == nil {
		t.Fatal("reveal should be public")
	}
}

func TestImposterGuessesWord(t *testing.T) {
	s, ps := imposterGame(t, 3)
	imp := s.Order[1]
	at := imposterToVote(t, s)
	for _, p := range ps {
		if p != imp {
			imposterMv(t, s, at, p, map[string]any{"type": "vote", "target": imp.String()})
		}
	}
	// One vote missing: the deadline closes the vote.
	s.Tick(imposterCtx(s.Due))
	if s.Phase != imposterPhaseGuess {
		t.Fatalf("phase %s", s.Phase)
	}
	imposterMv(t, s, at, imp, map[string]any{"type": "guess", "text": strings.ToUpper(s.Word)})
	if s.Result.Winner != "imposter" || s.Score[imp] != imposterGuessedPoints {
		t.Fatalf("guess not rewarded: %+v %v", s.Result, s.Score)
	}
}

func TestImposterWinsWhenNotVotedOut(t *testing.T) {
	for _, votes := range []string{"skip", "tie"} {
		s, ps := imposterGame(t, 4)
		imp := s.Order[1]
		at := imposterToVote(t, s)
		for i, p := range ps {
			target := imposterSkip
			if votes == "tie" {
				target = ps[(i+1)%len(ps)].String()
			}
			if err := imposterMv(t, s, at, p, map[string]any{"type": "vote", "target": target}); err != nil {
				t.Fatal(err)
			}
		}
		if s.Phase != imposterPhaseReveal || s.Result.Out != nil || s.Result.Winner != "imposter" {
			t.Fatalf("%s: got %s %+v", votes, s.Phase, s.Result)
		}
		for _, p := range ps {
			want := 0
			if p == imp {
				want = imposterHiddenPoints
			}
			if s.Score[p] != want {
				t.Errorf("%s: score %d want %d", votes, s.Score[p], want)
			}
		}
	}
}

func TestImposterTimeoutsAdvanceThroughRounds(t *testing.T) {
	s, _ := imposterGame(t, 3)
	if s.Tick(imposterCtx(s.Due.Add(-time.Second))) {
		t.Fatal("ticked before the deadline")
	}
	phases := []string{}
	for !s.Over() {
		at := *s.Deadline()
		if !s.Tick(imposterCtx(at)) {
			t.Fatal("tick did nothing at the deadline")
		}
		s = reloadImposter(t, s)
		if len(phases) == 0 || phases[len(phases)-1] != s.Phase {
			phases = append(phases, s.Phase)
		}
		if len(phases) > 50 {
			t.Fatal("never ends")
		}
	}
	got := strings.Join(phases, ",")
	want := "clue,discuss,vote,reveal,clue,discuss,vote,reveal,clue,discuss,vote,reveal,over"
	if got != want {
		t.Fatalf("phases %s", got)
	}
	if s.Round != imposterRounds || s.Deadline() != nil {
		t.Fatal("game should be over")
	}
	if len(s.Clues) != 3 || !s.Clues[0].Skipped {
		t.Fatal("timed out turns should be recorded as skipped")
	}
}

func TestImposterLeaving(t *testing.T) {
	s, _ := imposterGame(t, 5)
	imp := s.Order[1]
	// The current clue giver leaves: the next one gets a fresh turn.
	at := imposterT0.Add(20 * time.Second)
	first := s.Order[0]
	if !s.Leave(imposterCtx(at), first) {
		t.Fatal("leave did nothing")
	}
	if s.Order[s.Turn] != imp || !s.Due.Equal(at.Add(imposterClueTime)) || s.Phase != imposterPhaseClue {
		t.Fatalf("turn after leave: %v %v", s.Order[s.Turn], s.Due)
	}
	// The imposter walks out: the round goes to the crew.
	s.Leave(imposterCtx(at), imp)
	if s.Phase != imposterPhaseReveal || s.Result.Winner != "crew" || s.Result.Reason != "imposter_left" {
		t.Fatalf("got %s %+v", s.Phase, s.Result)
	}
	for _, p := range s.Players {
		if s.Score[p] != imposterCrewPoints {
			t.Errorf("crew score %d", s.Score[p])
		}
	}
	// Down to two players ends the game.
	s.Leave(imposterCtx(at), s.Players[0])
	if !s.Over() {
		t.Fatal("game should end below three players")
	}
}

func TestImposterChat(t *testing.T) {
	s, ps := imposterGame(t, 3)
	crew := s.Order[0]
	if err := imposterMv(t, s, imposterT0, crew, map[string]any{"type": "say", "text": "hi"}); err == nil {
		t.Fatal("chat during clues accepted")
	}
	at := imposterToVote(t, s)
	b, _ := json.Marshal(map[string]any{"type": "say", "text": "it was " + ps[1].String()[:4]})
	live, err := s.Move(imposterCtx(at), crew, b)
	if err != nil || live == nil || live.Type != "say" {
		t.Fatalf("say: %v %v", live, err)
	}
	if err := imposterMv(t, s, at, crew, map[string]any{"type": "say", "text": "the word is " + s.Word}); err == nil {
		t.Fatal("crew allowed to say the word")
	}
	for range 60 {
		imposterMv(t, s, at, crew, map[string]any{"type": "say", "text": "spam"})
	}
	if len(s.Chat) != imposterChatKeep {
		t.Fatalf("chat kept %d lines", len(s.Chat))
	}
}

func TestImposterWordsLoaded(t *testing.T) {
	if len(imposterCategories) < 20 {
		t.Fatalf("only %d categories", len(imposterCategories))
	}
	for _, c := range imposterCategories {
		if c.ID == "" || len(c.Words) < 10 {
			t.Errorf("category %q is thin", c.ID)
		}
	}
	// Eight players means two imposters.
	s, _ := imposterGame(t, 8)
	s.Round = 1
	s.newRound(imposterCtx(imposterT0))
	if len(s.Imposters) != 2 {
		t.Fatalf("%d imposters with 8 players", len(s.Imposters))
	}
}
