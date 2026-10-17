package studio_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/studio"
)

type world struct {
	env  *apitest.Env
	city uuid.UUID
}

func setup(t *testing.T) world {
	env := apitest.New(t)
	return world{env, env.City("DEL")}
}

func (w world) host(phone string) apitest.User {
	u := w.env.NewUser(phone, w.city)
	w.env.Do(http.MethodPost, "/v1/studio/host-application", u.AccessToken, map[string]any{
		"tagline": "Late night metro feels", "pitch": "A calm hour for anyone riding home late and wanting to talk it out.",
	}).Want(http.StatusCreated)
	w.env.App.DB.Model(&model.HostProfile{}).Where("persona_id = ?", u.PersonaID).Update("status", model.HostApproved)
	return u
}

func (w world) liveEpisode(h apitest.User) string {
	var sh studio.ShowDTO
	w.env.Do(http.MethodPost, "/v1/shows", h.AccessToken, map[string]any{"title": "Last Train Home", "topic_tags": []string{"night", "stress"}}).
		Want(http.StatusOK).JSON(&sh)
	var ep studio.EpisodeDTO
	w.env.Do(http.MethodPost, "/v1/shows/"+sh.ID.String()+"/episodes", h.AccessToken, map[string]any{
		"title": "Office drama", "scheduled_at": time.Now().UTC().Add(5 * time.Minute),
	}).Want(http.StatusOK).JSON(&ep)
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep.ID.String()+"/start", h.AccessToken, nil).Want(http.StatusOK)
	return ep.ID.String()
}

func (w world) join(u apitest.User, ep string) studio.JoinDTO {
	var j studio.JoinDTO
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/join", u.AccessToken, nil).Want(http.StatusOK).JSON(&j)
	return j
}

func (w world) callIn(u apitest.User, ep, topic string) studio.CallInDTO {
	var ci studio.CallInDTO
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/call-in", u.AccessToken, map[string]any{"topic": topic}).Want(http.StatusCreated).JSON(&ci)
	return ci
}

func TestOnlyApprovedHostsCreateShowsAndListenersCantPublish(t *testing.T) {
	w := setup(t)
	u := w.env.NewUser("9000000001", w.city)
	if r := w.env.Do(http.MethodPost, "/v1/shows", u.AccessToken, map[string]any{"title": "Nope show"}); r.Code != http.StatusForbidden {
		t.Fatalf("non-host created show: %d", r.Code)
	}
	h := w.host("9000000002")
	ep := w.liveEpisode(h)
	hj := w.join(h, ep)
	lj := w.join(u, ep)
	if hj.Role != model.StageHost || lj.Role != model.StageListener {
		t.Fatalf("roles %s %s", hj.Role, lj.Role)
	}
	for _, g := range w.env.Live.Tokens {
		if g.Identity == u.PersonaID.String() && (g.Publish || g.Admin) {
			t.Fatalf("listener token can publish: %+v", g)
		}
		if g.Identity == h.PersonaID.String() && !g.Publish {
			t.Fatalf("host token can't publish: %+v", g)
		}
	}
	var got studio.EpisodeDTO
	w.env.Do(http.MethodGet, "/v1/episodes/"+ep, u.AccessToken, nil).Want(http.StatusOK).JSON(&got)
	if got.Listeners != 2 || got.PeakListeners != 2 || got.Status != model.EpisodeLive {
		t.Fatalf("%+v", got)
	}
}

