package notify_test

import (
	"net/http"
	"testing"
	"time"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/chat"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/notify"
)

func registerToken(env *apitest.Env, u apitest.User, tok string) {
	env.Do(http.MethodPost, "/v1/me/push-token", u.AccessToken, map[string]any{"token": tok, "platform": "android"}).Want(http.StatusNoContent)
}

func TestQuietHoursBlockPushesExceptSOS(t *testing.T) {
	env := apitest.New(t)
	u := env.NewUser("9000000001", env.City("DEL"))
	registerToken(env, u, "ExponentPushToken[aaa]")
	now := time.Now().In(time.FixedZone("IST", 5*3600+1800))
	m := int16(now.Hour()*60 + now.Minute())
	env.Do(http.MethodPatch, "/v1/me/settings", u.AccessToken, map[string]any{"quiet_hours_start": (m + 1440 - 30) % 1440, "quiet_hours_end": (m + 30) % 1440}).Want(http.StatusOK)

	env.App.Notify.Notify(t.Context(), u.PersonaID, model.NotifReply, nil, notify.Content{Title: "t", Body: "b"}, true)
	if env.Pushes.Count() != 0 {
		t.Fatal("pushed during quiet hours")
	}
	env.App.Notify.Notify(t.Context(), u.PersonaID, model.NotifSOS, nil, notify.Content{Title: "SOS", Body: "b"}, true)
	if env.Pushes.Count() != 1 || env.Pushes.Sent[0].Priority != "high" {
		t.Fatalf("SOS must bypass quiet hours: %+v", env.Pushes.Sent)
	}
	var inbox struct {
		Items  []notify.NotificationDTO
		Unread int64
	}
	env.Do(http.MethodGet, "/v1/notifications", u.AccessToken, nil).Want(http.StatusOK).JSON(&inbox)
	if len(inbox.Items) != 2 || inbox.Unread != 2 {
		t.Fatalf("inbox still records quiet-hours items: %+v", inbox)
	}
	env.Do(http.MethodPost, "/v1/notifications/read", u.AccessToken, map[string]any{"all": true}).Want(http.StatusNoContent)
	env.Do(http.MethodGet, "/v1/notifications", u.AccessToken, nil).Want(http.StatusOK).JSON(&inbox)
	if inbox.Unread != 0 {
		t.Fatal("not marked read")
	}
}

func TestPrefsAndDeadTokens(t *testing.T) {
	env := apitest.New(t)
	u := env.NewUser("9000000001", env.City("DEL"))
	registerToken(env, u, "ExponentPushToken[dead]")
	env.Pushes.Dead["ExponentPushToken[dead]"] = true
	env.Do(http.MethodPatch, "/v1/me/settings", u.AccessToken, map[string]any{"notif_prefs": map[string]bool{"reply": false}}).Want(http.StatusOK)
	env.Do(http.MethodPatch, "/v1/me/settings", u.AccessToken, map[string]any{"notif_prefs": map[string]bool{"marketing": true}}).Want(http.StatusUnprocessableEntity)
	env.App.Notify.Notify(t.Context(), u.PersonaID, model.NotifReply, nil, notify.Content{}, true)
	if env.Pushes.Count() != 0 {
		t.Fatal("pushed a disabled kind")
	}
	env.App.Notify.Notify(t.Context(), u.PersonaID, model.NotifMatch, nil, notify.Content{}, true)
	var d model.Device
	env.App.DB.Where("push_token IS NOT NULL").Limit(1).Find(&d)
	if d.PushToken != nil {
		t.Fatal("DeviceNotRegistered token kept")
	}
}

func TestOfflineDMRecipientGetsPushWithoutMessageBody(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	registerToken(env, b, "ExponentPushToken[bbb]")
	var r chat.RequestDTO
	env.Do(http.MethodPost, "/v1/dm-requests", a.AccessToken, map[string]any{"to_persona_id": b.PersonaID, "intro": "hello friend", "context_type": "profile"}).Want(http.StatusCreated).JSON(&r)
	if env.Pushes.Count() != 1 || env.Pushes.Sent[0].Title != "New message request" {
		t.Fatalf("%+v", env.Pushes.Sent)
	}
	var conv chat.ConversationDTO
	env.Do(http.MethodPost, "/v1/dm-requests/"+r.ID.String()+"/accept", b.AccessToken, nil).Want(http.StatusOK).JSON(&conv)
	env.Do(http.MethodPost, "/v1/conversations/"+conv.ID.String()+"/messages", a.AccessToken, map[string]any{"id": model.NewID(), "body": "secret plans at 6pm"}).Want(http.StatusCreated)
	last := env.Pushes.Sent[len(env.Pushes.Sent)-1]
	if last.Title != "New message" || last.Body == "" || last.Data["link"] != "metrobuddy://chat/"+conv.ID.String() {
		t.Fatalf("%+v", last)
	}
	for _, p := range env.Pushes.Sent {
		if p.Body == "secret plans at 6pm" || p.Title == "secret plans at 6pm" {
			t.Fatal("message body leaked into push")
		}
	}
	n := env.Pushes.Count()
	env.Do(http.MethodPatch, "/v1/conversations/"+conv.ID.String(), b.AccessToken, map[string]any{"muted_until": time.Now().UTC().Add(time.Hour)}).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/conversations/"+conv.ID.String()+"/messages", a.AccessToken, map[string]any{"id": model.NewID(), "body": "you there?"}).Want(http.StatusCreated)
	if env.Pushes.Count() != n {
		t.Fatal("pushed to a muted chat")
	}
}
