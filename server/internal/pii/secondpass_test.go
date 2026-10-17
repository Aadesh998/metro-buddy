package pii_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"metro-buddy/server/internal/ai"
	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
)

func fakeAnthropic(t *testing.T, calls *atomic.Int32, findings string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		json.Unmarshal(body, &req)
		oc := req["output_config"].(map[string]any)
		if req["model"] != ai.ModelHaiku || oc["format"].(map[string]any)["type"] != "json_schema" || r.Header.Get("x-api-key") != "k" {
			t.Errorf("unexpected request %s", body)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"content":     []map[string]any{{"type": "text", "text": findings}},
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 300, "output_tokens": 40},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func setup(t *testing.T, aiConsent bool, findings string) (*apitest.Env, *pii.SecondPass, uuid.UUID, *atomic.Int32, *[]string) {
	env := apitest.New(t)
	u := env.NewUser("9876543210", env.City("DEL"))
	if !aiConsent {
		env.Do(http.MethodPost, "/v1/me/consents", u.AccessToken, map[string]any{
			"policy_version": identity.PolicyVersion, "items": []map[string]any{{"purpose": "ai_processing", "granted": false}},
		}).Want(http.StatusOK)
	}
	calls := &atomic.Int32{}
	srv := fakeAnthropic(t, calls, findings)
	var updates []string
	sp := &pii.SecondPass{
		DB: env.App.DB, Redis: nil, Log: nil,
		AI:        &ai.Client{APIKey: "k", BaseURL: srv.URL, DB: env.App.DB},
		OnUpdated: func(_ model.ContentType, _ uuid.UUID, m string) { updates = append(updates, m) },
	}
	msg := model.Message{ID: model.NewID(), ConversationID: model.NewID(), SenderPersonaID: &u.PersonaID, Kind: model.MsgText,
		BodyMasked: "call me nine eight seven six five four three two one zero, my num [phone hidden]"}
	if err := env.App.DB.Create(&msg).Error; err != nil {
		t.Fatal(err)
	}
	return env, sp, msg.ID, calls, &updates
}

func TestSecondPassMasksSpelledOutNumberAndNotifiesClients(t *testing.T) {
	env, sp, id, _, updates := setup(t, true,
		`{"findings":[{"text":"nine eight seven six five four three two one zero","kind":"phone"},{"text":"[phone hidden]","kind":"phone"},{"text":"not in text","kind":"email"}]}`)
	if err := sp.Process(t.Context(), model.CTMessage, id); err != nil {
		t.Fatal(err)
	}
	var m model.Message
	env.App.DB.First(&m, "id = ?", id)
	if strings.Contains(m.BodyMasked, "nine") || m.BodyMasked != "call me [phone hidden], my num [phone hidden]" || m.PIIFlags&pii.FlagPhone == 0 {
		t.Fatalf("masked = %q flags=%b", m.BodyMasked, m.PIIFlags)
	}
	if len(*updates) != 1 || (*updates)[0] != m.BodyMasked {
		t.Fatalf("updates %v", *updates)
	}
	var det model.PIIDetection
	env.App.DB.First(&det, "content_id = ?", id)
	if det.Source != model.DetectorLLM {
		t.Fatalf("%+v", det)
	}
	var usage model.AIUsage
	env.App.DB.First(&usage)
	if usage.Model != ai.ModelHaiku || usage.CostMicrosUSD != 50 || usage.Purpose != model.AIPIISecondPass {
		t.Fatalf("usage %+v", usage)
	}
}

func TestSecondPassSkipsAuthorsWithoutAIConsent(t *testing.T) {
	_, sp, id, calls, _ := setup(t, false, `{"findings":[]}`)
	if err := sp.Process(t.Context(), model.CTMessage, id); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("content sent to AI without consent")
	}
}