func TestCallInQueueStageCapAndDropRevokesPublish(t *testing.T) {
	w := setup(t)
	h := w.host("9000000001")
	a, b := w.env.NewUser("9000000002", w.city), w.env.NewUser("9000000003", w.city)
	ep := w.liveEpisode(h)
	w.join(a, ep)
	w.join(b, ep)
	ca := w.callIn(a, ep, "call me on 9876543210 about my boss")
	if strings.Contains(ca.Topic, "9876543210") {
		t.Fatalf("topic not masked: %q", ca.Topic)
	}
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/call-in", a.AccessToken, map[string]any{"topic": "again please"}).Want(http.StatusConflict)
	cb := w.callIn(b, ep, "roommate trouble")
	if r := w.env.Do(http.MethodGet, "/v1/episodes/"+ep+"/queue", a.AccessToken, nil); r.Code != http.StatusForbidden {
		t.Fatalf("listener read queue: %d", r.Code)
	}
	var q struct{ Items []studio.CallInDTO }
	w.env.Do(http.MethodGet, "/v1/episodes/"+ep+"/queue", h.AccessToken, nil).Want(http.StatusOK).JSON(&q)
	if len(q.Items) != 2 || q.Items[0].ID != ca.ID {
		t.Fatalf("queue %+v", q.Items)
	}

	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/stage", h.AccessToken, map[string]any{"call_in_id": ca.ID}).Want(http.StatusOK)
	if !w.env.Live.CanPublish(a.PersonaID.String()) {
		t.Fatal("staged caller can't publish")
	}
	// Staging the next caller takes the mic from the first.
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/stage", h.AccessToken, map[string]any{"call_in_id": cb.ID}).Want(http.StatusOK)
	if w.env.Live.CanPublish(a.PersonaID.String()) || !w.env.Live.CanPublish(b.PersonaID.String()) {
		t.Fatal("two callers on air")
	}
	var got studio.EpisodeDTO
	w.env.Do(http.MethodGet, "/v1/episodes/"+ep, a.AccessToken, nil).Want(http.StatusOK).JSON(&got)
	if got.OnAir == nil || got.OnAir.ID != cb.ID || got.OnAir.Topic != "" {
		t.Fatalf("listeners see %+v", got.OnAir)
	}

	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/extend", h.AccessToken, map[string]any{"minutes": 5}).Want(http.StatusOK)
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/extend", h.AccessToken, map[string]any{"minutes": 1}).Want(http.StatusUnprocessableEntity)
	w.env.App.Studio.Now = func() time.Time { return time.Now().UTC().Add(14 * time.Minute) }
	w.env.App.Studio.EnforceOnAirCap(t.Context())
	w.env.App.Studio.Now = nil
	if w.env.Live.CanPublish(b.PersonaID.String()) {
		t.Fatal("cap didn't revoke publish")
	}
	var ci model.CallInRequest
	w.env.App.DB.First(&ci, "id = ?", cb.ID)
	if ci.Status != model.CallInDone || ci.OffAirAt == nil {
		t.Fatalf("%+v", ci)
	}
}

func TestDroppedCallerLosesMicAndRecordingIsBlocked(t *testing.T) {
	w := setup(t)
	h := w.host("9000000001")
	a := w.env.NewUser("9000000002", w.city)
	ep := w.liveEpisode(h)
	w.join(a, ep)
	ca := w.callIn(a, ep, "need to vent")
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/stage", h.AccessToken, map[string]any{"call_in_id": ca.ID}).Want(http.StatusOK)

	room := "ep_" + ep
	w.env.Do(http.MethodPost, "/v1/livekit/webhook", "", map[string]any{
		"event": "participant_left", "room": map[string]any{"name": room}, "participant": map[string]any{"identity": a.PersonaID.String()},
	}).Want(http.StatusOK)
	if w.env.Live.CanPublish(a.PersonaID.String()) {
		t.Fatal("dropped caller kept publish")
	}
	var ci model.CallInRequest
	w.env.App.DB.First(&ci, "id = ?", ca.ID)
	if ci.Status != model.CallInDropped {
		t.Fatalf("status %s", ci.Status)
	}

	w.env.Do(http.MethodPost, "/v1/livekit/webhook", "", map[string]any{
		"event": "egress_started", "room": map[string]any{"name": room}, "egressInfo": map[string]any{"egressId": "EG_1"},
	}).Want(http.StatusOK)
	if len(w.env.Live.Stopped) != 1 || w.env.Live.Stopped[0] != "EG_1" {
		t.Fatalf("egress not stopped: %v", w.env.Live.Stopped)
	}
}

func TestRevokeFailureFallsBackToRemoval(t *testing.T) {
	w := setup(t)
	h := w.host("9000000001")
	a := w.env.NewUser("9000000002", w.city)
	ep := w.liveEpisode(h)
	w.join(a, ep)
	ca := w.callIn(a, ep, "need to vent")
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/stage", h.AccessToken, map[string]any{"call_in_id": ca.ID}).Want(http.StatusOK)
	w.env.Live.FailNext = true
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/unstage", h.AccessToken, nil).Want(http.StatusNoContent)
	if len(w.env.Live.Removed) != 1 || w.env.Live.CanPublish(a.PersonaID.String()) {
		t.Fatalf("mic left open: removed=%v", w.env.Live.Removed)
	}
}

