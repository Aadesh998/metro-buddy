package bowls

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"metro-buddy/server/internal/model"
)

const defaultRules = `1. Be kind. Vent about situations, not people by name.
2. No phone numbers, handles or links. They get hidden anyway.
3. No hate, harassment, sexual content or threats.
4. Not a dating app. No "DM me if you're single".
5. If someone seems at risk, report it so our team can help.`

var topics = []struct{ slug, name, desc string }{
	{"office-rant", "Office Rant", "Bosses, appraisals, meetings that should've been emails."},
	{"plans-cancelled", "Plans Cancelled Again", "Friends ditched you? You're not alone."},
	{"college-pressure", "College Pressure", "Exams, placements, parents, PGs."},
	{"late-night-thoughts", "Late Night Thoughts", "The last metro home hits different."},
	{"metro-etiquette", "Metro Etiquette", "Door blockers, loud calls, seat wars."},
	{"small-wins", "Small Wins", "Got a seat at Rajiv Chowk? Tell us."},
	{"heartbreak-hotel", "Heartbreak Hotel", "Breakups and situationships, anonymously."},
	{"career-switch", "Career Switch", "Thinking of quitting? Talk it out."},
}

var hubs = map[string][]struct {
	kind          model.BowlKind
	slug, name    string
	station, desc string
}{
	"DEL": {
		{model.BowlOfficeHub, "cyber-city", "Cyber City Crowd", "Cyber City", "DLF Cyber City and Cyber Hub office folks."},
		{model.BowlOfficeHub, "noida-62-it", "Noida 62 IT Park", "Noida Sector 62", "Sector 62 / Electronic City tech parks."},
		{model.BowlOfficeHub, "cp-offices", "Connaught Place Offices", "Rajiv Chowk", "CP and Barakhamba office goers."},
		{model.BowlOfficeHub, "aerocity", "Aerocity Hospitality", "Delhi Aerocity", "Aerocity hotels and offices."},
		{model.BowlCollege, "du-north", "DU North Campus", "Vishwavidyalaya", "Delhi University North Campus."},
		{model.BowlCollege, "du-south", "DU South Campus", "Durgabai Deshmukh South Campus", "Delhi University South Campus."},
		{model.BowlCollege, "iit-delhi", "IIT Delhi", "IIT Delhi", "IITD students and staff."},
		{model.BowlCollege, "jamia", "Jamia Millia Islamia", "Jamia Millia Islamia", "JMI students."},
	},
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	return strings.Trim(slugRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

func upsert(tx *gorm.DB, b model.Bowl) (*model.Bowl, error) {
	var out model.Bowl
	err := tx.Where(model.Bowl{Slug: b.Slug}).Attrs(b).FirstOrCreate(&out).Error
	return &out, err
}

// Seed creates national topic bowls and, per city, the city, line and hub/college bowls.
// Station bowls are created on demand (EnsureStationBowl).
func Seed(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, t := range topics {
			if _, err := upsert(tx, model.Bowl{Kind: model.BowlTopic, Slug: t.slug, Name: t.name, Description: t.desc, Visibility: model.BowlPublic, RulesMD: defaultRules}); err != nil {
				return err
			}
		}
		var cities []model.City
		if err := tx.Find(&cities).Error; err != nil {
			return err
		}
		for _, c := range cities {
			cid := c.ID
			if _, err := upsert(tx, model.Bowl{CityID: &cid, Kind: model.BowlCity, Slug: strings.ToLower(c.Code), Name: c.Name,
				Description: "Everything " + c.Name + ": commutes, weather, weekend plans.", Visibility: model.BowlPublic, RulesMD: defaultRules}); err != nil {
				return err
			}
			var lines []model.MetroLine
			if err := tx.Where("city_id = ?", c.ID).Find(&lines).Error; err != nil {
				return err
			}
			for _, l := range lines {
				lid := l.ID
				if _, err := upsert(tx, model.Bowl{CityID: &cid, Kind: model.BowlLine, LineID: &lid, Slug: slugify(l.GTFSRouteID),
					Name: l.Name, Description: "Riders of the " + l.Name + ".", Visibility: model.BowlPublic, RulesMD: defaultRules}); err != nil {
					return err
				}
			}
			for _, h := range hubs[c.Code] {
				var st model.Station
				if err := tx.Where("city_id = ? AND name = ?", c.ID, h.station).First(&st).Error; err != nil {
					return fmt.Errorf("hub %s: station %q: %w", h.slug, h.station, err)
				}
				sid := st.ID
				if _, err := upsert(tx, model.Bowl{CityID: &cid, Kind: h.kind, StationID: &sid, Slug: h.slug, Name: h.name,
					Description: h.desc, Visibility: model.BowlPublic, RulesMD: defaultRules}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func EnsureStationBowl(tx *gorm.DB, stationID uuid.UUID) (*model.Bowl, error) {
	var st model.Station
	if err := tx.First(&st, "id = ?", stationID).Error; err != nil {
		return nil, err
	}
	cid, sid := st.CityID, st.ID
	return upsert(tx, model.Bowl{CityID: &cid, Kind: model.BowlStation, StationID: &sid, Slug: "st-" + slugify(st.Code),
		Name: st.Name, Description: "People who pass through " + st.Name + ".", Visibility: model.BowlPublic, RulesMD: defaultRules})
}
