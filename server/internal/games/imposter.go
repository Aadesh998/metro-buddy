package games

import (
	_ "embed"
	"encoding/json"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Imposter: everyone gets the same secret word except the imposter(s), who only know the
// category. Players give one clue each, talk it over, then vote someone out. Only the server
// knows the roles until the reveal.
type imposterEngine struct{}

func init() { Register(imposterEngine{}) }

//go:embed words/imposter.json
var imposterWordsJSON []byte

type imposterCategory struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Words []string `json:"words"`
}

var imposterCategories = func() []imposterCategory {
	var cs []imposterCategory
	if err := json.Unmarshal(imposterWordsJSON, &cs); err != nil {
		panic("games: bad imposter words: " + err.Error())
	}
	return cs
}()

const (
	imposterRounds      = 3
	imposterClueTime    = 25 * time.Second
	imposterDiscussTime = 60 * time.Second
	imposterVoteTime    = 30 * time.Second
	imposterGuessTime   = 15 * time.Second
	imposterRevealTime  = 8 * time.Second
	imposterClueMax     = 30
	imposterSayMax      = 140
	imposterChatKeep    = 50
	// From this many players there are two imposters.
	imposterTwoAt = 8

	imposterCrewPoints     = 2
	imposterHiddenPoints   = 3
	imposterGuessedPoints  = 2
	imposterSkip           = "skip"
	imposterPhaseClue      = "clue"
	imposterPhaseDiscuss   = "discuss"
	imposterPhaseVote      = "vote"
	imposterPhaseGuess     = "guess"
	imposterPhaseReveal    = "reveal"
	imposterPhaseOver      = "over"
	imposterWinnerCrew     = "crew"
	imposterWinnerImposter = "imposter"
)

func (imposterEngine) Name() string            { return "imposter" }
func (imposterEngine) Players() (min, max int) { return 3, 10 }

type imposterClue struct {
	By      uuid.UUID `json:"by"`
	Text    string    `json:"text"`
	Skipped bool      `json:"skipped,omitempty"`
}

type imposterLine struct {
	By   uuid.UUID `json:"by"`
	Text string    `json:"text"`
}

// imposterResult is how a round ended; it stays in state until the next round starts.
type imposterResult struct {
	Out        *uuid.UUID        `json:"out"`
	OutRole    string            `json:"out_role,omitempty"`
	Guess      string            `json:"guess,omitempty"`
	GuessRight bool              `json:"guess_right"`
	Winner     string            `json:"winner"`
	Reason     string            `json:"reason"` // "voted", "imposter_left"
	Points     map[uuid.UUID]int `json:"points"`
}

type imposterState struct {
	// Players still in the game, in seat order.
	Players   []uuid.UUID          `json:"players"`
	Score     map[uuid.UUID]int    `json:"score"`
	Round     int                  `json:"round"`
	Phase     string               `json:"phase"`
	Due       time.Time            `json:"due"`
	Category  string               `json:"category"`
	Word      string               `json:"word"`
	Imposters []uuid.UUID          `json:"imposters"`
	Order     []uuid.UUID          `json:"order"` // clue order this round
	Turn      int                  `json:"turn"`  // index into Order during the clue phase
	Clues     []imposterClue       `json:"clues"`
	Chat      []imposterLine       `json:"chat"`
	Votes     map[uuid.UUID]string `json:"votes"` // voter -> persona id or "skip"
	Result    *imposterResult      `json:"result"`
	Used      []string             `json:"used"` // words already played, never repeated
}

func (imposterEngine) Start(c Ctx, players []uuid.UUID) (State, error) {
	st := &imposterState{Players: slices.Clone(players), Score: map[uuid.UUID]int{}}
	for _, p := range players {
		st.Score[p] = 0
	}
	st.newRound(c)
	return st, nil
}

func (imposterEngine) Load(data []byte) (State, error) {
	var st imposterState
	return &st, json.Unmarshal(data, &st)
}

