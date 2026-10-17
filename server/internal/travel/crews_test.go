package travel_test

import (
	"net/http"
	"testing"
	"time"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/chat"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/travel"
)

func (w world) acceptedMatch(t *testing.T, a, b apitest.User) string {
	w.travelNow(a, "Rajiv Chowk", "Millennium City Centre Gurugram", 0).Want(http.StatusCreated)
	w.travelNow(b, "Central Secretariat", "Saket", 5).Want(http.StatusCreated)
	var m travel.MatchDTO
	for _, x := range w.matches(a) {
		if x.Other.ID == b.PersonaID && x.Status == model.MatchProposed {
			m = x
		}
	}
	w.env.Do(http.MethodPost, "/v1/matches/"+m.ID.String()+"/accept", a.AccessToken, nil).Want(http.StatusOK)
	w.env.Do(http.MethodPost, "/v1/matches/"+m.ID.String()+"/accept", b.AccessToken, nil).Want(http.StatusOK)
	// Free both up for another Travel Now.
	w.env.App.DB.Model(&model.TravelIntent{}).Where("persona_id IN ?", []any{a.PersonaID, b.PersonaID}).Update("status", model.IntentStatus("cancelled"))
	return m.ID.String()
}

func TestCrewFromMatchInviteAcceptLeave(t *testing.T) {
	w := setup(t)
	a, b, c := w.user("9000000001", "man"), w.user("9000000002", "man"), w.user("9000000003", "man")
	ab := w.acceptedMatch(t, a, b)

	w.env.Do(http.MethodPost, "/v1/crews", c.AccessToken, map[string]any{"match_id": ab, "name": "Yellow Line 9:10"}).Want(http.StatusNotFound)
	var crew travel.CrewDTO
	w.env.Do(http.MethodPost, "/v1/crews", a.AccessToken, map[string]any{"match_id": ab, "name": "Yellow Line 9:10"}).Want(http.StatusCreated).JSON(&crew)
	if crew.Line.Name != "Yellow Line" || len(crew.Members) != 1 || crew.PendingInvites != 1 || crew.ConversationID == nil {
		t.Fatalf("%+v", crew)
	}
	// c never matched with a, so a can't pull them in.
	w.env.Do(http.MethodPost, "/v1/crews/"+crew.ID.String()+"/invites", a.AccessToken, map[string]any{"match_id": ab}).Want(http.StatusConflict)
	w.env.Do(http.MethodGet, "/v1/crews/"+crew.ID.String(), c.AccessToken, nil).Want(http.StatusForbidden)

	var invs struct{ Items []travel.CrewInviteDTO }
	w.env.Do(http.MethodGet, "/v1/crew-invites", b.AccessToken, nil).Want(http.StatusOK).JSON(&invs)
	if len(invs.Items) != 1 || invs.Items[0].From.ID != a.PersonaID {
		t.Fatalf("%+v", invs.Items)
	}
	w.env.Do(http.MethodPost, "/v1/crew-invites/"+invs.Items[0].ID.String()+"/accept", b.AccessToken, nil).Want(http.StatusOK).JSON(&crew)
	if len(crew.Members) != 2 {
		t.Fatalf("%+v", crew.Members)
	}
	var conv chat.ConversationDTO
	w.env.Do(http.MethodGet, "/v1/conversations/"+crew.ConversationID.String(), b.AccessToken, nil).Want(http.StatusOK).JSON(&conv)
	if conv.Kind != model.ConvCrew {
		t.Fatalf("%+v", conv)
	}
	w.env.Do(http.MethodPatch, "/v1/crews/"+crew.ID.String(), b.AccessToken, map[string]any{"depart_at": "08:45", "weekdays": []int{1, 3, 5}}).Want(http.StatusOK).JSON(&crew)
	if crew.DepartAt != "08:45" || len(crew.Weekdays) != 3 {
		t.Fatalf("%+v", crew)
	}

	w.env.Do(http.MethodPost, "/v1/crews/"+crew.ID.String()+"/leave", a.AccessToken, nil).Want(http.StatusNoContent)
	w.env.Do(http.MethodPost, "/v1/crews/"+crew.ID.String()+"/leave", b.AccessToken, nil).Want(http.StatusNoContent)
	var n int64
	w.env.App.DB.Model(&model.Crew{}).Where("id = ?", crew.ID).Count(&n)
	if n != 0 {
		t.Fatal("empty crew not removed")
	}
}

func TestCrewCapAndReminder(t *testing.T) {
	w := setup(t)
	a, b := w.user("9000000001", "man"), w.user("9000000002", "man")
	ab := w.acceptedMatch(t, a, b)
	var crew travel.CrewDTO
	w.env.Do(http.MethodPost, "/v1/crews", a.AccessToken, map[string]any{"match_id": ab, "name": "Morning Crew"}).Want(http.StatusCreated).JSON(&crew)
	for i := range 4 {
		w.env.App.DB.Create(&model.ConversationMember{ConversationID: *crew.ConversationID, PersonaID: model.NewID(), JoinedAt: time.Now().UTC().Add(time.Duration(i) * time.Second)})
	}
	// 5 members + 1 pending invite = full.
	c := w.user("9000000003", "man")
	ac := w.acceptedMatch(t, a, c)
	w.env.Do(http.MethodPost, "/v1/crews/"+crew.ID.String()+"/invites", a.AccessToken, map[string]any{"match_id": ac}).Want(http.StatusConflict)

	ist := time.FixedZone("IST", 5*3600+1800)
	at := time.Date(2026, 10, 19, 8, 50, 0, 0, ist) // a Monday
	w.env.App.DB.Model(&model.Crew{}).Where("id = ?", crew.ID).Updates(map[string]any{"depart_window_start": 9*60 + 10, "weekdays": 1})
	w.env.App.Travel.Now = func() time.Time { return at.UTC() }
	defer func() { w.env.App.Travel.Now = nil }()
	w.env.App.Travel.CrewReminders(t.Context())
	w.env.App.Travel.CrewReminders(t.Context())
	var reminders int64
	w.env.App.DB.Model(&model.Message{}).Where("conversation_id = ? AND body_masked LIKE ?", crew.ConversationID, "Crew ride at 09:10%").Count(&reminders)
	if reminders != 1 {
		t.Fatalf("reminders %d", reminders)
	}
}
