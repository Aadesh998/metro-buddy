package games

import (
	"encoding/json"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

var wordchainT0 = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

func wordchainCtx(at time.Time) Ctx {
	return Ctx{Now: at, Rand: rand.New(rand.NewPCG(1, 2)), Mask: func(s string) string { return s }}
}

// wordchainReload round-trips the state through JSON, as the service does between calls.
func wordchainReload(t *testing.T, s *wordchainState) *wordchainState {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	st, err := wordchainEngine{}.Load(b)
	if err != nil {
		t.Fatal(err)
	}
	return st.(*wordchainState)
}

// wordchainGame starts n players with seat 0 to answer an R.
func wordchainGame(t *testing.T, n int) (*wordchainState, []uuid.UUID) {
	t.Helper()
	ps := make([]uuid.UUID, n)
	for i := range ps {
		ps[i] = uuid.New()
	}
	st, err := wordchainEngine{}.Start(wordchainCtx(wordchainT0), ps)
	if err != nil {
		t.Fatal(err)
	}
	s := st.(*wordchainState)
	if strings.ContainsAny(s.Letter, "qxz") {
		t.Fatalf("bad starting letter %q", s.Letter)
	}
	s.Letter, s.Turn = "r", 0
	return wordchainReload(t, s), ps
}

func wordchainSay(t *testing.T, s *wordchainState, at time.Time, by uuid.UUID, word string) (*wordchainState, *Live, error) {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"type": "word", "text": word})
	live, err := s.Move(wordchainCtx(at), by, raw)
	return wordchainReload(t, s), live, err
}

func TestWordchainDictionary(t *testing.T) {
	if n := len(wordchainDict); n < 8000 || n > 15000 {
		t.Fatalf("dictionary has %d words", n)
	}
	for _, w := range []string{"river", "banana", "yellow", "zebra"} {
		if !wordchainDict[w] {
			t.Errorf("%q missing", w)
		}
	}
}

func TestWordchainFirstLetterAndScoring(t *testing.T) {
	s, ps := wordchainGame(t, 2)
	if _, err := s.Move(wordchainCtx(wordchainT0), ps[1], json.RawMessage(`{"type":"word","text":"rain"}`)); err == nil {
		t.Fatal("out-of-turn move accepted")
	}
	s, live, err := wordchainSay(t, s, wordchainT0, ps[0], "apple")
	if err != nil || live.Type != "refused" || s.Refusal != "must start with R" || s.Code != "letter" || s.Strikes != 1 || s.Turn != 0 {
		t.Fatalf("wrong letter: %v %+v %q %d", err, live, s.Refusal, s.Strikes)
	}
	// Another player never sees the refusal.
	if _, ok := s.View(ps[1], false).(map[string]any)["refusal"]; ok {
		t.Fatal("refusal leaked to another player")
	}
	if v := s.View(ps[0], false).(map[string]any); v["refusal"] != "must start with R" {
		t.Fatalf("own refusal hidden: %v", v["refusal"])
	}
	s, live, err = wordchainSay(t, s, wordchainT0, ps[0], "  River ")
	if err != nil || live.Type != "word" {
		t.Fatalf("river: %v %+v", err, live)
	}
	if s.Score[ps[0]] != 12 || s.Letter != "r" || s.Turn != 1 || s.Strikes != 0 || s.Refusal != "" {
		t.Fatalf("after river: score %d letter %q turn %d", s.Score[ps[0]], s.Letter, s.Turn)
	}
	if !s.TurnEnds.Equal(wordchainT0.Add(15 * time.Second)) {
		t.Fatalf("first round timer: %v", s.TurnEnds.Sub(wordchainT0))
	}
}

func TestWordchainReuseAndUnknownRefused(t *testing.T) {
	s, ps := wordchainGame(t, 2)
	s, _, _ = wordchainSay(t, s, wordchainT0, ps[0], "river")
	s, _, _ = wordchainSay(t, s, wordchainT0, ps[1], "river")
	if s.Refusal != "already used" {
		t.Fatalf("reuse: %q", s.Refusal)
	}
	s, _, _ = wordchainSay(t, s, wordchainT0, ps[1], "rzzqx")
	if s.Refusal != "not in our word list" || s.Strikes != 2 {
		t.Fatalf("unknown: %q %d", s.Refusal, s.Strikes)
	}
	// The third bad attempt costs a life and passes the turn.
	s, _, _ = wordchainSay(t, s, wordchainT0, ps[1], "r2d2")
	if s.Lives[ps[1]] != 2 || s.Turn != 0 || s.Strikes != 0 || s.Letter != "r" {
		t.Fatalf("third strike: lives %d turn %d", s.Lives[ps[1]], s.Turn)
	}
}

