package chat_test

import (
	"net/http"
	"testing"
	"time"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/chat"
	"metro-buddy/server/internal/model"
)

func TestDisappearingMessagesAreHiddenThenPurged(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	conv := dm(t, env, a, b)
	path := "/v1/conversations/" + conv.ID.String()
	send(env, a, conv.ID, model.NewID(), "this one stays").Want(http.StatusCreated)
	env.Do(http.MethodPut, path+"/disappearing", b.AccessToken, map[string]any{"seconds": 42}).Want(http.StatusUnprocessableEntity)
	var got chat.ConversationDTO
	env.Do(http.MethodPut, path+"/disappearing", b.AccessToken, map[string]any{"seconds": 3600}).Want(http.StatusOK).JSON(&got)
	if got.Disappear == nil || *got.Disappear != 3600 {
		t.Fatalf("%+v", got)
	}
	var m chat.MessageDTO
	send(env, a, conv.ID, model.NewID(), "this one vanishes").Want(http.StatusCreated).JSON(&m)
	reported := model.NewID()
	send(env, a, conv.ID, reported, "reported one").Want(http.StatusCreated)
	env.Do(http.MethodPost, "/v1/reports", b.AccessToken, map[string]any{"target_type": "message", "target_id": reported, "reason": "harassment"}).Want(http.StatusCreated)
	if m.ExpiresAt == nil {
		t.Fatal("no expiry on new message")
	}

	env.App.Chat.Now = func() time.Time { return time.Now().UTC().Add(2 * time.Hour) }
	defer func() { env.App.Chat.Now = nil }()
	var msgs struct{ Items []chat.MessageDTO }
	env.Do(http.MethodGet, path+"/messages", b.AccessToken, nil).Want(http.StatusOK).JSON(&msgs)
	for _, x := range msgs.Items {
		if x.ID == m.ID {
			t.Fatal("expired message still served")
		}
	}
	if err := env.App.Chat.PurgeExpired(t.Context()); err != nil {
		t.Fatal(err)
	}
	var left []string
	env.App.DB.Unscoped().Model(&model.Message{}).Where("conversation_id = ? AND kind = ?", conv.ID, model.MsgText).Order("created_at").Pluck("body_masked", &left)
	if len(left) != 3 || left[1] != "this one stays" || left[2] != "reported one" {
		t.Fatalf("left %v", left)
	}
	env.Do(http.MethodPut, path+"/disappearing", a.AccessToken, map[string]any{"seconds": 0}).Want(http.StatusOK).JSON(&got)
	if got.Disappear != nil {
		t.Fatal("not turned off")
	}
}
