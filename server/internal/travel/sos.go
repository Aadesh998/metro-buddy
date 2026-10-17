package travel

import (
	"context"
	"html/template"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/auth"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/ratelimit"
	"metro-buddy/server/internal/safety"
	"metro-buddy/server/internal/secure"
)

const (
	maxContacts = 5
	shareAfter  = 2 * time.Hour
)

func (s *Service) registerSafety(g *echo.Group, authed *echo.Group) {
	authed.GET("/me/trusted-contacts", s.listContacts)
	authed.POST("/me/trusted-contacts", s.addContact)
	authed.DELETE("/me/trusted-contacts/:id", s.removeContact)
	g.POST("/trips/:id/share", s.shareTrip)
	// SOS must work even if something else is rate limited; it has its own generous bucket.
	g.POST("/sos", s.sos, s.Limiter.Middleware(ratelimit.Rule{Name: "sos", Limit: 10, Window: time.Hour}))
}

type ContactDTO struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	PhoneMasked string    `json:"phone_masked"`
}

func (s *Service) contactDTOs(cs []model.TrustedContact) []ContactDTO {
	out := make([]ContactDTO, len(cs))
	for i, c := range cs {
		p, _ := s.Box.OpenString(c.PhoneEnc)
		masked := "••••••••"
		if len(p) >= 2 {
			masked += p[len(p)-2:]
		}
		out[i] = ContactDTO{ID: c.ID, Name: c.Name, PhoneMasked: masked}
	}
	return out
}

func (s *Service) listContacts(c echo.Context) error {
	var cs []model.TrustedContact
	if err := s.DB.WithContext(c.Request().Context()).Where("user_id = ?", httpx.PrincipalFrom(c).UserID).Order("created_at").Find(&cs).Error; err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": s.contactDTOs(cs)})
}

type contactRequest struct {
	Name  string `json:"name" validate:"required,min=1,max=60"`
	Phone string `json:"phone" validate:"required,max=20"`
}

func (s *Service) addContact(c echo.Context) error {
	req, err := httpx.Bind[contactRequest](c)
	if err != nil {
		return err
	}
	e164, err := auth.NormalizePhone(req.Phone)
	if err != nil {
		return err
	}
	uid := httpx.PrincipalFrom(c).UserID
	tx := s.DB.WithContext(c.Request().Context())
	var n int64
	tx.Model(&model.TrustedContact{}).Where("user_id = ?", uid).Count(&n)
	if n >= maxContacts {
		return httpx.Conflict("you can add up to 5 trusted contacts")
	}
	if err := tx.Create(&model.TrustedContact{UserID: uid, Name: req.Name, PhoneEnc: s.Box.SealString(e164)}).Error; err != nil {
		return err
	}
	return s.listContacts(c)
}

func (s *Service) removeContact(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	if err := s.DB.WithContext(c.Request().Context()).Where("id = ? AND user_id = ?", id, httpx.PrincipalFrom(c).UserID).Delete(&model.TrustedContact{}).Error; err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Service) ensureShare(tx *gorm.DB, t *model.Trip) (string, error) {
	now := s.now()
	if t.ShareToken != nil && t.ShareExpiresAt != nil && now.Before(*t.ShareExpiresAt) {
		return s.BaseURL + "/trip/" + *t.ShareToken, nil
	}
	tok := secure.Token(18)
	exp := t.MeetAt.Add(completeAfter + shareAfter)
	if exp.Before(now.Add(shareAfter)) {
		exp = now.Add(shareAfter)
	}
	if err := tx.Model(t).Updates(map[string]any{"share_token": tok, "share_expires_at": exp}).Error; err != nil {
		return "", err
	}
	t.ShareToken, t.ShareExpiresAt = &tok, &exp
	return s.BaseURL + "/trip/" + tok, nil
}

func (s *Service) shareTrip(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	t, err := s.myTrip(tx, c.Param("id"), p.PersonaID)
	if err != nil {
		return err
	}
	url, err := s.ensureShare(tx, t)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"url": url, "expires_at": t.ShareExpiresAt})
}

type sosRequest struct {
	TripID    *uuid.UUID `json:"trip_id"`
	StationID *uuid.UUID `json:"station_id"`
}

