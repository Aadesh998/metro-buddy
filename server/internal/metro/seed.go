package metro

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metro-buddy/server/internal/model"
)

//go:embed seed/*.json
var seedFS embed.FS

type SeedFile struct {
	City struct {
		Name   string `json:"name"`
		Code   string `json:"code"`
		IsLive bool   `json:"is_live"`
	} `json:"city"`
	WomensCoachNote string `json:"womens_coach_note"`
	Lines           []struct {
		GTFSRouteID string   `json:"gtfs_route_id"`
		Name        string   `json:"name"`
		Color       string   `json:"color"`
		Stations    []string `json:"stations"`
	} `json:"lines"`
	Transfers [][2]string       `json:"transfers"`
	Aliases   map[string]string `json:"aliases"`
}

func SeedAll(db *gorm.DB) error {
	names, err := fs.Glob(seedFS, "seed/*.json")
	if err != nil {
		return err
	}
	for _, n := range names {
		b, err := seedFS.ReadFile(n)
		if err != nil {
			return err
		}
		var f SeedFile
		if err := json.Unmarshal(b, &f); err != nil {
			return fmt.Errorf("%s: %w", n, err)
		}
		if err := Seed(db, &f); err != nil {
			return fmt.Errorf("%s: %w", n, err)
		}
	}
	return nil
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func stationCode(city, name string) string {
	slug := strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(name), "-"), "-")
	code := city + "-" + slug
	if len(code) > 40 {
		code = code[:40]
	}
	return code
}

// Seed is idempotent: rows are matched on stable codes so IDs survive re-seeding.
func Seed(db *gorm.DB, f *SeedFile) error {
	return db.Transaction(func(tx *gorm.DB) error {
		city := model.City{Name: f.City.Name, Code: f.City.Code, IsLive: f.City.IsLive}
		if err := tx.Where(model.City{Code: f.City.Code}).Assign(model.City{Name: city.Name, IsLive: city.IsLive}).FirstOrCreate(&city).Error; err != nil {
			return err
		}

		formerNames := map[string][]string{}
		for old, cur := range f.Aliases {
			formerNames[cur] = append(formerNames[cur], strings.ToLower(old))
		}
		lineCount := map[string]int{}
		for _, l := range f.Lines {
			for _, s := range l.Stations {
				lineCount[s]++
			}
		}
		for _, t := range f.Transfers {
			lineCount[t[0]]++
			lineCount[t[1]]++
		}

		stationIDs := map[string]model.Station{}
		ensure := func(name string) (model.Station, error) {
			if st, ok := stationIDs[name]; ok {
				return st, nil
			}
			st := model.Station{}
			search := strings.Join(append([]string{strings.ToLower(name)}, formerNames[name]...), " | ")
			attrs := model.Station{CityID: city.ID, Name: name, IsInterchange: lineCount[name] > 1, SearchText: search}
			if f.WomensCoachNote != "" {
				note := f.WomensCoachNote
				attrs.WomensCoachNote = &note
			}
			err := tx.Where(model.Station{Code: stationCode(f.City.Code, name)}).Assign(attrs).FirstOrCreate(&st).Error
			stationIDs[name] = st
			return st, err
		}

		for _, l := range f.Lines {
			line := model.MetroLine{}
			if err := tx.Where(model.MetroLine{GTFSRouteID: l.GTFSRouteID}).
				Assign(model.MetroLine{CityID: city.ID, Name: l.Name, ColorHex: l.Color}).FirstOrCreate(&line).Error; err != nil {
				return err
			}
			if err := tx.Where("line_id = ?", line.ID).Delete(&model.LineStation{}).Error; err != nil {
				return err
			}
			for i, name := range l.Stations {
				st, err := ensure(name)
				if err != nil {
					return err
				}
				if err := tx.Create(&model.LineStation{LineID: line.ID, StationID: st.ID, Seq: int16(i)}).Error; err != nil {
					return fmt.Errorf("line %s station %s: %w", l.Name, name, err)
				}
			}
		}
		for _, t := range f.Transfers {
			a, err := ensure(t[0])
			if err != nil {
				return err
			}
			b, err := ensure(t[1])
			if err != nil {
				return err
			}
			for _, pair := range [][2]model.Station{{a, b}, {b, a}} {
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
					Create(&model.StationTransfer{FromStationID: pair[0].ID, ToStationID: pair[1].ID}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
