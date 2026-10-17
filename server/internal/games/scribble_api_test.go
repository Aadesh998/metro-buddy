package games_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/games"
)

type scribbleView struct {
	Phase   string            `json:"phase"`
	Drawer  uuid.UUID         `json:"drawer"`
	Word    *string           `json:"word"`
	Choices []string          `json:"choices"`
	Strokes []json.RawMessage `json:"strokes"`
	Score   map[uuid.UUID]int `json:"score"`
	Hint    string            `json:"hint"`
}

func scribbleRoom(env *apitest.Env, u apitest.User, id string) scribbleView {
	r := room(env, u, id)
	var v scribbleView
	b, _ := json.Marshal(r.View)
	json.Unmarshal(b, &v)
	return v
}

func TestScribbleRoom(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	var r games.RoomDTO
	env.Do(http.MethodPost, "/v1/games/rooms", a.AccessToken, map[string]any{"game": "scribble", "visibility": "private", "max_players": 4}).
		Want(http.StatusCreated).JSON(&r)
	id := r.ID.String()
	env.Do(http.MethodPost, "/v1/games/rooms/join", b.AccessToken, map[string]any{"code": r.Code}).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/ready", b.AccessToken, map[string]any{"ready": true}).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/start", a.AccessToken, nil).Want(http.StatusOK)

	drawer, guesser := a, b
	if v := scribbleRoom(env, a, id); v.Drawer != a.PersonaID {
		drawer, guesser = b, a
	}
	dv, gv := scribbleRoom(env, drawer, id), scribbleRoom(env, guesser, id)
	if dv.Phase != "choosing" || len(dv.Choices) != 3 || len(gv.Choices) != 0 {
		t.Fatalf("choosing: drawer %+v guesser %+v", dv, gv)
	}
	move := func(u apitest.User, m map[string]any, want int) {
		t.Helper()
		env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", u.AccessToken, m).Want(want)
	}
	move(guesser, map[string]any{"type": "choose", "index": 0}, http.StatusUnprocessableEntity)
	move(drawer, map[string]any{"type": "choose", "index": 2}, http.StatusNoContent)
	stroke := map[string]any{"type": "stroke", "points": [][2]int{{100, 100}, {900, 900}}, "color": "#111111", "width": 10}
	move(guesser, stroke, http.StatusUnprocessableEntity)
	move(drawer, stroke, http.StatusNoContent)

	dv, gv = scribbleRoom(env, drawer, id), scribbleRoom(env, guesser, id)
	if dv.Word == nil || *dv.Word == "" || dv.Phase != "drawing" {
		t.Fatalf("drawer has no word: %+v", dv)
	}
	if gv.Word != nil || len(gv.Strokes) != 1 || gv.Hint == "" {
		t.Fatalf("guesser view: %+v", gv)
	}
	move(guesser, map[string]any{"type": "guess", "text": *dv.Word}, http.StatusNoContent)
	gv = scribbleRoom(env, guesser, id)
	if gv.Phase != "reveal" || gv.Score[guesser.PersonaID] < 50 || gv.Score[drawer.PersonaID] != 25 {
		t.Fatalf("after correct guess: %+v", gv)
	}
}
