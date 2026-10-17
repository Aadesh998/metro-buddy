package admin

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/trust"
)

func (h *Handlers) registerModeration(g *echo.Group, lead echo.MiddlewareFunc) {
	g.GET("/cases", h.listCases)
	g.GET("/cases/:id", h.getCase)
	g.POST("/cases/:id/claim", h.claimCase)
	g.POST("/cases/:id/actions", h.actOnCase)
	g.POST("/cases/:id/triage", h.retriage)
	g.POST("/cases/:id/dismiss", h.dismissCase)
	g.POST("/cases/:id/escalate", h.escalateCase)
	g.GET("/appeals", h.listAppeals)
	g.POST("/appeals/:id/decide", h.decideAppeal)
	g.GET("/users/:persona_id", h.lookupPersona, lead)
}

type CaseDTO struct {
	ID              uuid.UUID         `json:"id"`
	TargetType      model.ContentType `json:"target_type"`
	TargetID        uuid.UUID         `json:"target_id"`
	TargetPersonaID *uuid.UUID        `json:"target_persona_id"`
	Severity        model.Severity    `json:"severity"`
	AISeverity      *model.Severity   `json:"ai_severity"`
	AIRationale     *string           `json:"ai_rationale"`
	Status          model.CaseStatus  `json:"status"`
	Source          string            `json:"source"`
	ReportCount     int               `json:"report_count"`
	AssigneeEmail   *string           `json:"assignee_email"`
	SLADueAt        time.Time         `json:"sla_due_at"`
	Overdue         bool              `json:"overdue"`
	CreatedAt       time.Time         `json:"created_at"`
	TopReason       string            `json:"top_reason"`
}

// Most severe first, then the case closest to (or furthest past) its SLA.
const severityOrder = `CASE severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END`

func (h *Handlers) caseDTOs(tx *gorm.DB, cases []model.ModerationCase) ([]CaseDTO, error) {
	ids := make([]uuid.UUID, 0, len(cases))
	var admins []uuid.UUID
	for _, c := range cases {
		ids = append(ids, c.ID)
		if c.AssigneeAdminID != nil {
			admins = append(admins, *c.AssigneeAdminID)
		}
	}
	emails := map[uuid.UUID]string{}
	if len(admins) > 0 {
		var as []model.AdminUser
		if err := tx.Where("id IN ?", admins).Find(&as).Error; err != nil {
			return nil, err
		}
		for _, a := range as {
			emails[a.ID] = a.Email
		}
	}
	reasons := map[uuid.UUID]string{}
	if len(ids) > 0 {
		var rows []struct {
			CaseID uuid.UUID
			Reason string
			N      int
		}
		if err := tx.Model(&model.Report{}).Select("case_id, reason, COUNT(*) AS n").Where("case_id IN ?", ids).
			Group("case_id, reason").Order("n DESC").Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			if _, ok := reasons[r.CaseID]; !ok {
				reasons[r.CaseID] = r.Reason
			}
		}
	}
	now := h.now()
	out := make([]CaseDTO, len(cases))
	for i, c := range cases {
		d := CaseDTO{ID: c.ID, TargetType: c.TargetType, TargetID: c.TargetID, TargetPersonaID: c.TargetPersonaID, Severity: c.Severity,
			AISeverity: c.AISeverity, AIRationale: c.AIRationale, Status: c.Status, Source: c.Source, ReportCount: c.ReportCount,
			SLADueAt: c.SLADueAt, CreatedAt: c.CreatedAt, TopReason: reasons[c.ID],
			Overdue: now.After(c.SLADueAt) && (c.Status == model.CaseOpen || c.Status == model.CaseInReview || c.Status == model.CaseEscalated)}
		if c.AssigneeAdminID != nil {
			e := emails[*c.AssigneeAdminID]
			d.AssigneeEmail = &e
		}
		out[i] = d
	}
	return out, nil
}

