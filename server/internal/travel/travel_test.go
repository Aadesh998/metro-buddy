package travel_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/chat"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/travel"
)

type world struct {
	env  *apitest.Env
	city string
}

func setup(t *testing.T) world {
	env := apitest.New(t)
	return world{env, env.Seeded().String()}
}

func (w world) user(phone, gender string) apitest.User {
	u := w.env.NewUser(phone, w.env.StationCity())
	if gender != "" {
		w.env.Do(http.MethodPatch, "/v1/me", u.AccessToken, map[string]any{"gender": gender}).Want(http.StatusOK)
	}
	return u
}

func (w world) travelNow(u apitest.User, from, to string, inMin int) *apitest.Resp {
	return w.env.Do(http.MethodPost, "/v1/travel-intents", u.AccessToken, map[string]any{
		"from_station_id": w.env.StationID(from), "to_station_id": w.env.StationID(to), "depart_in_min": inMin, "mood": "chatty",
	})
}

func (w world) matches(u apitest.User) []travel.MatchDTO {
	var r struct{ Items []travel.MatchDTO }
	w.env.Do(http.MethodPost, "/v1/matches/refresh", u.AccessToken, nil).Want(http.StatusOK).JSON(&r)
	return r.Items
}

func TestMatchShowsOnlySharedSegment(t *testing.T) {
	w := setup(t)
	a, b := w.user("9000000001", "man"), w.user("9000000002", "man")
	w.travelNow(a, "Rajiv Chowk", "Millennium City Centre Gurugram", 0).Want(http.StatusCreated)
	w.travelNow(b, "Central Secretariat", "Saket", 5).Want(http.StatusCreated)
	ms := w.matches(b)
	if len(ms) != 1 {
		t.Fatalf("matches %+v", ms)
	}
	m := ms[0]
	if m.SegmentFrom.Name != "Central Secretariat" || m.SegmentTo.Name != "Saket" || m.Score <= 0 || len(m.Why.Chips) < 3 {
		t.Fatalf("%+v", m)
	}
	raw := w.env.Do(http.MethodGet, "/v1/matches", b.AccessToken, nil).Want(http.StatusOK).Body
	for _, leak := range []string{"Rajiv Chowk", "Millennium City Centre", "user_id", "gender", "birth"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("match leaks %q: %s", leak, raw)
		}
	}
	if !strings.Contains(strings.Join(m.Why.Chips, " "), "Yellow Line") {
		t.Fatalf("chips %v", m.Why.Chips)
	}
}

func TestNeverMatchedExpiredBlockedOppositeOrIncompatible(t *testing.T) {
	w := setup(t)
	a := w.user("9000000001", "woman")
	w.travelNow(a, "Rajiv Chowk", "Millennium City Centre Gurugram", 0).Want(http.StatusCreated)

	expired := w.user("9000000002", "woman")
	w.travelNow(expired, "Patel Chowk", "Saket", 0).Want(http.StatusCreated)
	w.env.App.DB.Model(&model.TravelIntent{}).Where("persona_id = ?", expired.PersonaID).Update("expires_at", time.Now().UTC().Add(-time.Minute))

	blocked := w.user("9000000003", "woman")
	w.travelNow(blocked, "Patel Chowk", "Saket", 0).Want(http.StatusCreated)
	w.env.Do(http.MethodPost, "/v1/blocks", blocked.AccessToken, map[string]any{"persona_id": a.PersonaID}).Want(http.StatusNoContent)

	opposite := w.user("9000000004", "woman")
	w.travelNow(opposite, "Saket", "Patel Chowk", 0).Want(http.StatusCreated)

	late := w.user("9000000005", "woman")
	w.travelNow(late, "Patel Chowk", "Saket", 45).Want(http.StatusCreated)

	man := w.user("9000000006", "man")
	w.travelNow(man, "Patel Chowk", "Saket", 0).Want(http.StatusCreated)
	w.env.Do(http.MethodPatch, "/v1/me/settings", a.AccessToken, map[string]any{"match_same_gender_only": true}).Want(http.StatusOK)

	ok := w.user("9000000007", "woman")
	w.travelNow(ok, "Patel Chowk", "Saket", 3).Want(http.StatusCreated)

	w.env.App.DB.Where("1=1").Delete(&model.Match{})
	ms := w.matches(a)
	if len(ms) != 1 || ms[0].Other.ID != ok.PersonaID {
		t.Fatalf("expected only the compatible rider, got %d: %+v", len(ms), ms)
	}
}

