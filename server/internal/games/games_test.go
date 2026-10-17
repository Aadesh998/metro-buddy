package games_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/games"
)

type sudokuView struct {
	Givens   [81]int8  `json:"givens"`
	Board    [81]int8  `json:"board"`
	Solution *[81]int8 `json:"solution"`
	Mistakes int       `json:"mistakes"`
}

func room(env *apitest.Env, u apitest.User, id string) games.RoomDTO {
	var r games.RoomDTO
	env.Do(http.MethodGet, "/v1/games/rooms/"+id, u.AccessToken, nil).Want(http.StatusOK).JSON(&r)
	return r
}

func TestPrivateRoomByCodeAndSudoku(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b, c := env.NewUser("9000000001", city), env.NewUser("9000000002", city), env.NewUser("9000000003", city)

	var r games.RoomDTO
	env.Do(http.MethodPost, "/v1/games/rooms", a.AccessToken, map[string]any{"game": "sudoku", "visibility": "private", "max_players": 2}).
		Want(http.StatusCreated).JSON(&r)
	id := r.ID.String()
	// Private rooms are invisible in the list and by id to outsiders.
	var list struct{ Items []games.RoomDTO }
	env.Do(http.MethodGet, "/v1/games/rooms", b.AccessToken, nil).Want(http.StatusOK).JSON(&list)
	if len(list.Items) != 0 {
		t.Fatal("private room listed")
	}
	env.Do(http.MethodGet, "/v1/games/rooms/"+id, b.AccessToken, nil).Want(http.StatusNotFound)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/join", b.AccessToken, map[string]any{}).Want(http.StatusNotFound)

	env.Do(http.MethodPost, "/v1/games/rooms/join", b.AccessToken, map[string]any{"code": " " + r.Code[:3] + "-" + r.Code[3:]}).Want(http.StatusOK)
	// Full: the third person can only watch.
	env.Do(http.MethodPost, "/v1/games/rooms/join", c.AccessToken, map[string]any{"code": r.Code}).Want(http.StatusConflict)
	env.Do(http.MethodPost, "/v1/games/rooms/join", c.AccessToken, map[string]any{"code": r.Code, "spectator": true}).Want(http.StatusOK)

	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/start", b.AccessToken, nil).Want(http.StatusForbidden)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/start", a.AccessToken, nil).Want(http.StatusUnprocessableEntity)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/ready", b.AccessToken, map[string]any{"ready": true}).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/start", a.AccessToken, nil).Want(http.StatusOK)

	r = room(env, a, id)
	var v sudokuView
	b2, _ := json.Marshal(r.View)
	json.Unmarshal(b2, &v)
	if v.Solution != nil {
		t.Fatal("solution leaked during play")
	}
	empty := -1
	for i, g := range v.Givens {
		if g == 0 {
			empty = i
			break
		}
	}
	// Find the right value by elimination: wrong answers count as mistakes.
	right := 0
	for val := 1; val <= 9; val++ {
		resp := env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", a.AccessToken, map[string]any{"cell": empty, "value": val})
		if resp.Code != http.StatusNoContent {
			t.Fatalf("move: %d %s", resp.Code, resp.Body)
		}
		r = room(env, a, id)
		b2, _ = json.Marshal(r.View)
		json.Unmarshal(b2, &v)
		if v.Board[empty] != 0 {
			right = val
			break
		}
	}
	if right == 0 {
		t.Fatal("no value accepted")
	}
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", b.AccessToken, map[string]any{"cell": empty, "value": right}).Want(http.StatusUnprocessableEntity)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", c.AccessToken, map[string]any{"cell": 0, "value": 1}).Want(http.StatusForbidden)
}

func TestPublicListBySize(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	env.Do(http.MethodPost, "/v1/games/rooms", a.AccessToken, map[string]any{"game": "sudoku", "visibility": "public", "max_players": 2}).Want(http.StatusCreated)
	env.Do(http.MethodPost, "/v1/games/rooms", a.AccessToken, map[string]any{"game": "sudoku", "visibility": "public", "max_players": 6}).Want(http.StatusCreated)
	env.Do(http.MethodPost, "/v1/games/rooms", a.AccessToken, map[string]any{"game": "sudoku", "visibility": "public", "max_players": 20}).Want(http.StatusUnprocessableEntity)
	for size, want := range map[string]int{"small": 1, "group": 1, "party": 0, "": 2} {
		var list struct{ Items []games.RoomDTO }
		env.Do(http.MethodGet, "/v1/games/rooms?size="+size, b.AccessToken, nil).Want(http.StatusOK).JSON(&list)
		if len(list.Items) != want {
			t.Errorf("size %q: got %d rooms", size, len(list.Items))
		}
	}
}