// sos texts every trusted contact a live trip link and opens a critical T&S case. Contacts get
// the SMS straight from this request so it doesn't wait on any queue.
func (s *Service) sos(c echo.Context) error {
	req, err := httpx.Bind[sosRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var ev model.SOSEvent
	var link string
	var contacts []model.TrustedContact
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ev = model.SOSEvent{UserID: p.UserID, TripID: req.TripID, LastStationID: req.StationID, Status: model.SOSOpen}
		target, targetType := p.PersonaID, model.CTPersona
		if req.TripID != nil {
			t, err := s.myTrip(tx, req.TripID.String(), p.PersonaID)
			if err != nil {
				return err
			}
			if link, err = s.ensureShare(tx, t); err != nil {
				return err
			}
			target, targetType = t.ID, model.CTTrip
			if req.StationID != nil {
				tx.Model(&model.TripParticipant{}).Where("trip_id = ? AND persona_id = ?", t.ID, p.PersonaID).Update("last_station_id", *req.StationID)
			}
		}
		if err := tx.Create(&ev).Error; err != nil {
			return err
		}
		if _, err := safety.OpenCase(tx, s.now(), targetType, target, nil, model.SevCritical, "sos"); err != nil {
			return err
		}
		if err := s.Audit.Log(tx, model.ActorUser, &p.UserID, "sos.triggered", "sos_event", &ev.ID, nil); err != nil {
			return err
		}
		return tx.Where("user_id = ?", p.UserID).Find(&contacts).Error
	})
	if err != nil {
		return err
	}
	alias := "A Metro Buddy user"
	if refs, err := identity.LoadRefs(s.DB.WithContext(ctx), []uuid.UUID{p.PersonaID}); err == nil {
		alias = refs[p.PersonaID].Alias
	}
	if link == "" {
		link = s.BaseURL + "/safety"
	}
	sent := 0
	sctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	for _, ct := range contacts {
		phone, err := s.Box.OpenString(ct.PhoneEnc)
		if err != nil {
			continue
		}
		if s.SMS.SendSOS(sctx, phone, alias, link) == nil {
			sent++
		}
	}
	if sent > 0 {
		s.DB.WithContext(ctx).Model(&ev).Update("status", model.SOSContactsNotified)
	}
	return c.JSON(http.StatusCreated, map[string]any{"sos_id": ev.ID, "contacts_notified": sent, "share_url": link, "emergency": "112"})
}

var sharePage = template.Must(template.New("trip").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex"><meta http-equiv="refresh" content="60"><title>Shared trip · Metro Buddy</title><link rel="stylesheet" href="/static/site.css"></head>
<body><main class="wrap doc"><h1>Shared trip</h1>
{{if .Expired}}<p class="meta">This trip link has expired.</p>{{else}}
<p class="meta">Someone you know shared their Metro Buddy trip with you. Station-level updates only; this page refreshes every minute.</p>
<table><tr><th>Trip status</th><td>{{.Status}}</td></tr><tr><th>Meeting point</th><td>{{.Meet}}</td></tr><tr><th>Time</th><td>{{.When}}</td></tr></table>
<table><tr><th>Traveller</th><th>Checked in</th><th>Last station</th></tr>
{{range .People}}<tr><td>{{.Alias}}</td><td>{{if .CheckedIn}}Yes{{else}}Not yet{{end}}</td><td>{{if .Last}}{{.Last}}{{else}}—{{end}}</td></tr>{{end}}</table>
<p>If you think they’re in danger, call <a href="tel:112">112</a>, or the Delhi Metro helpline <a href="tel:155370">155370</a>.</p>{{end}}
</main></body></html>`))

type sharePerson struct {
	Alias, Last string
	CheckedIn   bool
}

// SharePage is the public page trusted contacts open. It shows aliases and station-level
// progress only: never names, phone numbers, or where anyone lives or works.
func (s *Service) SharePage(c echo.Context) error {
	tok := c.Param("token")
	tx := s.DB.WithContext(c.Request().Context())
	c.Response().Header().Set("Cache-Control", "no-store")
	c.Response().Header().Set("X-Robots-Tag", "noindex")
	data := map[string]any{"Expired": true}
	var t model.Trip
	if len(tok) >= 16 && tx.Where("share_token = ?", tok).Limit(1).Find(&t).RowsAffected == 1 && t.ShareExpiresAt != nil && s.now().Before(*t.ShareExpiresAt) {
		var parts []model.TripParticipant
		tx.Where("trip_id = ?", t.ID).Find(&parts)
		ids := []uuid.UUID{t.MeetStationID}
		pids := []uuid.UUID{}
		for _, p := range parts {
			pids = append(pids, p.PersonaID)
			if p.LastStationID != nil {
				ids = append(ids, *p.LastStationID)
			}
		}
		refs, _ := identity.LoadRefs(tx, pids)
		st, _ := s.stationRefs(tx, ids...)
		var people []sharePerson
		for _, p := range parts {
			sp := sharePerson{Alias: refs[p.PersonaID].Alias, CheckedIn: p.CheckedInAt != nil}
			if p.LastStationID != nil {
				sp.Last = st[*p.LastStationID].Name
			}
			people = append(people, sp)
		}
		data = map[string]any{"Expired": false, "Status": string(t.Status), "People": people,
			"Meet": st[t.MeetStationID].Name + " station · " + t.MeetPlatform, "When": t.MeetAt.In(ist).Format("Mon 2 Jan, 3:04 PM")}
	}
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	return sharePage.Execute(c.Response(), data)
}
