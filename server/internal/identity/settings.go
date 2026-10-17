package identity

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
)

var notifKinds = map[string]bool{"dm_request": true, "message": true, "reply": true, "match": true, "trip_reminder": true, "episode_live": true, "mod_notice": true}

type SettingsDTO struct {
	MatchSameGenderOnly bool            `json:"match_same_gender_only"`
	MatchVerifiedOnly   bool            `json:"match_verified_only"`
	MatchAgeMin         *int16          `json:"match_age_min"`
	MatchAgeMax         *int16          `json:"match_age_max"`
	QuietHoursStart     *int16          `json:"quiet_hours_start"`
	QuietHoursEnd       *int16          `json:"quiet_hours_end"`
	DMFrom              model.DMPolicy  `json:"dm_from"`
	NotifPrefs          map[string]bool `json:"notif_prefs"`
}

func LoadSettings(tx *gorm.DB, uid any) (model.UserSettings, error) {
	var s model.UserSettings
	err := tx.Where("user_id = ?", uid).Limit(1).Find(&s).Error
	if s.DMFrom == "" {
		s.DMFrom = model.DMRequests
	}
	return s, err
}

func settingsDTO(s model.UserSettings) SettingsDTO {
	prefs := map[string]bool{}
	for k := range notifKinds {
		prefs[k] = true
	}
	for k, v := range s.NotifPrefs {
		prefs[k] = v
	}
	return SettingsDTO{MatchSameGenderOnly: s.MatchSameGenderOnly, MatchVerifiedOnly: s.MatchVerifiedOnly, MatchAgeMin: s.MatchAgeMin,
		MatchAgeMax: s.MatchAgeMax, QuietHoursStart: s.QuietHoursStart, QuietHoursEnd: s.QuietHoursEnd, DMFrom: s.DMFrom, NotifPrefs: prefs}
}

func (h *Handlers) getSettings(c echo.Context) error {
	s, err := LoadSettings(h.DB.WithContext(c.Request().Context()), httpx.PrincipalFrom(c).UserID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, settingsDTO(s))
}

type settingsPatch struct {
	MatchSameGenderOnly *bool           `json:"match_same_gender_only"`
	MatchVerifiedOnly   *bool           `json:"match_verified_only"`
	MatchAgeMin         *int16          `json:"match_age_min" validate:"omitempty,min=18,max=80"`
	MatchAgeMax         *int16          `json:"match_age_max" validate:"omitempty,min=18,max=80"`
	ClearAgeBand        bool            `json:"clear_age_band"`
	QuietHoursStart     *int16          `json:"quiet_hours_start" validate:"omitempty,min=0,max=1439"`
	QuietHoursEnd       *int16          `json:"quiet_hours_end" validate:"omitempty,min=0,max=1439"`
	ClearQuietHours     bool            `json:"clear_quiet_hours"`
	DMFrom              *model.DMPolicy `json:"dm_from" validate:"omitempty,oneof=requests nobody"`
	NotifPrefs          map[string]bool `json:"notif_prefs"`
}

func (h *Handlers) patchSettings(c echo.Context) error {
	req, err := httpx.Bind[settingsPatch](c)
	if err != nil {
		return err
	}
	uid := httpx.PrincipalFrom(c).UserID
	err = h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		s, err := LoadSettings(tx, uid)
		if err != nil {
			return err
		}
		s.UserID = uid
		if req.MatchSameGenderOnly != nil {
			s.MatchSameGenderOnly = *req.MatchSameGenderOnly
		}
		if req.MatchVerifiedOnly != nil {
			s.MatchVerifiedOnly = *req.MatchVerifiedOnly
		}
		if req.ClearAgeBand {
			s.MatchAgeMin, s.MatchAgeMax = nil, nil
		}
		if req.MatchAgeMin != nil {
			s.MatchAgeMin = req.MatchAgeMin
		}
		if req.MatchAgeMax != nil {
			s.MatchAgeMax = req.MatchAgeMax
		}
		if s.MatchAgeMin != nil && s.MatchAgeMax != nil && *s.MatchAgeMin > *s.MatchAgeMax {
			return httpx.Unprocessable("invalid_age_band", "minimum age must be below maximum")
		}
		if req.ClearQuietHours {
			s.QuietHoursStart, s.QuietHoursEnd = nil, nil
		}
		if (req.QuietHoursStart == nil) != (req.QuietHoursEnd == nil) {
			return httpx.Unprocessable("invalid_quiet_hours", "set both start and end")
		}
		if req.QuietHoursStart != nil {
			s.QuietHoursStart, s.QuietHoursEnd = req.QuietHoursStart, req.QuietHoursEnd
		}
		if req.DMFrom != nil {
			s.DMFrom = *req.DMFrom
		}
		if len(req.NotifPrefs) > 0 {
			if s.NotifPrefs == nil {
				s.NotifPrefs = model.BoolMap{}
			}
			for k, v := range req.NotifPrefs {
				if !notifKinds[k] {
					return httpx.Unprocessable("invalid_notification_kind", k)
				}
				s.NotifPrefs[k] = v
			}
		}
		s.UpdatedAt = time.Now().UTC()
		return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&s).Error
	})
	if err != nil {
		return err
	}
	return h.getSettings(c)
}
