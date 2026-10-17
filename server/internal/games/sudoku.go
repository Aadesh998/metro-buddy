package games

import (
	"encoding/json"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
)

// Co-op Sudoku: one shared board, every cell checked by the server against the solution.
// The team shares a mistake budget; each correct cell scores for whoever placed it.
type sudokuEngine struct{}

func init() { Register(sudokuEngine{}) }

const (
	sudokuBlanks      = 46
	sudokuMistakes    = 8
	sudokuTimeLimit   = 25 * time.Minute
	sudokuCellPoints  = 10
	sudokuWrongPoints = 5
)

func (sudokuEngine) Name() string            { return "sudoku" }
func (sudokuEngine) Players() (min, max int) { return 1, 8 }

type sudokuState struct {
	Givens   [81]int8          `json:"givens"`
	Solution [81]int8          `json:"solution"`
	Board    [81]int8          `json:"board"`
	FilledBy [81]int8          `json:"filled_by"` // seat+1 of who placed it; 0 for givens
	Players  []uuid.UUID       `json:"players"`
	Score    map[uuid.UUID]int `json:"score"`
	Mistakes int               `json:"mistakes"`
	Ends     time.Time         `json:"ends"`
	Solved   bool              `json:"solved"`
	Failed   bool              `json:"failed"`
}

func (sudokuEngine) Start(c Ctx, players []uuid.UUID) (State, error) {
	sol := solvedGrid(c.Rand)
	givens := carve(sol, sudokuBlanks, c.Rand)
	st := &sudokuState{Givens: givens, Solution: sol, Board: givens, Players: players, Score: map[uuid.UUID]int{}, Ends: c.Now.Add(sudokuTimeLimit)}
	for _, p := range players {
		st.Score[p] = 0
	}
	return st, nil
}

func (sudokuEngine) Load(data []byte) (State, error) {
	var st sudokuState
	return &st, json.Unmarshal(data, &st)
}

func (s *sudokuState) seat(p uuid.UUID) int {
	for i, x := range s.Players {
		if x == p {
			return i
		}
	}
	return -1
}

type sudokuMove struct {
	Cell  int `json:"cell"`
	Value int `json:"value"`
}

func (s *sudokuState) Move(c Ctx, by uuid.UUID, raw json.RawMessage) (*Live, error) {
	if s.Over() {
		return nil, Refuse("the puzzle is finished")
	}
	m, err := Decode[sudokuMove](raw)
	if err != nil {
		return nil, err
	}
	if m.Cell < 0 || m.Cell > 80 || m.Value < 1 || m.Value > 9 {
		return nil, Refuse("pick a cell and a number from 1 to 9")
	}
	if s.Board[m.Cell] != 0 {
		return nil, Refuse("that cell is already filled")
	}
	seat := s.seat(by)
	correct := s.Solution[m.Cell] == int8(m.Value)
	if correct {
		s.Board[m.Cell] = int8(m.Value)
		s.FilledBy[m.Cell] = int8(seat + 1)
		s.Score[by] += sudokuCellPoints
		s.Solved = s.Board == s.Solution
	} else {
		s.Mistakes++
		s.Score[by] = max(0, s.Score[by]-sudokuWrongPoints)
		s.Failed = s.Mistakes >= sudokuMistakes
	}
	return &Live{Type: "cell", Data: map[string]any{"cell": m.Cell, "value": m.Value, "correct": correct, "seat": seat}}, nil
}

func (s *sudokuState) Tick(c Ctx) bool {
	if !s.Over() && !c.Now.Before(s.Ends) {
		s.Failed = true
		return true
	}
	return false
}

func (s *sudokuState) Deadline() *time.Time {
	if s.Over() {
		return nil
	}
	return &s.Ends
}

func (s *sudokuState) View(viewer uuid.UUID, spectator bool) any {
	v := map[string]any{
		"givens": s.Givens, "board": s.Board, "filled_by": s.FilledBy, "mistakes": s.Mistakes, "max_mistakes": sudokuMistakes,
		"ends_at": s.Ends, "solved": s.Solved, "failed": s.Failed, "players": s.Players, "score": s.Score,
	}
	// The answer is revealed only when the game ends.
	if s.Over() {
		v["solution"] = s.Solution
	}
	return v
}

func (s *sudokuState) Scores() map[uuid.UUID]int { return s.Score }
func (s *sudokuState) Over() bool                { return s.Solved || s.Failed }
func (s *sudokuState) Leave(Ctx, uuid.UUID) bool { return false }

// solvedGrid fills a grid by randomized backtracking.
func solvedGrid(r *rand.Rand) [81]int8 {
	var g [81]int8
	var fill func(i int) bool
	fill = func(i int) bool {
		if i == 81 {
			return true
		}
		for _, v := range r.Perm(9) {
			if canPlace(&g, i, int8(v+1)) {
				g[i] = int8(v + 1)
				if fill(i + 1) {
					return true
				}
				g[i] = 0
			}
		}
		return false
	}
	fill(0)
	return g
}

func canPlace(g *[81]int8, i int, v int8) bool {
	row, col := i/9, i%9
	br, bc := row/3*3, col/3*3
	for k := 0; k < 9; k++ {
		if g[row*9+k] == v || g[k*9+col] == v || g[(br+k/3)*9+bc+k%3] == v {
			return false
		}
	}
	return true
}

// countSolutions stops at limit; used to keep the puzzle's solution unique.
func countSolutions(g *[81]int8, limit int) int {
	i := 0
	for i < 81 && g[i] != 0 {
		i++
	}
	if i == 81 {
		return 1
	}
	n := 0
	for v := int8(1); v <= 9 && n < limit; v++ {
		if canPlace(g, i, v) {
			g[i] = v
			n += countSolutions(g, limit-n)
			g[i] = 0
		}
	}
	return n
}

// carve blanks cells one by one, putting back any removal that would allow a second solution.
func carve(sol [81]int8, blanks int, r *rand.Rand) [81]int8 {
	g := sol
	removed := 0
	for _, i := range r.Perm(81) {
		if removed >= blanks {
			break
		}
		v := g[i]
		g[i] = 0
		probe := g
		if countSolutions(&probe, 2) != 1 {
			g[i] = v
			continue
		}
		removed++
	}
	return g
}
