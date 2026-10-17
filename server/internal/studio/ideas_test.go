package studio_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"metro-buddy/server/internal/ai"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/studio"
)

func TestTopicIdeasUseOnlyAggregatedMaskedTitles(t *testing.T) {
	w := setup(t)
	h := w.host("9000000001")
	listener := w.env.NewUser("9000000002", w.city)
	w.env.Do(http.MethodGet, "/v1/studio/topic-ideas", listener.AccessToken, nil).Want(http.StatusForbidden)

	db := w.env.App.DB
	big := model.Bowl{CityID: &w.city, Kind: model.BowlTopic, Slug: "office-rant", Name: "Office Rant", MemberCount: 500, Visibility: model.BowlPublic}
	tiny := model.Bowl{CityID: &w.city, Kind: model.BowlTopic, Slug: "tiny", Name: "Tiny Circle", MemberCount: 3, Visibility: model.BowlPublic}
	db.Create(&big)
	db.Create(&tiny)
	for i := range 6 {
		db.Create(&model.Post{BowlID: big.ID, AuthorPersonaID: listener.PersonaID, Type: model.PostText, Title: fmt.Sprintf("Manager drama part %d", i),
			BodyMasked: "secret body text", Status: model.ContentVisible, CommentCount: 10 - i})
	}
	db.Create(&model.Post{BowlID: tiny.ID, AuthorPersonaID: listener.PersonaID, Type: model.PostText, Title: "Only five of us know this", Status: model.ContentVisible, CommentCount: 99})

	var d studio.IdeasDTO
	w.env.Do(http.MethodGet, "/v1/studio/topic-ideas", h.AccessToken, nil).Want(http.StatusOK).JSON(&d)
	if d.FromAI || len(d.Ideas) != 4 || d.Ideas[0].Topic != "This week in Office Rant" {
		t.Fatalf("fallback %+v", d)
	}

	var prompt string
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		prompt = string(b)
		json.NewEncoder(rw).Encode(map[string]any{
			"content":     []map[string]any{{"type": "text", "text": `{"ideas":[{"topic":"When the boss takes credit","hook":"Call in and tell us how you handled it."},{"topic":"Call 9876543210 now","hook":"x"}]}`}},
			"stop_reason": "end_turn", "usage": map[string]int{"input_tokens": 300, "output_tokens": 80},
		})
	}))
	defer srv.Close()
	w.env.App.Studio.AI = &ai.Client{APIKey: "k", BaseURL: srv.URL, DB: db}
	w.env.Do(http.MethodGet, "/v1/studio/topic-ideas?refresh=1", h.AccessToken, nil).Want(http.StatusOK).JSON(&d)
	if !d.FromAI || len(d.Ideas) != 1 || d.Ideas[0].Topic != "When the boss takes credit" {
		t.Fatalf("ai %+v", d)
	}
	var alias string
	db.Model(&model.Persona{}).Where("id = ?", listener.PersonaID).Pluck("alias", &alias)
	if !strings.Contains(prompt, "Manager drama part 0") || strings.Contains(prompt, "secret body") || strings.Contains(prompt, "Only five") || strings.Contains(prompt, alias) {
		t.Fatalf("prompt %s", prompt)
	}
	// Cached for everyone in the city until the next refresh.
	var again studio.IdeasDTO
	w.env.Do(http.MethodGet, "/v1/studio/topic-ideas", h.AccessToken, nil).Want(http.StatusOK).JSON(&again)
	if !again.FromAI {
		t.Fatal("cache miss")
	}
}