// newRound picks a fresh word and imposter(s) and starts the clue phase.
func (s *imposterState) newRound(c Ctx) {
	s.Round++
	for range 50 {
		cat := imposterCategories[c.Rand.IntN(len(imposterCategories))]
		w := cat.Words[c.Rand.IntN(len(cat.Words))]
		s.Category, s.Word = cat.ID, w
		if !slices.Contains(s.Used, w) {
			break
		}
	}
	s.Used = append(s.Used, s.Word)
	n := 1
	if len(s.Players) >= imposterTwoAt {
		n = 2
	}
	s.Imposters = nil
	for _, i := range c.Rand.Perm(len(s.Players))[:n] {
		s.Imposters = append(s.Imposters, s.Players[i])
	}
	// Clues start one seat later each round.
	k := (s.Round - 1) % len(s.Players)
	s.Order = append(slices.Clone(s.Players[k:]), s.Players[:k]...)
	s.Turn = 0
	s.Clues, s.Chat, s.Votes, s.Result = []imposterClue{}, []imposterLine{}, map[uuid.UUID]string{}, nil
	s.Phase = imposterPhaseClue
	s.Due = c.Now.Add(imposterClueTime)
}

func (s *imposterState) isImposter(p uuid.UUID) bool { return slices.Contains(s.Imposters, p) }
func (s *imposterState) inGame(p uuid.UUID) bool     { return slices.Contains(s.Players, p) }

// imposterNorm folds text for comparing with the secret word: lower case, letters and digits only.
func imposterNorm(t string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(t) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// givesAway reports whether text spells out the word. Only checked for crew members: refusing an
// imposter's text would tell them the word.
func (s *imposterState) givesAway(by uuid.UUID, text string) bool {
	w := imposterNorm(s.Word)
	return !s.isImposter(by) && w != "" && strings.Contains(imposterNorm(text), w)
}

func imposterCleanText(t string, maxLen int) (string, bool) {
	t = strings.Join(strings.Fields(t), " ")
	if t == "" || utf8.RuneCountInString(t) > maxLen {
		return t, false
	}
	return t, true
}

type imposterMove struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Target string `json:"target"`
}

func (s *imposterState) Move(c Ctx, by uuid.UUID, raw json.RawMessage) (*Live, error) {
	if s.Over() {
		return nil, Refuse("the game is over")
	}
	if !s.inGame(by) {
		return nil, Refuse("you're not in this game")
	}
	m, err := Decode[imposterMove](raw)
	if err != nil {
		return nil, err
	}
	switch m.Type {
	case "clue":
		if s.Phase != imposterPhaseClue {
			return nil, Refuse("it's not time for clues")
		}
		if s.Order[s.Turn] != by {
			return nil, Refuse("wait for your turn")
		}
		text, ok := imposterCleanText(m.Text, imposterClueMax)
		if !ok {
			return nil, Refuse("give a short clue, up to 30 characters")
		}
		if s.givesAway(by, text) {
			return nil, Refuse("that gives the word away, try a subtler clue")
		}
		s.Clues = append(s.Clues, imposterClue{By: by, Text: c.Mask(text)})
		s.nextClue(c)
		return nil, nil
	case "say":
		if s.Phase != imposterPhaseDiscuss && s.Phase != imposterPhaseVote && s.Phase != imposterPhaseReveal {
			return nil, Refuse("chat opens after the clues")
		}
		text, ok := imposterCleanText(m.Text, imposterSayMax)
		if !ok {
			return nil, Refuse("keep messages under 140 characters")
		}
		if s.Phase != imposterPhaseReveal && s.givesAway(by, text) {
			return nil, Refuse("don't say the word, the imposter is listening")
		}
		line := imposterLine{By: by, Text: c.Mask(text)}
		s.Chat = append(s.Chat, line)
		if len(s.Chat) > imposterChatKeep {
			s.Chat = slices.Clone(s.Chat[len(s.Chat)-imposterChatKeep:])
		}
		return &Live{Type: "say", Data: line}, nil
	case "vote":
		if s.Phase != imposterPhaseVote {
			return nil, Refuse("voting hasn't started")
		}
		if m.Target != imposterSkip {
			t, err := uuid.Parse(m.Target)
			if err != nil || !s.inGame(t) {
				return nil, Refuse("vote for a player or skip")
			}
			if t == by {
				return nil, Refuse("you can't vote for yourself")
			}
		}
		s.Votes[by] = m.Target
		if len(s.Votes) >= len(s.Players) {
			s.endVote(c)
		}
		return nil, nil
	case "guess":
		if s.Phase != imposterPhaseGuess || s.Result == nil || s.Result.Out == nil || *s.Result.Out != by {
			return nil, Refuse("only the caught imposter can guess now")
		}
		text, ok := imposterCleanText(m.Text, imposterClueMax)
		if !ok {
			return nil, Refuse("guess the word in up to 30 characters")
		}
		s.Result.GuessRight = imposterNorm(text) == imposterNorm(s.Word)
		s.Result.Guess = c.Mask(text)
		s.finishRound(c)
		return nil, nil
	}
	return nil, Refuse("that move doesn't make sense")
}

