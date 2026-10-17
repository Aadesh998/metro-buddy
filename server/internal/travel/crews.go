package travel

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/chat"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/notify"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/ratelimit"
	"metro-buddy/server/internal/safety"
)

// Commute crews (F46): a small standing group of people who already matched and rode together,
// with a shared plan (line, time, days) and a group chat. You can only bring in someone you
// have an accepted match with, and they must say yes.
const MaxCrew = 6

var (
	ErrCrewFull     = httpx.Conflict("crews are capped at 6 people")
	ErrNotCrew      = httpx.Forbidden("you're not in this crew")
	ErrNeedMatch    = httpx.Forbidden("you can only invite someone you've matched and travelled with")
	crewReminderLed = 20 * time.Minute
)

func (s *Service) registerCrews(g *echo.Group) {
	g.GET("/crews", s.myCrews)
	g.POST("/crews", s.createCrew, s.Limiter.Middleware(ratelimit.Rule{Name: "crews", Limit: 3, Window: 24 * time.Hour}))
	g.GET("/crews/:id", s.getCrew)
	g.PATCH("/crews/:id", s.updateCrew)
	g.POST("/crews/:id/invites", s.inviteToCrew, s.Limiter.Middleware(ratelimit.Rule{Name: "crew_invites", Limit: 10, Window: 24 * time.Hour}))
	g.POST("/crews/:id/leave", s.leaveCrew)
	g.GET("/crew-invites", s.crewInvites)
	g.POST("/crew-invites/:id/accept", s.acceptCrewInvite)
	g.POST("/crew-invites/:id/decline", s.declineCrewInvite)
}

type CrewDTO struct {
	ID             uuid.UUID             `json:"id"`
	Name           string                `json:"name"`
	Line           LineDTO               `json:"line"`
	DepartAt       string                `json:"depart_at"`
	Weekdays       []int                 `json:"weekdays"`
	Members        []identity.PersonaRef `json:"members"`
	PendingInvites int64                 `json:"pending_invites"`
	ConversationID *uuid.UUID            `json:"conversation_id"`
	IsCreator      bool                  `json:"is_creator"`
}

type LineDTO struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Color string    `json:"color"`
}

func crewMembers(tx *gorm.DB, convID uuid.UUID) []uuid.UUID {
	var ids []uuid.UUID
	tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND left_at IS NULL", convID).Order("joined_at").Pluck("persona_id", &ids)
	return ids
}

func (s *Service) crewConv(tx *gorm.DB, crewID uuid.UUID) (*model.Conversation, error) {
	var conv model.Conversation
	if err := tx.First(&conv, "crew_id = ?", crewID).Error; err != nil {
		return nil, err
	}
	return &conv, nil
}

func (s *Service) crewDTO(tx *gorm.DB, viewer uuid.UUID, c *model.Crew) (*CrewDTO, error) {
	var line model.MetroLine
	tx.First(&line, "id = ?", c.LineID)
	d := &CrewDTO{ID: c.ID, Name: c.Name, Line: LineDTO{line.ID, line.Name, line.ColorHex}, DepartAt: clock(c.DepartWindowStart),
		Weekdays: weekdays(c.Weekdays), IsCreator: c.CreatedByPersonaID == viewer, Members: []identity.PersonaRef{}}
	if conv, err := s.crewConv(tx, c.ID); err == nil {
		d.ConversationID = &conv.ID
		ids := crewMembers(tx, conv.ID)
		refs, err := identity.LoadRefs(tx, ids)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			d.Members = append(d.Members, refs[id])
		}
	}
	tx.Model(&model.CrewInvite{}).Where("crew_id = ? AND status = ?", c.ID, model.RequestPending).Count(&d.PendingInvites)
	return d, nil
}

// loadMyCrew returns the crew and its chat if the viewer is a current member.
func (s *Service) loadMyCrew(tx *gorm.DB, idParam string, viewer uuid.UUID) (*model.Crew, *model.Conversation, error) {
	id, err := uuid.Parse(idParam)
	if err != nil {
		return nil, nil, httpx.NotFound()
	}
	var c model.Crew
	if err := tx.First(&c, "id = ?", id).Error; err != nil {
		return nil, nil, err
	}
	conv, err := s.crewConv(tx, c.ID)
	if err != nil {
		return nil, nil, err
	}
	var n int64
	tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND persona_id = ? AND left_at IS NULL", conv.ID, viewer).Count(&n)
	if n == 0 {
		return nil, nil, ErrNotCrew
	}
	return &c, conv, nil
}