func (h *Handlers) listCases(c echo.Context) error {
	status := c.QueryParam("status")
	tx := h.DB.WithContext(c.Request().Context())
	q := tx.Model(&model.ModerationCase{})
	switch status {
	case "", "active":
		q = q.Where("status IN ?", []model.CaseStatus{model.CaseOpen, model.CaseInReview, model.CaseEscalated})
	case "closed":
		q = q.Where("status IN ?", []model.CaseStatus{model.CaseActioned, model.CaseDismissed})
	default:
		q = q.Where("status = ?", status)
	}
	if c.QueryParam("mine") == "1" {
		q = q.Where("assignee_admin_id = ?", PrincipalFrom(c).ID)
	}
	var cases []model.ModerationCase
	if err := q.Order(severityOrder).Order("sla_due_at").Limit(200).Find(&cases).Error; err != nil {
		return err
	}
	out, err := h.caseDTOs(tx, cases)
	if err != nil {
		return err
	}
	var counts struct{ Open, Overdue int64 }
	tx.Model(&model.ModerationCase{}).Where("status IN ?", []model.CaseStatus{model.CaseOpen, model.CaseInReview, model.CaseEscalated}).Count(&counts.Open)
	tx.Model(&model.ModerationCase{}).Where("status IN ? AND sla_due_at < ?", []model.CaseStatus{model.CaseOpen, model.CaseInReview, model.CaseEscalated}, h.now()).Count(&counts.Overdue)
	return c.JSON(http.StatusOK, map[string]any{"items": out, "open": counts.Open, "overdue": counts.Overdue})
}