func TestMutualAcceptCreatesChatTripAndInStationMeetPoint(t *testing.T) {
	w := setup(t)
	a, b := w.user("9000000001", "woman"), w.user("9000000002", "woman")
	w.travelNow(a, "Rajiv Chowk", "Millennium City Centre Gurugram", 0).Want(http.StatusCreated)
	w.travelNow(b, "Central Secretariat", "Saket", 5).Want(http.StatusCreated)
	m := w.matches(a)[0]

	var res travel.MatchDTO
	w.env.Do(http.MethodPost, "/v1/matches/"+m.ID.String()+"/accept", a.AccessToken, nil).Want(http.StatusOK).JSON(&res)
	if res.ConversationID != nil || res.Status != model.MatchProposed {
		t.Fatal("chat created before both accepted")
	}
	var convs struct{ Items []chat.ConversationDTO }
	w.env.Do(http.MethodGet, "/v1/conversations", a.AccessToken, nil).Want(http.StatusOK).JSON(&convs)
	if len(convs.Items) != 0 {
		t.Fatal("conversation exists after one-sided accept")
	}
	w.env.Do(http.MethodPost, "/v1/matches/"+m.ID.String()+"/accept", b.AccessToken, nil).Want(http.StatusOK).JSON(&res)
	if res.ConversationID == nil || res.TripID == nil || res.Status != model.MatchAccepted {
		t.Fatalf("%+v", res)
	}
	var trip travel.TripDTO
	w.env.Do(http.MethodGet, "/v1/trips/"+res.TripID.String(), a.AccessToken, nil).Want(http.StatusOK).JSON(&trip)
	if trip.MeetStation.Name != "Central Secretariat" || !strings.Contains(trip.MeetPlatform, "towards Millennium City Centre Gurugram") || trip.MeetCoach != "Coach 1 (women's coach)" {
		t.Fatalf("%+v", trip)
	}
	var page struct{ Items []chat.MessageDTO }
	w.env.Do(http.MethodGet, "/v1/conversations/"+res.ConversationID.String()+"/messages", b.AccessToken, nil).Want(http.StatusOK).JSON(&page)
	found := false
	for _, msg := range page.Items {
		if msg.Kind == model.MsgMeetPoint && strings.Contains(msg.Body, "Stay inside the station") {
			found = true
		}
	}
	if !found {
		t.Fatalf("meet point message missing: %+v", page.Items)
	}
	stranger := w.user("9000000003", "")
	w.env.Do(http.MethodGet, "/v1/trips/"+res.TripID.String(), stranger.AccessToken, nil).Want(http.StatusNotFound)
}

func TestMixedPairNeverGetsWomensCoach(t *testing.T) {
	w := setup(t)
	a, b := w.user("9000000001", "woman"), w.user("9000000002", "man")
	w.travelNow(a, "Rajiv Chowk", "Millennium City Centre Gurugram", 0).Want(http.StatusCreated)
	w.travelNow(b, "Central Secretariat", "Saket", 5).Want(http.StatusCreated)
	m := w.matches(a)[0]
	w.env.Do(http.MethodPost, "/v1/matches/"+m.ID.String()+"/accept", a.AccessToken, nil).Want(http.StatusOK)
	var res travel.MatchDTO
	w.env.Do(http.MethodPost, "/v1/matches/"+m.ID.String()+"/accept", b.AccessToken, nil).Want(http.StatusOK).JSON(&res)
	var trip travel.TripDTO
	w.env.Do(http.MethodGet, "/v1/trips/"+res.TripID.String(), a.AccessToken, nil).Want(http.StatusOK).JSON(&trip)
	if strings.Contains(trip.MeetCoach, "women") {
		t.Fatalf("mixed pair sent to women's coach: %s", trip.MeetCoach)
	}
}