func (s *Service) myCrews(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	var crews []model.Crew
	tx.Where("id IN (SELECT crew_id FROM conversations WHERE crew_id IS NOT NULL AND id IN (SELECT conversation_id FROM conversation_members WHERE persona_id = ? AND left_at IS NULL))", p.PersonaID).
		Order("created_at").Find(&crews)
	out := make([]*CrewDTO, 0, len(crews))
	for i := range crews {
		d, err := s.crewDTO(tx, p.PersonaID, &crews[i])
		if err != nil {
			return err
		}
		out = append(out, d)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (s *Service) getCrew(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	cr, _, err := s.loadMyCrew(tx, c.Param("id"), p.PersonaID)
	if err != nil {
		return err
	}
	d, err := s.crewDTO(tx, p.PersonaID, cr)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, d)
}

type createCrewRequest struct {
	MatchID uuid.UUID `json:"match_id" validate:"required"`
	Name    string    `json:"name" validate:"required,min=3,max=40"`
}

// sharedLine finds the line both stations of the overlap sit on.
func sharedLine(tx *gorm.DB, a, b uuid.UUID) (uuid.UUID, error) {
	var ids []uuid.UUID
	err := tx.Model(&model.LineStation{}).Where("station_id = ? AND line_id IN (SELECT line_id FROM line_stations WHERE station_id = ?)", a, b).Limit(1).Pluck("line_id", &ids).Error
	if err != nil || len(ids) == 0 {
		return uuid.Nil, httpx.Unprocessable("no_line", "couldn't work out a shared line for this match")
	}
	return ids[0], nil
}

func isoWeekday(t time.Time) int { return (int(t.In(ist).Weekday())+6)%7 + 1 }

func (s *Service) createCrew(c echo.Context) error {
	req, err := httpx.Bind[createCrewRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var crew model.Crew
	var invitee uuid.UUID
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		m, isA, err := s.loadMyMatch(tx, req.MatchID.String(), p.PersonaID)
		if err != nil {
			return err
		}
		if m.Status != model.MatchAccepted {
			return ErrNeedMatch
		}
		invitee = m.PersonaBID
		if !isA {
			invitee = m.PersonaAID
		}
		if blocked, _ := safety.IsBlocked(ctx, tx, p.PersonaID, invitee); blocked {
			return ErrNeedMatch
		}
		line, err := sharedLine(tx, m.OverlapFromStationID, m.OverlapToStationID)
		if err != nil {
			return err
		}
		var intent model.TravelIntent
		myIntent := m.IntentAID
		if !isA {
			myIntent = m.IntentBID
		}
		tx.First(&intent, "id = ?", myIntent)
		depart := intent.DepartAt.In(ist)
		mins := int16(depart.Hour()*60 + depart.Minute())
		days := weekdayMask([]int{1, 2, 3, 4, 5})
		if intent.CommutePlanID != nil {
			var plan model.CommutePlan
			if tx.First(&plan, "id = ?", *intent.CommutePlanID).Error == nil {
				days = plan.Weekdays
			}
		}
		crew = model.Crew{Name: pii.Mask(req.Name, pii.Options{}).Text, LineID: line, DepartWindowStart: mins, Weekdays: days, MemberCap: MaxCrew, CreatedByPersonaID: p.PersonaID}
		crew.ID = model.NewID()
		if err := tx.Create(&crew).Error; err != nil {
			return err
		}
		title := crew.Name
		conv := &model.Conversation{Kind: model.ConvCrew, CrewID: &crew.ID, Title: &title, MemberCap: MaxCrew}
		if err := chat.Create(tx, s.now(), conv, map[uuid.UUID]model.ConvRole{p.PersonaID: model.ConvOwner}); err != nil {
			return err
		}
		if _, err := s.Chat.SystemMessage(ctx, tx, conv.ID, model.MsgSystem, "Crew created. Plan: "+clock(mins)+" on the line you share. Invite people you've ridden with, up to 6."); err != nil {
			return err
		}
		return tx.Create(&model.CrewInvite{CrewID: crew.ID, FromPersonaID: p.PersonaID, ToPersonaID: invitee, Status: model.RequestPending}).Error
	})
	if err != nil {
		return err
	}
	s.notifyInvite(ctx, invitee, &crew)
	d, err := s.crewDTO(s.DB.WithContext(ctx), p.PersonaID, &crew)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, d)
}

func (s *Service) notifyInvite(ctx context.Context, to uuid.UUID, crew *model.Crew) {
	s.Notify.Notify(ctx, to, model.NotifMatch, model.JSONMap{"crew_id": crew.ID.String(), "text": "You've been invited to a commute crew"},
		notify.Content{Title: "Crew invite", Body: "Someone you rode with invited you to " + crew.Name, Link: "metrobuddy://travel"}, true)
}

