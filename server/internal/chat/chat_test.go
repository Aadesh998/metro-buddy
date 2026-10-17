package chat_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/chat"
	"metro-buddy/server/internal/model"
)

func dm(t *testing.T, env *apitest.Env, from, to apitest.User) chat.ConversationDTO {
	t.Helper()
	var r chat.RequestDTO
	env.Do(http.MethodPost, "/v1/dm-requests", from.AccessToken, map[string]any{
		"to_persona_id": to.PersonaID, "intro": "Hey, saw your post about the Yellow line", "context_type": "profile",
	}).Want(http.StatusCreated).JSON(&r)
	var conv chat.ConversationDTO
	env.Do(http.MethodPost, "/v1/dm-requests/"+r.ID.String()+"/accept", to.AccessToken, nil).Want(http.StatusOK).JSON(&conv)
	return conv
}

func send(env *apitest.Env, u apitest.User, conv uuid.UUID, id uuid.UUID, body string) *apitest.Resp {
	return env.Do(http.MethodPost, "/v1/conversations/"+conv.String()+"/messages", u.AccessToken, map[string]any{"id": id, "body": body})
}

func TestCannotMessageBeforeAcceptance(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	var r chat.RequestDTO
	env.Do(http.MethodPost, "/v1/dm-requests", a.AccessToken, map[string]any{"to_persona_id": b.PersonaID, "intro": "hi there, call 9876543210", "context_type": "profile"}).
		Want(http.StatusCreated).JSON(&r)
	if strings.Contains(r.Intro, "9876543210") {
		t.Fatal("intro not masked")
	}
	env.Do(http.MethodPost, "/v1/dm-requests", a.AccessToken, map[string]any{"to_persona_id": b.PersonaID, "intro": "again?", "context_type": "profile"}).Want(http.StatusConflict)
	var convs struct{ Items []chat.ConversationDTO }
	env.Do(http.MethodGet, "/v1/conversations", a.AccessToken, nil).Want(http.StatusOK).JSON(&convs)
	if len(convs.Items) != 0 {
		t.Fatal("conversation exists before acceptance")
	}
	env.Do(http.MethodPost, "/v1/dm-requests/"+r.ID.String()+"/accept", a.AccessToken, nil).Want(http.StatusNotFound)

	var in struct{ Items []chat.RequestDTO }
	env.Do(http.MethodGet, "/v1/dm-requests", b.AccessToken, nil).Want(http.StatusOK).JSON(&in)
	if len(in.Items) != 1 {
		t.Fatalf("%+v", in)
	}
	var conv chat.ConversationDTO
	env.Do(http.MethodPost, "/v1/dm-requests/"+r.ID.String()+"/accept", b.AccessToken, nil).Want(http.StatusOK).JSON(&conv)
	if conv.Kind != model.ConvDM || len(conv.Members) != 2 || conv.LastMessage == nil {
		t.Fatalf("%+v", conv)
	}
}

func TestDuplicateClientIDStoredOnce(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	conv := dm(t, env, a, b)
	id := model.NewID()
	var m1, m2 chat.MessageDTO
	send(env, a, conv.ID, id, "on my way").Want(http.StatusCreated).JSON(&m1)
	send(env, a, conv.ID, id, "on my way").Want(http.StatusOK).JSON(&m2)
	if m1.ID != m2.ID {
		t.Fatal("ids differ")
	}
	var n int64
	env.App.DB.Model(&model.Message{}).Where("id = ?", id).Count(&n)
	if n != 1 {
		t.Fatalf("stored %d times", n)
	}
	send(env, b, conv.ID, id, "hijack").Want(http.StatusConflict)
	send(env, a, conv.ID, uuid.New(), "v4 id").Want(http.StatusUnprocessableEntity)
	backdated := model.NewID()
	ts := time.Now().Add(-time.Hour).UnixMilli()
	for i := range 6 {
		backdated[i] = byte(ts >> (40 - 8*i))
	}
	send(env, a, conv.ID, backdated, "from the past").Want(http.StatusUnprocessableEntity)
}