func TestTripCheckinFeedbackFlagAndSOS(t *testing.T) {
	w := setup(t)
	a, b := w.user("9000000001", "man"), w.user("9000000002", "man")
	w.travelNow(a, "Rajiv Chowk", "Millennium City Centre Gurugram", 0).Want(http.StatusCreated)
	w.travelNow(b, "Central Secretariat", "Saket", 5).Want(http.StatusCreated)
	m := w.matches(a)[0]
	w.env.Do(http.MethodPost, "/v1/matches/"+m.ID.String()+"/accept", a.AccessToken, nil).Want(http.StatusOK)
	var res travel.MatchDTO
	w.env.Do(http.MethodPost, "/v1/matches/"+m.ID.String()+"/accept", b.AccessToken, nil).Want(http.StatusOK).JSON(&res)
	trip := "/v1/trips/" + res.TripID.String()

	w.env.Do(http.MethodPost, "/v1/me/trusted-contacts", a.AccessToken, map[string]any{"name": "Didi", "phone": "98111 22233"}).Want(http.StatusOK)
	var contacts struct{ Items []travel.ContactDTO }
	w.env.Do(http.MethodGet, "/v1/me/trusted-contacts", a.AccessToken, nil).Want(http.StatusOK).JSON(&contacts)
	if len(contacts.Items) != 1 || strings.Contains(contacts.Items[0].PhoneMasked, "98111") {
		t.Fatalf("%+v", contacts)
	}

	w.env.Do(http.MethodPost, trip+"/checkin", a.AccessToken, nil).Want(http.StatusOK)
	var tr travel.TripDTO
	w.env.Do(http.MethodPost, trip+"/checkin", b.AccessToken, nil).Want(http.StatusOK).JSON(&tr)
	if tr.Status != model.TripMet {
		t.Fatalf("status %s", tr.Status)
	}

	start := time.Now()
	var sos struct {
		ContactsNotified int    `json:"contacts_notified"`
		ShareURL         string `json:"share_url"`
	}
	w.env.Do(http.MethodPost, "/v1/sos", a.AccessToken, map[string]any{"trip_id": res.TripID, "station_id": w.env.StationID("Hauz Khas")}).Want(http.StatusCreated).JSON(&sos)
	if time.Since(start) > 30*time.Second || sos.ContactsNotified != 1 || len(w.env.SMS.SOS) != 1 || w.env.SMS.SOS[0] != "+919811122233" {
		t.Fatalf("%+v %v", sos, w.env.SMS.SOS)
	}
	var crit int64
	w.env.App.DB.Model(&model.ModerationCase{}).Where("source = ? AND severity = ?", "sos", model.SevCritical).Count(&crit)
	if crit != 1 {
		t.Fatal("no critical T&S case for SOS")
	}
	page := w.env.Do(http.MethodGet, strings.TrimPrefix(sos.ShareURL, "https://metrobuddy.test"), "", nil).Want(http.StatusOK)
	body := string(page.Body)
	if !strings.Contains(body, "Hauz Khas") || !strings.Contains(body, "Central Secretariat") || strings.Contains(body, "98111") || strings.Contains(body, "Rajiv Chowk") {
		t.Fatalf("share page: %s", body)
	}

	w.env.Do(http.MethodPost, trip+"/feedback", b.AccessToken, map[string]any{"thumbs": -1, "felt_safe": false, "note": "made me uncomfortable"}).Want(http.StatusOK)
	// The flag joins the trip's open SOS case rather than opening a second one.
	var cases []model.ModerationCase
	w.env.App.DB.Where("target_type = ?", model.CTTrip).Find(&cases)
	if len(cases) != 1 || cases[0].ReportCount != 2 || cases[0].Severity != model.SevCritical {
		t.Fatalf("safety flag not attached to the trip case: %+v", cases)
	}
}

func TestCommutePlansStorePathAndNeedConsent(t *testing.T) {
	w := setup(t)
	u := w.user("9000000001", "")
	var p travel.PlanDTO
	w.env.Do(http.MethodPost, "/v1/commute-plans", u.AccessToken, map[string]any{
		"from_station_id": w.env.StationID("Rajiv Chowk"), "to_station_id": w.env.StationID("Millennium City Centre Gurugram"),
		"depart_from": "08:30", "depart_to": "09:15", "weekdays": []int{1, 2, 3, 4, 5},
	}).Want(http.StatusCreated).JSON(&p)
	if p.Stops != 21 || p.DepartFrom != "08:30" || len(p.Weekdays) != 5 {
		t.Fatalf("%+v", p)
	}
	w.env.Do(http.MethodPost, "/v1/commute-plans", u.AccessToken, map[string]any{
		"from_station_id": w.env.StationID("Rajiv Chowk"), "to_station_id": w.env.StationID("Saket"),
		"depart_from": "08:30", "depart_to": "11:30", "weekdays": []int{1},
	}).Want(http.StatusUnprocessableEntity)
	w.env.Do(http.MethodPost, "/v1/me/consents", u.AccessToken, map[string]any{"policy_version": "2026-10", "items": []map[string]any{{"purpose": "travel_matching", "granted": false}}}).Want(http.StatusOK)
	if r := w.env.Do(http.MethodGet, "/v1/commute-plans", u.AccessToken, nil); r.ErrCode() != "consent_required" {
		t.Fatalf("%s", r.Body)
	}
}