// nextClue moves to the next clue giver, or on to the discussion when everyone has given one.
func (s *imposterState) nextClue(c Ctx) {
	s.Turn++
	if s.Turn >= len(s.Order) {
		s.Phase = imposterPhaseDiscuss
		s.Due = c.Now.Add(imposterDiscussTime)
		return
	}
	s.Due = c.Now.Add(imposterClueTime)
}

// endVote tallies: the most-voted player is out, unless there's a tie or skip has as many votes.
func (s *imposterState) endVote(c Ctx) {
	count := map[string]int{}
	for _, t := range s.Votes {
		count[t]++
	}
	top, best, tie := "", 0, false
	for t, n := range count {
		if t == imposterSkip {
			continue
		}
		switch {
		case n > best:
			top, best, tie = t, n, false
		case n == best:
			tie = true
		}
	}
	s.Result = &imposterResult{Reason: "voted"}
	if best > 0 && !tie && best > count[imposterSkip] {
		out := uuid.MustParse(top)
		s.Result.Out = &out
		s.Result.OutRole = imposterWinnerCrew
		if s.isImposter(out) {
			// A caught imposter gets one last chance: name the word.
			s.Result.OutRole = imposterWinnerImposter
			s.Phase = imposterPhaseGuess
			s.Due = c.Now.Add(imposterGuessTime)
			return
		}
	}
	s.finishRound(c)
}

// finishRound scores the round and shows the reveal.
func (s *imposterState) finishRound(c Ctx) {
	r := s.Result
	caught := r.Out != nil && s.isImposter(*r.Out)
	r.Points = map[uuid.UUID]int{}
	crewWins := caught && !r.GuessRight
	for _, p := range s.Players {
		switch {
		case !s.isImposter(p):
			if crewWins {
				r.Points[p] = imposterCrewPoints
			}
		case r.Reason == "imposter_left":
			// A partner walked out: the crew takes this round.
		case caught && *r.Out == p:
			if r.GuessRight {
				r.Points[p] = imposterGuessedPoints
			}
		default:
			r.Points[p] = imposterHiddenPoints
		}
	}
	r.Winner = imposterWinnerImposter
	if crewWins {
		r.Winner = imposterWinnerCrew
	}
	for p, n := range r.Points {
		s.Score[p] += n
	}
	s.Phase = imposterPhaseReveal
	s.Due = c.Now.Add(imposterRevealTime)
}

