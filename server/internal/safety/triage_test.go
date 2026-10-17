package safety_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"metro-buddy/server/internal/ai"
	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/safety"
)

func TestTriageAnnotatesCaseWithMaskedTextOnly(t *testing.T) {
	env := apitest.New(t)
	city := env.Seeded()
	author, reporter := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	var bowl struct{ Items []struct{ ID string } }
	env.Do(http.MethodGet, "/v1/bowls?kind=topic", author.AccessToken, nil).Want(http.StatusOK).JSON(&bowl)
	env.Do(http.MethodPost, "/v1/bowls/"+bowl.Items[0].ID+"/join", author.AccessToken, nil).Want(http.StatusOK)
	var post struct{ ID string }
	env.Do(http.MethodPost, "/v1/posts", author.AccessToken, map[string]any{
		"bowl_id": bowl.Items[0].ID, "type": "text", "title": "you all are useless", "body": "call me 9876543210 and I'll show you",
	}).Want(http.StatusCreated).JSON(&post)
	env.Do(http.MethodPost, "/v1/reports", reporter.AccessToken, map[string]any{"target_type": "post", "target_id": post.ID, "reason": "harassment", "details": "threatening"}).Want(http.StatusCreated)

	var calls atomic.Int32
	var sent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		b, _ := io.ReadAll(r.Body)
		sent = string(b)
		json.NewEncoder(w).Encode(map[string]any{
			"content":     []map[string]any{{"type": "text", "text": `{"severity":"high","rationale":"Hostile post with an implied threat; reporter says it is threatening."}`}},
			"stop_reason": "end_turn", "usage": map[string]int{"input_tokens": 200, "output_tokens": 30},
		})
	}))
	defer srv.Close()
	tr := &safety.Triage{DB: env.App.DB, AI: &ai.Client{APIKey: "k", BaseURL: srv.URL, DB: env.App.DB}}
	var mc model.ModerationCase
	env.App.DB.First(&mc, "target_id = ?", post.ID)
	if err := tr.Process(t.Context(), mc.ID); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sent, "9876543210") || !strings.Contains(sent, "[phone hidden]") || !strings.Contains(sent, "harassment") {
		t.Fatalf("prompt: %s", sent)
	}
	env.App.DB.First(&mc, "id = ?", mc.ID)
	if mc.AISeverity == nil || *mc.AISeverity != model.SevHigh || mc.AIRationale == nil || mc.Status != model.CaseOpen {
		t.Fatalf("%+v", mc)
	}
	var visible int64
	env.App.DB.Model(&model.Post{}).Where("id = ? AND status = ?", post.ID, model.ContentVisible).Count(&visible)
	if visible != 1 {
		t.Fatal("triage must never act on content")
	}

	// Authors who turned off AI processing are never sent to the model.
	env.Do(http.MethodPost, "/v1/me/consents", author.AccessToken, map[string]any{
		"policy_version": identity.PolicyVersion, "items": []map[string]any{{"purpose": "ai_processing", "granted": false}},
	}).Want(http.StatusOK)
	if err := tr.Process(t.Context(), mc.ID); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls %d", calls.Load())
	}
}
