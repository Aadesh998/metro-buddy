package bowls_test

import (
	"net/http"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/bowls"
)

func TestNewUserGetsLineStationAndCityBowlsSuggested(t *testing.T) {
	env := apitest.New(t)
	city := env.Seeded()
	u := env.NewUser("9876543210", city)
	env.Do(http.MethodPatch, "/v1/me/persona", u.AccessToken, map[string]any{
		"home_station_id": env.StationID("Rajiv Chowk"), "work_station_id": env.StationID("Cyber City"),
	}).Want(http.StatusOK)

	var got struct{ Items []bowls.BowlDTO }
	env.Do(http.MethodGet, "/v1/bowls?suggested=1", u.AccessToken, nil).Want(http.StatusOK).JSON(&got)
	has := map[string]bool{}
	for _, b := range got.Items {
		has[b.Name] = true
	}
	for _, want := range []string{"Rajiv Chowk", "Yellow Line", "Blue Line", "Delhi NCR", "Cyber City", "Rapid Metro Gurugram", "Cyber City Crowd", "Connaught Place Offices", "Office Rant"} {
		if !has[want] {
			t.Errorf("missing suggestion %q in %v", want, has)
		}
	}
	if got.Items[0].Reason != "Your station" {
		t.Fatalf("first suggestion should be the home station bowl: %+v", got.Items[0])
	}
}

func TestJoinAndLeaveUpdateMemberCountIdempotently(t *testing.T) {
	env := apitest.New(t)
	city := env.Seeded()
	a, b := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	var list struct{ Items []bowls.BowlDTO }
	env.Do(http.MethodGet, "/v1/bowls?kind=topic", a.AccessToken, nil).Want(http.StatusOK).JSON(&list)
	id := list.Items[0].ID.String()

	var d bowls.BowlDTO
	env.Do(http.MethodPost, "/v1/bowls/"+id+"/join", a.AccessToken, nil).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/bowls/"+id+"/join", a.AccessToken, nil).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/bowls/"+id+"/join", b.AccessToken, nil).Want(http.StatusOK).JSON(&d)
	if d.MemberCount != 2 || !d.Joined || *d.Role != "member" {
		t.Fatalf("%+v", d)
	}
	var joined struct{ Items []bowls.BowlDTO }
	env.Do(http.MethodGet, "/v1/bowls/joined", a.AccessToken, nil).Want(http.StatusOK).JSON(&joined)
	if len(joined.Items) != 1 {
		t.Fatalf("joined %+v", joined)
	}
	env.Do(http.MethodDelete, "/v1/bowls/"+id+"/join", a.AccessToken, nil).Want(http.StatusOK)
	env.Do(http.MethodDelete, "/v1/bowls/"+id+"/join", a.AccessToken, nil).Want(http.StatusOK).JSON(&d)
	if d.MemberCount != 1 || d.Joined {
		t.Fatalf("%+v", d)
	}
}
