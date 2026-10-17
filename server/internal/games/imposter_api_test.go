package games_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/games"
)

type imposterAPIView struct {
	Phase     string      `json:"phase"`
	Role      string      `json:"role"`
	Word      string      `json:"word"`
	Category  string      `json:"category"`
	Turn      uuid.UUID   `json:"turn"`
	Imposters []uuid.UUID `json:"imposters"`
}

func imposterAPIRoom(env *apitest.Env, u apitest.User, id string) imposterAPIView {
	var v imposterAPIView
	b, _ := json.Marshal(room(env, u, id).View)
	json.Unmarshal(b, &v)
	return v
}

func TestImposterAPI(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b, c, d := env.NewUser("9000000001", city), env.NewUser("9000000002", city), env.NewUser("9000000003", city), env.NewUser("9000000004", city)

	var r games.RoomDTO
	env.Do(http.MethodPost, "/v1/games/rooms", a.AccessToken, map[string]any{"game": "imposter", "visibility": "private", "max_players": 3}).
		Want(http.StatusCreated).JSON(&r)
	id := r.ID.String()
	for _, u := range []apitest.User{b, c} {
		env.Do(http.MethodPost, "/v1/games/rooms/join", u.AccessToken, map[string]any{"code": r.Code}).Want(http.StatusOK)
		env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/ready", u.AccessToken, map[string]any{"ready": true}).Want(http.StatusOK)
	}
	env.Do(http.MethodPost, "/v1/games/rooms/join", d.AccessToken, map[string]any{"code": r.Code, "spectator": true}).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/start", a.AccessToken, nil).Want(http.StatusOK)

	// Exactly one player is the imposter and only the crew sees the word.
	imposters, word := 0, ""
	var turn uuid.UUID
	users := map[uuid.UUID]apitest.User{}
	for _, u := range []apitest.User{a, b, c} {
		users[u.PersonaID] = u
		v := imposterAPIRoom(env, u, id)
		if v.Phase != "clue" || v.Category == "" || v.Imposters != nil {
			t.Fatalf("view: %+v", v)
		}
		turn = v.Turn
		switch v.Role {
		case "imposter":
			imposters++
			if v.Word != "" {
				t.Fatal("imposter sees the word")
			}
		case "crew":
			word = v.Word
		default:
			t.Fatalf("role %q", v.Role)
		}
	}
	if imposters != 1 || word == "" {
		t.Fatalf("%d imposters, word %q", imposters, word)
	}
	if v := imposterAPIRoom(env, d, id); v.Role != "" || v.Word != "" {
		t.Fatal("spectator sees secrets")
	}

	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", d.AccessToken, map[string]any{"type": "clue", "text": "hi"}).Want(http.StatusForbidden)
	giver := users[turn]
	if imposterAPIRoom(env, giver, id).Role == "crew" {
		env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", giver.AccessToken, map[string]any{"type": "clue", "text": word}).Want(http.StatusUnprocessableEntity)
	}
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", giver.AccessToken, map[string]any{"type": "clue", "text": "hmm"}).Want(http.StatusNoContent)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", giver.AccessToken, map[string]any{"type": "clue", "text": "again"}).Want(http.StatusUnprocessableEntity)
}
