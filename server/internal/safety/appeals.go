package safety

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/ratelimit"
	"metro-buddy/server/internal/secure"
	"metro-buddy/server/internal/web"
)

const AppealSLA = 15 * 24 * time.Hour

type Appeals struct {
	DB      *gorm.DB
	Box     *secure.Box
	Limiter *ratelimit.Limiter
	Now     func() time.Time
}

func (a *Appeals) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now().UTC()
}

// Registered on the authed group: suspended and persona-less users must still be able to appeal.
func (a *Appeals) Register(g *echo.Group) {
	g.GET("/me/moderation", a.myActions)
	g.POST("/appeals", a.create, a.Limiter.Middleware(ratelimit.Rule{Name: "appeals", Limit: 5, Window: 24 * time.Hour}))
	g.POST("/grievances", a.grievance, a.Limiter.Middleware(ratelimit.Rule{Name: "grievances", Limit: 5, Window: 24 * time.Hour}))
}

type NoticeDTO struct {
	ActionID     uuid.UUID           `json:"action_id"`
	Action       model.ModAction     `json:"action"`
	TargetType   model.ContentType   `json:"target_type"`
	Until        *time.Time          `json:"until"`
	CreatedAt    time.Time           `json:"created_at"`
	AppealStatus *model.AppealStatus `json:"appeal_status"`
	CanAppeal    bool                `json:"can_appeal"`
}

func (a *Appeals) myActions(c echo.Context) error {
	uid := httpx.PrincipalFrom(c).UserID
	tx := a.DB.WithContext(c.Request().Context())
	var rows []struct {
		model.ModerationAction
		TargetType model.ContentType
	}
	if err := tx.Table("moderation_actions").Select("moderation_actions.*, moderation_cases.target_type").
		Joins("JOIN moderation_cases ON moderation_cases.id = moderation_actions.case_id").
		Where("moderation_actions.target_user_id = ? AND moderation_actions.action <> ?", uid, model.ActRestore).
		Order("moderation_actions.created_at DESC").Limit(50).Scan(&rows).Error; err != nil {
		return err
	}
	out := make([]NoticeDTO, len(rows))
	for i, r := range rows {
		var ap model.Appeal
		has := tx.Where("action_id = ?", r.ID).Limit(1).Find(&ap).RowsAffected == 1
		d := NoticeDTO{ActionID: r.ID, Action: r.Action, TargetType: r.TargetType, Until: r.Until, CreatedAt: r.CreatedAt}
		if has {
			d.AppealStatus = &ap.Status
		}
		d.CanAppeal = !has && a.now().Sub(r.CreatedAt) < 30*24*time.Hour
		out[i] = d
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

type appealRequest struct {
	ActionID uuid.UUID `json:"action_id" validate:"required"`
	Text     string    `json:"text" validate:"required,min=10,max=1000"`
}

func (a *Appeals) create(c echo.Context) error {
	req, err := httpx.Bind[appealRequest](c)
	if err != nil {
		return err
	}
	uid := httpx.PrincipalFrom(c).UserID
	err = a.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var act model.ModerationAction
		if err := tx.First(&act, "id = ? AND target_user_id = ?", req.ActionID, uid).Error; err != nil {
			return httpx.NotFound()
		}
		var n int64
		if err := tx.Model(&model.Appeal{}).Where("action_id = ?", act.ID).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return httpx.Conflict("you already appealed this decision")
		}
		return tx.Create(&model.Appeal{ActionID: act.ID, UserID: uid, TextMasked: pii.Mask(req.Text, pii.Options{}).Text,
			Status: model.AppealOpen, SLADueAt: a.now().Add(AppealSLA)}).Error
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, map[string]any{"received": true, "decide_by": a.now().Add(AppealSLA)})
}

type grievanceRequest struct {
	Category string `json:"category" validate:"required,oneof=content privacy impersonation ncii other"`
	Body     string `json:"body" validate:"required,min=10,max=4000"`
}

func (a *Appeals) grievance(c echo.Context) error {
	req, err := httpx.Bind[grievanceRequest](c)
	if err != nil {
		return err
	}
	uid := httpx.PrincipalFrom(c).UserID
	t, err := web.CreateTicket(a.DB.WithContext(c.Request().Context()), a.Box, &uid, req.Category, req.Body, "", a.now())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, map[string]any{"ref": web.TicketRef(t.ID.String()), "ack_due_at": t.AckDueAt, "resolve_due_at": t.ResolveDueAt})
}
