package chat_test

import (
	"net/http"
	"strings"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/chat"
	"metro-buddy/server/internal/model"
)

func reveal(env *apitest.Env, u apitest.User, conv string, level string) *apitest.Resp {
	return env.Do(http.MethodPost, "/v1/conversations/"+conv+"/reveal", u.AccessToken, map[string]any{"level": level})
}

func TestRevealLadderNeedsBothAndLiftsContactMaskingOnlyHere(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b, c := env.NewUser("9000000001", city), env.NewUser("9000000002", city), env.NewUser("9000000003", city)
	conv := dm(t, env, a, b)
	other := dm(t, env, a, c)
	id := conv.ID.String()

	reveal(env, a, id, "first_name").Want(http.StatusUnprocessableEntity)
	env.Do(http.MethodPut, "/v1/me/reveal-card", a.AccessToken, map[string]any{"first_name": "Riya", "contact": "@riya_23"}).Want(http.StatusOK)
	env.Do(http.MethodPut, "/v1/me/reveal-card", b.AccessToken, map[string]any{"first_name": "Kabir", "contact": "9876543210"}).Want(http.StatusOK)
	reveal(env, a, id, "photo").Want(http.StatusUnprocessableEntity)

	var d chat.RevealDTO
	reveal(env, a, id, "first_name").Want(http.StatusOK).JSON(&d)
	if d.Mutual != model.RevealNone || d.Revealed.FirstName != "" {
		t.Fatalf("one-sided reveal leaked: %+v", d)
	}
	env.Do(http.MethodGet, "/v1/conversations/"+id+"/reveal", b.AccessToken, nil).Want(http.StatusOK).JSON(&d)
	if d.Theirs != model.RevealFirstName || d.Revealed.FirstName != "" {
		t.Fatalf("%+v", d)
	}
	reveal(env, b, id, "first_name").Want(http.StatusOK).JSON(&d)
	if d.Mutual != model.RevealFirstName || d.Revealed.FirstName != "Riya" {
		t.Fatalf("%+v", d)
	}

	// Contact needs a photo rung first; force both to contact to check masking rules.
	env.App.DB.Model(&model.ConversationMember{}).Where("conversation_id = ?", conv.ID).Update("reveal_level", model.RevealContact)
	var m chat.MessageDTO
	send(env, a, conv.ID, model.NewID(), "text me on 9812345678 or insta @riya_23").Want(http.StatusCreated).JSON(&m)
	if !strings.Contains(m.Body, "9812345678") || !strings.Contains(m.Body, "@riya_23") {
		t.Fatalf("contact still masked after mutual contact reveal: %q", m.Body)
	}
	send(env, a, other.ID, model.NewID(), "text me on 9812345678").Want(http.StatusCreated).JSON(&m)
	if strings.Contains(m.Body, "9812345678") {
		t.Fatal("reveal leaked into another conversation")
	}
	env.Do(http.MethodGet, "/v1/conversations/"+id+"/reveal", b.AccessToken, nil).Want(http.StatusOK).JSON(&d)
	if d.Revealed.Contact != "@riya_23" {
		t.Fatalf("%+v", d)
	}

	// Either side can step back; new messages are masked again and shared details disappear.
	reveal(env, b, id, "none").Want(http.StatusOK).JSON(&d)
	if d.Mutual != model.RevealNone || d.Revealed.FirstName != "" {
		t.Fatalf("%+v", d)
	}
	send(env, a, conv.ID, model.NewID(), "call 9812345678").Want(http.StatusCreated).JSON(&m)
	if strings.Contains(m.Body, "9812345678") {
		t.Fatal("masking not restored after revoke")
	}
	var events int64
	env.App.DB.Model(&model.RevealEvent{}).Where("conversation_id = ?", conv.ID).Count(&events)
	if events != 3 {
		t.Fatalf("reveal events %d", events)
	}
	var audits int64
	env.App.DB.Model(&model.AuditLog{}).Where("verb LIKE ?", "reveal.%").Count(&audits)
	if audits != 2 {
		t.Fatalf("audits %d", audits)
	}
}