func TestWordchainTimeoutCostsLife(t *testing.T) {
	s, ps := wordchainGame(t, 3)
	if s.Tick(wordchainCtx(wordchainT0.Add(14 * time.Second))) {
		t.Fatal("ticked early")
	}
	at := wordchainT0.Add(15 * time.Second)
	if !s.Tick(wordchainCtx(at)) {
		t.Fatal("timeout ignored")
	}
	s = wordchainReload(t, s)
	if s.Lives[ps[0]] != 2 || s.Turn != 1 || !s.Deadline().Equal(at.Add(15*time.Second)) {
		t.Fatalf("after timeout: lives %d turn %d", s.Lives[ps[0]], s.Turn)
	}
}

func TestWordchainEliminationAndWinner(t *testing.T) {
	s, ps := wordchainGame(t, 3)
	want := 0
	// Seat 0 always answers; seats 1 and 2 keep timing out.
	for i := 0; !s.Done; i++ {
		if i > 20 {
			t.Fatal("game never ended")
		}
		if s.Turn != 0 {
			s.Tick(wordchainCtx(s.TurnEnds))
			s = wordchainReload(t, s)
			continue
		}
		w := wordchainPick(t, s)
		var err error
		if s, _, err = wordchainSay(t, s, s.TurnEnds.Add(-time.Second), ps[0], w); err != nil {
			t.Fatal(err)
		}
		want += wordchainWordPoints + len(w) - wordchainMinLen
	}
	if !s.Out[ps[1]] || !s.Out[ps[2]] || s.Lives[ps[1]] != 0 || s.Winner == nil || *s.Winner != ps[0] {
		t.Fatalf("winner %v out %v", s.Winner, s.Out)
	}
	if want += wordchainSurvivorPts; s.Score[ps[0]] != want || s.Deadline() != nil {
		t.Fatalf("score %d want %d", s.Score[ps[0]], want)
	}
	// Timers shrink by a second each full round.
	if s.Round == 0 || s.turnLength() != wordchainFirstTurn-time.Duration(s.Round)*time.Second {
		t.Fatalf("round %d length %v", s.Round, s.turnLength())
	}
	if _, _, err := wordchainSay(t, s, s.TurnEnds, ps[0], "rope"); err == nil {
		t.Fatal("move after the end accepted")
	}
}

// wordchainPick finds an unused word for the required letter.
func wordchainPick(t *testing.T, s *wordchainState) string {
	t.Helper()
	for w := range wordchainDict {
		if code, _ := s.check(w); code == "" {
			return w
		}
	}
	t.Fatalf("no word for %q", s.Letter)
	return ""
}

func TestWordchainChainLimit(t *testing.T) {
	s, ps := wordchainGame(t, 2)
	for i := 0; !s.Done; i++ {
		if i > wordchainMaxWords {
			t.Fatal("chain never ended")
		}
		var err error
		if s, _, err = wordchainSay(t, s, s.TurnEnds.Add(-time.Second), ps[s.Turn], wordchainPick(t, s)); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.Chain) != wordchainMaxWords || s.Winner != nil {
		t.Fatalf("chain %d winner %v", len(s.Chain), s.Winner)
	}
	if v := s.View(ps[0], false).(map[string]any); len(v["chain"].([]wordchainLink)) != wordchainChainShown || v["used"] != wordchainMaxWords {
		t.Fatalf("view chain %d", len(v["chain"].([]wordchainLink)))
	}
	if s.turnLength() != wordchainMinTurn {
		t.Fatalf("timer never reached its floor: %v", s.turnLength())
	}
}

func TestWordchainLeave(t *testing.T) {
	s, ps := wordchainGame(t, 3)
	if !s.Leave(wordchainCtx(wordchainT0), ps[0]) || s.Turn != 1 || !s.Out[ps[0]] || s.Done {
		t.Fatalf("leave on turn: turn %d", s.Turn)
	}
	s = wordchainReload(t, s)
	if !s.Leave(wordchainCtx(wordchainT0), ps[2]) || !s.Done || *s.Winner != ps[1] || s.Score[ps[1]] != 50 {
		t.Fatalf("last leave: done %v score %d", s.Done, s.Score[ps[1]])
	}
}
