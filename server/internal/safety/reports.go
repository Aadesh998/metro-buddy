package safety

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metro-buddy/server/internal/audit"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/ratelimit"
)

type Handlers struct {
	DB      *gorm.DB
	Audit   *audit.Recorder
	Limiter *ratelimit.Limiter
	// OnCase fires after a case is created or escalated (AI triage, admin alerts).
	OnCase func(caseID uuid.UUID)
	Now    func() time.Time
}

func (h *Handlers) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now().UTC()
}

func (h *Handlers) Register(g *echo.Group) {
	write := h.Limiter.Middleware(ratelimit.Rule{Name: "safety_write", Limit: 30, Window: time.Hour, ByTier: true})
	g.POST("/blocks", h.block, write)
	g.DELETE("/blocks/:persona_id", h.unblock)
	g.GET("/blocks", h.listBlocks)
	g.POST("/mutes", h.mute, write)
	g.DELETE("/mutes/:persona_id", h.unmute)
	g.GET("/mutes", h.listMutes)
	g.POST("/reports", h.report, h.Limiter.Middleware(ratelimit.Rule{Name: "reports", Limit: 20, Window: time.Hour, ByTier: true}))
}

type personaTarget struct {
	PersonaID uuid.UUID `json:"persona_id" validate:"required"`
}

func (h *Handlers) personaExists(ctx context.Context, id uuid.UUID) error {
	var n int64
	if err := h.DB.WithContext(ctx).Model(&model.Persona{}).Where("id = ?", id).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return httpx.NotFound()
	}
	return nil
}