func TestHistoryUnreadAndReceipts(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	conv := dm(t, env, a, b)
	var last chat.MessageDTO
	for i := range 5 {
		send(env, a, conv.ID, model.NewID(), fmt.Sprintf("msg %d", i)).Want(http.StatusCreated).JSON(&last)
	}
	var c chat.ConversationDTO
	env.Do(http.MethodGet, "/v1/conversations/"+conv.ID.String(), b.AccessToken, nil).Want(http.StatusOK).JSON(&c)
	if c.Unread != 6 {
		t.Fatalf("unread %d, want 6 (intro + 5)", c.Unread)
	}
	env.Do(http.MethodPost, "/v1/conversations/"+conv.ID.String()+"/read", b.AccessToken, map[string]any{"message_id": last.ID}).Want(http.StatusNoContent)
	env.Do(http.MethodGet, "/v1/conversations/"+conv.ID.String(), b.AccessToken, nil).Want(http.StatusOK).JSON(&c)
	if c.Unread != 0 {
		t.Fatalf("unread after read %d", c.Unread)
	}
	var page struct {
		Items      []chat.MessageDTO
		NextCursor string `json:"next_cursor"`
	}
	env.Do(http.MethodGet, "/v1/conversations/"+conv.ID.String()+"/messages?limit=4", b.AccessToken, nil).Want(http.StatusOK).JSON(&page)
	if len(page.Items) != 4 || page.Items[0].Body != "msg 4" || page.NextCursor == "" {
		t.Fatalf("%+v", page)
	}
	stranger := env.NewUser("9000000003", city)
	env.Do(http.MethodGet, "/v1/conversations/"+conv.ID.String()+"/messages", stranger.AccessToken, nil).Want(http.StatusNotFound)
	send(env, stranger, conv.ID, model.NewID(), "let me in").Want(http.StatusNotFound)
}

func TestBlockStopsMessagesAndHidesDM(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	conv := dm(t, env, a, b)
	env.Do(http.MethodPost, "/v1/blocks", b.AccessToken, map[string]any{"persona_id": a.PersonaID}).Want(http.StatusNoContent)
	send(env, a, conv.ID, model.NewID(), "hello?").Want(http.StatusForbidden)
	var list struct{ Items []chat.ConversationDTO }
	env.Do(http.MethodGet, "/v1/conversations", a.AccessToken, nil).Want(http.StatusOK).JSON(&list)
	if len(list.Items) != 0 {
		t.Fatal("blocked DM still listed")
	}
	env.Do(http.MethodPost, "/v1/dm-requests", a.AccessToken, map[string]any{"to_persona_id": b.PersonaID, "intro": "sorry", "context_type": "profile"}).Want(http.StatusNotFound)
}

func TestGroupsInviteRolesAndNonMembers404(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	owner, m1, outsider := env.NewUser("9000000001", city), env.NewUser("9000000002", city), env.NewUser("9000000003", city)
	var g chat.ConversationDTO
	env.Do(http.MethodPost, "/v1/groups", owner.AccessToken, map[string]any{"title": "Yellow Line 9am crew"}).Want(http.StatusOK).JSON(&g)
	if g.InviteCode == nil || len(*g.InviteCode) != 8 {
		t.Fatalf("%+v", g)
	}
	base := "/v1/conversations/" + g.ID.String()
	env.Do(http.MethodGet, base, outsider.AccessToken, nil).Want(http.StatusNotFound)
	env.Do(http.MethodGet, base+"/messages", outsider.AccessToken, nil).Want(http.StatusNotFound)
	send(env, outsider, g.ID, model.NewID(), "hi").Want(http.StatusNotFound)
	env.Do(http.MethodPost, "/v1/groups/join", m1.AccessToken, map[string]any{"code": "WRONG123"}).Want(http.StatusNotFound)
	var joined chat.ConversationDTO
	env.Do(http.MethodPost, "/v1/groups/join", m1.AccessToken, map[string]any{"code": *g.InviteCode}).Want(http.StatusOK).JSON(&joined)
	if joined.MemberCount != 2 || joined.InviteCode != nil {
		t.Fatalf("members should not see the invite code: %+v", joined)
	}
	env.Do(http.MethodDelete, base+"/members/"+owner.PersonaID.String(), m1.AccessToken, nil).Want(http.StatusForbidden)
	env.Do(http.MethodPatch, base+"/members/"+m1.PersonaID.String(), owner.AccessToken, map[string]any{"role": "admin"}).Want(http.StatusOK)
	env.Do(http.MethodDelete, base+"/members/"+m1.PersonaID.String(), owner.AccessToken, nil).Want(http.StatusNoContent)
	env.Do(http.MethodGet, base, m1.AccessToken, nil).Want(http.StatusNotFound)
}