func (s *imposterState) Tick(c Ctx) bool {
	if s.Over() || c.Now.Before(s.Due) {
		return false
	}
	switch s.Phase {
	case imposterPhaseClue:
		// Out of time: the turn passes with no clue.
		s.Clues = append(s.Clues, imposterClue{By: s.Order[s.Turn], Skipped: true})
		s.nextClue(c)
	case imposterPhaseDiscuss:
		s.Phase = imposterPhaseVote
		s.Due = c.Now.Add(imposterVoteTime)
	case imposterPhaseVote:
		s.endVote(c)
	case imposterPhaseGuess:
		s.finishRound(c)
	case imposterPhaseReveal:
		if s.Round >= imposterRounds {
			s.Phase = imposterPhaseOver
		} else {
			s.newRound(c)
		}
	}
	return true
}

func (s *imposterState) Deadline() *time.Time {
	if s.Over() {
		return nil
	}
	return &s.Due
}

func (s *imposterState) View(viewer uuid.UUID, spectator bool) any {
	player := !spectator && s.inGame(viewer)
	public := s.Phase == imposterPhaseReveal || s.Phase == imposterPhaseOver
	voted := []uuid.UUID{}
	for _, p := range s.Players {
		if _, ok := s.Votes[p]; ok {
			voted = append(voted, p)
		}
	}
	v := map[string]any{
		"phase": s.Phase, "round": s.Round, "rounds": imposterRounds, "category": s.Category,
		"players": s.Players, "order": s.Order, "clues": s.Clues, "chat": s.Chat, "voted": voted,
		"score": s.Score, "imposter_count": len(s.Imposters),
	}
	if !s.Over() {
		v["deadline"] = s.Due
	}
	if s.Phase == imposterPhaseClue && s.Turn < len(s.Order) {
		v["turn"] = s.Order[s.Turn]
	}
	if player {
		if s.isImposter(viewer) {
			v["role"] = imposterWinnerImposter
			// Fellow imposters know each other.
			others := []uuid.UUID{}
			for _, p := range s.Imposters {
				if p != viewer {
					others = append(others, p)
				}
			}
			v["partners"] = others
		} else {
			v["role"] = imposterWinnerCrew
			v["word"] = s.Word
		}
		if mine, ok := s.Votes[viewer]; ok && s.Phase == imposterPhaseVote {
			v["my_vote"] = mine
		}
	}
	if s.Phase == imposterPhaseGuess && s.Result != nil {
		// Who was voted out is public; their teammate, if any, stays hidden.
		v["result"] = map[string]any{"out": s.Result.Out, "out_role": s.Result.OutRole}
	}
	if public {
		v["word"] = s.Word
		v["imposters"] = s.Imposters
		v["votes"] = s.Votes
		v["result"] = s.Result
	}
	return v
}

func (s *imposterState) Scores() map[uuid.UUID]int { return s.Score }
func (s *imposterState) Over() bool                { return s.Phase == imposterPhaseOver }

func (s *imposterState) Leave(c Ctx, who uuid.UUID) bool {
	if s.Over() || !s.inGame(who) {
		return false
	}
	current := false
	if i := slices.Index(s.Order, who); i >= 0 {
		current = s.Phase == imposterPhaseClue && i == s.Turn
		s.Order = slices.Delete(s.Order, i, i+1)
		if i < s.Turn {
			s.Turn--
		}
	}
	s.Players = slices.DeleteFunc(s.Players, func(p uuid.UUID) bool { return p == who })
	delete(s.Votes, who)
	for v, t := range s.Votes {
		if t == who.String() {
			delete(s.Votes, v)
		}
	}
	if len(s.Players) < 3 {
		s.Phase = imposterPhaseOver
		return true
	}
	switch {
	case s.Phase == imposterPhaseReveal:
		// Already scored; nothing more to settle this round.
	case s.isImposter(who):
		// An imposter walking out hands the round to the crew. They stay listed for the reveal.
		s.Result = &imposterResult{Reason: "imposter_left", Out: &who, OutRole: imposterWinnerImposter}
		s.finishRound(c)
	case current:
		// The leaver was giving a clue: the next person gets a full turn.
		s.Turn--
		s.nextClue(c)
	case s.Phase == imposterPhaseVote && len(s.Votes) >= len(s.Players):
		s.endVote(c)
	}
	return true
}
