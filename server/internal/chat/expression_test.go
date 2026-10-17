package chat_test

import (
	"bytes"
	"image"
	"image/jpeg"
	"net/http"
	"strings"
	"testing"
	"time"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/chat"
	"metro-buddy/server/internal/media"
	"metro-buddy/server/internal/model"
)

func TestStickersAndEdits(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	conv := dm(t, env, a, b)
	path := "/v1/conversations/" + conv.ID.String() + "/messages"

	var st chat.MessageDTO
	env.Do(http.MethodPost, path, a.AccessToken, map[string]any{"id": model.NewID(), "sticker": "chai"}).Want(http.StatusCreated).JSON(&st)
	if st.Kind != model.MsgSticker || st.Body != "chai" {
		t.Fatalf("%+v", st)
	}
	env.Do(http.MethodPost, path, a.AccessToken, map[string]any{"id": model.NewID(), "sticker": "<script>"}).Want(http.StatusUnprocessableEntity)

	var m chat.MessageDTO
	env.Do(http.MethodPost, path, a.AccessToken, map[string]any{"id": model.NewID(), "body": "see you at coach 4"}).Want(http.StatusCreated).JSON(&m)
	env.Do(http.MethodPatch, "/v1/messages/"+m.ID.String(), b.AccessToken, map[string]any{"body": "hijack"}).Want(http.StatusNotFound)
	var e chat.MessageDTO
	env.Do(http.MethodPatch, "/v1/messages/"+m.ID.String(), a.AccessToken, map[string]any{"body": "coach 5 actually, call 9876543210"}).Want(http.StatusOK).JSON(&e)
	if e.EditedAt == nil || strings.Contains(e.Body, "9876543210") || !strings.Contains(e.Body, "coach 5") {
		t.Fatalf("edit not masked/labelled: %+v", e)
	}
	env.Do(http.MethodPatch, "/v1/messages/"+st.ID.String(), a.AccessToken, map[string]any{"body": "x"}).Want(http.StatusConflict)

	env.App.Chat.Now = func() time.Time { return time.Now().UTC().Add(20 * time.Minute) }
	defer func() { env.App.Chat.Now = nil }()
	env.Do(http.MethodPatch, "/v1/messages/"+m.ID.String(), a.AccessToken, map[string]any{"body": "too late"}).Want(http.StatusConflict)
}

func TestViewOncePhotoOpensOncePerRecipientThenIsDeleted(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	conv := dm(t, env, a, b)
	var ticket media.UploadTicket
	env.Do(http.MethodPost, "/v1/uploads", a.AccessToken, map[string]any{"mime": "image/jpeg", "size": 2000}).Want(http.StatusOK).JSON(&ticket)
	var buf bytes.Buffer
	jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 20, 20)), nil)
	store := env.App.Media.Store.(*media.Memory)
	store.Put(t.Context(), ticket.Key, "image/jpeg", buf.Bytes())

	env.Do(http.MethodPost, "/v1/conversations/"+conv.ID.String()+"/messages", a.AccessToken, map[string]any{"id": model.NewID(), "body": "hi", "view_once": true}).Want(http.StatusUnprocessableEntity)
	var m chat.MessageDTO
	env.Do(http.MethodPost, "/v1/conversations/"+conv.ID.String()+"/messages", a.AccessToken, map[string]any{"id": model.NewID(), "media_key": ticket.Key, "view_once": true}).Want(http.StatusCreated).JSON(&m)
	if !m.ViewOnce || len(m.Media) != 1 || m.Media[0].URL != "" {
		t.Fatalf("view-once leaked a URL: %+v", m.Media)
	}
	var hist struct{ Items []chat.MessageDTO }
	env.Do(http.MethodGet, "/v1/conversations/"+conv.ID.String()+"/messages", b.AccessToken, nil).Want(http.StatusOK).JSON(&hist)
	for _, x := range hist.Items {
		if x.ID == m.ID && (x.ViewedByMe || x.Media[0].URL != "") {
			t.Fatalf("%+v", x)
		}
	}
	env.Do(http.MethodPost, "/v1/messages/"+m.ID.String()+"/view", a.AccessToken, nil).Want(http.StatusForbidden)
	var v struct{ URL string }
	env.Do(http.MethodPost, "/v1/messages/"+m.ID.String()+"/view", b.AccessToken, nil).Want(http.StatusOK).JSON(&v)
	if v.URL == "" {
		t.Fatal("no url")
	}
	env.Do(http.MethodPost, "/v1/messages/"+m.ID.String()+"/view", b.AccessToken, nil).Want(http.StatusGone)
	var stored model.Message
	env.App.DB.First(&stored, "id = ?", m.ID)
	if len(stored.Media) != 0 {
		t.Fatal("media reference kept after everyone viewed")
	}
	env.Do(http.MethodGet, "/v1/conversations/"+conv.ID.String()+"/messages", b.AccessToken, nil).Want(http.StatusOK).JSON(&hist)
	for _, x := range hist.Items {
		if x.ID == m.ID && !x.ViewedByMe {
			t.Fatal("viewed_by_me not set")
		}
	}
}
