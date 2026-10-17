package admin

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/web"
)

func (h *Handlers) registerGrievance(g *echo.Group, lead echo.MiddlewareFunc) {
	g.GET("/grievances", h.listGrievances, lead)
	g.POST("/grievances/:id/ack", h.ackGrievance, lead)
	g.POST("/grievances/:id/resolve", h.resolveGrievance, lead)
}

type GrievanceDTO struct {
	ID             uuid.UUID        `json:"id"`
	Ref            string           `json:"ref"`
	Category       string           `json:"category"`
	Body           string           `json:"body"`
	ContactEmail   string           `json:"contact_email,omitempty"`
	FromUser       bool             `json:"from_user"`
	Status         model.CaseStatus `json:"status"`
	AcknowledgedAt *time.Time       `json:"acknowledged_at"`
	ResolvedAt     *time.Time       `json:"resolved_at"`
	AckDueAt       time.Time        `json:"ack_due_at"`
	ResolveDueAt   time.Time        `json:"resolve_due_at"`
	AckOverdue     bool             `json:"ack_overdue"`
	ResolveOverdue bool             `json:"resolve_overdue"`
	Resolution     *string          `json:"resolution"`
	CreatedAt      time.Time        `json:"created_at"`
}

func (h *Handlers) grievanceDTO(t model.GrievanceTicket) GrievanceDTO {
	body, _ := h.Box.OpenString(t.BodyEnc)
	email := ""
	if len(t.ContactEmailEnc) > 0 {
		email, _ = h.Box.OpenString(t.ContactEmailEnc)
	}
	now := h.now()
	return GrievanceDTO{
		ID: t.ID, Ref: web.TicketRef(t.ID.String()), Category: t.Category, Body: body, ContactEmail: email, FromUser: t.UserID != nil,
		Status: t.Status, AcknowledgedAt: t.AcknowledgedAt, ResolvedAt: t.ResolvedAt, AckDueAt: t.AckDueAt, ResolveDueAt: t.ResolveDueAt,
		AckOverdue: t.AcknowledgedAt == nil && now.After(t.AckDueAt), ResolveOverdue: t.ResolvedAt == nil && now.After(t.ResolveDueAt),
		Resolution: t.Resolution, CreatedAt: t.CreatedAt,
	}
}

