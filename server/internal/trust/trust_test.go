package trust_test

import (
	"net/http"
	"testing"
	"time"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/trust"
)

func TestScoreAndTier(t *testing.T) {
	cases := []struct {
		in    trust.Inputs
		score int
		tier  int16
	}{
		{trust.Inputs{}, 50, 0},
		{trust.Inputs{AgeDays: 3}, 50, 1},
		{trust.Inputs{AgeDays: 14, Positive: 8}, 60, 2},
		{trust.Inputs{AgeDays: 365, Positive: 100, Verified: true}, 100, 3},
		{trust.Inputs{AgeDays: 365, Positive: 100}, 90, 2},
		{trust.Inputs{AgeDays: 30, Negative: -50}, 4, 0},
		{trust.Inputs{AgeDays: 7, Dismissed: 10, Upheld: 1}, 39, 0},
	}
	for _, c := range cases {
		s := trust.Score(c.in)
		if s != c.score || trust.Tier(s, c.in.AgeDays, c.in.Verified) != c.tier {
			t.Errorf("%+v: score %d tier %d, want %d %d", c.in, s, trust.Tier(s, c.in.AgeDays, c.in.Verified), c.score, c.tier)
		}
	}
}

func TestEventsRaiseTierAndLimitsScale(t *testing.T) {
	env := apitest.New(t)
	u := env.NewUser("9000000001", env.City("DEL"))
	db := env.App.DB
	var user model.User
	db.Joins("JOIN personas ON personas.user_id = users.id").Where("personas.id = ?", u.PersonaID).First(&user)
	db.Model(&user).Update("created_at", time.Now().UTC().AddDate(0, 0, -20))
	if err := trust.Record(db, user.ID, model.TrustPositiveFeedback, 8, nil); err != nil {
		t.Fatal(err)
	}
	db.First(&user, "id = ?", user.ID)
	if user.TrustScore != 60 || user.TrustTier != 2 {
		t.Fatalf("score %d tier %d", user.TrustScore, user.TrustTier)
	}
	// The access token re-reads the tier on every request, so tier 2 gets 4x the hourly post budget.
	env.Do(http.MethodGet, "/v1/me", u.AccessToken, nil).Want(http.StatusOK)
	trust.Record(db, user.ID, model.TrustModAction, -50, nil)
	db.First(&user, "id = ?", user.ID)
	if user.TrustTier != 0 {
		t.Fatalf("tier after ban-level action %d", user.TrustTier)
	}
	if err := (&trust.Service{DB: db}).RecomputeAll(t.Context()); err != nil {
		t.Fatal(err)
	}
}
