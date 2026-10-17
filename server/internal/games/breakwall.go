package games

import (
	"encoding/json"
	"math"
	"time"

	"github.com/google/uuid"
)

// Break the Wall: a brick-breaker score race. Every player plays their own copy of the same
// seeded wall at the same time; the app builds the wall from the seed and reports progress,
// and the server checks each report against the layout, the clock and the scoring formula.
type breakwallEngine struct{}

func init() { Register(breakwallEngine{}) }

const (
	breakwallRounds    = 3
	breakwallCountdown = 5 * time.Second
	breakwallPlay      = 45 * time.Second
	breakwallResults   = 6 * time.Second
	// Reports may arrive a little after the play phase ends (network, final ball).
	breakwallGrace = 3 * time.Second

	breakwallBrickPoints = 10
	breakwallClearBonus  = 50
	// Nobody can break bricks faster than this; a little slack covers clock skew.
	breakwallMaxBricksPerSec = 4
	breakwallBrickSlack      = 2

	breakwallCountdownPhase = "countdown"
	breakwallPlayPhase      = "play"
	breakwallResultsPhase   = "results"
)

// breakwallLayouts are the wall shapes: rows, columns and how dense (percent) the wall is.
// The app has the same table (breakwall/layout.ts); keep them in step.
var breakwallLayouts = []struct{ Rows, Cols, Density int }{
	{5, 8, 85},
	{6, 8, 75},
	{6, 7, 80},
	{7, 8, 65},
}

// breakwallMulberry32 is mulberry32, a tiny PRNG that is easy to reproduce exactly in TypeScript.
func breakwallMulberry32(seed uint32) func() uint32 {
	a := seed
	return func() uint32 {
		a += 0x6D2B79F5
		t := a
		t = (t ^ (t >> 15)) * (t | 1)
		t ^= t + (t^(t>>7))*(t|61)
		return t ^ (t >> 14)
	}
}

// breakwallLayout returns the wall for a seed: its size and the cells (row*cols+col, ascending)
// that hold a brick. Walls are mirrored left to right so they look designed, not noisy.
func breakwallLayout(seed uint32, layout int) (rows, cols int, cells []int) {
	l := breakwallLayouts[((layout%len(breakwallLayouts))+len(breakwallLayouts))%len(breakwallLayouts)]
	rows, cols = l.Rows, l.Cols
	next := breakwallMulberry32(seed)
	grid := make([]bool, rows*cols)
	half := (cols + 1) / 2
	for r := 0; r < rows; r++ {
		for c := 0; c < half; c++ {
			on := int(next()%100) < l.Density
			grid[r*cols+c] = on
			grid[r*cols+cols-1-c] = on
		}
	}
	cells = []int{}
	for i, on := range grid {
		if on {
			cells = append(cells, i)
		}
	}
	// An empty wall would be a dull round; fall back to a full top row.
	if len(cells) == 0 {
		for c := 0; c < cols; c++ {
			cells = append(cells, c)
		}
	}
	return rows, cols, cells
}

func (breakwallEngine) Name() string            { return "breakwall" }
func (breakwallEngine) Players() (min, max int) { return 1, 10 }

type breakwallEntry struct {
	Bricks int  `json:"bricks"`
	Score  int  `json:"score"`
	Final  bool `json:"final"`
}

type breakwallRound struct {
	Seed       uint32                        `json:"seed"`
	Layout     int                           `json:"layout"`
	Total      int                           `json:"total"`
	PlayStarts time.Time                     `json:"play_starts"`
	PlayEnds   time.Time                     `json:"play_ends"`
	Scores     map[uuid.UUID]*breakwallEntry `json:"scores"`
}

type breakwallState struct {
	Players   []uuid.UUID      `json:"players"`
	Rounds    []breakwallRound `json:"rounds"`
	Phase     string           `json:"phase"`
	PhaseEnds time.Time        `json:"phase_ends"`
	Done      bool             `json:"done"`
}

func (breakwallEngine) Start(c Ctx, players []uuid.UUID) (State, error) {
	st := &breakwallState{Players: players}
	st.newRound(c, c.Now)
	return st, nil
}

func (breakwallEngine) Load(data []byte) (State, error) {
	var st breakwallState
	return &st, json.Unmarshal(data, &st)
}

// newRound deals a fresh wall and starts its countdown at `at`.
func (s *breakwallState) newRound(c Ctx, at time.Time) {
	r := breakwallRound{Seed: c.Rand.Uint32(), Layout: c.Rand.IntN(len(breakwallLayouts)), Scores: map[uuid.UUID]*breakwallEntry{}}
	_, _, cells := breakwallLayout(r.Seed, r.Layout)
	r.Total = len(cells)
	r.PlayStarts = at.Add(breakwallCountdown)
	r.PlayEnds = r.PlayStarts.Add(breakwallPlay)
	for _, p := range s.Players {
		r.Scores[p] = &breakwallEntry{}
	}
	s.Rounds = append(s.Rounds, r)
	s.Phase = breakwallCountdownPhase
	s.PhaseEnds = r.PlayStarts
}

func (s *breakwallState) cur() *breakwallRound { return &s.Rounds[len(s.Rounds)-1] }

// breakwallScore is the only valid score for a report: points per brick, plus a bonus and
// one point per second left when the whole wall is cleared.
func breakwallScore(bricks, total, secondsLeft int) int {
	sc := bricks * breakwallBrickPoints
	if bricks == total {
		sc += breakwallClearBonus + secondsLeft
	}
	return sc
}

