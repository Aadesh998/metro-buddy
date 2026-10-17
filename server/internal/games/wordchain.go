package games

import (
	_ "embed"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Word Chain: players take turns by seat, each word starting with the last letter of the one before.
// A slow turn or three bad attempts costs a life; the last player with lives left gets a bonus.
type wordchainEngine struct{}

func init() { Register(wordchainEngine{}) }

const (
	wordchainLives       = 3
	wordchainStrikes     = 3
	wordchainMaxWords    = 60
	wordchainMinLen      = 3
	wordchainMaxLen      = 30
	wordchainChainShown  = 40
	wordchainFirstTurn   = 15 * time.Second
	wordchainMinTurn     = 7 * time.Second
	wordchainWordPoints  = 10
	wordchainSurvivorPts = 50
)

//go:embed words/wordchain.txt
var wordchainList string

// wordchainDict is every playable word, loaded once.
var wordchainDict = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Split(wordchainList, "\n") {
		if w = strings.TrimSpace(w); w != "" {
			m[w] = true
		}
	}
	return m
}()

// Starting letters with plenty of words behind them.
const wordchainStarts = "abcdefghijklmnoprstuvwy"

func (wordchainEngine) Name() string            { return "wordchain" }
func (wordchainEngine) Players() (min, max int) { return 2, 8 }

type wordchainLink struct {
	Word string    `json:"word"`
	By   uuid.UUID `json:"by"`
}

type wordchainState struct {
	Players  []uuid.UUID        `json:"players"`
	Lives    map[uuid.UUID]int  `json:"lives"`
	Out      map[uuid.UUID]bool `json:"out"`
	Score    map[uuid.UUID]int  `json:"score"`
	Chain    []wordchainLink    `json:"chain"`
	Letter   string             `json:"letter"`
	Turn     int                `json:"turn"` // seat of the player who must answer
	TurnEnds time.Time          `json:"turn_ends"`
	Round    int                `json:"round"` // full rounds completed; each one shortens the timer
	Strikes  int                `json:"strikes"`
	Refusal  string             `json:"refusal"` // why the current player's last attempt failed
	Code     string             `json:"code"`    // the same, as a code the app translates
	Winner   *uuid.UUID         `json:"winner,omitempty"`
	Done     bool               `json:"done"`
}

func (wordchainEngine) Start(c Ctx, players []uuid.UUID) (State, error) {
	st := &wordchainState{
		Players: players, Lives: map[uuid.UUID]int{}, Out: map[uuid.UUID]bool{}, Score: map[uuid.UUID]int{},
		Chain: []wordchainLink{}, Letter: string(wordchainStarts[c.Rand.IntN(len(wordchainStarts))]), Turn: c.Rand.IntN(len(players)),
	}
	for _, p := range players {
		st.Lives[p] = wordchainLives
		st.Score[p] = 0
	}
	st.TurnEnds = c.Now.Add(st.turnLength())
	return st, nil
}

func (wordchainEngine) Load(data []byte) (State, error) {
	var st wordchainState
	return &st, json.Unmarshal(data, &st)
}

func (s *wordchainState) turnLength() time.Duration {
	return max(wordchainMinTurn, wordchainFirstTurn-time.Duration(s.Round)*time.Second)
}

func (s *wordchainState) seat(p uuid.UUID) int {
	for i, x := range s.Players {
		if x == p {
			return i
		}
	}
	return -1
}

func (s *wordchainState) current() uuid.UUID { return s.Players[s.Turn] }

func (s *wordchainState) alive() []uuid.UUID {
	out := []uuid.UUID{}
	for _, p := range s.Players {
		if !s.Out[p] {
			out = append(out, p)
		}
	}
	return out
}

// advance hands the turn to the next player still in; passing the last seat completes a round.
func (s *wordchainState) advance(c Ctx) {
	n := len(s.Players)
	for i := 1; i <= n; i++ {
		if s.Turn+i == n {
			s.Round++
		}
		next := (s.Turn + i) % n
		if !s.Out[s.Players[next]] {
			s.Turn = next
			break
		}
	}
	s.Strikes, s.Refusal, s.Code = 0, "", ""
	s.TurnEnds = c.Now.Add(s.turnLength())
}

// finish ends the game if at most one player is left, or the chain is long enough.
func (s *wordchainState) finish() bool {
	left := s.alive()
	if len(left) <= 1 {
		if len(left) == 1 {
			s.Score[left[0]] += wordchainSurvivorPts
			s.Winner = &left[0]
		}
		s.Done = true
	} else if len(s.Chain) >= wordchainMaxWords {
		s.Done = true
	}
	if s.Done {
		s.Strikes, s.Refusal, s.Code = 0, "", ""
	}
	return s.Done
}