func TestStopAndRestartKeepsTheSameBuddyAndTheirYes(t *testing.T) {
	w := setup(t)
	a, b := w.user("9000000001", "man"), w.user("9000000002", "man")
	w.travelNow(a, "Rajiv Chowk", "Millennium City Centre Gurugram", 0).Want(http.StatusCreated)
	var bIntent struct{ ID string }
	w.travelNow(b, "Central Secretariat", "Saket", 5).Want(http.StatusCreated).JSON(&bIntent)
	m := w.matches(a)[0]
	w.env.Do(http.MethodPost, "/v1/matches/"+m.ID.String()+"/accept", a.AccessToken, nil).Want(http.StatusOK)

	// b stops and starts again: a is still recommended, already saying yes, and b can accept.
	w.env.Do(http.MethodDelete, "/v1/travel-intents/"+bIntent.ID, b.AccessToken, nil).Want(http.StatusNoContent)
	w.travelNow(b, "Central Secretariat", "Saket", 5).Want(http.StatusCreated)
	ms := w.matches(b)
	if len(ms) != 1 {
		t.Fatalf("after restart %+v", ms)
	}
	var res travel.MatchDTO
	w.env.Do(http.MethodPost, "/v1/matches/"+ms[0].ID.String()+"/accept", b.AccessToken, nil).Want(http.StatusOK).JSON(&res)
	// One tap from b is enough: a's earlier yes carried over to the new match.
	if res.Status != model.MatchAccepted || res.ConversationID == nil {
		t.Fatalf("a's yes was lost on restart: %+v", res)
	}
}

func TestDeclineSticksAcrossRestart(t *testing.T) {
	w := setup(t)
	a, b := w.user("9000000001", "man"), w.user("9000000002", "man")
	w.travelNow(a, "Rajiv Chowk", "Millennium City Centre Gurugram", 0).Want(http.StatusCreated)
	w.travelNow(b, "Central Secretariat", "Saket", 5).Want(http.StatusCreated)
	m := w.matches(b)[0]
	w.env.Do(http.MethodPost, "/v1/matches/"+m.ID.String()+"/decline", b.AccessToken, nil).Want(http.StatusNoContent)
	w.travelNow(b, "Central Secretariat", "Saket", 5).Want(http.StatusCreated)
	if ms := w.matches(b); len(ms) != 0 {
		t.Fatalf("declined person came back: %+v", ms)
	}
}

func TestStillSearchingAfterLeavingTimeCountsAsLeavingNow(t *testing.T) {
	w := setup(t)
	a, b := w.user("9000000001", "man"), w.user("9000000002", "man")
	w.travelNow(a, "Rajiv Chowk", "Millennium City Centre Gurugram", 0).Want(http.StatusCreated)
	// a said "now" 30 minutes ago and is still on the platform looking.
	w.env.App.DB.Model(&model.TravelIntent{}).Where("persona_id = ?", a.PersonaID).Update("depart_at", time.Now().UTC().Add(-30*time.Minute))
	w.travelNow(b, "Central Secretariat", "Saket", 0).Want(http.StatusCreated)
	if ms := w.matches(b); len(ms) != 1 {
		t.Fatalf("waiting person not matched: %+v", ms)
	}
}

func TestFailedAcceptsDoNotUseUpTheDailyLimit(t *testing.T) {
	w := setup(t)
	a, b := w.user("9000000001", "man"), w.user("9000000002", "man")
	w.travelNow(a, "Rajiv Chowk", "Millennium City Centre Gurugram", 0).Want(http.StatusCreated)
	var bIntent struct{ ID string }
	w.travelNow(b, "Central Secretariat", "Saket", 5).Want(http.StatusCreated).JSON(&bIntent)
	stale := w.matches(a)[0]
	w.env.Do(http.MethodDelete, "/v1/travel-intents/"+bIntent.ID, b.AccessToken, nil).Want(http.StatusNoContent)
	for range 12 {
		w.env.Do(http.MethodPost, "/v1/matches/"+stale.ID.String()+"/accept", a.AccessToken, nil).Want(http.StatusConflict)
	}
	w.travelNow(b, "Central Secretariat", "Saket", 5).Want(http.StatusCreated)
	fresh := w.matches(a)[0]
	w.env.Do(http.MethodPost, "/v1/matches/"+fresh.ID.String()+"/accept", a.AccessToken, nil).Want(http.StatusOK)
}
