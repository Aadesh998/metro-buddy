package travel_test

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
	"metro-buddy/server/internal/chat"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/travel"
)

func (w world) matchedChat(t *testing.T, a, b string) (string, []chat.MessageDTO) {
	return w.matchedChatFor(t, w.user(a, "man"), w.user(b, "man"))
}

func (w world) matchedChatFor(t *testing.T, ua, ub apitest.User) (string, []chat.MessageDTO) {
	for _, u := range []string{ua.AccessToken, ub.AccessToken} {
		w.env.Do(http.MethodPut, "/v1/me/vibes", u, map[string]any{"slugs": []string{"chai-over-coffee", "cricket", "introvert"}})
	}
	w.travelNow(ua, "Rajiv Chowk", "Millennium City Centre Gurugram", 0).Want(http.StatusCreated)
	w.travelNow(ub, "Central Secretariat", "Saket", 5).Want(http.StatusCreated)
	m := w.matches(ua)[0]
	w.env.Do(http.MethodPost, "/v1/matches/"+m.ID.String()+"/accept", ua.AccessToken, nil).Want(http.StatusOK)
	var res travel.MatchDTO
	w.env.Do(http.MethodPost, "/v1/matches/"+m.ID.String()+"/accept", ub.AccessToken, nil).Want(http.StatusOK).JSON(&res)
	var msgs struct{ Items []chat.MessageDTO }
	w.env.Do(http.MethodGet, "/v1/conversations/"+res.ConversationID.String()+"/messages", ua.AccessToken, nil).Want(http.StatusOK).JSON(&msgs)
	return res.ConversationID.String(), msgs.Items
}

func icebreaker(msgs []chat.MessageDTO) string {
	for _, m := range msgs {
		if m.Kind == model.MsgIcebreaker {
			return m.Body
		}
	}
	return ""
}

func TestTemplateIcebreakersWithoutAI(t *testing.T) {
	w := setup(t)
	_, msgs := w.matchedChat(t, "9000000001", "9000000002")
	ib := icebreaker(msgs)
	if strings.Count(ib, "\n") != 2 {
		t.Fatalf("want 3 openers: %q", ib)
	}
}

func TestAIIcebreakersCarryNoPersonalDataAndAreCapped(t *testing.T) {
	w := setup(t)
	var calls atomic.Int32
	var prompts []string
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		b, _ := io.ReadAll(r.Body)
		prompts = append(prompts, string(b))
		json.NewEncoder(rw).Encode(map[string]any{
			"content":     []map[string]any{{"type": "text", "text": `{"openers":["Team chai here, you?","Who wins the next match?","DM me on insta @riya_23","Quiet ride or chatty ride?"]}`}},
			"stop_reason": "end_turn", "usage": map[string]int{"input_tokens": 120, "output_tokens": 40},
		})
	}))
	defer srv.Close()
	w.env.App.Travel.AI = &ai.Client{APIKey: "k", BaseURL: srv.URL, DB: w.env.App.DB}

	_, msgs := w.matchedChat(t, "9000000001", "9000000002")
	ib := icebreaker(msgs)
	if ib != "Team chai here, you?\nWho wins the next match?\nQuiet ride or chatty ride?" {
		t.Fatalf("openers %q", ib)
	}
	p := prompts[0]
	if !strings.Contains(p, "claude-sonnet") || strings.Contains(p, "Rajiv Chowk") || strings.Contains(p, "Saket") || strings.Contains(p, "Central Secretariat") {
		t.Fatalf("prompt leaks or wrong model: %s", p)
	}
	var aliases []string
	w.env.App.DB.Model(&model.Persona{}).Pluck("alias", &aliases)
	for _, a := range aliases {
		if strings.Contains(p, a) {
			t.Fatalf("prompt carries alias %q", a)
		}
	}
	// The accepting user's daily budget runs out; the chat still gets template openers.
	c, d := w.user("9000000003", "man"), w.user("9000000004", "man")
	var actor model.Persona
	w.env.App.DB.First(&actor, "id = ?", d.PersonaID)
	for range travel.IcebreakerDailyCap {
		w.env.App.DB.Create(&model.AIUsage{Purpose: model.AIIcebreaker, UserID: &actor.UserID, Model: ai.ModelSonnet})
	}
	_, msgs = w.matchedChatFor(t, c, d)
	if calls.Load() != 1 || icebreaker(msgs) == "" {
		t.Fatalf("calls %d, ib %q", calls.Load(), icebreaker(msgs))
	}
}
