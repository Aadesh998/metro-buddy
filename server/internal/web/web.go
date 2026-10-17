// Package web serves the public, crawlable pages: server-rendered html/template plus htmx.
package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/ratelimit"
	"metro-buddy/server/internal/secure"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

type Meta struct {
	Title, Description, Canonical string
	NoIndex                       bool
	JSONLD                        template.JS
}

type page struct {
	path, tmpl string
	meta       Meta
}

var pages = []page{
	{"/", "home.html", Meta{Title: "Metro Buddy · Anonymous company for your commute", Description: "Vent, chat and find a travel buddy on your metro line, anonymously. Built for Delhi, Mumbai, Bengaluru and more."}},
	{"/guidelines", "guidelines.html", Meta{Title: "Community guidelines · Metro Buddy", Description: "How we keep Metro Buddy kind, anonymous and safe."}},
	{"/privacy", "privacy.html", Meta{Title: "Privacy policy · Metro Buddy", Description: "What we collect, why, and your rights under the DPDP Act, 2023."}},
	{"/terms", "terms.html", Meta{Title: "Terms of service · Metro Buddy", Description: "The rules for using Metro Buddy."}},
	{"/grievance", "grievance.html", Meta{Title: "Grievance officer · Metro Buddy", Description: "Contact our Grievance Officer. We acknowledge within 24 hours and resolve within 15 days."}},
}

const orgJSONLD = `{"@context":"https://schema.org","@type":"Organization","name":"Metro Buddy","url":"%BASE%","contactPoint":{"@type":"ContactPoint","contactType":"customer support","email":"grievance@metrobuddy.in","areaServed":"IN"}}`

type Handlers struct {
	DB      *gorm.DB
	Box     *secure.Box
	Limiter *ratelimit.Limiter
	Base    string
	tmpls   map[string]*template.Template
}

func (h *Handlers) Register(e *echo.Echo) error {
	h.tmpls = map[string]*template.Template{}
	for _, p := range pages {
		t, err := template.ParseFS(templatesFS, "templates/layout.html", "templates/"+p.tmpl, "templates/grievance_result.html")
		if err != nil {
			return err
		}
		h.tmpls[p.tmpl] = t
	}
	g := e.Group("", publicHeaders)
	for _, p := range pages {
		g.GET(p.path, h.render(p))
	}
	g.POST("/grievance", h.grievance, h.Limiter.Middleware(ratelimit.Rule{Name: "grievance", Limit: 5, Window: time.Hour, Key: ratelimit.ByIP}))
	g.GET("/robots.txt", h.robots)
	g.GET("/sitemap.xml", h.sitemap)
	static, _ := fs.Sub(staticFS, "static")
	g.GET("/static/*", echo.WrapHandler(http.StripPrefix("/static/", http.FileServer(http.FS(static)))), longCache)
	return nil
}

func publicHeaders(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		h := c.Response().Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		return next(c)
	}
}

func longCache(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "public, max-age=86400")
		return next(c)
	}
}

type view struct {
	Meta          Meta
	Base          string
	Year          int
	PolicyVersion string
}

func (h *Handlers) render(p page) echo.HandlerFunc {
	return func(c echo.Context) error {
		m := p.meta
		m.Canonical = h.Base + p.path
		if p.path == "/" {
			m.JSONLD = template.JS(strings.ReplaceAll(orgJSONLD, "%BASE%", h.Base))
		}
		c.Response().Header().Set("Cache-Control", "public, max-age=3600")
		c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
		c.Response().WriteHeader(http.StatusOK)
		return h.tmpls[p.tmpl].ExecuteTemplate(c.Response(), "layout", view{Meta: m, Base: h.Base, Year: time.Now().Year(), PolicyVersion: identity.PolicyVersion})
	}
}

type grievanceResult struct {
	Error    string
	Ref      string
	HasEmail bool
}

func (h *Handlers) fragment(c echo.Context, status int, r grievanceResult) error {
	if r.Error != "" {
		// Keep the form in place and show the error above it.
		c.Response().Header().Set("HX-Retarget", "#grievance-result")
	}
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().WriteHeader(status)
	return h.tmpls["grievance.html"].ExecuteTemplate(c.Response(), "grievance_result", r)
}

func (h *Handlers) grievance(c echo.Context) error {
	cat := c.FormValue("category")
	body := strings.TrimSpace(c.FormValue("body"))
	email := strings.TrimSpace(c.FormValue("email"))
	if !slices.Contains(model.GrievanceCategories, cat) {
		return h.fragment(c, http.StatusUnprocessableEntity, grievanceResult{Error: "Pick what your complaint is about."})
	}
	if len(body) < 10 || len(body) > 4000 {
		return h.fragment(c, http.StatusUnprocessableEntity, grievanceResult{Error: "Please describe what happened (10 to 4000 characters)."})
	}
	if email != "" {
		if a, err := mail.ParseAddress(email); err != nil || a.Address != email || len(email) > 120 {
			return h.fragment(c, http.StatusUnprocessableEntity, grievanceResult{Error: "That email address doesn't look right."})
		}
	}
	t, err := CreateTicket(h.DB.WithContext(c.Request().Context()), h.Box, nil, cat, body, email, time.Now().UTC())
	if err != nil {
		return err
	}
	return h.fragment(c, http.StatusCreated, grievanceResult{Ref: TicketRef(t.ID.String()), HasEmail: email != ""})
}

func TicketRef(id string) string {
	return "GRV-" + strings.ToUpper(strings.ReplaceAll(id, "-", "")[20:28])
}

// SLA under the IT Rules: acknowledge in 24h, resolve in 15 days; intimate-image complaints in 24h.
func CreateTicket(tx *gorm.DB, box *secure.Box, userID *uuid.UUID, category, body, email string, now time.Time) (*model.GrievanceTicket, error) {
	t := model.GrievanceTicket{
		UserID: userID, Category: category, BodyEnc: box.SealString(body), Status: model.CaseOpen,
		AckDueAt: now.Add(24 * time.Hour), ResolveDueAt: now.Add(15 * 24 * time.Hour),
	}
	if category == "ncii" {
		t.ResolveDueAt = now.Add(24 * time.Hour)
	}
	if email != "" {
		t.ContactEmailEnc = box.SealString(email)
	}
	return &t, tx.Create(&t).Error
}

func (h *Handlers) robots(c echo.Context) error {
	return c.String(http.StatusOK, "User-agent: *\nAllow: /\nDisallow: /admin\nDisallow: /v1\nDisallow: /trip/\nSitemap: "+h.Base+"/sitemap.xml\n")
}

func (h *Handlers) sitemap(c echo.Context) error {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, p := range pages {
		b.WriteString("<url><loc>" + template.HTMLEscapeString(h.Base+p.path) + "</loc></url>\n")
	}
	b.WriteString("</urlset>\n")
	c.Response().Header().Set("Cache-Control", "public, max-age=3600")
	return c.Blob(http.StatusOK, "application/xml", []byte(b.String()))
}
