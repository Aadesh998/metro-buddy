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

// Scribble: one player draws a word, everyone else races to guess it. The word stays on the
// server; guessers get only blanks with a letter or two revealed as time runs down.
type scribbleEngine struct{}

func init() { Register(scribbleEngine{}) }

//go:embed words/scribble.txt
var scribbleWordFile string

var scribbleWords = func() []string {
	var out []string
	for _, l := range strings.Split(scribbleWordFile, "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out
}()

const (
	scribbleChooseTime = 10 * time.Second
	scribbleDrawTime   = 80 * time.Second
	scribbleRevealTime = 5 * time.Second

	scribbleChoices      = 3
	scribbleMaxPoints    = 200 // per stroke
	scribbleMaxStrokes   = 400 // per turn
	scribbleMaxGuess     = 40  // characters
	scribbleFeedKeep     = 60
	scribbleFeedShow     = 30
	scribbleGuessBase    = 50
	scribbleGuessBonus   = 100 // scaled by time left
	scribbleDrawerPoints = 25  // per correct guesser
)

// The drawing palette and brush sizes; the client offers exactly these.
var (
	scribblePalette = []string{"#111111", "#FFFFFF", "#E53935", "#FB8C00", "#FDD835", "#43A047", "#1E88E5", "#8D6E63"}
	scribbleWidths  = []int{2, 5, 10}
)

const (
	scribbleChoosing = "choosing"
	scribbleDrawing  = "drawing"
	scribbleReveal   = "reveal"
	scribbleOver     = "over"
)

func (scribbleEngine) Name() string            { return "scribble" }
func (scribbleEngine) Players() (min, max int) { return 2, 8 }

type scribbleStroke struct {
	ID     int      `json:"id"`
	Points [][2]int `json:"points"`
	Color  string   `json:"color"`
	Width  int      `json:"width"`
	// Cont marks a piece sent mid-drag that continues the previous stroke; undo removes the whole drag.
	Cont bool `json:"cont,omitempty"`
}

// A feed line. To limits who sees it; empty means everyone.
type scribbleMsg struct {
	ID   int         `json:"id"`
	Kind string      `json:"kind"` // guess | correct | close | chat
	By   uuid.UUID   `json:"by"`
	Text string      `json:"text,omitempty"`
	To   []uuid.UUID `json:"to,omitempty"`
}

type scribbleState struct {
	Players []uuid.UUID        `json:"players"`
	Gone    map[uuid.UUID]bool `json:"gone"`
	Score   map[uuid.UUID]int  `json:"score"`
	Turn    int                `json:"turn"` // 0-based; the drawer is Players[Turn % len]
	Turns   int                `json:"turns"`
	Phase   string             `json:"phase"`
	Drawer  uuid.UUID          `json:"drawer"`
	Choices []string           `json:"choices"`
	Word    string             `json:"word"`
	Shown   []int              `json:"shown"` // revealed letter positions
	Started time.Time          `json:"started"`
	Ends    time.Time          `json:"ends"`
	Strokes []scribbleStroke   `json:"strokes"`
	Seq     int                `json:"seq"` // last stroke/message id
	Feed    []scribbleMsg      `json:"feed"`
	Guessed map[uuid.UUID]int  `json:"guessed"` // points each correct guesser earned this turn
	Used    []string           `json:"used"`
}

func (scribbleEngine) Start(c Ctx, players []uuid.UUID) (State, error) {
	cycles := 2
	if len(players) > 4 {
		cycles = 1
	}
	st := &scribbleState{Players: players, Gone: map[uuid.UUID]bool{}, Score: map[uuid.UUID]int{}, Turns: cycles * len(players)}
	for _, p := range players {
		st.Score[p] = 0
	}
	st.beginTurn(c)
	return st, nil
}

func (scribbleEngine) Load(data []byte) (State, error) {
	var st scribbleState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	if st.Gone == nil {
		st.Gone = map[uuid.UUID]bool{}
	}
	if st.Guessed == nil {
		st.Guessed = map[uuid.UUID]int{}
	}
	return &st, nil
}

func (s *scribbleState) beginTurn(c Ctx) {
	s.Drawer = s.Players[s.Turn%len(s.Players)]
	s.Phase = scribbleChoosing
	s.Choices = s.pickWords(c)
	s.Word, s.Shown, s.Strokes, s.Feed = "", nil, nil, nil
	s.Guessed = map[uuid.UUID]int{}
	s.Ends = c.Now.Add(scribbleChooseTime)
}

// pickWords draws distinct words not used earlier in this game.
func (s *scribbleState) pickWords(c Ctx) []string {
	out := []string{}
	for _, i := range c.Rand.Perm(len(scribbleWords)) {
		w := scribbleWords[i]
		if !slices.Contains(s.Used, w) {
			out = append(out, w)
		}
		if len(out) == scribbleChoices {
			break
		}
	}
	return out
}

func (s *scribbleState) choose(c Ctx, word string) {
	s.Word, s.Choices = word, nil
	s.Used = append(s.Used, word)
	s.Phase = scribbleDrawing
	s.Started = c.Now
	s.Ends = c.Now.Add(scribbleDrawTime)
}

func (s *scribbleState) endTurn(c Ctx) {
	s.Phase = scribbleReveal
	s.Choices = nil
	s.Ends = c.Now.Add(scribbleRevealTime)
}

// next moves to the following drawer still in the game, or ends the game.
func (s *scribbleState) next(c Ctx) {
	for {
		s.Turn++
		if s.Turn >= s.Turns || s.active() < 2 {
			s.Phase = scribbleOver
			s.Choices = nil
			return
		}
		if !s.Gone[s.Players[s.Turn%len(s.Players)]] {
			break
		}
	}
	s.beginTurn(c)
}

func (s *scribbleState) active() int {
	n := 0
	for _, p := range s.Players {
		if !s.Gone[p] {
			n++
		}
	}
	return n
}

// allGuessed: every player still here, other than the drawer, has the word.
func (s *scribbleState) allGuessed() bool {
	for _, p := range s.Players {
		if p != s.Drawer && !s.Gone[p] {
			if _, ok := s.Guessed[p]; !ok {
				return false
			}
		}
	}
	return true
}

// letterSlots are the word's letter positions (spaces excluded).
func (s *scribbleState) letterSlots() []int {
	out := []int{}
	for i, r := range []rune(s.Word) {
		if r != ' ' {
			out = append(out, i)
		}
	}
	return out
}

// hintTimes are when letters get revealed: half and three quarters of the way through.
// Short words get fewer, so the hint never gives most of the word away.
func (s *scribbleState) hintTimes() []time.Time {
	n := min(2, (len(s.letterSlots())-1)/2)
	out := []time.Time{s.Started.Add(scribbleDrawTime / 2), s.Started.Add(scribbleDrawTime * 3 / 4)}
	return out[:max(0, n)]
}

func (s *scribbleState) revealLetter(c Ctx) {
	hidden := []int{}
	for _, i := range s.letterSlots() {
		if !slices.Contains(s.Shown, i) {
			hidden = append(hidden, i)
		}
	}
	if len(hidden) > 0 {
		s.Shown = append(s.Shown, hidden[c.Rand.IntN(len(hidden))])
	}
}

// hint is the word as blanks: "_" per hidden letter, revealed letters as themselves, spaces kept.
func (s *scribbleState) hint() string {
	rs := []rune(s.Word)
	for i, r := range rs {
		if r != ' ' && !slices.Contains(s.Shown, i) {
			rs[i] = '_'
		}
	}
	return string(rs)
}

type scribbleMove struct {
	Type   string   `json:"type"`
	Index  int      `json:"index"`
	Points [][2]int `json:"points"`
	Color  string   `json:"color"`
	Width  int      `json:"width"`
	Cont   bool     `json:"cont"`
	Text   string   `json:"text"`
}

func (s *scribbleState) Move(c Ctx, by uuid.UUID, raw json.RawMessage) (*Live, error) {
	if s.Over() {
		return nil, Refuse("the game is over")
	}
	if !slices.Contains(s.Players, by) || s.Gone[by] {
		return nil, Refuse("you're not in this game")
	}
	m, err := Decode[scribbleMove](raw)
	if err != nil {
		return nil, err
	}
	switch m.Type {
	case "choose":
		if s.Phase != scribbleChoosing || by != s.Drawer {
			return nil, Refuse("it's not your turn to pick a word")
		}
		if m.Index < 0 || m.Index >= len(s.Choices) {
			return nil, Refuse("pick one of the words")
		}
		s.choose(c, s.Choices[m.Index])
		return nil, nil
	case "stroke", "undo", "clear":
		if s.Phase != scribbleDrawing || by != s.Drawer {
			return nil, Refuse("only the drawer can draw")
		}
		return s.draw(m)
	case "guess":
		return nil, s.guess(c, by, m.Text)
	}
	return nil, Refuse("that move doesn't make sense")
}

func (s *scribbleState) draw(m scribbleMove) (*Live, error) {
	data := map[string]any{"turn": s.Turn, "op": m.Type}
	switch m.Type {
	case "stroke":
		if len(m.Points) == 0 || len(m.Points) > scribbleMaxPoints {
			return nil, Refuse("that stroke is too long")
		}
		if len(s.Strokes) >= scribbleMaxStrokes {
			return nil, Refuse("the canvas is full; clear it or undo")
		}
		if !slices.Contains(scribblePalette, strings.ToUpper(m.Color)) {
			return nil, Refuse("pick a colour from the palette")
		}
		if !slices.Contains(scribbleWidths, m.Width) {
			return nil, Refuse("pick a brush size")
		}
		for i, p := range m.Points {
			m.Points[i] = [2]int{min(1000, max(0, p[0])), min(1000, max(0, p[1]))}
		}
		s.Seq++
		st := scribbleStroke{ID: s.Seq, Points: m.Points, Color: strings.ToUpper(m.Color), Width: m.Width, Cont: m.Cont && len(s.Strokes) > 0}
		s.Strokes = append(s.Strokes, st)
		data["stroke"] = st
	case "undo":
		// Remove the last drag: its final piece and every continuation before it.
		ids := []int{}
		for len(s.Strokes) > 0 {
			last := s.Strokes[len(s.Strokes)-1]
			s.Strokes = s.Strokes[:len(s.Strokes)-1]
			ids = append(ids, last.ID)
			if !last.Cont {
				break
			}
		}
		data["ids"] = ids
	case "clear":
		s.Strokes = nil
	}
	return &Live{Type: "draw", Data: data}, nil
}

func (s *scribbleState) guess(c Ctx, by uuid.UUID, text string) error {
	if s.Phase != scribbleDrawing {
		return Refuse("wait for the drawing to start")
	}
	if by == s.Drawer {
		return Refuse("you're the one drawing")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return Refuse("type a guess")
	}
	if utf8.RuneCountInString(text) > scribbleMaxGuess {
		text = string([]rune(text)[:scribbleMaxGuess])
	}
	shown := c.Mask(text)
	// Once you have the word, you chat only with others who have it (and the drawer).
	if _, ok := s.Guessed[by]; ok {
		to := []uuid.UUID{s.Drawer}
		for p := range s.Guessed {
			to = append(to, p)
		}
		s.post(scribbleMsg{Kind: "chat", By: by, Text: shown, To: to})
		return nil
	}
	g, w := scribbleNorm(text), scribbleNorm(s.Word)
	switch {
	case g == w:
		left := max(0, s.Ends.Sub(c.Now))
		pts := scribbleGuessBase + int(int64(scribbleGuessBonus)*int64(left)/int64(scribbleDrawTime))
		s.Guessed[by] = pts
		s.Score[by] += pts
		s.Score[s.Drawer] += scribbleDrawerPoints
		s.post(scribbleMsg{Kind: "correct", By: by})
		if s.allGuessed() {
			s.endTurn(c)
		}
	default:
		var to []uuid.UUID
		if strings.Contains(g, w) {
			// The word is inside a longer message: don't show it to anyone else.
			to = []uuid.UUID{by}
		}
		s.post(scribbleMsg{Kind: "guess", By: by, Text: shown, To: to})
		if len([]rune(w)) >= 3 && scribbleEditDistance(g, w) == 1 {
			s.post(scribbleMsg{Kind: "close", By: by, Text: shown, To: []uuid.UUID{by}})
		}
	}
	return nil
}

func (s *scribbleState) post(m scribbleMsg) {
	s.Seq++
	m.ID = s.Seq
	s.Feed = append(s.Feed, m)
	if len(s.Feed) > scribbleFeedKeep {
		s.Feed = s.Feed[len(s.Feed)-scribbleFeedKeep:]
	}
}

// scribbleNorm lowercases and drops everything but letters and digits.
func scribbleNorm(t string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(t) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func scribbleEditDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

func (s *scribbleState) Tick(c Ctx) bool {
	changed := false
	// A few steps at most, in case the ticker fell behind across a phase change.
	for range 6 {
		if !s.step(c) {
			break
		}
		changed = true
	}
	return changed
}

func (s *scribbleState) step(c Ctx) bool {
	switch s.Phase {
	case scribbleChoosing:
		if !c.Now.Before(s.Ends) {
			s.choose(c, s.Choices[c.Rand.IntN(len(s.Choices))])
			return true
		}
	case scribbleDrawing:
		if hs := s.hintTimes(); len(s.Shown) < len(hs) && !c.Now.Before(hs[len(s.Shown)]) && c.Now.Before(s.Ends) {
			s.revealLetter(c)
			return true
		}
		if !c.Now.Before(s.Ends) {
			s.endTurn(c)
			return true
		}
	case scribbleReveal:
		if !c.Now.Before(s.Ends) {
			s.next(c)
			return true
		}
	}
	return false
}

func (s *scribbleState) Deadline() *time.Time {
	if s.Over() {
		return nil
	}
	if s.Phase == scribbleDrawing {
		if hs := s.hintTimes(); len(s.Shown) < len(hs) {
			t := hs[len(s.Shown)]
			return &t
		}
	}
	t := s.Ends
	return &t
}

func (s *scribbleState) View(viewer uuid.UUID, spectator bool) any {
	drawer := !spectator && viewer == s.Drawer
	_, guessed := s.Guessed[viewer]
	guessed = guessed && !spectator
	guessedIDs := make([]uuid.UUID, 0, len(s.Guessed))
	for _, p := range s.Players {
		if _, ok := s.Guessed[p]; ok {
			guessedIDs = append(guessedIDs, p)
		}
	}
	feed := []scribbleMsg{}
	for _, m := range s.Feed {
		if len(m.To) == 0 || (!spectator && slices.Contains(m.To, viewer)) {
			m.To = nil
			feed = append(feed, m)
		}
	}
	if len(feed) > scribbleFeedShow {
		feed = feed[len(feed)-scribbleFeedShow:]
	}
	n := len(s.Players)
	v := map[string]any{
		"phase": s.Phase, "drawer": s.Drawer, "players": s.Players, "gone": s.Gone, "score": s.Score,
		"round": min(s.Turn, s.Turns-1)/n + 1, "rounds": s.Turns / n, "turn": s.Turn, "turns": s.Turns,
		"ends_at": s.Ends, "strokes": s.Strokes, "feed": feed, "guessed": guessedIDs, "turn_points": s.Guessed,
		"palette": scribblePalette, "widths": scribbleWidths, "draw_seconds": int(scribbleDrawTime / time.Second),
	}
	if s.Strokes == nil {
		v["strokes"] = []scribbleStroke{}
	}
	if s.Phase == scribbleOver {
		v["ends_at"] = nil
	}
	if s.Phase == scribbleDrawing || s.Phase == scribbleReveal || s.Phase == scribbleOver {
		v["hint"] = s.hint()
	}
	switch {
	case s.Phase == scribbleReveal || s.Phase == scribbleOver:
		v["word"] = s.Word
	case drawer && s.Phase == scribbleChoosing:
		v["choices"] = s.Choices
	case drawer || guessed:
		// Those who already have it may see it too.
		v["word"] = s.Word
	}
	return v
}

func (s *scribbleState) Scores() map[uuid.UUID]int { return s.Score }
func (s *scribbleState) Over() bool                { return s.Phase == scribbleOver }

func (s *scribbleState) Leave(c Ctx, who uuid.UUID) bool {
	if s.Over() || !slices.Contains(s.Players, who) || s.Gone[who] {
		return false
	}
	s.Gone[who] = true
	switch {
	case s.active() < 2:
		s.Phase = scribbleOver
		s.Choices = nil
	case who == s.Drawer && s.Phase == scribbleChoosing:
		// Nothing was drawn yet: go straight to the next drawer.
		s.next(c)
	case who == s.Drawer && s.Phase == scribbleDrawing:
		s.endTurn(c)
	case s.Phase == scribbleDrawing && s.allGuessed():
		s.endTurn(c)
	}
	return true
}
