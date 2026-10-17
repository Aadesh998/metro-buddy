package bowls

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/ratelimit"
)

// User-created bowls (F14). Creating one needs trust tier 2; the creator owns it and can
// appoint moderators. Bowl mods act locally; platform moderation still applies on top.
const (
	MinCreateTier   = 2
	MaxOwnedBowls   = 3
	MaxBowlMods     = 10
	maxBowlMuteDays = 30
)

var (
	ErrTierTooLow = httpx.Forbidden("you can create bowls once your account has been around a while and is in good standing")
	ErrNotMod     = httpx.Forbidden("only this bowl's moderators can do that")
	ErrLocked     = httpx.Forbidden("comments are closed on this post")
)

func (h *Handlers) registerManage(g *echo.Group) {
	g.POST("/bowls", h.createBowl, h.Limiter.Middleware(ratelimit.Rule{Name: "bowl_create", Limit: 3, Window: 24 * time.Hour}))
	g.PATCH("/bowls/:id", h.updateBowl)
	g.GET("/bowls/:id/moderators", h.listMods)
	g.POST("/bowls/:id/moderators", h.addMod)
	g.DELETE("/bowls/:id/moderators/:persona_id", h.removeMod)
	g.POST("/bowls/:id/mutes", h.muteMember)
	g.DELETE("/bowls/:id/mutes/:persona_id", h.unmuteMember)
	g.GET("/bowls/:id/modlog", h.modLog)
	g.POST("/posts/:id/mod", h.moderatePost)
	g.POST("/comments/:id/mod", h.moderateComment)
}

type createBowlRequest struct {
	Name        string     `json:"name" validate:"required,min=3,max=60"`
	Description string     `json:"description" validate:"max=300"`
	Kind        string     `json:"kind" validate:"required,oneof=topic office_hub college"`
	RulesMD     string     `json:"rules_md" validate:"max=2000"`
	Visibility  string     `json:"visibility" validate:"omitempty,oneof=public members_only"`
	StationID   *uuid.UUID `json:"station_id"`
}

func shortSlug(name string) string {
	s := slugify(name)
	if len(s) > 36 {
		s = strings.TrimRight(s[:36], "-")
	}
	return s
}

func (h *Handlers) createBowl(c echo.Context) error {
	req, err := httpx.Bind[createBowlRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	if p.TrustTier < MinCreateTier {
		return ErrTierTooLow
	}
	var b model.Bowl
	err = h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var owned int64
		tx.Model(&model.Bowl{}).Where("created_by_persona_id = ?", p.PersonaID).Count(&owned)
		if owned >= MaxOwnedBowls {
			return httpx.Conflict("you can run up to 3 bowls")
		}
		var per model.Persona
		if err := tx.Select("city_id").First(&per, "id = ?", p.PersonaID).Error; err != nil {
			return err
		}
		name := pii.Mask(req.Name, pii.Options{}).Text
		var dup int64
		tx.Model(&model.Bowl{}).Where("LOWER(name) = ? AND (city_id = ? OR city_id IS NULL)", strings.ToLower(name), per.CityID).Count(&dup)
		if dup > 0 {
			return httpx.Conflict("a bowl with that name already exists")
		}
		vis := model.BowlPublic
		if req.Visibility != "" {
			vis = model.BowlVisibility(req.Visibility)
		}
		b = model.Bowl{CityID: &per.CityID, Kind: model.BowlKind(req.Kind), Name: name, Description: pii.Mask(req.Description, pii.Options{}).Text,
			RulesMD: pii.Mask(req.RulesMD, pii.Options{}).Text, Visibility: vis, CreatedByPersonaID: &p.PersonaID, MemberCount: 1, StationID: req.StationID}
		b.ID = model.NewID()
		b.Slug = shortSlug(name) + "-" + b.ID.String()[len(b.ID.String())-6:]
		if req.StationID != nil {
			var n int64
			tx.Model(&model.Station{}).Where("id = ? AND city_id = ?", *req.StationID, per.CityID).Count(&n)
			if n == 0 {
				return httpx.Unprocessable("invalid_station", "pick a station in your city")
			}
		}
		if err := tx.Create(&b).Error; err != nil {
			return err
		}
		return tx.Create(&model.BowlMember{BowlID: b.ID, PersonaID: p.PersonaID, Role: model.BowlOwner, JoinedAt: h.now()}).Error
	})
	if err != nil {
		return err
	}
	out, err := h.dtos(h.DB.WithContext(c.Request().Context()), p.PersonaID, []model.Bowl{b}, true)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, out[0])
}

