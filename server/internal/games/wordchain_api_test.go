package games_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/games"
)

type wordchainAPIView struct {
	Letter  string         `json:"letter"`
	Turn    string         `json:"turn"`
	Used    int            `json:"used"`
	Refusal *string        `json:"refusal"`
	Lives   map[string]int `json:"lives"`
	Chain   []struct {
		Word string `json:"word"`
		By   string `json:"by"`
	} `json:"chain"`
}

func wordchainAPIRoom(env *apitest.Env, u apitest.User, id string) wordchainAPIView {
	var v wordchainAPIView
	b, _ := json.Marshal(room(env, u, id).View)
	json.Unmarshal(b, &v)
	return v
}

// A known word for every starting letter the game can pick.
var wordchainAPIWords = map[string]string{
	"a": "area", "b": "bomb", "c": "comic", "d": "dead", "e": "edge", "f": "fabric", "g": "giving", "h": "health", "i": "idea",
	"j": "jazz", "k": "knock", "l": "level", "m": "medium", "n": "nation", "o": "open", "p": "pump", "r": "rumor", "s": "sees",
	"t": "tent", "u": "until", "v": "valley", "w": "window", "y": "yummy",
}

func TestWordchainRoom(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b, c := env.NewUser("9000000001", city), env.NewUser("9000000002", city), env.NewUser("9000000003", city)

	var r games.RoomDTO
	env.Do(http.MethodPost, "/v1/games/rooms", a.AccessToken, map[string]any{"game": "wordchain", "visibility": "public", "max_players": 2}).
		Want(http.StatusCreated).JSON(&r)
	id := r.ID.String()
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/join", b.AccessToken, map[string]any{}).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/join", c.AccessToken, map[string]any{"spectator": true}).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/ready", b.AccessToken, map[string]any{"ready": true}).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/start", a.AccessToken, nil).Want(http.StatusOK)

	v := wordchainAPIRoom(env, a, id)
	if v.Letter == "" || len(v.Lives) != 2 {
		t.Fatalf("view: %+v", v)
	}
	users := map[string]apitest.User{}
	for _, u := range []apitest.User{a, b} {
		users[room(env, u, id).Me.Persona.ID.String()] = u
	}
	on, off := users[v.Turn], a
	if on == a {
		off = b
	}
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", off.AccessToken, map[string]any{"type": "word", "text": "anything"}).Want(http.StatusUnprocessableEntity)
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", c.AccessToken, map[string]any{"type": "word", "text": "anything"}).Want(http.StatusForbidden)

	// A miss is saved as a strike; only the player whose turn it is sees why.
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", on.AccessToken, map[string]any{"type": "word", "text": "zzzzzz"}).Want(http.StatusNoContent)
	if w := wordchainAPIRoom(env, on, id); w.Refusal == nil || *w.Refusal == "" {
		t.Fatal("refusal not shown to the player")
	}
	if w := wordchainAPIRoom(env, c, id); w.Refusal != nil {
		t.Fatal("refusal shown to a spectator")
	}

	word := wordchainAPIWords[v.Letter]
	env.Do(http.MethodPost, "/v1/games/rooms/"+id+"/move", on.AccessToken, map[string]any{"type": "word", "text": word}).Want(http.StatusNoContent)
	v = wordchainAPIRoom(env, c, id)
	if v.Used != 1 || len(v.Chain) != 1 || v.Chain[0].Word != word || users[v.Turn] != off {
		t.Fatalf("after %q: %+v", word, v)
	}
}
