package analytics_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
)

func TestEventsCarryNoIdentifiersAndRespectConsent(t *testing.T) {
	env := apitest.New(t)
	u := env.NewUser("9876543210", env.City("DEL"))
	sess := uuid.NewString()
	body := map[string]any{"session_id": sess, "platform": "android", "events": []map[string]any{
		{"name": "post.created", "props": map[string]any{"post_type": "poll", "persona_id": u.PersonaID, "text": "my number 9876543210", "bowl_kind": "line"}},
		{"name": "BAD NAME", "props": map[string]any{}},
	}}
	env.Do(http.MethodPost, "/v1/events", u.AccessToken, body).Want(http.StatusNoContent)
	var rows []model.AnalyticsEvent
	env.App.DB.Find(&rows)
	if len(rows) != 1 {
		t.Fatalf("%d rows", len(rows))
	}
	dump := fmt.Sprint(rows[0])
	for _, leak := range []string{u.PersonaID.String(), "9876543210", sess} {
		if strings.Contains(dump, leak) {
			t.Fatalf("event leaks %q: %s", leak, dump)
		}
	}
	if rows[0].Props["post_type"] != "poll" || rows[0].Props["bowl_kind"] != "line" {
		t.Fatalf("%+v", rows[0].Props)
	}

	env.Do(http.MethodPost, "/v1/me/consents", u.AccessToken, map[string]any{"policy_version": identity.PolicyVersion,
		"items": []map[string]any{{"purpose": "analytics", "granted": false}}}).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/events", u.AccessToken, body).Want(http.StatusNoContent)
	var n int64
	env.App.DB.Model(&model.AnalyticsEvent{}).Count(&n)
	if n != 1 {
		t.Fatal("events stored without consent")
	}
}