func TestRoomsNeedTrustRespectSlowModeAndAutoClose(t *testing.T) {
	env := apitest.New(t)
	city := env.Seeded()
	host, guest := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	var bowl model.Bowl
	env.App.DB.First(&bowl, "slug = ?", "office-rant")
	env.Do(http.MethodPost, "/v1/bowls/"+bowl.ID.String()+"/join", host.AccessToken, nil).Want(http.StatusOK)
	body := map[string]any{"bowl_id": bowl.ID, "title": "Monday Office Rant", "starts_at": time.Now().UTC(), "duration_min": 30, "slow_mode_sec": 30}
	env.Do(http.MethodPost, "/v1/rooms", host.AccessToken, body).Want(http.StatusForbidden)
	env.App.DB.Model(&model.User{}).Where("id IN (SELECT user_id FROM personas WHERE id = ?)", host.PersonaID).Update("trust_tier", 1)
	var room chat.ConversationDTO
	env.Do(http.MethodPost, "/v1/rooms", host.AccessToken, body).Want(http.StatusOK).JSON(&room)

	var live struct{ Items []chat.RoomDTO }
	env.Do(http.MethodGet, "/v1/rooms/live", guest.AccessToken, nil).Want(http.StatusOK).JSON(&live)
	if len(live.Items) != 1 || !live.Items[0].Live {
		t.Fatalf("%+v", live)
	}
	env.Do(http.MethodPost, "/v1/rooms/"+room.ID.String()+"/join", guest.AccessToken, nil).Want(http.StatusOK)
	send(env, guest, room.ID, model.NewID(), "first").Want(http.StatusCreated)
	if r := send(env, guest, room.ID, model.NewID(), "second"); r.ErrCode() != "slow_mode" {
		t.Fatalf("slow mode not enforced: %s", r.Body)
	}
	send(env, host, room.ID, model.NewID(), "host isn't slowed").Want(http.StatusCreated)
	send(env, host, room.ID, model.NewID(), "still not").Want(http.StatusCreated)

	env.App.DB.Model(&model.Conversation{}).Where("id = ?", room.ID).Update("ends_at", time.Now().UTC().Add(-time.Second))
	if err := env.App.Chat.CloseExpiredRooms(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r := send(env, host, room.ID, model.NewID(), "too late"); r.Code != http.StatusConflict {
		t.Fatalf("closed room accepted message: %d", r.Code)
	}
}

func TestMessageDeliveredOverWebSocket(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	conv := dm(t, env, a, b)
	srv := httptest.NewServer(env.App.Echo)
	defer srv.Close()
	var tk struct{ Ticket string }
	env.Do(http.MethodPost, "/v1/ws/ticket", b.AccessToken, nil).Want(http.StatusOK).JSON(&tk)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws?ticket="+tk.Ticket, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	ws.Read(ctx)
	time.Sleep(50 * time.Millisecond)
	send(env, a, conv.ID, model.NewID(), "see you at coach 3").Want(http.StatusCreated)
	for {
		_, raw, err := ws.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var ev struct {
			Type string
			Data chat.MessageDTO
		}
		json.Unmarshal(raw, &ev)
		if ev.Type == "message.new" {
			if ev.Data.Body != "see you at coach 3" {
				t.Fatalf("%+v", ev)
			}
			return
		}
	}
}