func TestReportSpikeAutoMutesCaller(t *testing.T) {
	w := setup(t)
	h := w.host("9000000001")
	caller := w.env.NewUser("9000000002", w.city)
	ep := w.liveEpisode(h)
	w.join(caller, ep)
	ca := w.callIn(caller, ep, "hot take incoming")
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/stage", h.AccessToken, map[string]any{"call_in_id": ca.ID}).Want(http.StatusOK)
	for i, phone := range []string{"9000000003", "9000000004", "9000000005"} {
		r := w.env.NewUser(phone, w.city)
		w.join(r, ep)
		w.env.Do(http.MethodPost, "/v1/reports", r.AccessToken, map[string]any{"target_type": "call_in", "target_id": ca.ID, "reason": "harassment"}).Want(http.StatusCreated)
		if want := i < 2; w.env.Live.CanPublish(caller.PersonaID.String()) != want {
			t.Fatalf("after %d reports publish=%v", i+1, !want)
		}
	}
	var got studio.EpisodeDTO
	w.env.Do(http.MethodGet, "/v1/episodes/"+ep, h.AccessToken, nil).Want(http.StatusOK).JSON(&got)
	if !got.AutoMuted || got.OnAir != nil {
		t.Fatalf("%+v", got)
	}
}

func TestQuestionsMaskedReactionsAndRemoval(t *testing.T) {
	w := setup(t)
	h := w.host("9000000001")
	a := w.env.NewUser("9000000002", w.city)
	ep := w.liveEpisode(h)
	w.join(a, ep)
	var q studio.QuestionDTO
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/questions", a.AccessToken, map[string]any{"text": "mail me at riya@gmail.com pls"}).Want(http.StatusCreated).JSON(&q)
	if strings.Contains(q.Text, "riya@gmail.com") {
		t.Fatalf("question not masked: %q", q.Text)
	}
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/reactions", a.AccessToken, map[string]any{"reaction": "hug"}).Want(http.StatusNoContent)
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/reactions", a.AccessToken, map[string]any{"reaction": "poop"}).Want(http.StatusUnprocessableEntity)
	w.env.Do(http.MethodPost, "/v1/questions/"+q.ID.String()+"/answered", h.AccessToken, nil).Want(http.StatusNoContent)

	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/remove", h.AccessToken, map[string]any{"persona_id": a.PersonaID}).Want(http.StatusNoContent)
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/join", a.AccessToken, nil).Want(http.StatusForbidden)
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/remove", a.AccessToken, map[string]any{"persona_id": h.PersonaID}).Want(http.StatusForbidden)

	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/end", h.AccessToken, nil).Want(http.StatusOK)
	w.env.Do(http.MethodPost, "/v1/episodes/"+ep+"/reactions", a.AccessToken, map[string]any{"reaction": "hug"}).Want(http.StatusConflict)
}

func TestFollowersGetReminderOnce(t *testing.T) {
	w := setup(t)
	h := w.host("9000000001")
	f := w.env.NewUser("9000000002", w.city)
	w.env.Do(http.MethodPut, "/v1/me/push-token", f.AccessToken, map[string]any{"token": "ExponentPushToken[f]", "platform": "android"})
	var sh studio.ShowDTO
	w.env.Do(http.MethodPost, "/v1/shows", h.AccessToken, map[string]any{"title": "Morning Rush"}).Want(http.StatusOK).JSON(&sh)
	w.env.Do(http.MethodPost, "/v1/shows/"+sh.ID.String()+"/follow", f.AccessToken, nil).Want(http.StatusOK)
	w.env.Do(http.MethodPost, "/v1/shows/"+sh.ID.String()+"/follow", f.AccessToken, nil).Want(http.StatusOK)
	w.env.Do(http.MethodPost, "/v1/shows/"+sh.ID.String()+"/episodes", h.AccessToken, map[string]any{
		"title": "Monday blues", "scheduled_at": time.Now().UTC().Add(8 * time.Minute),
	}).Want(http.StatusOK)
	w.env.App.Studio.SendReminders(t.Context())
	w.env.App.Studio.SendReminders(t.Context())
	var n int64
	w.env.App.DB.Model(&model.Notification{}).Where("persona_id = ? AND kind = ?", f.PersonaID, model.NotifEpisodeLive).Count(&n)
	if n != 1 {
		t.Fatalf("reminders %d", n)
	}
	var got studio.ShowDTO
	w.env.Do(http.MethodGet, "/v1/shows/"+sh.ID.String(), f.AccessToken, nil).Want(http.StatusOK).JSON(&got)
	if got.FollowerCount != 1 || !got.Following || got.Next == nil {
		t.Fatalf("%+v", got)
	}
}
