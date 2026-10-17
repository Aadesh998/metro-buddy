package games_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/games"
)

type chessView struct {
	You   string   `json:"you"`
	Turn  string   `json:"turn"`
	Moves []string `json:"moves"`
	Legal []string `json:"legal"`
	White string   `json:"white"`
}

func chessViewOf(env *apitest.Env, u apitest.User, id string) chessView {
	var v chessView
	b, _ := json.Marshal(room(env, u, id).View)
	json.Unmarshal(b, &v)
	return v
}

func TestChessRoom(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b, c := env.NewUser("9000000001", city), env.NewUser("9000000002", city), env.NewUser("9000000003", city)

	var r games.RoomDTO
	env.Do(http.MethodPost, "/v1/games/rooms", a.AccessToken, map[string]any{"game": "chess", "visibility": "public"}).
		Want(http.StatusCreated).JSON(&r)
	id := r.ID.String()
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/join", b.AccessToken, map[string]any{}).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/join", c.AccessToken, map[string]any{}).Want(http.StatusConflict)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/join", c.AccessToken, map[string]any{"spectator": true}).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/ready", b.AccessToken, map[string]any{"ready": true}).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/start", a.AccessToken, nil).Want(http.StatusOK)

	white, black := a, b
	if chessViewOf(env, a, id).You != "white" {
		white, black = b, a
	}
	if v := chessViewOf(env, white, id); v.Turn != "white" || len(v.Legal) != 20 {
		t.Fatalf("white view: %+v", v)
	}
	if v := chessViewOf(env, c, id); v.You != "spectator" || len(v.Legal) != 0 {
		t.Fatalf("spectator view: %+v", v)
	}
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", black.AccessToken, map[string]any{"type": "move", "uci": "e7e5"}).Want(http.StatusUnprocessableEntity)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", white.AccessToken, map[string]any{"type": "move", "uci": "e2e5"}).Want(http.StatusUnprocessableEntity)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", white.AccessToken, map[string]any{"type": "move", "uci": "e2e4"}).Want(http.StatusNoContent)
	if v := chessViewOf(env, black, id); v.Turn != "black" || len(v.Moves) != 1 || v.Moves[0] != "e4" || len(v.Legal) != 20 {
		t.Fatalf("black view: %+v", v)
	}
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", black.AccessToken, map[string]any{"type": "resign"}).Want(http.StatusNoContent)
	r = room(env, white, id)
	if r.Status != games.StatusFinished {
		t.Fatalf("status %s", r.Status)
	}
	for _, p := range r.Players {
		if !p.Spectator && p.Persona.ID == white.PersonaID && p.Score != 3 {
			t.Fatalf("white scored %d", p.Score)
		}
	}
}
