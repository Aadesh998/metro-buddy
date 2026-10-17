package games_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/games"
)

type breakwallView struct {
	Round int     `json:"round"`
	Phase string  `json:"phase"`
	Seed  *uint32 `json:"seed"`
	Rows  int     `json:"rows"`
	Cols  int     `json:"cols"`
	Total int     `json:"total"`
	Board []struct {
		Score int  `json:"score"`
		Final bool `json:"final"`
	} `json:"board"`
}

func TestBreakwallRoom(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)

	var r games.RoomDTO
	env.Do(http.MethodPost, "/v1/games/rooms", a.AccessToken, map[string]any{"game": "breakwall", "visibility": "public", "max_players": 10}).
		Want(http.StatusCreated).JSON(&r)
	id := r.ID.String()
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/join", b.AccessToken, map[string]any{"spectator": true}).Want(http.StatusOK)
	// One player is enough.
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/start", a.AccessToken, nil).Want(http.StatusOK)

	r = room(env, b, id)
	var v breakwallView
	raw, _ := json.Marshal(r.View)
	json.Unmarshal(raw, &v)
	if v.Round != 1 || v.Phase != "countdown" || v.Seed == nil || v.Rows == 0 || v.Total == 0 || len(v.Board) != 1 {
		t.Fatalf("view %s", raw)
	}
	// Still counting down: no scores yet; spectators can't report at all.
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", a.AccessToken, map[string]any{"type": "score", "round": 1, "bricks": 0, "score": 0}).
		Want(http.StatusUnprocessableEntity)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", b.AccessToken, map[string]any{"type": "score", "round": 1, "bricks": 0, "score": 0}).
		Want(http.StatusForbidden)
}
