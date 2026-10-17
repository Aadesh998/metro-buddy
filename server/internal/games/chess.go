package games

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/notnil/chess"
)

// Chess for two, with a server-side clock. The library (github.com/notnil/chess) does the
// rules; state keeps the starting FEN and the UCI moves and replays them on load, so
// repetition history survives the JSON round trip.
type chessEngine struct{}

func init() { Register(chessEngine{}) }

const (
	chessClock = 10 * time.Minute
	chessWin   = 3
	chessDraw  = 1
)

func (chessEngine) Name() string            { return "chess" }
func (chessEngine) Players() (min, max int) { return 2, 2 }

type chessState struct {
	White    uuid.UUID `json:"white"`
	Black    uuid.UUID `json:"black"`
	StartFEN string    `json:"start_fen"`
	FEN      string    `json:"fen"`
	UCI      []string  `json:"uci"`
	SAN      []string  `json:"san"`
	// Time left per side, not counting the turn in progress, which started at TurnStart.
	WhiteMs   int64     `json:"white_ms"`
	BlackMs   int64     `json:"black_ms"`
	TurnStart time.Time `json:"turn_start"`
	DrawOffer string    `json:"draw_offer"` // "white", "black" or ""
	Result    string    `json:"result"`     // "white", "black", "draw" or "" while playing
	Reason    string    `json:"reason"`

	g *chess.Game
}

func (chessEngine) Start(c Ctx, players []uuid.UUID) (State, error) {
	if len(players) != 2 {
		return nil, Refuse("chess needs exactly two players")
	}
	w, b := players[0], players[1]
	if c.Rand.IntN(2) == 1 {
		w, b = b, w
	}
	g := chess.NewGame()
	st := &chessState{White: w, Black: b, StartFEN: g.FEN(), FEN: g.FEN(), UCI: []string{}, SAN: []string{},
		WhiteMs: chessClock.Milliseconds(), BlackMs: chessClock.Milliseconds(), TurnStart: c.Now, g: g}
	return st, nil
}

func (chessEngine) Load(data []byte) (State, error) {
	var st chessState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	fen, err := chess.FEN(st.StartFEN)
	if err != nil {
		return nil, err
	}
	st.g = chess.NewGame(fen, chess.UseNotation(chess.UCINotation{}))
	for _, m := range st.UCI {
		if err := st.g.MoveStr(m); err != nil {
			return nil, err
		}
	}
	return &st, nil
}

func chessColorName(c chess.Color) string {
	if c == chess.White {
		return "white"
	}
	return "black"
}

func (s *chessState) colorOf(p uuid.UUID) string {
	switch p {
	case s.White:
		return "white"
	case s.Black:
		return "black"
	}
	return ""
}

func chessOther(color string) string {
	if color == "white" {
		return "black"
	}
	return "white"
}

func (s *chessState) turn() string { return chessColorName(s.g.Position().Turn()) }

func (s *chessState) clock(color string) *int64 {
	if color == "white" {
		return &s.WhiteMs
	}
	return &s.BlackMs
}

func (s *chessState) end(result, reason string) {
	s.Result, s.Reason, s.DrawOffer = result, reason, ""
}

type chessMove struct {
	Type string `json:"type"`
	UCI  string `json:"uci"`
}

func (s *chessState) Move(c Ctx, by uuid.UUID, raw json.RawMessage) (*Live, error) {
	s.Tick(c)
	if s.Over() {
		return nil, Refuse("the game is over")
	}
	me := s.colorOf(by)
	if me == "" {
		return nil, Refuse("only the two players can move")
	}
	m, err := Decode[chessMove](raw)
	if err != nil {
		return nil, err
	}
	switch m.Type {
	case "move":
		return s.play(c, me, m.UCI)
	case "resign":
		s.end(chessOther(me), "resign")
	case "draw_offer":
		if s.DrawOffer == chessOther(me) {
			return nil, Refuse("your opponent has already offered a draw")
		}
		s.DrawOffer = me
	case "draw_accept":
		if s.DrawOffer != chessOther(me) {
			return nil, Refuse("there's no draw offer to accept")
		}
		s.end("draw", "agreement")
	case "draw_decline":
		if s.DrawOffer != chessOther(me) {
			return nil, Refuse("there's no draw offer to decline")
		}
		s.DrawOffer = ""
	default:
		return nil, Refuse("that move doesn't make sense")
	}
	return nil, nil
}

