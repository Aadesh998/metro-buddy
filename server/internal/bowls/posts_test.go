package bowls_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/bowls"
	"metro-buddy/server/internal/media"
	"metro-buddy/server/internal/model"
)

type fixture struct {
	env  *apitest.Env
	bowl string
	a, b apitest.User
}

func setup(t *testing.T) fixture {
	env := apitest.New(t)
	city := env.Seeded()
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	var list struct{ Items []bowls.BowlDTO }
	env.Do(http.MethodGet, "/v1/bowls?kind=topic&q=office", a.AccessToken, nil).Want(http.StatusOK).JSON(&list)
	id := list.Items[0].ID.String()
	for _, u := range []apitest.User{a, b} {
		env.Do(http.MethodPost, "/v1/bowls/"+id+"/join", u.AccessToken, nil).Want(http.StatusOK)
	}
	return fixture{env, id, a, b}
}

func (f fixture) post(u apitest.User, body map[string]any) *apitest.Resp {
	if _, ok := body["bowl_id"]; !ok {
		body["bowl_id"] = f.bowl
	}
	return f.env.Do(http.MethodPost, "/v1/posts", u.AccessToken, body)
}

func TestPostBodyIsNeverStoredUnmasked(t *testing.T) {
	f := setup(t)
	var p bowls.PostDTO
	f.post(f.a, map[string]any{"type": "text", "title": "Boss called me at 98765 43210", "body": "mail me a@b.com or insta: riya_23", "flair": "Rant"}).
		Want(http.StatusCreated).JSON(&p)
	if !p.PIIHidden || strings.Contains(p.Title, "43210") {
		t.Fatalf("%+v", p)
	}
	var rows []map[string]any
	f.env.App.DB.Raw("SELECT * FROM posts").Scan(&rows)
	dump := fmt.Sprint(rows)
	for _, leak := range []string{"43210", "a@b.com", "riya_23"} {
		if strings.Contains(dump, leak) {
			t.Fatalf("posts table contains %q", leak)
		}
	}
	var vault int64
	f.env.App.DB.Model(&model.PIIVault{}).Where("content_id = ?", p.ID).Count(&vault)
	if vault != 1 {
		t.Fatal("raw post not vaulted")
	}
}

func TestPostingRequiresMembershipAndValidShape(t *testing.T) {
	f := setup(t)
	f.env.Do(http.MethodDelete, "/v1/bowls/"+f.bowl+"/join", f.b.AccessToken, nil).Want(http.StatusOK)
	if r := f.post(f.b, map[string]any{"type": "text", "title": "hi"}); r.Code != http.StatusForbidden {
		t.Fatalf("non-member posted: %s", r.Body)
	}
	f.post(f.a, map[string]any{"type": "poll", "title": "x", "poll": map[string]any{"options": []string{"only one"}, "closes_in_hours": 2}}).Want(http.StatusUnprocessableEntity)
	f.post(f.a, map[string]any{"type": "text", "title": "x", "flair": "NotAFlair"}).Want(http.StatusUnprocessableEntity)
	f.post(f.a, map[string]any{"type": "image", "title": "x"}).Want(http.StatusUnprocessableEntity)
}

func TestPollVotingOncePerPersona(t *testing.T) {
	f := setup(t)
	var p bowls.PostDTO
	f.post(f.a, map[string]any{"type": "poll", "title": "Worst line?", "poll": map[string]any{"options": []string{"Blue", "Yellow", "Call 9876543210"}, "closes_in_hours": 24}}).
		Want(http.StatusCreated).JSON(&p)
	if len(p.Poll.Options) != 3 || p.Poll.Options[2].Label != "Call [phone hidden]" {
		t.Fatalf("%+v", p.Poll)
	}
	opt := p.Poll.Options[0].ID
	path := "/v1/posts/" + p.ID.String() + "/poll/vote"
	f.env.Do(http.MethodPost, path, f.b.AccessToken, map[string]any{"option_ids": []uuid.UUID{opt, p.Poll.Options[1].ID}}).Want(http.StatusUnprocessableEntity)
	f.env.Do(http.MethodPost, path, f.b.AccessToken, map[string]any{"option_ids": []uuid.UUID{opt}}).Want(http.StatusOK).JSON(&p)
	f.env.Do(http.MethodPost, path, f.b.AccessToken, map[string]any{"option_ids": []uuid.UUID{opt}}).Want(http.StatusConflict)
	if p.Poll.Total != 1 || !p.Poll.Options[0].Mine {
		t.Fatalf("%+v", p.Poll)
	}
}

// jpegWithEXIF builds a JPEG carrying an EXIF block with orientation 6 and a fake GPS marker.
func jpegWithEXIF(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for x := range 40 {
		for y := range 20 {
			img.Set(x, y, color.RGBA{uint8(x * 6), 100, 50, 255})
		}
	}
	var plain bytes.Buffer
	jpeg.Encode(&plain, img, nil)
	tiff := []byte("II*\x00\x08\x00\x00\x00\x01\x00")
	entry := make([]byte, 12)
	binary.LittleEndian.PutUint16(entry[0:], 0x0112)
	binary.LittleEndian.PutUint16(entry[2:], 3)
	binary.LittleEndian.PutUint32(entry[4:], 1)
	binary.LittleEndian.PutUint16(entry[8:], 6)
	tiff = append(append(tiff, entry...), []byte("\x00\x00\x00\x00GPS-28.6139N-77.2090E")...)
	app1 := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(app1)+2))
	out := append([]byte{0xFF, 0xD8}, append(seg, app1...)...)
	return append(out, plain.Bytes()[2:]...)
}

func TestImagePostStripsEXIFAndKeepsOrientation(t *testing.T) {
	f := setup(t)
	var ticket media.UploadTicket
	f.env.Do(http.MethodPost, "/v1/uploads", f.a.AccessToken, map[string]any{"mime": "image/jpeg", "size": 2000}).Want(http.StatusOK).JSON(&ticket)
	f.env.Do(http.MethodPost, "/v1/uploads", f.a.AccessToken, map[string]any{"mime": "image/gif", "size": 2000}).Want(http.StatusUnprocessableEntity)
	store := f.env.App.Media.Store.(*media.Memory)
	store.Put(t.Context(), ticket.Key, "image/jpeg", jpegWithEXIF(t))

	f.post(f.b, map[string]any{"type": "image", "title": "steal", "media_keys": []string{ticket.Key}}).Want(http.StatusUnprocessableEntity)

	var p bowls.PostDTO
	f.post(f.a, map[string]any{"type": "image", "title": "Sunset at Yamuna Bank", "media_keys": []string{ticket.Key}}).Want(http.StatusCreated).JSON(&p)
	if len(p.Media) != 1 || p.Media[0].Pending || p.Media[0].URL == "" {
		t.Fatalf("%+v", p.Media)
	}
	if p.Media[0].Width != 20 || p.Media[0].Height != 40 {
		t.Fatalf("orientation not applied: %dx%d", p.Media[0].Width, p.Media[0].Height)
	}
	if _, err := store.Get(t.Context(), ticket.Key, media.MaxUploadBytes); err == nil {
		t.Fatal("temporary upload not deleted")
	}
	for k, b := range store.Objects {
		if bytes.Contains(b, []byte("Exif")) || bytes.Contains(b, []byte("GPS-28")) {
			t.Fatalf("%s still has metadata", k)
		}
	}
}
