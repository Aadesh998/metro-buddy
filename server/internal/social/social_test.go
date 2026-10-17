package social_test

import (
	"net/http"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/chat"
	"metro-buddy/server/internal/social"
)

func rel(env *apitest.Env, viewer, other apitest.User) social.RelationshipDTO {
	var r social.RelationshipDTO
	env.Do(http.MethodGet, "/v1/personas/"+other.PersonaID.String()+"/relationship", viewer.AccessToken, nil).Want(http.StatusOK).JSON(&r)
	return r
}

func TestFollowIsOneWay(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	env.Do(http.MethodPost, "/v1/personas/"+b.PersonaID.String()+"/follow", a.AccessToken, nil).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/personas/"+b.PersonaID.String()+"/follow", a.AccessToken, nil).Want(http.StatusOK)
	if r := rel(env, a, b); !r.Following || r.FollowsYou || r.Followers != 1 {
		t.Fatalf("%+v", r)
	}
	if r := rel(env, b, a); r.Following || !r.FollowsYou {
		t.Fatalf("%+v", r)
	}
	var list struct{ Items []struct{ ID string } }
	env.Do(http.MethodGet, "/v1/me/followers", b.AccessToken, nil).Want(http.StatusOK).JSON(&list)
	if len(list.Items) != 1 || list.Items[0].ID != a.PersonaID.String() {
		t.Fatalf("%+v", list)
	}
	env.Do(http.MethodDelete, "/v1/personas/"+b.PersonaID.String()+"/follow", a.AccessToken, nil).Want(http.StatusNoContent)
	if r := rel(env, a, b); r.Following || r.Followers != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestConnectionGatesDirectChat(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	open := map[string]any{"persona_id": b.PersonaID}
	if code := env.Do(http.MethodPost, "/v1/dm/open", a.AccessToken, open).Want(http.StatusForbidden).ErrCode(); code != "not_connected" {
		t.Fatal(code)
	}

	env.Do(http.MethodPost, "/v1/connections", a.AccessToken, map[string]any{"to_persona_id": b.PersonaID, "note": "call me 9876543210"}).Want(http.StatusCreated)
	if r := rel(env, a, b); r.Connection != social.ConnOutgoing {
		t.Fatalf("%+v", r)
	}
	var in struct{ Items []social.ConnectionDTO }
	env.Do(http.MethodGet, "/v1/connections?box=incoming", b.AccessToken, nil).Want(http.StatusOK).JSON(&in)
	if len(in.Items) != 1 || in.Items[0].Note == "" || in.Items[0].Note == "call me 9876543210" {
		t.Fatalf("%+v", in)
	}
	// Only the recipient can accept.
	env.Do(http.MethodPost, "/v1/connections/"+in.Items[0].ID.String()+"/accept", a.AccessToken, nil).Want(http.StatusNotFound)
	env.Do(http.MethodPost, "/v1/connections/"+in.Items[0].ID.String()+"/accept", b.AccessToken, nil).Want(http.StatusOK)
	if r := rel(env, a, b); r.Connection != social.ConnDone || r.Connections != 1 {
		t.Fatalf("%+v", r)
	}

	var conv chat.ConversationDTO
	env.Do(http.MethodPost, "/v1/dm/open", a.AccessToken, open).Want(http.StatusOK).JSON(&conv)
	var again chat.ConversationDTO
	env.Do(http.MethodPost, "/v1/dm/open", b.AccessToken, map[string]any{"persona_id": a.PersonaID}).Want(http.StatusOK).JSON(&again)
	if conv.ID != again.ID || len(conv.Members) != 2 {
		t.Fatalf("%+v %+v", conv, again)
	}
}

func TestDeclineStaysHiddenFromSender(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	env.Do(http.MethodPost, "/v1/connections", a.AccessToken, map[string]any{"to_persona_id": b.PersonaID}).Want(http.StatusCreated)
	var in struct{ Items []social.ConnectionDTO }
	env.Do(http.MethodGet, "/v1/connections?box=incoming", b.AccessToken, nil).Want(http.StatusOK).JSON(&in)
	env.Do(http.MethodPost, "/v1/connections/"+in.Items[0].ID.String()+"/decline", b.AccessToken, nil).Want(http.StatusNoContent)
	if r := rel(env, a, b); r.Connection != social.ConnOutgoing {
		t.Fatalf("sender should still see pending: %+v", r)
	}
	if r := rel(env, b, a); r.Connection != social.ConnNone {
		t.Fatalf("%+v", r)
	}
	// Re-sending is a no-op, no new notification.
	env.Do(http.MethodPost, "/v1/connections", a.AccessToken, map[string]any{"to_persona_id": b.PersonaID}).Want(http.StatusOK)
	// The decliner can still ask later.
	env.Do(http.MethodPost, "/v1/connections", b.AccessToken, map[string]any{"to_persona_id": a.PersonaID}).Want(http.StatusCreated)
	if r := rel(env, a, b); r.Connection != social.ConnIncoming {
		t.Fatalf("%+v", r)
	}
}

func TestMutualRequestAccepts(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	env.Do(http.MethodPost, "/v1/connections", a.AccessToken, map[string]any{"to_persona_id": b.PersonaID}).Want(http.StatusCreated)
	env.Do(http.MethodPost, "/v1/connections", b.AccessToken, map[string]any{"to_persona_id": a.PersonaID}).Want(http.StatusOK)
	if r := rel(env, a, b); r.Connection != social.ConnDone {
		t.Fatalf("%+v", r)
	}
}

func TestBlockSevers(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	env.Do(http.MethodPost, "/v1/personas/"+b.PersonaID.String()+"/follow", a.AccessToken, nil).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/connections", a.AccessToken, map[string]any{"to_persona_id": b.PersonaID}).Want(http.StatusCreated)
	env.Do(http.MethodPost, "/v1/blocks", b.AccessToken, map[string]any{"persona_id": a.PersonaID}).Want(http.StatusNoContent)
	env.Do(http.MethodGet, "/v1/personas/"+b.PersonaID.String()+"/relationship", a.AccessToken, nil).Want(http.StatusNotFound)
	env.Do(http.MethodPost, "/v1/personas/"+b.PersonaID.String()+"/follow", a.AccessToken, nil).Want(http.StatusNotFound)
	env.Do(http.MethodDelete, "/v1/blocks/"+a.PersonaID.String(), b.AccessToken, nil).Want(http.StatusNoContent)
	if r := rel(env, a, b); r.Following || r.Connection != social.ConnNone {
		t.Fatalf("block should have cleared everything: %+v", r)
	}
}
