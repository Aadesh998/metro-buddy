package metro_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/metro"
	"metro-buddy/server/internal/model"
)

func seeded(t *testing.T) *apitest.Env {
	env := apitest.New(t)
	if err := metro.SeedAll(env.App.DB); err != nil {
		t.Fatal(err)
	}
	return env
}

func station(t *testing.T, env *apitest.Env, name string) uuid.UUID {
	t.Helper()
	var s model.Station
	if err := env.App.DB.Where("search_text LIKE ?", "%"+name+"%").First(&s).Error; err != nil {
		t.Fatalf("station %q: %v", name, err)
	}
	return s.ID
}

func names(env *apitest.Env, ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		var s model.Station
		env.App.DB.First(&s, "id = ?", id)
		out[i] = s.Name
	}
	return out
}

func TestBFSRajivChowkToHudaCityCentre(t *testing.T) {
	env := seeded(t)
	from, to := station(t, env, "rajiv chowk"), station(t, env, "huda city centre")
	path, lines, err := env.App.Metro.Route(t.Context(), from, to)
	if err != nil {
		t.Fatal(err)
	}
	got := names(env, path)
	want := []string{"Rajiv Chowk", "Patel Chowk", "Central Secretariat", "Udyog Bhawan", "Lok Kalyan Marg", "Jor Bagh",
		"Dilli Haat - INA", "AIIMS", "Green Park", "Hauz Khas", "Malviya Nagar", "Saket", "Qutab Minar", "Chhattarpur",
		"Sultanpur", "Ghitorni", "Arjan Garh", "Guru Dronacharya", "Sikanderpur", "MG Road", "IFFCO Chowk", "Millennium City Centre Gurugram"}
	if len(got) != len(want) {
		t.Fatalf("path %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stop %d = %q, want %q (path %v)", i, got[i], want[i], got)
		}
	}
	var yellow model.MetroLine
	env.App.DB.First(&yellow, "gtfs_route_id = ?", "DEL-YELLOW")
	if len(lines) != 1 || lines[0] != yellow.ID {
		t.Fatalf("lines %v", lines)
	}
}

func TestBFSUsesInterchangesAndWalkways(t *testing.T) {
	env := seeded(t)
	path, lines, err := env.App.Metro.Route(t.Context(), station(t, env, "hauz khas"), station(t, env, "noida city centre"))
	if err != nil || len(lines) < 2 {
		t.Fatalf("path %v lines %v err %v", names(env, path), lines, err)
	}
	// Aqua line only connects to the rest of the network through the Sector 51/52 walkway.
	path, _, err = env.App.Metro.Route(t.Context(), station(t, env, "pari chowk"), station(t, env, "rajiv chowk"))
	if err != nil {
		t.Fatal(err)
	}
	got := names(env, path)
	found := false
	for i := 1; i < len(got); i++ {
		if got[i-1] == "Noida Sector 51" && got[i] == "Noida Sector 52" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected walkway transfer in %v", got)
	}
}

func TestSeedIsIdempotentAndMarksInterchanges(t *testing.T) {
	env := seeded(t)
	var before []model.Station
	env.App.DB.Order("code").Find(&before)
	if err := metro.SeedAll(env.App.DB); err != nil {
		t.Fatal(err)
	}
	var after []model.Station
	env.App.DB.Order("code").Find(&after)
	if len(before) != len(after) || before[0].ID != after[0].ID {
		t.Fatal("re-seed changed station identity")
	}
	var kg model.Station
	env.App.DB.First(&kg, "name = ?", "Kashmere Gate")
	var saket model.Station
	env.App.DB.First(&saket, "name = ?", "Saket")
	if !kg.IsInterchange || saket.IsInterchange {
		t.Fatalf("interchange flags wrong: kg=%v saket=%v", kg.IsInterchange, saket.IsInterchange)
	}
	if len(after) < 250 {
		t.Fatalf("only %d stations seeded", len(after))
	}
}

func TestStationSearchAndCityStationsEndpoints(t *testing.T) {
	env := seeded(t)
	var cities struct{ Items []metro.CityDTO }
	env.Do(http.MethodGet, "/v1/cities", "", nil).Want(http.StatusOK).JSON(&cities)
	if len(cities.Items) != 1 || cities.Items[0].Code != "DEL" {
		t.Fatalf("%+v", cities)
	}
	var res struct{ Items []metro.StationDTO }
	env.Do(http.MethodGet, "/v1/stations?q=huda", "", nil).Want(http.StatusOK).JSON(&res)
	if len(res.Items) != 1 || res.Items[0].Name != "Millennium City Centre Gurugram" || len(res.Items[0].Lines) != 1 {
		t.Fatalf("%+v", res)
	}
	env.Do(http.MethodGet, "/v1/cities/"+cities.Items[0].ID.String()+"/stations", "", nil).Want(http.StatusOK).JSON(&res)
	if len(res.Items) < 250 {
		t.Fatalf("%d stations", len(res.Items))
	}
	env.Do(http.MethodGet, "/v1/stations?q=%25%25", "", nil).Want(http.StatusOK).JSON(&res)
	if len(res.Items) != 0 {
		t.Fatal("LIKE wildcard not escaped")
	}
}