func (h *Handlers) listGrievances(c echo.Context) error {
	tx := h.DB.WithContext(c.Request().Context())
	q := tx.Order("resolve_due_at")
	if c.QueryParam("status") == "closed" {
		q = q.Where("resolved_at IS NOT NULL")
	} else {
		q = q.Where("resolved_at IS NULL")
	}
	var rows []model.GrievanceTicket
	if err := q.Limit(200).Find(&rows).Error; err != nil {
		return err
	}
	out := make([]GrievanceDTO, len(rows))
	for i, r := range rows {
		out[i] = h.grievanceDTO(r)
	}
	actor := PrincipalFrom(c).ID
	h.Audit.Log(tx, model.ActorAdmin, &actor, "grievance.list", "grievance", nil, model.JSONMap{"count": len(rows)})
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (h *Handlers) loadTicket(tx *gorm.DB, idParam string) (*model.GrievanceTicket, error) {
	id, err := uuid.Parse(idParam)
	if err != nil {
		return nil, httpx.NotFound()
	}
	var t model.GrievanceTicket
	return &t, tx.First(&t, "id = ?", id).Error
}

func (h *Handlers) ackGrievance(c echo.Context) error {
	tx := h.DB.WithContext(c.Request().Context())
	t, err := h.loadTicket(tx, c.Param("id"))
	if err != nil {
		return err
	}
	if t.AcknowledgedAt == nil {
		now := h.now()
		if err := tx.Model(t).Updates(map[string]any{"acknowledged_at": now, "status": model.CaseInReview}).Error; err != nil {
			return err
		}
		t.AcknowledgedAt, t.Status = &now, model.CaseInReview
		actor := PrincipalFrom(c).ID
		h.Audit.Log(tx, model.ActorAdmin, &actor, "grievance.ack", "grievance", &t.ID, nil)
	}
	return c.JSON(http.StatusOK, h.grievanceDTO(*t))
}

type resolveRequest struct {
	Resolution string `json:"resolution" validate:"required,min=10,max=4000"`
}

func (h *Handlers) resolveGrievance(c echo.Context) error {
	req, err := httpx.Bind[resolveRequest](c)
	if err != nil {
		return err
	}
	tx := h.DB.WithContext(c.Request().Context())
	t, err := h.loadTicket(tx, c.Param("id"))
	if err != nil {
		return err
	}
	now := h.now()
	updates := map[string]any{"resolved_at": now, "status": model.CaseActioned, "resolution": req.Resolution}
	if t.AcknowledgedAt == nil {
		updates["acknowledged_at"] = now
	}
	if err := tx.Model(t).Updates(updates).Error; err != nil {
		return err
	}
	actor := PrincipalFrom(c).ID
	h.Audit.Log(tx, model.ActorAdmin, &actor, "grievance.resolve", "grievance", &t.ID, nil)
	tx.First(t, "id = ?", t.ID)
	return c.JSON(http.StatusOK, h.grievanceDTO(*t))
}

// AlertOverdue raises one alert per ticket or appeal that misses its legal SLA.
func (h *Handlers) AlertOverdue(ctx context.Context) error {
	now := h.now()
	var tickets []model.GrievanceTicket
	if err := h.DB.WithContext(ctx).Where("overdue_alert_at IS NULL AND ((acknowledged_at IS NULL AND ack_due_at < ?) OR (resolved_at IS NULL AND resolve_due_at < ?))", now, now).
		Find(&tickets).Error; err != nil {
		return err
	}
	for _, t := range tickets {
		h.Audit.Log(h.DB.WithContext(ctx), model.ActorSystem, nil, "grievance.overdue", "grievance", &t.ID, model.JSONMap{"ref": web.TicketRef(t.ID.String())})
		h.DB.WithContext(ctx).Model(&t).Update("overdue_alert_at", now)
	}
	if len(tickets) > 0 && h.OnOverdue != nil {
		h.OnOverdue(len(tickets))
	}
	return nil
}

type AppealDTO struct {
	ID         uuid.UUID          `json:"id"`
	Text       string             `json:"text"`
	Status     model.AppealStatus `json:"status"`
	Action     model.ModAction    `json:"action"`
	ActionNote string             `json:"action_note"`
	CaseID     uuid.UUID          `json:"case_id"`
	SLADueAt   time.Time          `json:"sla_due_at"`
	Overdue    bool               `json:"overdue"`
	CreatedAt  time.Time          `json:"created_at"`
}

func (h *Handlers) listAppeals(c echo.Context) error {
	tx := h.DB.WithContext(c.Request().Context())
	q := tx.Model(&model.Appeal{}).Order("sla_due_at")
	if c.QueryParam("status") == "closed" {
		q = q.Where("status <> ?", model.AppealOpen)
	} else {
		q = q.Where("status = ?", model.AppealOpen)
	}
	var rows []model.Appeal
	if err := q.Limit(200).Find(&rows).Error; err != nil {
		return err
	}
	out := make([]AppealDTO, len(rows))
	now := h.now()
	for i, a := range rows {
		var act model.ModerationAction
		tx.First(&act, "id = ?", a.ActionID)
		out[i] = AppealDTO{ID: a.ID, Text: a.TextMasked, Status: a.Status, Action: act.Action, ActionNote: act.Note, CaseID: act.CaseID,
			SLADueAt: a.SLADueAt, Overdue: a.Status == model.AppealOpen && now.After(a.SLADueAt), CreatedAt: a.CreatedAt}
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

type decideRequest struct {
	Decision model.AppealStatus `json:"decision" validate:"required,oneof=upheld overturned"`
	Note     string             `json:"note" validate:"required,min=3,max=2000"`
}

// decideAppeal: "upheld" keeps the action, "overturned" reverses it through a restore action.
func (h *Handlers) decideAppeal(c echo.Context) error {
	req, err := httpx.Bind[decideRequest](c)
	if err != nil {
		return err
	}
	actor := PrincipalFrom(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	var notify *uuid.UUID
	err = h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var ap model.Appeal
		if err := tx.First(&ap, "id = ?", id).Error; err != nil {
			return err
		}
		if ap.Status != model.AppealOpen {
			return httpx.Conflict("already decided")
		}
		var act model.ModerationAction
		if err := tx.First(&act, "id = ?", ap.ActionID).Error; err != nil {
			return err
		}
		// The person who took the action shouldn't judge the appeal against it.
		if act.AdminID == actor.ID {
			return httpx.Forbidden("another team member must review this appeal")
		}
		if err := tx.Model(&ap).Updates(map[string]any{"status": req.Decision, "decided_by": actor.ID}).Error; err != nil {
			return err
		}
		var mc model.ModerationCase
		if err := tx.First(&mc, "id = ?", act.CaseID).Error; err != nil {
			return err
		}
		if req.Decision == model.AppealOverturned {
			restore := model.ModerationAction{CaseID: mc.ID, AdminID: actor.ID, Action: model.ActRestore, TargetUserID: act.TargetUserID, Note: "Appeal overturned: " + req.Note}
			if err := h.applyAction(tx, &mc, &restore, act.TargetUserID, 0, h.now()); err != nil {
				return err
			}
			if err := tx.Create(&restore).Error; err != nil {
				return err
			}
			if act.TargetUserID != nil {
				if err := tx.Where("ref_id = ? AND kind = ?", act.ID, model.TrustModAction).Delete(&model.TrustEvent{}).Error; err != nil {
					return err
				}
			}
		}
		notify = mc.TargetPersonaID
		return h.Audit.Log(tx, model.ActorAdmin, &actor.ID, "appeal."+string(req.Decision), "appeal", &ap.ID, model.JSONMap{"note": req.Note})
	})
	if err != nil {
		return err
	}
	if notify != nil && h.NotifyPersona != nil {
		h.NotifyPersona(*notify, model.NotifModNotice, model.JSONMap{"appeal": string(req.Decision)})
	}
	return h.listAppeals(c)
}
