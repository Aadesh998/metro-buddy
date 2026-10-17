package identity_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
)

func TestAliasGeneratorOutputIsAlwaysValid(t *testing.T) {
	for i := range 500 {
		if a := identity.NewAlias(i%2 == 0); !identity.ValidAlias(a) {
			t.Fatalf("generator produced invalid alias %q", a)
		}
	}
	for _, bad := range []string{"Riya Sharma 23", "Chai Comet 7", "chai comet 27", "Chai Comet 27 x", "Chai Chai 22"} {
		if identity.ValidAlias(bad) {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestPersonaRequiredBeforeSocialRoutes(t *testing.T) {
	env := apitest.New(t)
	s := env.Login("9876543210")
	if r := env.Do(http.MethodGet, "/v1/me/persona", s.AccessToken, nil); r.ErrCode() != "persona_required" {
		t.Fatalf("got %s", r.Body)
	}
	var me struct {
		HasPersona bool `json:"has_persona"`
	}
	env.Do(http.MethodGet, "/v1/me", s.AccessToken, nil).Want(http.StatusOK).JSON(&me)
	if me.HasPersona {
		t.Fatal("has_persona true before creation")
	}
}

func TestCreatePersonaMasksBioAndRejectsContactInAlias(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	s := env.LoginOnboarded("9876543210")
	env.Do(http.MethodPost, "/v1/me/persona", s.AccessToken, map[string]any{
		"city_id": city, "alias": "Riya 9876543210", "avatar_seed": "abcdefabcdef",
	}).Want(http.StatusUnprocessableEntity)

	var p identity.OwnPersona
	env.Do(http.MethodPost, "/v1/me/persona", s.AccessToken, map[string]any{
		"city_id": city, "alias": "Chai Comet 27", "avatar_seed": "abcdefabcdef",
		"bio": "dm me on insta: riya_23", "intents": []string{"vent", "vent", "travel_buddy"},
	}).Want(http.StatusOK).JSON(&p)
	if strings.Contains(p.Bio, "riya_23") || len(p.Intents) != 2 {
		t.Fatalf("%+v", p)
	}
	env.Do(http.MethodPost, "/v1/me/persona", s.AccessToken, map[string]any{
		"city_id": city, "alias": "Chai Pigeon 27", "avatar_seed": "abcdefabcdef",
	}).Want(http.StatusConflict)
}

func TestAliasUniquePerCity(t *testing.T) {
	env := apitest.New(t)
	del, mum := env.City("DEL"), env.City("MUM")
	body := func(city any) map[string]any {
		return map[string]any{"city_id": city, "alias": "Neon Kite 42", "avatar_seed": "abcdefabcdef"}
	}
	env.Do(http.MethodPost, "/v1/me/persona", env.LoginOnboarded("9000000001").AccessToken, body(del)).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/me/persona", env.LoginOnboarded("9000000002").AccessToken, body(del)).Want(http.StatusConflict)
	env.Do(http.MethodPost, "/v1/me/persona", env.LoginOnboarded("9000000003").AccessToken, body(mum)).Want(http.StatusOK)
}

func TestAliasChangeEvery30DaysKeepsHistoryAndReservesOldAlias(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	u := env.NewUser("9876543210", city)
	old := u.Alias

	env.Do(http.MethodPut, "/v1/me/persona/alias", u.AccessToken, map[string]string{"alias": "Brave Otter 11"}).Want(http.StatusOK)
	if r := env.Do(http.MethodPut, "/v1/me/persona/alias", u.AccessToken, map[string]string{"alias": "Brave Otter 12"}); r.ErrCode() != "alias_change_too_soon" {
		t.Fatalf("second change within 30 days: %s", r.Body)
	}
	env.App.DB.Model(&model.Persona{}).Where("id = ?", u.PersonaID).Update("alias_changed_at", time.Now().UTC().Add(-31*24*time.Hour))
	env.Do(http.MethodPut, "/v1/me/persona/alias", u.AccessToken, map[string]string{"alias": "Brave Otter 12"}).Want(http.StatusOK)

	var hist []model.PersonaAliasHistory
	env.App.DB.Order("used_to").Find(&hist, "persona_id = ?", u.PersonaID)
	if len(hist) != 2 || hist[0].Alias != old || hist[1].Alias != "Brave Otter 11" {
		t.Fatalf("history %+v", hist)
	}
	other := env.LoginOnboarded("9123456789")
	env.Do(http.MethodPost, "/v1/me/persona", other.AccessToken, map[string]any{
		"city_id": city, "alias": old, "avatar_seed": "abcdefabcdef",
	}).Want(http.StatusConflict)
}

func TestPublicPersonaNeverExposesPrivateFields(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	a := env.NewUser("9000000001", city)
	b := env.NewUser("9000000002", city)
	r := env.Do(http.MethodGet, "/v1/personas/"+a.PersonaID.String(), b.AccessToken, nil).Want(http.StatusOK)
	for _, k := range []string{"user_id", "home_station", "work_station", "phone", "gender", "birth"} {
		if strings.Contains(string(r.Body), k) {
			t.Fatalf("public persona leaks %q: %s", k, r.Body)
		}
	}
}

func TestIllustratedAvatarSeeds(t *testing.T) {
	for _, s := range []string{"img-m01", "img-f10", "abcdef123456"} {
		if !identity.ValidSeed(s) {
			t.Errorf("%s should be valid", s)
		}
	}
	for _, s := range []string{"img-m11", "img-x01", "img-m00", "IMG-m01", "short"} {
		if identity.ValidSeed(s) {
			t.Errorf("%s should be invalid", s)
		}
	}
}

func TestAliasProblem(t *testing.T) {
	for _, ok := range []string{"Chai Lover", "night_owl.22", "राहुल मेट्रो", "Ana"} {
		if p := identity.AliasProblem(ok); p != "" {
			t.Errorf("%q rejected: %s", ok, p)
		}
	}
	for _, bad := range []string{"ab", " Riya", "Riya  K", "9876543210", "call 98765", "riya@gmail.com", "Metro Buddy Support", "Admin Riya", "x.com/riya", "@riya", "fuck_this", "AnExtremelyLongNameThatGoesOn"} {
		if identity.AliasProblem(bad) == "" {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestChooseOwnAlias(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	s := env.LoginOnboarded("9000000001")
	env.Do(http.MethodPost, "/v1/me/persona", s.AccessToken, map[string]any{"city_id": city, "alias": "call 98765 43210", "avatar_seed": "img-f01"}).
		Want(http.StatusUnprocessableEntity)
	env.Do(http.MethodPost, "/v1/me/persona/alias-check", s.AccessToken, map[string]any{"city_id": city, "alias": "Chai Lover"}).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/me/persona", s.AccessToken, map[string]any{"city_id": city, "alias": "Chai Lover", "avatar_seed": "img-f01"}).Want(http.StatusOK)

	other := env.LoginOnboarded("9000000002")
	var r struct{ Available bool }
	env.Do(http.MethodPost, "/v1/me/persona/alias-check", other.AccessToken, map[string]any{"city_id": city, "alias": "chai_lover"}).Want(http.StatusOK).JSON(&r)
	if r.Available {
		t.Fatal("same name with different case and punctuation must count as taken")
	}
	env.Do(http.MethodPost, "/v1/me/persona", other.AccessToken, map[string]any{"city_id": city, "alias": "CHAI.LOVER", "avatar_seed": "img-m01"}).Want(http.StatusConflict)
}