// requireMod loads the bowl and checks the caller's role. Owners pass owner-only checks.
func (h *Handlers) requireMod(tx *gorm.DB, bowlID, persona uuid.UUID, ownerOnly bool) (*model.Bowl, *model.BowlMember, error) {
	var b model.Bowl
	if err := tx.First(&b, "id = ?", bowlID).Error; err != nil {
		return nil, nil, err
	}
	m, err := membership(tx, b.ID, persona)
	if err != nil {
		return nil, nil, err
	}
	if m == nil || m.Role == model.BowlMemberRole || (ownerOnly && m.Role != model.BowlOwner) {
		return nil, nil, ErrNotMod
	}
	return &b, m, nil
}

func (h *Handlers) modAudit(tx *gorm.DB, actor uuid.UUID, verb string, bowl uuid.UUID, meta model.JSONMap) {
	if meta == nil {
		meta = model.JSONMap{}
	}
	meta["actor_persona_id"] = actor.String()
	tx.Create(&model.AuditLog{ActorType: model.ActorUser, Verb: "bowl." + verb, TargetType: "bowl", TargetID: &bowl, Meta: meta})
}

func paramID(c echo.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, httpx.NotFound()
	}
	return id, nil
}

type updateBowlRequest struct {
	Description  *string    `json:"description" validate:"omitempty,max=300"`
	RulesMD      *string    `json:"rules_md" validate:"omitempty,max=2000"`
	Visibility   *string    `json:"visibility" validate:"omitempty,oneof=public members_only"`
	PinnedPostID *uuid.UUID `json:"pinned_post_id"`
	Unpin        bool       `json:"unpin"`
}

func (h *Handlers) updateBowl(c echo.Context) error {
	req, err := httpx.Bind[updateBowlRequest](c)
	if err != nil {
		return err
	}
	id, err := paramID(c, "id")
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	tx := h.DB.WithContext(c.Request().Context())
	b, m, err := h.requireMod(tx, id, p.PersonaID, false)
	if err != nil {
		return err
	}
	updates := map[string]any{}
	if req.Description != nil {
		updates["description"] = pii.Mask(*req.Description, pii.Options{}).Text
	}
	if req.RulesMD != nil {
		updates["rules_md"] = pii.Mask(*req.RulesMD, pii.Options{}).Text
	}
	if req.Visibility != nil {
		if m.Role != model.BowlOwner {
			return ErrNotMod
		}
		updates["visibility"] = *req.Visibility
	}
	if req.PinnedPostID != nil {
		var n int64
		tx.Model(&model.Post{}).Where("id = ? AND bowl_id = ? AND status = ?", *req.PinnedPostID, b.ID, model.ContentVisible).Count(&n)
		if n == 0 {
			return httpx.NotFound()
		}
		updates["pinned_post_id"] = *req.PinnedPostID
	}
	if req.Unpin {
		updates["pinned_post_id"] = nil
	}
	if len(updates) > 0 {
		if err := tx.Model(b).Updates(updates).Error; err != nil {
			return err
		}
		keys := make([]string, 0, len(updates))
		for k := range updates {
			keys = append(keys, k)
		}
		verb := "update"
		if req.PinnedPostID != nil {
			verb = "pin"
		} else if req.Unpin {
			verb = "unpin"
		}
		h.modAudit(tx, p.PersonaID, verb, b.ID, model.JSONMap{"fields": keys})
	}
	tx.First(b, "id = ?", b.ID)
	out, err := h.dtos(tx, p.PersonaID, []model.Bowl{*b}, true)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out[0])
}

type ModDTO struct {
	Persona identity.PersonaRef `json:"persona"`
	Role    model.BowlRole      `json:"role"`
}