func (s *chessState) play(c Ctx, me, uci string) (*Live, error) {
	if s.turn() != me {
		return nil, Refuse("It's not your turn")
	}
	pos := s.g.Position()
	mv, err := chess.UCINotation{}.Decode(pos, uci)
	if err != nil {
		return nil, Refuse("That move isn't legal")
	}
	san := chess.AlgebraicNotation{}.Encode(pos, mv)
	if err := s.g.Move(mv); err != nil {
		return nil, Refuse("That move isn't legal")
	}
	uci = chess.UCINotation{}.Encode(pos, mv)
	*s.clock(me) -= c.Now.Sub(s.TurnStart).Milliseconds()
	s.TurnStart = c.Now
	s.UCI = append(s.UCI, uci)
	s.SAN = append(s.SAN, san)
	s.FEN = s.g.FEN()
	// Moving answers a pending offer from the opponent with a no.
	if s.DrawOffer == chessOther(me) {
		s.DrawOffer = ""
	}
	// Threefold repetition and the fifty-move rule end the game straight away rather than waiting for a claim.
	for _, d := range s.g.EligibleDraws() {
		if d == chess.ThreefoldRepetition || d == chess.FiftyMoveRule {
			_ = s.g.Draw(d)
			break
		}
	}
	if s.g.Outcome() != chess.NoOutcome {
		switch s.g.Outcome() {
		case chess.WhiteWon:
			s.end("white", chessReason(s.g.Method()))
		case chess.BlackWon:
			s.end("black", chessReason(s.g.Method()))
		default:
			s.end("draw", chessReason(s.g.Method()))
		}
	}
	return &Live{Type: "move", Data: map[string]any{"uci": uci, "san": san, "fen": s.FEN, "ply": len(s.UCI)}}, nil
}

func chessReason(m chess.Method) string {
	switch m {
	case chess.Checkmate:
		return "checkmate"
	case chess.Stalemate:
		return "stalemate"
	case chess.ThreefoldRepetition, chess.FivefoldRepetition:
		return "repetition"
	case chess.FiftyMoveRule, chess.SeventyFiveMoveRule:
		return "fifty_moves"
	case chess.InsufficientMaterial:
		return "insufficient_material"
	}
	return "draw"
}

// chessCanMate is false when a side has only its king, or king and a single knight or bishop.
func chessCanMate(b *chess.Board, color chess.Color) bool {
	minors := 0
	for _, p := range b.SquareMap() {
		if p.Color() != color {
			continue
		}
		switch p.Type() {
		case chess.Pawn, chess.Rook, chess.Queen:
			return true
		case chess.Knight, chess.Bishop:
			minors++
		}
	}
	return minors > 1
}

func (s *chessState) Tick(c Ctx) bool {
	d := s.Deadline()
	if d == nil || c.Now.Before(*d) {
		return false
	}
	side := s.turn()
	*s.clock(side) = 0
	s.TurnStart = c.Now
	winner := chessOther(side)
	wc := chess.White
	if winner == "black" {
		wc = chess.Black
	}
	if !chessCanMate(s.g.Position().Board(), wc) {
		winner = "draw"
	}
	s.end(winner, "flag")
	return true
}

func (s *chessState) Deadline() *time.Time {
	if s.Over() {
		return nil
	}
	t := s.TurnStart.Add(time.Duration(*s.clock(s.turn())) * time.Millisecond)
	return &t
}

func (s *chessState) View(viewer uuid.UUID, spectator bool) any {
	you := "spectator"
	if !spectator {
		if c := s.colorOf(viewer); c != "" {
			you = c
		}
	}
	var last []string
	if n := len(s.UCI); n > 0 {
		last = []string{s.UCI[n-1][0:2], s.UCI[n-1][2:4]}
	}
	check := false
	if ms := s.g.Moves(); len(ms) > 0 {
		check = ms[len(ms)-1].HasTag(chess.Check)
	}
	turn := s.turn()
	legal := []string{}
	if !s.Over() && you == turn {
		pos := s.g.Position()
		for _, m := range s.g.ValidMoves() {
			legal = append(legal, chess.UCINotation{}.Encode(pos, m))
		}
		slices.Sort(legal)
	}
	var running *time.Time
	if !s.Over() {
		running = &s.TurnStart
	}
	var offer, result, reason any
	if s.DrawOffer != "" {
		offer = s.DrawOffer
	}
	if s.Result != "" {
		result, reason = s.Result, s.Reason
	}
	return map[string]any{
		"fen": s.FEN, "moves": s.SAN, "uci": s.UCI, "last": last, "you": you, "turn": turn, "check": check, "legal": legal,
		"clocks":     map[string]any{"white_ms": s.WhiteMs, "black_ms": s.BlackMs, "running_since": running},
		"draw_offer": offer, "result": result, "reason": reason, "white": s.White, "black": s.Black,
	}
}

func (s *chessState) Scores() map[uuid.UUID]int {
	out := map[uuid.UUID]int{s.White: 0, s.Black: 0}
	switch s.Result {
	case "white":
		out[s.White] = chessWin
	case "black":
		out[s.Black] = chessWin
	case "draw":
		out[s.White], out[s.Black] = chessDraw, chessDraw
	}
	return out
}

func (s *chessState) Over() bool { return s.Result != "" }

// Leave counts as resigning.
func (s *chessState) Leave(_ Ctx, who uuid.UUID) bool {
	me := s.colorOf(who)
	if s.Over() || me == "" {
		return false
	}
	s.end(chessOther(me), "resign")
	return true
}
