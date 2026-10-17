package safety_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/safety"
)

func TestBlockHidesBothWaysInPersonaLookupAndScopes(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b, c := env.NewUser("9000000001", city), env.NewUser("9000000002", city), env.NewUser("9000000003", city)

	env.Do(http.MethodPost, "/v1/blocks", a.AccessToken, map[string]any{"persona_id": b.PersonaID}).Want(http.StatusNoContent)
	env.Do(http.MethodGet, "/v1/personas/"+b.PersonaID.String(), a.AccessToken, nil).Want(http.StatusNotFound)
	env.Do(http.MethodGet, "/v1/personas/"+a.PersonaID.String(), b.AccessToken, nil).Want(http.StatusNotFound)
	env.Do(http.MethodGet, "/v1/personas/"+a.PersonaID.String(), c.AccessToken, nil).Want(http.StatusOK)

	visible := func(viewer uuid.UUID) map[uuid.UUID]bool {
		var ids []uuid.UUID
		env.App.DB.Model(&model.Persona{}).Scopes(safety.ExcludeBlocked(viewer, "id")).Pluck("id", &ids)
		m := map[uuid.UUID]bool{}
		for _, id := range ids {
			m[id] = true
		}
		return m
	}
	if v := visible(a.PersonaID); v[b.PersonaID] || !v[c.PersonaID] {
		t.Fatalf("blocker's list: %v", v)
	}
	if v := visible(b.PersonaID); v[a.PersonaID] || !v[c.PersonaID] {
		t.Fatalf("blocked user's list: %v", v)
	}

	env.Do(http.MethodDelete, "/v1/blocks/"+b.PersonaID.String(), a.AccessToken, nil).Want(http.StatusNoContent)
	env.Do(http.MethodGet, "/v1/personas/"+a.PersonaID.String(), b.AccessToken, nil).Want(http.StatusOK)
}

func TestMuteIsOneWay(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	env.Do(http.MethodPost, "/v1/mutes", a.AccessToken, map[string]any{"persona_id": b.PersonaID}).Want(http.StatusNoContent)
	count := func(viewer uuid.UUID, target uuid.UUID) int64 {
		var n int64
		env.App.DB.Model(&model.Persona{}).Scopes(safety.Hidden(viewer, "id")).Where("id = ?", target).Count(&n)
		return n
	}
	if count(a.PersonaID, b.PersonaID) != 0 || count(b.PersonaID, a.PersonaID) != 1 {
		t.Fatal("mute should hide only for the muter")
	}
	var list struct {
		Items []struct{ Persona struct{ ID uuid.UUID } }
	}
	env.Do(http.MethodGet, "/v1/mutes", a.AccessToken, nil).Want(http.StatusOK).JSON(&list)
	if len(list.Items) != 1 || list.Items[0].Persona.ID != b.PersonaID {
		t.Fatalf("%+v", list)
	}
}

func TestReportsGroupIntoOneCaseAndEscalateSeverity(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	target := env.NewUser("9000000001", city)
	r1, r2 := env.NewUser("9000000002", city), env.NewUser("9000000003", city)
	report := func(u apitest.User, reason string) *apitest.Resp {
		return env.Do(http.MethodPost, "/v1/reports", u.AccessToken, map[string]any{
			"target_type": "persona", "target_id": target.PersonaID, "reason": reason, "details": "he sent 9876543210",
		})
	}
	report(r1, "spam").Want(http.StatusCreated)
	report(r1, "spam").Want(http.StatusConflict)
	report(r2, "threat").Want(http.StatusCreated)
	report(r1, "made-up").Want(http.StatusUnprocessableEntity)
	env.Do(http.MethodPost, "/v1/reports", target.AccessToken, map[string]any{
		"target_type": "persona", "target_id": target.PersonaID, "reason": "spam",
	}).Want(http.StatusBadRequest)

	var cases []model.ModerationCase
	env.App.DB.Find(&cases)
	if len(cases) != 1 || cases[0].ReportCount != 2 || cases[0].Severity != model.SevCritical || *cases[0].TargetPersonaID != target.PersonaID {
		t.Fatalf("%+v", cases)
	}
	var rep model.Report
	env.App.DB.First(&rep)
	if rep.DetailsMasked != "he sent [phone hidden]" {
		t.Fatalf("details not masked: %q", rep.DetailsMasked)
	}
}

func TestReportAndBlockInOneStep(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	r := env.Do(http.MethodPost, "/v1/reports", a.AccessToken, map[string]any{
		"target_type": "persona", "target_id": b.PersonaID, "reason": "self_harm", "also_block": true,
	}).Want(http.StatusCreated)
	if r.Map()["show_support"] != true {
		t.Fatal("self-harm report should prompt support sheet")
	}
	env.Do(http.MethodGet, "/v1/personas/"+b.PersonaID.String(), a.AccessToken, nil).Want(http.StatusNotFound)
	env.Do(http.MethodPost, "/v1/reports", a.AccessToken, map[string]any{
		"target_type": "post", "target_id": uuid.New(), "reason": "spam",
	}).Want(http.StatusNotFound)
}