type ReportDTO struct {
	Reason    string    `json:"reason"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}

type TargetPreview struct {
	Kind    model.ContentType `json:"kind"`
	Title   string            `json:"title,omitempty"`
	Body    string            `json:"body"`
	Context string            `json:"context,omitempty"`
	Status  string            `json:"status,omitempty"`
	Media   []string          `json:"media,omitempty"`
}

type PersonaSummary struct {
	ID            uuid.UUID        `json:"id"`
	Alias         string           `json:"alias"`
	Bio           string           `json:"bio"`
	UserStatus    model.UserStatus `json:"user_status"`
	AccountAgeDay int              `json:"account_age_days"`
	TrustScore    int              `json:"trust_score"`
	TrustTier     int16            `json:"trust_tier"`
	Verified      bool             `json:"verified"`
	PriorActions  int64            `json:"prior_actions"`
	OpenCases     int64            `json:"open_cases"`
	IsHost        bool             `json:"is_host"`
}

type ActionDTO struct {
	ID         uuid.UUID       `json:"id"`
	Action     model.ModAction `json:"action"`
	AdminEmail string          `json:"admin_email"`
	Until      *time.Time      `json:"until"`
	Note       string          `json:"note"`
	CreatedAt  time.Time       `json:"created_at"`
}

func (h *Handlers) getCase(c echo.Context) error {
	tx := h.DB.WithContext(c.Request().Context())
	mc, err := h.loadCase(tx, c.Param("id"))
	if err != nil {
		return err
	}
	dtos, err := h.caseDTOs(tx, []model.ModerationCase{*mc})
	if err != nil {
		return err
	}
	var reports []model.Report
	if err := tx.Where("case_id = ?", mc.ID).Order("created_at").Limit(100).Find(&reports).Error; err != nil {
		return err
	}
	rs := make([]ReportDTO, len(reports))
	for i, r := range reports {
		rs[i] = ReportDTO{Reason: r.Reason, Details: r.DetailsMasked, CreatedAt: r.CreatedAt}
	}
	preview, err := h.preview(tx, mc.TargetType, mc.TargetID)
	if err != nil {
		return err
	}
	var persona *PersonaSummary
	if mc.TargetPersonaID != nil {
		if persona, err = h.personaSummary(tx, *mc.TargetPersonaID); err != nil {
			return err
		}
	}
	var acts []model.ModerationAction
	if err := tx.Where("case_id = ?", mc.ID).Order("created_at").Find(&acts).Error; err != nil {
		return err
	}
	actions := make([]ActionDTO, len(acts))
	for i, a := range acts {
		var ad model.AdminUser
		tx.Select("email").First(&ad, "id = ?", a.AdminID)
		actions[i] = ActionDTO{ID: a.ID, Action: a.Action, AdminEmail: ad.Email, Until: a.Until, Note: a.Note, CreatedAt: a.CreatedAt}
	}
	actor := PrincipalFrom(c).ID
	h.Audit.Log(tx, model.ActorAdmin, &actor, "case.view", "moderation_case", &mc.ID, nil)
	return c.JSON(http.StatusOK, map[string]any{"case": dtos[0], "reports": rs, "target": preview, "persona": persona, "actions": actions})
}

func (h *Handlers) loadCase(tx *gorm.DB, idParam string) (*model.ModerationCase, error) {
	id, err := uuid.Parse(idParam)
	if err != nil {
		return nil, httpx.NotFound()
	}
	var mc model.ModerationCase
	if err := tx.First(&mc, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &mc, nil
}

// preview shows moderators only the reported item, already masked, never surrounding private chat.
func (h *Handlers) preview(tx *gorm.DB, ct model.ContentType, id uuid.UUID) (*TargetPreview, error) {
	p := &TargetPreview{Kind: ct}
	switch ct {
	case model.CTPost:
		var post model.Post
		if err := tx.Unscoped().First(&post, "id = ?", id).Error; err != nil {
			return p, nil
		}
		var b model.Bowl
		tx.Unscoped().Select("name").First(&b, "id = ?", post.BowlID)
		p.Title, p.Body, p.Status, p.Context = post.Title, post.BodyMasked, string(post.Status), "in "+b.Name
		for _, m := range post.Media {
			p.Media = append(p.Media, m.Key)
		}
	case model.CTComment:
		var cm model.Comment
		if err := tx.Unscoped().First(&cm, "id = ?", id).Error; err != nil {
			return p, nil
		}
		var post model.Post
		tx.Unscoped().Select("title").First(&post, "id = ?", cm.PostID)
		p.Body, p.Status, p.Context = cm.BodyMasked, string(cm.Status), "comment on: "+post.Title
	case model.CTMessage:
		var m model.Message
		if err := tx.First(&m, "id = ?", id).Error; err != nil {
			return p, nil
		}
		var conv model.Conversation
		tx.Unscoped().Select("kind").First(&conv, "id = ?", m.ConversationID)
		p.Body, p.Status, p.Context = m.BodyMasked, string(m.Status), string(conv.Kind)+" chat message"
	case model.CTPersona:
		var per model.Persona
		if err := tx.Unscoped().First(&per, "id = ?", id).Error; err != nil {
			return p, nil
		}
		p.Title, p.Body = per.Alias, per.Bio
	case model.CTCallIn:
		var ci model.CallInRequest
		if err := tx.First(&ci, "id = ?", id).Error; err != nil {
			return p, nil
		}
		p.Body, p.Status, p.Context = ci.TopicMasked, string(ci.Status), "studio call-in topic"
	case model.CTEpisode:
		var ep model.ShowEpisode
		if err := tx.First(&ep, "id = ?", id).Error; err != nil {
			return p, nil
		}
		p.Title, p.Status, p.Context = ep.Title, string(ep.Status), "live studio episode"
	case model.CTTrip:
		p.Context = "travel buddy trip (see safety flags)"
		var fb []model.TripFeedback
		tx.Where("trip_id = ? AND safety_flag = ?", id, true).Find(&fb)
		for _, f := range fb {
			p.Body += f.NoteMasked + "\n"
		}
	}
	return p, nil
}

func (h *Handlers) personaSummary(tx *gorm.DB, pid uuid.UUID) (*PersonaSummary, error) {
	var per model.Persona
	if err := tx.Unscoped().First(&per, "id = ?", pid).Error; err != nil {
		return nil, nil
	}
	var u model.User
	if err := tx.Unscoped().First(&u, "id = ?", per.UserID).Error; err != nil {
		return nil, err
	}
	s := &PersonaSummary{ID: per.ID, Alias: per.Alias, Bio: per.Bio, UserStatus: u.Status, TrustScore: u.TrustScore, TrustTier: u.TrustTier,
		Verified: u.VerifiedAt != nil, IsHost: per.IsHost, AccountAgeDay: int(h.now().Sub(u.CreatedAt).Hours() / 24)}
	tx.Model(&model.ModerationAction{}).Where("target_user_id = ? AND action <> ?", u.ID, model.ActRestore).Count(&s.PriorActions)
	tx.Model(&model.ModerationCase{}).Where("target_persona_id = ? AND status IN ?", pid,
		[]model.CaseStatus{model.CaseOpen, model.CaseInReview, model.CaseEscalated}).Count(&s.OpenCases)
	return s, nil
}

func (h *Handlers) claimCase(c echo.Context) error {
	tx := h.DB.WithContext(c.Request().Context())
	mc, err := h.loadCase(tx, c.Param("id"))
	if err != nil {
		return err
	}
	actor := PrincipalFrom(c).ID
	updates := map[string]any{"assignee_admin_id": actor}
	if mc.Status == model.CaseOpen {
		updates["status"] = model.CaseInReview
	}
	if err := tx.Model(mc).Updates(updates).Error; err != nil {
		return err
	}
	h.Audit.Log(tx, model.ActorAdmin, &actor, "case.claim", "moderation_case", &mc.ID, nil)
	return h.getCase(c)
}

type actionRequest struct {
	Action    model.ModAction `json:"action" validate:"required,oneof=remove_content warn mute_24h suspend ban restore revoke_host"`
	Note      string          `json:"note" validate:"required,min=3,max=2000"`
	UntilDays int             `json:"until_days" validate:"omitempty,min=1,max=365"`
}

var ErrCaseClosed = httpx.Conflict("this case is already closed")

// actOnCase applies a moderator decision. The action row, audit row, and side effects commit together.
func (h *Handlers) actOnCase(c echo.Context) error {
	req, err := httpx.Bind[actionRequest](c)
	if err != nil {
		return err
	}
	actor := PrincipalFrom(c)
	if req.Action == model.ActBan && model.AdminRoleRank[actor.Role] < model.AdminRoleRank[model.AdminTSLead] {
		return httpx.Forbidden("only trust & safety leads can ban")
	}
	now := h.now()
	var notify *uuid.UUID
	var actionID uuid.UUID
	err = h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		mc, err := h.loadCase(tx, c.Param("id"))
		if err != nil {
			return err
		}
		if (mc.Status == model.CaseActioned || mc.Status == model.CaseDismissed) && req.Action != model.ActRestore {
			return ErrCaseClosed
		}
		var userID *uuid.UUID
		if mc.TargetPersonaID != nil {
			var per model.Persona
			if err := tx.Unscoped().Select("user_id").First(&per, "id = ?", *mc.TargetPersonaID).Error; err == nil {
				userID = &per.UserID
			}
		}
		act := model.ModerationAction{CaseID: mc.ID, AdminID: actor.ID, Action: req.Action, TargetUserID: userID, Note: req.Note}
		if err := h.applyAction(tx, mc, &act, userID, req.UntilDays, now); err != nil {
			return err
		}
		if err := tx.Create(&act).Error; err != nil {
			return err
		}
		actionID = act.ID
		status := model.CaseActioned
		if req.Action == model.ActRestore {
			status = model.CaseDismissed
		}
		if err := tx.Model(mc).Updates(map[string]any{"status": status, "assignee_admin_id": actor.ID}).Error; err != nil {
			return err
		}
		if userID != nil && req.Action != model.ActRestore {
			if err := trust.Record(tx, *userID, model.TrustModAction, actionDelta[req.Action], &act.ID); err != nil {
				return err
			}
		}
		if err := h.creditReporters(tx, mc, req.Action != model.ActRestore); err != nil {
			return err
		}
		notify = mc.TargetPersonaID
		return h.Audit.Log(tx, model.ActorAdmin, &actor.ID, "case.action."+string(req.Action), string(mc.TargetType), &mc.TargetID,
			model.JSONMap{"case_id": mc.ID.String(), "action_id": act.ID.String()})
	})
	if err != nil {
		return err
	}
	if notify != nil && h.NotifyPersona != nil && req.Action != model.ActRestore {
		h.NotifyPersona(*notify, model.NotifModNotice, model.JSONMap{"action": string(req.Action), "action_id": actionID.String(), "appealable": true})
	}
	return h.getCase(c)
}

var actionDelta = map[model.ModAction]int16{
	model.ActRemoveContent: -5, model.ActWarn: -3, model.ActMute24h: -8, model.ActSuspend: -15, model.ActBan: -50, model.ActRevokeHost: -10,
}

func (h *Handlers) applyAction(tx *gorm.DB, mc *model.ModerationCase, act *model.ModerationAction, userID *uuid.UUID, days int, now time.Time) error {
	setContent := func(status model.ContentStatus) error {
		tables := map[model.ContentType]string{model.CTPost: "posts", model.CTComment: "comments", model.CTMessage: "messages", model.CTCallIn: "call_in_requests"}
		if t, ok := tables[mc.TargetType]; ok {
			col := "status"
			if mc.TargetType == model.CTCallIn {
				if status == model.ContentVisible {
					return nil
				}
				return tx.Table(t).Where("id = ?", mc.TargetID).Update(col, model.CallInDropped).Error
			}
			return tx.Table(t).Where("id = ?", mc.TargetID).Update(col, status).Error
		}
		if mc.TargetType == model.CTPersona && status == model.ContentRemovedByMod {
			return tx.Model(&model.Persona{}).Where("id = ?", mc.TargetID).Update("bio", "").Error
		}
		return nil
	}
	setUser := func(status model.UserStatus, until *time.Time) error {
		if userID == nil {
			return httpx.Unprocessable("no_target_user", "this case has no account to act on")
		}
		if err := tx.Model(&model.User{}).Where("id = ?", *userID).Updates(map[string]any{"status": status, "suspended_until": until}).Error; err != nil {
			return err
		}
		if status == model.UserBanned && h.RevokeSessions != nil {
			return h.RevokeSessions(tx, *userID)
		}
		return nil
	}
	switch act.Action {
	case model.ActRemoveContent:
		return setContent(model.ContentRemovedByMod)
	case model.ActWarn:
		return nil
	case model.ActMute24h:
		until := now.Add(24 * time.Hour)
		act.Until = &until
		return setUser(model.UserSuspended, &until)
	case model.ActSuspend:
		if days == 0 {
			days = 7
		}
		until := now.Add(time.Duration(days) * 24 * time.Hour)
		act.Until = &until
		if err := setContent(model.ContentRemovedByMod); err != nil {
			return err
		}
		return setUser(model.UserSuspended, &until)
	case model.ActBan:
		if err := setContent(model.ContentRemovedByMod); err != nil {
			return err
		}
		return setUser(model.UserBanned, nil)
	case model.ActRevokeHost:
		if mc.TargetPersonaID == nil {
			return httpx.Unprocessable("no_target_user", "no host to revoke")
		}
		if err := tx.Model(&model.HostProfile{}).Where("persona_id = ?", *mc.TargetPersonaID).Update("status", model.HostSuspended).Error; err != nil {
			return err
		}
		return tx.Model(&model.Persona{}).Where("id = ?", *mc.TargetPersonaID).Update("is_host", false).Error
	case model.ActRestore:
		if err := setContent(model.ContentVisible); err != nil {
			return err
		}
		if userID != nil {
			return tx.Model(&model.User{}).Where("id = ? AND status IN ?", *userID, []model.UserStatus{model.UserSuspended, model.UserBanned}).
				Updates(map[string]any{"status": model.UserActive, "suspended_until": nil}).Error
		}
	}
	return nil
}

// creditReporters records whether a reporter's report was upheld; trust scoring reads these.
func (h *Handlers) creditReporters(tx *gorm.DB, mc *model.ModerationCase, upheld bool) error {
	var reporters []uuid.UUID
	if err := tx.Model(&model.Report{}).Where("case_id = ?", mc.ID).Distinct().Pluck("reporter_persona_id", &reporters).Error; err != nil {
		return err
	}
	kind, delta := model.TrustReportUpheld, int16(1)
	if !upheld {
		kind, delta = model.TrustReportDismissed, 0
	}
	for _, rp := range reporters {
		var per model.Persona
		if err := tx.Unscoped().Select("user_id").First(&per, "id = ?", rp).Error; err != nil {
			continue
		}
		if err := trust.Record(tx, per.UserID, kind, delta, &mc.ID); err != nil {
			return err
		}
	}
	return nil
}

type noteRequest struct {
	Note string `json:"note" validate:"required,min=3,max=2000"`
}

func (h *Handlers) dismissCase(c echo.Context) error {
	return h.closeCase(c, model.CaseDismissed, "case.dismiss")
}

func (h *Handlers) escalateCase(c echo.Context) error {
	return h.closeCase(c, model.CaseEscalated, "case.escalate")
}

func (h *Handlers) closeCase(c echo.Context, status model.CaseStatus, verb string) error {
	req, err := httpx.Bind[noteRequest](c)
	if err != nil {
		return err
	}
	actor := PrincipalFrom(c).ID
	err = h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		mc, err := h.loadCase(tx, c.Param("id"))
		if err != nil {
			return err
		}
		if mc.Status == model.CaseActioned || mc.Status == model.CaseDismissed {
			return ErrCaseClosed
		}
		updates := map[string]any{"status": status}
		if status == model.CaseEscalated {
			// Escalations jump to the front of the lead queue.
			updates["severity"] = model.SevCritical
			updates["assignee_admin_id"] = nil
		}
		if err := tx.Model(mc).Updates(updates).Error; err != nil {
			return err
		}
		if status == model.CaseDismissed {
			if err := h.creditReporters(tx, mc, false); err != nil {
				return err
			}
		}
		return h.Audit.Log(tx, model.ActorAdmin, &actor, verb, "moderation_case", &mc.ID, model.JSONMap{"note": req.Note})
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return httpx.NotFound()
		}
		return err
	}
	return h.getCase(c)
}

func (h *Handlers) lookupPersona(c echo.Context) error {
	pid, err := uuid.Parse(c.Param("persona_id"))
	if err != nil {
		return httpx.NotFound()
	}
	tx := h.DB.WithContext(c.Request().Context())
	s, err := h.personaSummary(tx, pid)
	if err != nil || s == nil {
		return httpx.NotFound()
	}
	actor := PrincipalFrom(c).ID
	h.Audit.Log(tx, model.ActorAdmin, &actor, "user.lookup", "persona", &pid, nil)
	var cases []model.ModerationCase
	tx.Where("target_persona_id = ?", pid).Order("created_at DESC").Limit(50).Find(&cases)
	dtos, err := h.caseDTOs(tx, cases)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"persona": s, "cases": dtos})
}

// retriage queues a fresh AI suggestion, e.g. after more reports came in.
func (h *Handlers) retriage(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	if h.Triage == nil {
		return httpx.E(http.StatusServiceUnavailable, "ai_disabled", "AI triage isn't configured")
	}
	h.Triage(id)
	return c.NoContent(http.StatusAccepted)
}