func (h *Handlers) listMods(c echo.Context) error {
	id, err := paramID(c, "id")
	if err != nil {
		return err
	}
	tx := h.DB.WithContext(c.Request().Context())
	var ms []model.BowlMember
	tx.Where("bowl_id = ? AND role IN ?", id, []model.BowlRole{model.BowlOwner, model.BowlModerator}).Order("joined_at").Find(&ms)
	ids := make([]uuid.UUID, len(ms))
	for i, m := range ms {
		ids[i] = m.PersonaID
	}
	refs, err := identity.LoadRefs(tx, ids)
	if err != nil {
		return err
	}
	out := make([]ModDTO, len(ms))
	for i, m := range ms {
		out[i] = ModDTO{Persona: refs[m.PersonaID], Role: m.Role}
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

type personaRequest struct {
	PersonaID uuid.UUID `json:"persona_id" validate:"required"`
}

func (h *Handlers) addMod(c echo.Context) error {
	req, err := httpx.Bind[personaRequest](c)
	if err != nil {
		return err
	}
	id, err := paramID(c, "id")
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	return h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		b, _, err := h.requireMod(tx, id, p.PersonaID, true)
		if err != nil {
			return err
		}
		var n int64
		tx.Model(&model.BowlMember{}).Where("bowl_id = ? AND role = ?", b.ID, model.BowlModerator).Count(&n)
		if n >= MaxBowlMods {
			return httpx.Conflict("a bowl can have up to 10 moderators")
		}
		res := tx.Model(&model.BowlMember{}).Where("bowl_id = ? AND persona_id = ? AND role = ?", b.ID, req.PersonaID, model.BowlMemberRole).Update("role", model.BowlModerator)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return httpx.Unprocessable("not_member", "only members of this bowl can be moderators")
		}
		h.modAudit(tx, p.PersonaID, "add_mod", b.ID, model.JSONMap{"persona_id": req.PersonaID.String()})
		return c.NoContent(http.StatusNoContent)
	})
}

func (h *Handlers) removeMod(c echo.Context) error {
	id, err := paramID(c, "id")
	if err != nil {
		return err
	}
	target, err := paramID(c, "persona_id")
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	tx := h.DB.WithContext(c.Request().Context())
	// Owners remove any mod; a mod can step down.
	b, _, err := h.requireMod(tx, id, p.PersonaID, target != p.PersonaID)
	if err != nil {
		return err
	}
	tx.Model(&model.BowlMember{}).Where("bowl_id = ? AND persona_id = ? AND role = ?", b.ID, target, model.BowlModerator).Update("role", model.BowlMemberRole)
	h.modAudit(tx, p.PersonaID, "remove_mod", b.ID, model.JSONMap{"persona_id": target.String()})
	return c.NoContent(http.StatusNoContent)
}

type muteRequest struct {
	PersonaID uuid.UUID `json:"persona_id" validate:"required"`
	Days      int       `json:"days" validate:"required,min=1,max=30"`
	Reason    string    `json:"reason" validate:"max=200"`
}