func (h *Handlers) block(c echo.Context) error {
	req, err := httpx.Bind[personaTarget](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	if req.PersonaID == p.PersonaID {
		return httpx.BadRequest("you can't block yourself")
	}
	ctx := c.Request().Context()
	if err := h.personaExists(ctx, req.PersonaID); err != nil {
		return err
	}
	err = h.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.Block{
			BlockerPersonaID: p.PersonaID, BlockedPersonaID: req.PersonaID, BlockerUserID: p.UserID,
		}).Error; err != nil {
			return err
		}
		// A block also cancels any pending contact requests between the two.
		return tx.Model(&model.DMRequest{}).
			Where("status = ? AND ((from_persona_id = ? AND to_persona_id = ?) OR (from_persona_id = ? AND to_persona_id = ?))",
				model.RequestPending, p.PersonaID, req.PersonaID, req.PersonaID, p.PersonaID).
			Update("status", model.RequestBlocked).Error
	})
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handlers) unblock(c echo.Context) error {
	id, err := uuid.Parse(c.Param("persona_id"))
	if err != nil {
		return httpx.NotFound()
	}
	p := httpx.PrincipalFrom(c)
	if err := h.DB.WithContext(c.Request().Context()).
		Where("blocker_persona_id = ? AND blocked_persona_id = ?", p.PersonaID, id).Delete(&model.Block{}).Error; err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

type relationDTO struct {
	Persona   identity.PersonaRef `json:"persona"`
	CreatedAt time.Time           `json:"created_at"`
}

func (h *Handlers) listRelations(c echo.Context, table, ownerCol, otherCol string) error {
	var rows []struct {
		Other     uuid.UUID
		CreatedAt time.Time
	}
	db := h.DB.WithContext(c.Request().Context())
	if err := db.Table(table).Select(otherCol+" AS other, created_at").
		Where(ownerCol+" = ?", httpx.PrincipalFrom(c).PersonaID).Order("created_at DESC").Limit(500).Scan(&rows).Error; err != nil {
		return err
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.Other
	}
	refs, err := identity.LoadRefs(db, ids)
	if err != nil {
		return err
	}
	out := make([]relationDTO, len(rows))
	for i, r := range rows {
		out[i] = relationDTO{Persona: refs[r.Other], CreatedAt: r.CreatedAt}
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (h *Handlers) listBlocks(c echo.Context) error {
	return h.listRelations(c, "blocks", "blocker_persona_id", "blocked_persona_id")
}

func (h *Handlers) listMutes(c echo.Context) error {
	return h.listRelations(c, "mutes", "muter_persona_id", "muted_persona_id")
}

func (h *Handlers) mute(c echo.Context) error {
	req, err := httpx.Bind[personaTarget](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	if req.PersonaID == p.PersonaID {
		return httpx.BadRequest("you can't mute yourself")
	}
	ctx := c.Request().Context()
	if err := h.personaExists(ctx, req.PersonaID); err != nil {
		return err
	}
	if err := h.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model.Mute{MuterPersonaID: p.PersonaID, MutedPersonaID: req.PersonaID}).Error; err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handlers) unmute(c echo.Context) error {
	id, err := uuid.Parse(c.Param("persona_id"))
	if err != nil {
		return httpx.NotFound()
	}
	if err := h.DB.WithContext(c.Request().Context()).
		Where("muter_persona_id = ? AND muted_persona_id = ?", httpx.PrincipalFrom(c).PersonaID, id).Delete(&model.Mute{}).Error; err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

type reportRequest struct {
	TargetType model.ContentType `json:"target_type" validate:"required,oneof=persona post comment message conversation episode call_in trip"`
	TargetID   uuid.UUID         `json:"target_id" validate:"required"`
	Reason     string            `json:"reason" validate:"required"`
	Details    string            `json:"details" validate:"max=500"`
	AlsoBlock  bool              `json:"also_block"`
}

// Severity drives queue order and SLA. Non-consensual intimate content has a 24h legal takedown window.
var reasonSeverity = map[string]model.Severity{
	"threat": model.SevCritical, "underage": model.SevCritical, "sexual": model.SevCritical,
	"doxxing": model.SevHigh, "self_harm": model.SevHigh, "harassment": model.SevHigh, "hate": model.SevHigh,
	"impersonation": model.SevMedium, "spam": model.SevLow, "other": model.SevMedium,
}

var SLA = map[model.Severity]time.Duration{
	model.SevCritical: time.Hour, model.SevHigh: 4 * time.Hour, model.SevMedium: 24 * time.Hour, model.SevLow: 72 * time.Hour,
}

var ErrAlreadyReported = httpx.Conflict("you already reported this")

func (h *Handlers) report(c echo.Context) error {
	req, err := httpx.Bind[reportRequest](c)
	if err != nil {
		return err
	}
	if !slices.Contains(model.ReportReasons, req.Reason) {
		return httpx.Unprocessable("invalid_reason", "unknown report reason")
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var caseID uuid.UUID
	err = h.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		owner, err := TargetOwner(tx, req.TargetType, req.TargetID)
		if err != nil {
			return err
		}
		if owner != nil && *owner == p.PersonaID {
			return httpx.BadRequest("you can't report your own content")
		}
		var dup int64
		if err := tx.Model(&model.Report{}).Where("reporter_persona_id = ? AND target_type = ? AND target_id = ?",
			p.PersonaID, req.TargetType, req.TargetID).Count(&dup).Error; err != nil {
			return err
		}
		if dup > 0 {
			return ErrAlreadyReported
		}
		sev := reasonSeverity[req.Reason]
		mc, err := h.openCase(tx, req.TargetType, req.TargetID, owner, sev, "report")
		if err != nil {
			return err
		}
		caseID = mc.ID
		if err := tx.Create(&model.Report{
			ReporterPersonaID: p.PersonaID, TargetType: req.TargetType, TargetID: req.TargetID,
			Reason: req.Reason, DetailsMasked: pii.Mask(req.Details, pii.Options{}).Text, CaseID: mc.ID,
		}).Error; err != nil {
			return err
		}
		if req.AlsoBlock && owner != nil {
			return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.Block{
				BlockerPersonaID: p.PersonaID, BlockedPersonaID: *owner, BlockerUserID: p.UserID,
			}).Error
		}
		return nil
	})
	if err != nil {
		return err
	}
	if h.OnCase != nil {
		h.OnCase(caseID)
	}
	return c.JSON(http.StatusCreated, map[string]any{"received": true, "show_support": req.Reason == "self_harm"})
}

// OpenCase groups reports on one target into a single open case, raising severity if needed.
func (h *Handlers) openCase(tx *gorm.DB, ct model.ContentType, id uuid.UUID, owner *uuid.UUID, sev model.Severity, source string) (*model.ModerationCase, error) {
	return OpenCase(tx, h.now(), ct, id, owner, sev, source)
}

func OpenCase(tx *gorm.DB, now time.Time, ct model.ContentType, id uuid.UUID, owner *uuid.UUID, sev model.Severity, source string) (*model.ModerationCase, error) {
	var mc model.ModerationCase
	err := tx.Where("target_type = ? AND target_id = ? AND status IN ?", ct, id,
		[]model.CaseStatus{model.CaseOpen, model.CaseInReview, model.CaseEscalated}).First(&mc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		mc = model.ModerationCase{
			TargetType: ct, TargetID: id, TargetPersonaID: owner, Severity: sev, Status: model.CaseOpen,
			Source: source, ReportCount: 1, SLADueAt: now.Add(SLA[sev]),
		}
		return &mc, tx.Create(&mc).Error
	}
	if err != nil {
		return nil, err
	}
	updates := map[string]any{"report_count": gorm.Expr("report_count + 1")}
	if model.SeverityRank[sev] > model.SeverityRank[mc.Severity] {
		updates["severity"] = sev
		if due := now.Add(SLA[sev]); due.Before(mc.SLADueAt) {
			updates["sla_due_at"] = due
		}
	}
	return &mc, tx.Model(&mc).Updates(updates).Error
}

// TargetOwner resolves the persona responsible for a piece of content; nil when there is none.
func TargetOwner(tx *gorm.DB, ct model.ContentType, id uuid.UUID) (*uuid.UUID, error) {
	var q *gorm.DB
	switch ct {
	case model.CTPersona:
		q = tx.Model(&model.Persona{}).Where("id = ?", id).Select("id")
	case model.CTPost:
		q = tx.Model(&model.Post{}).Where("id = ?", id).Select("author_persona_id")
	case model.CTComment:
		q = tx.Model(&model.Comment{}).Where("id = ?", id).Select("author_persona_id")
	case model.CTMessage:
		q = tx.Model(&model.Message{}).Where("id = ?", id).Select("sender_persona_id")
	case model.CTCallIn:
		q = tx.Model(&model.CallInRequest{}).Where("id = ?", id).Select("persona_id")
	case model.CTEpisode:
		q = tx.Table("show_episodes").Joins("JOIN shows ON shows.id = show_episodes.show_id").
			Where("show_episodes.id = ?", id).Select("shows.host_persona_id")
	case model.CTConversation, model.CTTrip:
		table := map[model.ContentType]string{model.CTConversation: "conversations", model.CTTrip: "trips"}[ct]
		var n int64
		if err := tx.Table(table).Where("id = ?", id).Count(&n).Error; err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, httpx.NotFound()
		}
		return nil, nil
	default:
		return nil, httpx.NotFound()
	}
	var ids []uuid.UUID
	if err := q.Limit(1).Scan(&ids).Error; err != nil {
		return nil, err
	}
	if len(ids) == 0 || ids[0] == uuid.Nil {
		return nil, httpx.NotFound()
	}
	return &ids[0], nil
}