type breakwallMove struct {
	Type   string `json:"type"`
	Round  int    `json:"round"`
	Bricks int    `json:"bricks"`
	Score  int    `json:"score"`
	Final  bool   `json:"final"`
}

func (s *breakwallState) Move(c Ctx, by uuid.UUID, raw json.RawMessage) (*Live, error) {
	if s.Done {
		return nil, Refuse("the game is over")
	}
	m, err := Decode[breakwallMove](raw)
	if err != nil {
		return nil, err
	}
	if m.Type != "score" {
		return nil, Refuse("that move doesn't make sense")
	}
	r := s.cur()
	if m.Round != len(s.Rounds) {
		return nil, Refuse("that round is over")
	}
	if c.Now.Before(r.PlayStarts) || c.Now.After(r.PlayEnds.Add(breakwallGrace)) {
		return nil, Refuse("the round isn't being played right now")
	}
	sc, ok := r.Scores[by]
	if !ok {
		return nil, Refuse("you're not playing this round")
	}
	if sc.Final {
		return nil, Refuse("your score for this round is already in")
	}
	if m.Bricks < 0 || m.Bricks > r.Total {
		return nil, Refuse("that score isn't possible")
	}
	// Nobody breaks bricks faster than the ball can travel.
	elapsed := c.Now.Sub(r.PlayStarts).Seconds()
	if m.Bricks > int(math.Ceil(elapsed*breakwallMaxBricksPerSec))+breakwallBrickSlack {
		return nil, Refuse("that score isn't possible")
	}
	if m.Bricks < r.Total {
		if m.Score != breakwallScore(m.Bricks, r.Total, 0) {
			return nil, Refuse("that score isn't possible")
		}
	} else {
		// The time bonus can't be more than the time that was left (plus the grace window).
		maxLeft := min(int(breakwallPlay/time.Second), int(math.Ceil(r.PlayEnds.Sub(c.Now).Seconds()+breakwallGrace.Seconds())))
		base := breakwallScore(m.Bricks, r.Total, 0)
		if m.Score < base || m.Score > base+max(0, maxLeft) {
			return nil, Refuse("that score isn't possible")
		}
	}
	if m.Bricks < sc.Bricks || m.Score < sc.Score {
		return nil, Refuse("scores can't go backwards")
	}
	sc.Bricks, sc.Score, sc.Final = m.Bricks, m.Score, m.Final
	s.endEarly(c)
	return &Live{Type: "score", Data: map[string]any{"persona": by, "round": m.Round, "score": m.Score, "bricks": m.Bricks, "final": m.Final}}, nil
}

// endEarly skips to the results once everyone has finished the round.
func (s *breakwallState) endEarly(c Ctx) bool {
	if s.Phase != breakwallPlayPhase {
		return false
	}
	for _, sc := range s.cur().Scores {
		if !sc.Final {
			return false
		}
	}
	s.Phase = breakwallResultsPhase
	s.PhaseEnds = c.Now.Add(breakwallResults)
	return true
}

func (s *breakwallState) Tick(c Ctx) bool {
	changed := s.endEarly(c)
	// Catch up through every phase whose deadline has passed.
	for !s.Done && !c.Now.Before(s.PhaseEnds) {
		changed = true
		switch s.Phase {
		case breakwallCountdownPhase:
			s.Phase = breakwallPlayPhase
			s.PhaseEnds = s.cur().PlayEnds
		case breakwallPlayPhase:
			s.Phase = breakwallResultsPhase
			s.PhaseEnds = s.PhaseEnds.Add(breakwallResults)
		default:
			if len(s.Rounds) >= breakwallRounds {
				s.Done = true
			} else {
				s.newRound(c, s.PhaseEnds)
			}
		}
	}
	return changed
}

func (s *breakwallState) Deadline() *time.Time {
	if s.Done {
		return nil
	}
	return &s.PhaseEnds
}

func (s *breakwallState) View(viewer uuid.UUID, spectator bool) any {
	r := s.cur()
	board := []map[string]any{}
	for _, p := range s.Players {
		sc := r.Scores[p]
		if sc == nil {
			sc = &breakwallEntry{}
		}
		board = append(board, map[string]any{"persona": p, "bricks": sc.Bricks, "score": sc.Score, "final": sc.Final})
	}
	past := []map[uuid.UUID]int{}
	for _, pr := range s.Rounds {
		m := map[uuid.UUID]int{}
		for p, sc := range pr.Scores {
			m[p] = sc.Score
		}
		past = append(past, m)
	}
	v := map[string]any{
		"round": len(s.Rounds), "rounds": breakwallRounds, "phase": s.Phase, "phase_ends_at": s.PhaseEnds,
		"play_starts_at": r.PlayStarts, "play_ends_at": r.PlayEnds, "total": r.Total,
		"players": s.Players, "board": board, "round_scores": past, "totals": s.Scores(), "done": s.Done,
	}
	// The wall itself is only needed while it's being played.
	if !s.Done && s.Phase != breakwallResultsPhase {
		rows, cols, _ := breakwallLayout(r.Seed, r.Layout)
		v["seed"], v["layout"], v["rows"], v["cols"] = r.Seed, r.Layout, rows, cols
	}
	return v
}

func (s *breakwallState) Scores() map[uuid.UUID]int {
	out := map[uuid.UUID]int{}
	for _, p := range s.Players {
		out[p] = 0
	}
	for _, r := range s.Rounds {
		for p, sc := range r.Scores {
			out[p] += sc.Score
		}
	}
	return out
}

func (s *breakwallState) Over() bool { return s.Done }

// A leaver's scores stay on the board; the round timers carry on without them.
func (s *breakwallState) Leave(Ctx, uuid.UUID) bool { return false }