type updateCrewRequest struct {
	Name     *string `json:"name" validate:"omitempty,min=3,max=40"`
	DepartAt *string `json:"depart_at" validate:"omitempty,len=5"`
	Weekdays []int   `json:"weekdays" validate:"omitempty,min=1,max=7,unique,dive,min=1,max=7"`
}

func (s *Service) updateCrew(c echo.Context) error {
	req, err := httpx.Bind[updateCrewRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	tx := s.DB.WithContext(ctx)
	cr, conv, err := s.loadMyCrew(tx, c.Param("id"), p.PersonaID)
	if err != nil {
		return err
	}
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = pii.Mask(*req.Name, pii.Options{}).Text
		tx.Model(conv).Update("title", updates["name"])
	}
	if req.DepartAt != nil {
		m, ok := parseClock(*req.DepartAt)
		if !ok {
			return httpx.Unprocessable("invalid_time", "use HH:MM")
		}
		updates["depart_window_start"] = m
	}
	if len(req.Weekdays) > 0 {
		updates["weekdays"] = weekdayMask(req.Weekdays)
	}
	if len(updates) > 0 {
		if err := tx.Model(cr).Updates(updates).Error; err != nil {
			return err
		}
		refs, _ := identity.LoadRefs(tx, []uuid.UUID{p.PersonaID})
		tx.First(cr, "id = ?", cr.ID)
		_ = s.Chat.PostSystem(ctx, conv.ID, model.MsgSystem, refs[p.PersonaID].Alias+" updated the crew plan: "+clock(cr.DepartWindowStart)+".")
	}
	d, err := s.crewDTO(tx, p.PersonaID, cr)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, d)
}

type inviteRequest struct {
	MatchID uuid.UUID `json:"match_id" validate:"required"`
}

func (s *Service) inviteToCrew(c echo.Context) error {
	req, err := httpx.Bind[inviteRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var crew *model.Crew
	var to uuid.UUID
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conv *model.Conversation
		var err error
		if crew, conv, err = s.loadMyCrew(tx, c.Param("id"), p.PersonaID); err != nil {
			return err
		}
		m, isA, err := s.loadMyMatch(tx, req.MatchID.String(), p.PersonaID)
		if err != nil || m.Status != model.MatchAccepted {
			return ErrNeedMatch
		}
		to = m.PersonaBID
		if !isA {
			to = m.PersonaAID
		}
		members := crewMembers(tx, conv.ID)
		for _, mem := range members {
			if mem == to {
				return httpx.Conflict("they're already in the crew")
			}
		}
		var pending int64
		tx.Model(&model.CrewInvite{}).Where("crew_id = ? AND status = ?", crew.ID, model.RequestPending).Count(&pending)
		if len(members)+int(pending) >= MaxCrew {
			return ErrCrewFull
		}
		var dup int64
		tx.Model(&model.CrewInvite{}).Where("crew_id = ? AND to_persona_id = ? AND status = ?", crew.ID, to, model.RequestPending).Count(&dup)
		if dup > 0 {
			return httpx.Conflict("already invited")
		}
		return tx.Create(&model.CrewInvite{CrewID: crew.ID, FromPersonaID: p.PersonaID, ToPersonaID: to, Status: model.RequestPending}).Error
	})
	if err != nil {
		return err
	}
	s.notifyInvite(ctx, to, crew)
	return c.NoContent(http.StatusNoContent)
}

type CrewInviteDTO struct {
	ID        uuid.UUID           `json:"id"`
	Crew      CrewDTO             `json:"crew"`
	From      identity.PersonaRef `json:"from"`
	CreatedAt time.Time           `json:"created_at"`
}

