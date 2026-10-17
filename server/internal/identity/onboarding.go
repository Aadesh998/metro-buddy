package identity

import (
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
)

const PolicyVersion = "2026-10"

var (
	ErrUnderage     = httpx.Unprocessable("underage", "Metro Buddy is only for people 18 and over")
	ErrNotOnboarded = httpx.E(http.StatusForbidden, "onboarding_required", "confirm your age and consents first")
	ErrCoreConsent  = httpx.Unprocessable("core_consent_required", "core service consent is required to use Metro Buddy")
	istLoc          = time.FixedZone("IST", 5*3600+1800)
)

type ageRequest struct {
	BirthDate string `json:"birth_date" validate:"required,datetime=2006-01-02"`
}

// Only the birth year is stored; the full date is used once for an exact 18+ check.
func (h *Handlers) setAge(c echo.Context) error {
	req, err := httpx.Bind[ageRequest](c)
	if err != nil {
		return err
	}
	dob, _ := time.ParseInLocation("2006-01-02", req.BirthDate, istLoc)
	now := h.now().In(istLoc)
	if dob.AddDate(18, 0, 0).After(now) {
		return ErrUnderage
	}
	if dob.Year() < now.Year()-100 {
		return httpx.Unprocessable("invalid_birth_date", "enter your real date of birth")
	}
	uid := httpx.PrincipalFrom(c).UserID
	err = h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var u model.User
		if err := tx.First(&u, "id = ?", uid).Error; err != nil {
			return err
		}
		if u.BirthYear != nil {
			return httpx.Conflict("date of birth is already set")
		}
		year := int16(dob.Year())
		if err := tx.Model(&u).Update("birth_year", year).Error; err != nil {
			return err
		}
		return h.maybeOnboard(tx, uid.String())
	})
	if err != nil {
		return err
	}
	return h.me(c)
}

type consentItem struct {
	Purpose model.ConsentPurpose `json:"purpose" validate:"required,oneof=core_service travel_matching ai_processing analytics marketing_push"`
	Granted bool                 `json:"granted"`
}

type consentRequest struct {
	PolicyVersion string        `json:"policy_version" validate:"required,max=16"`
	Items         []consentItem `json:"items" validate:"required,min=1,max=5,dive"`
}

func (h *Handlers) setConsents(c echo.Context) error {
	req, err := httpx.Bind[consentRequest](c)
	if err != nil {
		return err
	}
	if req.PolicyVersion != PolicyVersion {
		return httpx.Conflict("the privacy policy changed, please review it again")
	}
	uid := httpx.PrincipalFrom(c).UserID
	err = h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var seen []model.ConsentPurpose
		for _, it := range req.Items {
			if slices.Contains(seen, it.Purpose) {
				return httpx.Unprocessable("duplicate_purpose", "each purpose once")
			}
			seen = append(seen, it.Purpose)
			if err := tx.Create(&model.Consent{UserID: uid, Purpose: it.Purpose, Granted: it.Granted, PolicyVersion: req.PolicyVersion}).Error; err != nil {
				return err
			}
		}
		return h.maybeOnboard(tx, uid.String())
	})
	if err != nil {
		return err
	}
	return h.getConsents(c)
}

type ConsentState struct {
	Purpose       model.ConsentPurpose `json:"purpose"`
	Granted       bool                 `json:"granted"`
	PolicyVersion string               `json:"policy_version"`
	UpdatedAt     *time.Time           `json:"updated_at"`
}

// CurrentConsents returns the latest record per purpose; records are append-only.
func CurrentConsents(tx *gorm.DB, userID string) (map[model.ConsentPurpose]model.Consent, error) {
	var rows []model.Consent
	if err := tx.Where("user_id = ?", userID).Order("created_at, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := map[model.ConsentPurpose]model.Consent{}
	for _, r := range rows {
		out[r.Purpose] = r
	}
	return out, nil
}

func HasConsent(tx *gorm.DB, userID string, p model.ConsentPurpose) (bool, error) {
	m, err := CurrentConsents(tx, userID)
	if err != nil {
		return false, err
	}
	c, ok := m[p]
	return ok && c.Granted, nil
}

func (h *Handlers) getConsents(c echo.Context) error {
	m, err := CurrentConsents(h.DB.WithContext(c.Request().Context()), httpx.PrincipalFrom(c).UserID.String())
	if err != nil {
		return err
	}
	out := make([]ConsentState, 0, len(model.ConsentPurposes))
	for _, p := range model.ConsentPurposes {
		st := ConsentState{Purpose: p}
		if r, ok := m[p]; ok {
			t := r.CreatedAt
			st.Granted, st.PolicyVersion, st.UpdatedAt = r.Granted, r.PolicyVersion, &t
		}
		out = append(out, st)
	}
	return c.JSON(http.StatusOK, map[string]any{"policy_version": PolicyVersion, "items": out})
}

func (h *Handlers) maybeOnboard(tx *gorm.DB, uid string) error {
	var u model.User
	if err := tx.First(&u, "id = ?", uid).Error; err != nil {
		return err
	}
	core, err := HasConsent(tx, uid, model.ConsentCore)
	if err != nil {
		return err
	}
	switch {
	case u.BirthYear != nil && core && u.OnboardedAt == nil:
		return tx.Model(&u).Update("onboarded_at", h.now()).Error
	case !core && u.OnboardedAt != nil:
		// Withdrawing core consent pauses the account until it is granted again.
		return tx.Model(&u).Update("onboarded_at", nil).Error
	}
	return nil
}

func (h *Handlers) requireOnboarded(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var u model.User
		err := h.DB.WithContext(c.Request().Context()).Select("onboarded_at").First(&u, "id = ?", httpx.PrincipalFrom(c).UserID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && u.OnboardedAt == nil) {
			return ErrNotOnboarded
		}
		if err != nil {
			return err
		}
		return next(c)
	}
}

type patchMeRequest struct {
	Gender *model.Gender `json:"gender" validate:"omitempty,oneof=woman man non_binary prefer_not_to_say"`
	Locale *string       `json:"locale" validate:"omitempty,oneof=en hi es fr de pt ja zh id ru"`
}

func (h *Handlers) patchMe(c echo.Context) error {
	req, err := httpx.Bind[patchMeRequest](c)
	if err != nil {
		return err
	}
	updates := map[string]any{}
	if req.Gender != nil {
		updates["gender"] = *req.Gender
	}
	if req.Locale != nil {
		updates["locale"] = *req.Locale
	}
	if len(updates) > 0 {
		if err := h.DB.WithContext(c.Request().Context()).Model(&model.User{}).
			Where("id = ?", httpx.PrincipalFrom(c).UserID).Updates(updates).Error; err != nil {
			return err
		}
	}
	return h.me(c)
}
