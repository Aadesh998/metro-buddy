package admin

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
)

func (h *Handlers) registerStudio(g *echo.Group) {
	lead := h.RequireRole(model.AdminTSLead)
	g.GET("/hosts", h.listHosts)
	g.POST("/hosts/:id/decision", h.decideHost, lead)
	g.GET("/episodes/live", h.liveEpisodes)
	g.POST("/episodes/:id/end", h.forceEndEpisode, lead)
}

type HostAppDTO struct {
	ID         uuid.UUID        `json:"id"`
	PersonaID  uuid.UUID        `json:"persona_id"`
	Alias      string           `json:"alias"`
	Status     model.HostStatus `json:"status"`
	Tagline    string           `json:"tagline"`
	Pitch      string           `json:"pitch"`
	TrustScore int              `json:"trust_score"`
	Reports    int64            `json:"reports_90d"`
	CreatedAt  time.Time        `json:"created_at"`
}

func (h *Handlers) listHosts(c echo.Context) error {
	tx := h.DB.WithContext(c.Request().Context())
	status := c.QueryParam("status")
	if status == "" {
		status = string(model.HostApplied)
	}
	var rows []model.HostProfile
	if err := tx.Where("status = ?", status).Order("created_at").Limit(200).Find(&rows).Error; err != nil {
		return err
	}
	out := make([]HostAppDTO, 0, len(rows))
	for _, r := range rows {
		var p model.Persona
		tx.Unscoped().First(&p, "id = ?", r.PersonaID)
		var u model.User
		tx.Unscoped().First(&u, "id = ?", p.UserID)
		d := HostAppDTO{ID: r.ID, PersonaID: r.PersonaID, Alias: p.Alias, Status: r.Status, Tagline: r.Tagline, Pitch: r.Pitch,
			TrustScore: u.TrustScore, CreatedAt: r.CreatedAt}
		tx.Model(&model.Report{}).Where("target_type = ? AND target_id = ? AND created_at > ?", model.CTPersona, r.PersonaID, h.now().AddDate(0, 0, -90)).Count(&d.Reports)
		out = append(out, d)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

type hostDecision struct {
	Decision string `json:"decision" validate:"required,oneof=approve reject suspend"`
	Note     string `json:"note" validate:"max=500"`
}

func (h *Handlers) decideHost(c echo.Context) error {
	req, err := httpx.Bind[hostDecision](c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	actor := PrincipalFrom(c).ID
	var hp model.HostProfile
	err = h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&hp, "id = ?", id).Error; err != nil {
			return err
		}
		status := map[string]model.HostStatus{"approve": model.HostApproved, "reject": model.HostRejected, "suspend": model.HostSuspended}[req.Decision]
		updates := map[string]any{"status": status}
		if status == model.HostApproved {
			updates["approved_by"] = actor
		}
		if err := tx.Model(&hp).Updates(updates).Error; err != nil {
			return err
		}
		hp.Status = status
		if err := tx.Model(&model.Persona{}).Where("id = ?", hp.PersonaID).Update("is_host", status == model.HostApproved).Error; err != nil {
			return err
		}
		return h.Audit.Log(tx, model.ActorAdmin, &actor, "host."+req.Decision, "host_profile", &hp.ID, model.JSONMap{"note": req.Note})
	})
	if err != nil {
		return err
	}
	if h.NotifyPersona != nil && hp.Status != model.HostSuspended {
		h.NotifyPersona(hp.PersonaID, model.NotifHostDecision, model.JSONMap{"status": string(hp.Status)})
	}
	if hp.Status == model.HostApproved && h.OnHostApproved != nil {
		h.OnHostApproved(hp.PersonaID)
	}
	return c.JSON(http.StatusOK, map[string]any{"id": hp.ID, "status": hp.Status})
}

type LiveEpisodeDTO struct {
	ID        uuid.UUID  `json:"id"`
	Title     string     `json:"title"`
	ShowTitle string     `json:"show_title"`
	HostAlias string     `json:"host_alias"`
	StartedAt *time.Time `json:"started_at"`
	Listeners int64      `json:"listeners"`
	Reports   int64      `json:"reports"`
	AutoMuted bool       `json:"auto_muted"`
}

func (h *Handlers) liveEpisodes(c echo.Context) error {
	tx := h.DB.WithContext(c.Request().Context())
	var eps []model.ShowEpisode
	if err := tx.Where("status = ?", model.EpisodeLive).Order("started_at").Find(&eps).Error; err != nil {
		return err
	}
	out := make([]LiveEpisodeDTO, 0, len(eps))
	for _, ep := range eps {
		var sh model.Show
		tx.Unscoped().First(&sh, "id = ?", ep.ShowID)
		var p model.Persona
		tx.Unscoped().Select("alias").First(&p, "id = ?", sh.HostPersonaID)
		d := LiveEpisodeDTO{ID: ep.ID, Title: ep.Title, ShowTitle: sh.Title, HostAlias: p.Alias, StartedAt: ep.StartedAt, AutoMuted: ep.AutoMutedAt != nil}
		tx.Model(&model.EpisodeParticipant{}).Where("episode_id = ? AND left_at IS NULL", ep.ID).Count(&d.Listeners)
		tx.Model(&model.Report{}).Where("(target_type = ? AND target_id = ?) OR (target_type = ? AND target_id IN (SELECT id FROM call_in_requests WHERE episode_id = ?))",
			model.CTEpisode, ep.ID, model.CTCallIn, ep.ID).Count(&d.Reports)
		out = append(out, d)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (h *Handlers) forceEndEpisode(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	if h.EndEpisode == nil {
		return httpx.E(http.StatusServiceUnavailable, "unavailable", "studio isn't configured")
	}
	if err := h.EndEpisode(c.Request().Context(), id); err != nil {
		return err
	}
	actor := PrincipalFrom(c).ID
	h.Audit.Log(h.DB.WithContext(c.Request().Context()), model.ActorAdmin, &actor, "episode.force_end", "episode", &id, nil)
	return c.NoContent(http.StatusNoContent)
}
