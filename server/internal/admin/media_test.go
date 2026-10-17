package admin_test

import (
	"bytes"
	"image"
	"image/jpeg"
	"net/http"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/chat"
	"metro-buddy/server/internal/media"
	"metro-buddy/server/internal/model"
)

func TestHeldChatImageStaysHiddenUntilModeratorClearsIt(t *testing.T) {
	env := apitest.New(t)
	c := console{env}
	env.App.Media.Scanner = media.ReviewAll{}
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	var r chat.RequestDTO
	env.Do(http.MethodPost, "/v1/dm-requests", a.AccessToken, map[string]any{"to_persona_id": b.PersonaID, "intro": "Hi from the Blue line", "context_type": "profile"}).Want(http.StatusCreated).JSON(&r)
	var conv chat.ConversationDTO
	env.Do(http.MethodPost, "/v1/dm-requests/"+r.ID.String()+"/accept", b.AccessToken, nil).Want(http.StatusOK).JSON(&conv)

	var ticket media.UploadTicket
	env.Do(http.MethodPost, "/v1/uploads", a.AccessToken, map[string]any{"mime": "image/jpeg", "size": 2000}).Want(http.StatusOK).JSON(&ticket)
	var buf bytes.Buffer
	jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 30, 30)), nil)
	env.App.Media.Store.(*media.Memory).Put(t.Context(), ticket.Key, "image/jpeg", buf.Bytes())
	msgID := model.NewID()
	var m chat.MessageDTO
	env.Do(http.MethodPost, "/v1/conversations/"+conv.ID.String()+"/messages", a.AccessToken, map[string]any{"id": msgID, "media_key": ticket.Key}).Want(http.StatusCreated).JSON(&m)
	if m.Kind != model.MsgImage || len(m.Media) != 1 || !m.Media[0].Pending || m.Media[0].URL != "" {
		t.Fatalf("image visible before scan cleared: %+v", m.Media)
	}
	var mc model.ModerationCase
	if err := env.App.DB.First(&mc, "target_id = ? AND source = ?", msgID, "image_review").Error; err != nil {
		t.Fatal("no review case", err)
	}
	lead := cookieOf(c.login(t, c.newAdmin(t, "lead@m.in", model.AdminTSLead), ""))
	if rec := c.do(http.MethodPost, "/admin/api/cases/"+mc.ID.String()+"/dismiss", lead, `{"note":"Just a photo of a train"}`, "application/json"); rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var msgs struct{ Items []chat.MessageDTO }
	env.Do(http.MethodGet, "/v1/conversations/"+conv.ID.String()+"/messages", b.AccessToken, nil).Want(http.StatusOK).JSON(&msgs)
	for _, x := range msgs.Items {
		if x.ID == msgID && (x.Media[0].Pending || x.Media[0].URL == "") {
			t.Fatalf("still held after dismissal: %+v", x.Media)
		}
	}
}