func (h *Handlers) muteMember(c echo.Context) error {
	req, err := httpx.Bind[muteRequest](c)
	if err != nil {
		return err
	}
	id, err := paramID(c, "id")
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	tx := h.DB.WithContext(c.Request().Context())
	b, _, err := h.requireMod(tx, id, p.PersonaID, false)
	if err != nil {
		return err
	}
	until := h.now().Add(time.Duration(req.Days) * 24 * time.Hour)
	res := tx.Model(&model.BowlMember{}).Where("bowl_id = ? AND persona_id = ? AND role = ?", b.ID, req.PersonaID, model.BowlMemberRole).Update("muted_until", until)
	if res.RowsAffected == 0 {
		return httpx.Unprocessable("not_member", "you can only mute regular members")
	}
	h.modAudit(tx, p.PersonaID, "mute", b.ID, model.JSONMap{"persona_id": req.PersonaID.String(), "days": req.Days, "reason": pii.Mask(req.Reason, pii.Options{}).Text})
	if h.OnBowlNotice != nil {
		h.OnBowlNotice(c.Request().Context(), req.PersonaID, b.Name, "You’ve been muted in "+b.Name+" for "+strconv.Itoa(req.Days)+" days by its moderators.")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handlers) unmuteMember(c echo.Context) error {
	id, err := paramID(c, "id")
	if err != nil {
		return err
	}
	target, err := paramID(c, "persona_id")
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	tx := h.DB.WithContext(c.Request().Context())
	b, _, err := h.requireMod(tx, id, p.PersonaID, false)
	if err != nil {
		return err
	}
	tx.Model(&model.BowlMember{}).Where("bowl_id = ? AND persona_id = ?", b.ID, target).Update("muted_until", nil)
	h.modAudit(tx, p.PersonaID, "unmute", b.ID, model.JSONMap{"persona_id": target.String()})
	return c.NoContent(http.StatusNoContent)
}

type modRequest struct {
	Action string `json:"action" validate:"required,oneof=remove restore lock unlock"`
	Reason string `json:"reason" validate:"max=200"`
}

func (h *Handlers) moderatePost(c echo.Context) error {
	req, err := httpx.Bind[modRequest](c)
	if err != nil {
		return err
	}
	id, err := paramID(c, "id")
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	var post model.Post
	err = h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&post, "id = ?", id).Error; err != nil {
			return err
		}
		b, _, err := h.requireMod(tx, post.BowlID, p.PersonaID, false)
		if err != nil {
			return err
		}
		switch req.Action {
		case "remove":
			// Bowl mods can't undo a platform removal, and can't resurrect author deletions.
			if post.Status != model.ContentVisible {
				return httpx.Conflict("this post isn't visible")
			}
			post.Status = model.ContentRemovedByMod
			if b.PinnedPostID != nil && *b.PinnedPostID == post.ID {
				tx.Model(b).Update("pinned_post_id", nil)
			}
		case "restore":
			var platform int64
			tx.Model(&model.ModerationAction{}).Joins("JOIN moderation_cases ON moderation_cases.id = moderation_actions.case_id").
				Where("moderation_cases.target_type = ? AND moderation_cases.target_id = ? AND moderation_actions.action = ?", model.CTPost, post.ID, model.ActRemoveContent).Count(&platform)
			if post.Status != model.ContentRemovedByMod || platform > 0 {
				return httpx.Conflict("this post can't be restored here")
			}
			post.Status = model.ContentVisible
		case "lock":
			now := h.now()
			post.LockedAt = &now
		case "unlock":
			post.LockedAt = nil
		}
		if err := tx.Model(&post).Select("status", "locked_at").Updates(&post).Error; err != nil {
			return err
		}
		h.modAudit(tx, p.PersonaID, "post_"+req.Action, b.ID, model.JSONMap{"post_id": post.ID.String(), "reason": pii.Mask(req.Reason, pii.Options{}).Text})
		return nil
	})
	if err != nil {
		return err
	}
	if req.Action == "remove" && h.OnBowlNotice != nil {
		h.OnBowlNotice(c.Request().Context(), post.AuthorPersonaID, "", "A moderator removed your post “"+post.Title+"”.")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handlers) moderateComment(c echo.Context) error {
	req, err := httpx.Bind[modRequest](c)
	if err != nil {
		return err
	}
	if req.Action != "remove" && req.Action != "restore" {
		return httpx.Unprocessable("invalid_action", "comments can be removed or restored")
	}
	id, err := paramID(c, "id")
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	return h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var cm model.Comment
		if err := tx.First(&cm, "id = ?", id).Error; err != nil {
			return err
		}
		var post model.Post
		if err := tx.Unscoped().First(&post, "id = ?", cm.PostID).Error; err != nil {
			return err
		}
		b, _, err := h.requireMod(tx, post.BowlID, p.PersonaID, false)
		if err != nil {
			return err
		}
		from, to := model.ContentVisible, model.ContentRemovedByMod
		if req.Action == "restore" {
			from, to = to, from
		}
		res := tx.Model(&cm).Where("status = ?", from).Update("status", to)
		if res.RowsAffected == 0 {
			return httpx.Conflict("nothing to change")
		}
		h.modAudit(tx, p.PersonaID, "comment_"+req.Action, b.ID, model.JSONMap{"comment_id": cm.ID.String()})
		return c.NoContent(http.StatusNoContent)
	})
}

type ModLogDTO struct {
	Verb      string              `json:"verb"`
	Actor     identity.PersonaRef `json:"actor"`
	Meta      model.JSONMap       `json:"meta"`
	CreatedAt time.Time           `json:"created_at"`
}

// modLog is visible to the bowl's mods so they can keep each other honest.
func (h *Handlers) modLog(c echo.Context) error {
	id, err := paramID(c, "id")
	if err != nil {
		return err
	}
	tx := h.DB.WithContext(c.Request().Context())
	if _, _, err := h.requireMod(tx, id, httpx.PrincipalFrom(c).PersonaID, false); err != nil {
		return err
	}
	var rows []model.AuditLog
	tx.Where("target_type = ? AND target_id = ? AND verb LIKE ?", "bowl", id, "bowl.%").Order("created_at DESC").Limit(100).Find(&rows)
	var actors []uuid.UUID
	for _, r := range rows {
		if a, err := uuid.Parse(asString(r.Meta["actor_persona_id"])); err == nil {
			actors = append(actors, a)
		}
	}
	refs, err := identity.LoadRefs(tx, actors)
	if err != nil {
		return err
	}
	out := make([]ModLogDTO, len(rows))
	for i, r := range rows {
		a, _ := uuid.Parse(asString(r.Meta["actor_persona_id"]))
		meta := model.JSONMap{}
		for k, v := range r.Meta {
			if k != "actor_persona_id" {
				meta[k] = v
			}
		}
		out[i] = ModLogDTO{Verb: strings.TrimPrefix(r.Verb, "bowl."), Actor: refs[a], Meta: meta, CreatedAt: r.CreatedAt}
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
