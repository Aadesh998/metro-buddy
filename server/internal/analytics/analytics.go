// Package analytics stores product events with no link to any user or persona.
package analytics

import (
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/secure"
)

var (
	nameRe = regexp.MustCompile(`^[a-z][a-z0-9_.]{2,47}$`)
	// Only these property names are kept; anything else (ids, text, locations) is dropped.
	allowedProps = map[string]bool{
		"screen": true, "tab": true, "source": true, "kind": true, "sort": true, "bowl_kind": true, "post_type": true,
		"step": true, "result": true, "count": true, "duration_ms": true, "line": true, "city": true, "error_code": true,
	}
)

type Handlers struct {
	DB     *gorm.DB
	Pepper []byte
}

type event struct {
	Name  string         `json:"name" validate:"required"`
	Props map[string]any `json:"props"`
}

type batch struct {
	SessionID  string  `json:"session_id" validate:"required,uuid"`
	Platform   string  `json:"platform" validate:"omitempty,oneof=ios android web"`
	AppVersion string  `json:"app_version" validate:"max=16"`
	Events     []event `json:"events" validate:"required,min=1,max=50,dive"`
}

func cleanProps(in map[string]any) model.JSONMap {
	out := model.JSONMap{}
	for k, v := range in {
		if !allowedProps[k] {
			continue
		}
		switch t := v.(type) {
		case string:
			if len(t) <= 40 && !strings.ContainsAny(t, "@/") {
				out[k] = t
			}
		case float64, bool:
			out[k] = t
		}
	}
	return out
}

// Track records events only for users who opted in to analytics. The session hash is salted
// and rotates daily, so events can't be joined across days or back to an account.
func (h *Handlers) Track(c echo.Context) error {
	req, err := httpx.Bind[batch](c)
	if err != nil {
		return err
	}
	tx := h.DB.WithContext(c.Request().Context())
	ok, err := identity.HasConsent(tx, httpx.PrincipalFrom(c).UserID.String(), model.ConsentAnalytics)
	if err != nil {
		return err
	}
	if !ok {
		return c.NoContent(http.StatusNoContent)
	}
	day := time.Now().UTC().Format("2006-01-02")
	sess := hex.EncodeToString(secure.HMAC(h.Pepper, req.SessionID, day))
	rows := make([]model.AnalyticsEvent, 0, len(req.Events))
	for _, e := range req.Events {
		if !nameRe.MatchString(e.Name) {
			continue
		}
		rows = append(rows, model.AnalyticsEvent{Name: e.Name, SessionHash: sess, Platform: req.Platform, AppVersion: req.AppVersion, Props: cleanProps(e.Props)})
	}
	if len(rows) > 0 {
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
	}
	return c.NoContent(http.StatusNoContent)
}
