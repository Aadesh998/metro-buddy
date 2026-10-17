package identity_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
)

func TestCannotProceedWithoutAdultAgeAndCoreConsent(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	s := env.Login("9876543210")
	suggest := func() *apitest.Resp {
		return env.Do(http.MethodPost, "/v1/me/persona/suggestions", s.AccessToken, map[string]any{"city_id": city})
	}
	if r := suggest(); r.ErrCode() != "onboarding_required" {
		t.Fatalf("persona allowed before onboarding: %s", r.Body)
	}

	ist := time.FixedZone("IST", 5*3600+1800)
	almost18 := time.Now().In(ist).AddDate(-18, 0, 1).Format("2006-01-02")
	if r := env.Do(http.MethodPut, "/v1/me/age", s.AccessToken, map[string]string{"birth_date": almost18}); r.ErrCode() != "underage" {
		t.Fatalf("17-year-old accepted: %s", r.Body)
	}
	exactly18 := time.Now().In(ist).AddDate(-18, 0, 0).Format("2006-01-02")
	env.Do(http.MethodPut, "/v1/me/age", s.AccessToken, map[string]string{"birth_date": exactly18}).Want(http.StatusOK)
	env.Do(http.MethodPut, "/v1/me/age", s.AccessToken, map[string]string{"birth_date": "1990-01-01"}).Want(http.StatusConflict)

	consent := func(core bool) {
		env.Do(http.MethodPost, "/v1/me/consents", s.AccessToken, map[string]any{
			"policy_version": identity.PolicyVersion,
			"items":          []map[string]any{{"purpose": "core_service", "granted": core}, {"purpose": "analytics", "granted": false}},
		}).Want(http.StatusOK)
	}
	consent(false)
	if r := suggest(); r.ErrCode() != "onboarding_required" {
		t.Fatalf("allowed without core consent: %s", r.Body)
	}
	consent(true)
	suggest().Want(http.StatusOK)

	var u model.User
	env.App.DB.First(&u)
	if *u.BirthYear != int16(time.Now().In(ist).Year()-18) {
		t.Fatalf("birth year %d", *u.BirthYear)
	}
	var n int64
	env.App.DB.Model(&model.Consent{}).Count(&n)
	if n != 4 {
		t.Fatalf("consent records are append-only, want 4 rows, got %d", n)
	}
	consent(false)
	if r := suggest(); r.ErrCode() != "onboarding_required" {
		t.Fatalf("withdrawing core consent should pause access: %s", r.Body)
	}
}

func TestConsentRejectsStalePolicyVersion(t *testing.T) {
	env := apitest.New(t)
	s := env.Login("9876543210")
	env.Do(http.MethodPost, "/v1/me/consents", s.AccessToken, map[string]any{
		"policy_version": "2020-01", "items": []map[string]any{{"purpose": "core_service", "granted": true}},
	}).Want(http.StatusConflict)
	var got struct {
		Items []identity.ConsentState
	}
	env.Do(http.MethodGet, "/v1/me/consents", s.AccessToken, nil).Want(http.StatusOK).JSON(&got)
	if len(got.Items) != len(model.ConsentPurposes) {
		t.Fatal(fmt.Sprint(got))
	}
}