func (s *Service) crewInvites(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	var invs []model.CrewInvite
	tx.Where("to_persona_id = ? AND status = ?", p.PersonaID, model.RequestPending).Order("created_at DESC").Find(&invs)
	out := make([]CrewInviteDTO, 0, len(invs))
	for _, inv := range invs {
		var cr model.Crew
		if tx.First(&cr, "id = ?", inv.CrewID).Error != nil {
			continue
		}
		d, err := s.crewDTO(tx, p.PersonaID, &cr)
		if err != nil {
			return err
		}
		refs, _ := identity.LoadRefs(tx, []uuid.UUID{inv.FromPersonaID})
		out = append(out, CrewInviteDTO{ID: inv.ID, Crew: *d, From: refs[inv.FromPersonaID], CreatedAt: inv.CreatedAt})
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (s *Service) loadInvite(tx *gorm.DB, idParam string, viewer uuid.UUID) (*model.CrewInvite, error) {
	id, err := uuid.Parse(idParam)
	if err != nil {
		return nil, httpx.NotFound()
	}
	var inv model.CrewInvite
	if err := tx.First(&inv, "id = ? AND to_persona_id = ? AND status = ?", id, viewer, model.RequestPending).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

func (s *Service) acceptCrewInvite(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var conv *model.Conversation
	var crew model.Crew
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		inv, err := s.loadInvite(tx, c.Param("id"), p.PersonaID)
		if err != nil {
			return err
		}
		if err := tx.First(&crew, "id = ?", inv.CrewID).Error; err != nil {
			return err
		}
		if conv, err = s.crewConv(tx, crew.ID); err != nil {
			return err
		}
		members := crewMembers(tx, conv.ID)
		if len(members) >= MaxCrew {
			return ErrCrewFull
		}
		// Joining a crew would put you in a chat with anyone you've blocked or who blocked you.
		for _, m := range members {
			if blocked, _ := safety.IsBlocked(ctx, tx, p.PersonaID, m); blocked {
				tx.Model(inv).Update("status", model.RequestIgnored)
				return httpx.Forbidden("you can't join this crew")
			}
		}
		if err := tx.Model(inv).Update("status", model.RequestAccepted).Error; err != nil {
			return err
		}
		var left model.ConversationMember
		if tx.Where("conversation_id = ? AND persona_id = ?", conv.ID, p.PersonaID).Limit(1).Find(&left).RowsAffected == 1 {
			return tx.Model(&left).Where("conversation_id = ? AND persona_id = ?", conv.ID, p.PersonaID).
				Updates(map[string]any{"left_at": nil, "joined_at": s.now()}).Error
		}
		return tx.Create(&model.ConversationMember{ConversationID: conv.ID, PersonaID: p.PersonaID, Role: model.ConvMember, JoinedAt: s.now()}).Error
	})
	if err != nil {
		return err
	}
	tx := s.DB.WithContext(ctx)
	refs, _ := identity.LoadRefs(tx, []uuid.UUID{p.PersonaID})
	_ = s.Chat.PostSystem(ctx, conv.ID, model.MsgSystem, refs[p.PersonaID].Alias+" joined the crew.")
	s.Chat.Hub.Publish(ctx, []uuid.UUID{p.PersonaID}, "conversation.new", map[string]any{"id": conv.ID, "kind": conv.Kind})
	d, err := s.crewDTO(tx, p.PersonaID, &crew)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, d)
}

func (s *Service) declineCrewInvite(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	inv, err := s.loadInvite(tx, c.Param("id"), p.PersonaID)
	if err != nil {
		return err
	}
	tx.Model(inv).Update("status", model.RequestIgnored)
	return c.NoContent(http.StatusNoContent)
}

func (s *Service) leaveCrew(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	tx := s.DB.WithContext(ctx)
	cr, conv, err := s.loadMyCrew(tx, c.Param("id"), p.PersonaID)
	if err != nil {
		return err
	}
	tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND persona_id = ?", conv.ID, p.PersonaID).Update("left_at", s.now())
	if len(crewMembers(tx, conv.ID)) == 0 {
		tx.Model(&model.CrewInvite{}).Where("crew_id = ? AND status = ?", cr.ID, model.RequestPending).Update("status", model.RequestExpired)
		tx.Delete(cr)
		return c.NoContent(http.StatusNoContent)
	}
	refs, _ := identity.LoadRefs(tx, []uuid.UUID{p.PersonaID})
	_ = s.Chat.PostSystem(ctx, conv.ID, model.MsgSystem, refs[p.PersonaID].Alias+" left the crew.")
	return c.NoContent(http.StatusNoContent)
}

// CrewReminders drops a nudge into each crew chat shortly before its usual departure.
func (s *Service) CrewReminders(ctx context.Context) error {
	now := s.now().In(ist)
	day := isoWeekday(now)
	nowMin := now.Hour()*60 + now.Minute()
	tx := s.DB.WithContext(ctx)
	var crews []model.Crew
	tx.Where("depart_window_start > ? AND depart_window_start <= ?", nowMin, nowMin+int(crewReminderLed.Minutes())).Find(&crews)
	for _, cr := range crews {
		if cr.Weekdays&(1<<(day-1)) == 0 {
			continue
		}
		key := "crew:remind:" + cr.ID.String() + ":" + now.Format("2006-01-02")
		if s.Redis != nil {
			if ok, err := s.Redis.SetNX(ctx, key, 1, 24*time.Hour).Result(); err != nil || !ok {
				continue
			}
		}
		conv, err := s.crewConv(tx, cr.ID)
		if err != nil {
			continue
		}
		_ = s.Chat.PostSystem(ctx, conv.ID, model.MsgSystem, "Crew ride at "+clock(cr.DepartWindowStart)+" today. Who's in? Meet inside the station as usual.")
	}
	return nil
}