// loseLife costs the current player a life and moves play on.
func (s *wordchainState) loseLife(c Ctx) {
	p := s.current()
	s.Lives[p]--
	if s.Lives[p] <= 0 {
		s.Lives[p] = 0
		s.Out[p] = true
	}
	if !s.finish() {
		s.advance(c)
	}
}

type wordchainMove struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (s *wordchainState) Move(c Ctx, by uuid.UUID, raw json.RawMessage) (*Live, error) {
	if s.Done {
		return nil, Refuse("the game is over")
	}
	seat := s.seat(by)
	if seat < 0 || s.Out[by] {
		return nil, Refuse("you're out of this game")
	}
	if seat != s.Turn {
		return nil, Refuse("it's not your turn")
	}
	m, err := Decode[wordchainMove](raw)
	if err != nil {
		return nil, err
	}
	if m.Type != "word" {
		return nil, Refuse("that move doesn't make sense")
	}
	if len(m.Text) > 200 {
		return nil, Refuse("that word is too long")
	}
	word := strings.ToLower(strings.TrimSpace(c.Mask(m.Text)))
	if word == "" {
		return nil, Refuse("type a word")
	}
	if code, reason := s.check(word); code != "" {
		// A bad attempt is saved (not refused) so strikes persist; only the player sees why.
		s.Strikes++
		s.Code, s.Refusal = code, reason
		if s.Strikes >= wordchainStrikes {
			s.loseLife(c)
		}
		return &Live{Type: "refused", Data: map[string]any{"reason": reason, "code": code, "strikes": s.Strikes}, To: []uuid.UUID{by}}, nil
	}
	s.Chain = append(s.Chain, wordchainLink{Word: word, By: by})
	s.Score[by] += wordchainWordPoints + len(word) - wordchainMinLen
	s.Letter = word[len(word)-1:]
	if !s.finish() {
		s.advance(c)
	}
	return &Live{Type: "word", Data: map[string]any{"word": word, "by": by, "seat": seat, "accepted": true, "letter": s.Letter}}, nil
}

// check returns a code and message for why a word can't be played, or "" if it can.
func (s *wordchainState) check(word string) (code, msg string) {
	if len(word) > wordchainMaxLen {
		return "long", "that word is too long"
	}
	for _, r := range word {
		if r < 'a' || r > 'z' {
			return "letters", "letters a to z only"
		}
	}
	if len(word) < wordchainMinLen {
		return "short", "at least 3 letters"
	}
	if word[:1] != s.Letter {
		return "letter", "must start with " + strings.ToUpper(s.Letter)
	}
	for _, l := range s.Chain {
		if l.Word == word {
			return "used", "already used"
		}
	}
	if !wordchainDict[word] {
		return "unknown", "not in our word list"
	}
	return "", ""
}

func (s *wordchainState) Tick(c Ctx) bool {
	if s.Done || c.Now.Before(s.TurnEnds) {
		return false
	}
	s.loseLife(c)
	return true
}

func (s *wordchainState) Deadline() *time.Time {
	if s.Done {
		return nil
	}
	return &s.TurnEnds
}

func (s *wordchainState) View(viewer uuid.UUID, spectator bool) any {
	chain := s.Chain
	if len(chain) > wordchainChainShown {
		chain = chain[len(chain)-wordchainChainShown:]
	}
	out := []uuid.UUID{}
	for _, p := range s.Players {
		if s.Out[p] {
			out = append(out, p)
		}
	}
	v := map[string]any{
		"players": s.Players, "chain": chain, "letter": s.Letter, "turn": s.current(), "turn_ends_at": s.TurnEnds,
		"turn_seconds": int(s.turnLength() / time.Second), "round": s.Round, "lives": s.Lives, "max_lives": wordchainLives,
		"out": out, "used": len(s.Chain), "max_words": wordchainMaxWords, "score": s.Score, "over": s.Done, "winner": s.Winner,
	}
	// Failed attempts stay private to the player making them.
	if !spectator && viewer == s.current() && !s.Done {
		v["strikes"] = s.Strikes
		v["max_strikes"] = wordchainStrikes
		if s.Refusal != "" {
			v["refusal"] = s.Refusal
			v["refusal_code"] = s.Code
		}
	}
	return v
}

func (s *wordchainState) Scores() map[uuid.UUID]int { return s.Score }
func (s *wordchainState) Over() bool                { return s.Done }

func (s *wordchainState) Leave(c Ctx, who uuid.UUID) bool {
	seat := s.seat(who)
	if s.Done || seat < 0 || s.Out[who] {
		return false
	}
	s.Out[who] = true
	s.Lives[who] = 0
	if !s.finish() && seat == s.Turn {
		s.advance(c)
	}
	return true
}
