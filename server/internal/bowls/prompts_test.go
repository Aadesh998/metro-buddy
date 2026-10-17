package bowls_test

import (
	"net/http"
	"testing"

	"metro-buddy/server/internal/bowls"
	"metro-buddy/server/internal/model"
)

func TestOneDailyPromptPerCityPerDayPinnedOnCityBowl(t *testing.T) {
	f := setup(t)
	for range 3 {
		if err := f.env.App.Bowls.PostDailyPrompts(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	var n int64
	f.env.App.DB.Model(&model.DailyPrompt{}).Count(&n)
	if n != 1 {
		t.Fatalf("%d prompts", n)
	}
	var city model.Bowl
	f.env.App.DB.First(&city, "kind = ?", model.BowlCity)
	var page struct {
		Items  []bowls.PostDTO
		Pinned *bowls.PostDTO
	}
	f.env.Do(http.MethodGet, "/v1/bowls/"+city.ID.String()+"/posts", f.a.AccessToken, nil).Want(http.StatusOK).JSON(&page)
	if page.Pinned == nil || !page.Pinned.IsPrompt || page.Pinned.Author.Alias != "Metro Buddy" || len(page.Items) != 0 {
		t.Fatalf("%+v", page)
	}
	f.env.Do(http.MethodGet, "/v1/feed", f.a.AccessToken, nil).Want(http.StatusOK).JSON(&page)
	if page.Pinned == nil {
		t.Fatal("home feed missing today's prompt")
	}
}
