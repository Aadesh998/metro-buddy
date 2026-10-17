// Package travel is Travel Buddy: commute plans, live intents, matching, trips and SOS.
// Location is only ever a station; home and work stations never leave this package.
package travel

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"metro-buddy/server/internal/audit"
	"metro-buddy/server/internal/chat"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/metro"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/notify"
	"metro-buddy/server/internal/ratelimit"
	"metro-buddy/server/internal/secure"
	"metro-buddy/server/internal/sms"
)

var (
	ErrConsent     = httpx.E(http.StatusForbidden, "consent_required", "turn on Travel Buddy matching in Privacy settings first")
	ErrBadWindow   = httpx.Unprocessable("invalid_window", "pick a departure window of up to 2 hours")
	ErrSameStation = httpx.Unprocessable("same_station", "pick two different stations")
	ist            = time.FixedZone("IST", 5*3600+1800)
)

type Service struct {
	DB      *gorm.DB
	Redis   *redis.Client
	Metro   *metro.Service
	Chat    *chat.Service
	Notify  *notify.Service
	Limiter *ratelimit.Limiter
	Audit   *audit.Recorder
	Box     *secure.Box
	SMS     sms.Sender
	BaseURL string
	Now     func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

func (s *Service) Register(g *echo.Group, authed *echo.Group) {
	t := g.Group("", s.requireConsent)
	t.GET("/commute-plans", s.listPlans)
	t.POST("/commute-plans", s.createPlan)
	t.PATCH("/commute-plans/:id", s.patchPlan)
	t.DELETE("/commute-plans/:id", s.deletePlan)
	t.POST("/travel-intents", s.createIntent, s.Limiter.Middleware(ratelimit.Rule{Name: "travel_now", Limit: 10, Window: time.Hour}))
	t.GET("/travel-intents/active", s.activeIntent)
	t.DELETE("/travel-intents/:id", s.cancelIntent)
	t.GET("/matches", s.listMatches)
	t.POST("/matches/refresh", s.refreshMatches, s.Limiter.Middleware(ratelimit.Rule{Name: "match_refresh", Limit: 20, Window: time.Hour}))
	t.POST("/matches/:id/accept", s.acceptMatch, s.Limiter.Middleware(ratelimit.Rule{Name: "match_requests", Limit: 10, Window: 24 * time.Hour, ByTier: true}))
	t.POST("/matches/:id/decline", s.declineMatch)
	s.registerTrips(g)
	s.registerSafety(g, authed)
}

// requireConsent: nothing about your commute is processed without the travel_matching consent.
func (s *Service) requireConsent(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ok, err := identity.HasConsent(s.DB.WithContext(c.Request().Context()), httpx.PrincipalFrom(c).UserID.String(), model.ConsentTravel)
		if err != nil {
			return err
		}
		if !ok {
			return ErrConsent
		}
		return next(c)
	}
}

// parseClock accepts "HH:MM" (IST) and returns minutes after midnight.
func parseClock(s string) (int16, bool) {
	h, m, ok := strings.Cut(s, ":")
	if !ok {
		return 0, false
	}
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, false
	}
	return int16(hh*60 + mm), true
}

func clock(m int16) string {
	return strconv.Itoa(int(m)/60/10) + strconv.Itoa(int(m)/60%10) + ":" + strconv.Itoa(int(m)%60/10) + strconv.Itoa(int(m)%60%10)
}

type StationRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (s *Service) stationRefs(tx *gorm.DB, ids ...uuid.UUID) (map[uuid.UUID]StationRef, error) {
	var rows []model.Station
	if err := tx.Select("id", "name").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := map[uuid.UUID]StationRef{}
	for _, r := range rows {
		out[r.ID] = StationRef{ID: r.ID, Name: r.Name}
	}
	return out, nil
}

// --- Commute plans (F40) ---

type planRequest struct {
	FromStationID uuid.UUID `json:"from_station_id" validate:"required"`
	ToStationID   uuid.UUID `json:"to_station_id" validate:"required"`
	DepartFrom    string    `json:"depart_from" validate:"required"`
	DepartTo      string    `json:"depart_to" validate:"required"`
	Weekdays      []int     `json:"weekdays" validate:"required,min=1,max=7,unique,dive,min=1,max=7"`
}

type PlanDTO struct {
	ID         uuid.UUID  `json:"id"`
	From       StationRef `json:"from"`
	To         StationRef `json:"to"`
	DepartFrom string     `json:"depart_from"`
	DepartTo   string     `json:"depart_to"`
	Weekdays   []int      `json:"weekdays"`
	Stops      int        `json:"stops"`
	Active     bool       `json:"active"`
}

func weekdayMask(days []int) int16 {
	var m int16
	for _, d := range days {
		m |= 1 << (d - 1)
	}
	return m
}

func weekdays(mask int16) []int {
	out := []int{}
	for d := 1; d <= 7; d++ {
		if mask&(1<<(d-1)) != 0 {
			out = append(out, d)
		}
	}
	return out
}

func (s *Service) planDTOs(tx *gorm.DB, plans []model.CommutePlan) ([]PlanDTO, error) {
	var ids []uuid.UUID
	for _, p := range plans {
		ids = append(ids, p.FromStationID, p.ToStationID)
	}
	refs, err := s.stationRefs(tx, ids...)
	if err != nil {
		return nil, err
	}
	out := make([]PlanDTO, len(plans))
	for i, p := range plans {
		out[i] = PlanDTO{ID: p.ID, From: refs[p.FromStationID], To: refs[p.ToStationID], DepartFrom: clock(p.DepartWindowStart),
			DepartTo: clock(p.DepartWindowEnd), Weekdays: weekdays(p.Weekdays), Stops: len(p.PathStationIDs) - 1, Active: p.Active}
	}
	return out, nil
}

func (s *Service) listPlans(c echo.Context) error {
	tx := s.DB.WithContext(c.Request().Context())
	var plans []model.CommutePlan
	if err := tx.Where("persona_id = ?", httpx.PrincipalFrom(c).PersonaID).Order("depart_window_start").Find(&plans).Error; err != nil {
		return err
	}
	out, err := s.planDTOs(tx, plans)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (s *Service) validatePlan(req *planRequest) (int16, int16, error) {
	a, ok1 := parseClock(req.DepartFrom)
	b, ok2 := parseClock(req.DepartTo)
	if !ok1 || !ok2 || b < a || b-a > 120 {
		return 0, 0, ErrBadWindow
	}
	return a, b, nil
}

// createPlan stores the BFS path on save so matching never recomputes routes.
func (s *Service) createPlan(c echo.Context) error {
	req, err := httpx.Bind[planRequest](c)
	if err != nil {
		return err
	}
	if req.FromStationID == req.ToStationID {
		return ErrSameStation
	}
	a, b, err := s.validatePlan(req)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	path, _, err := s.Metro.Route(ctx, req.FromStationID, req.ToStationID)
	if err != nil {
		return err
	}
	pid := httpx.PrincipalFrom(c).PersonaID
	tx := s.DB.WithContext(ctx)
	var n int64
	tx.Model(&model.CommutePlan{}).Where("persona_id = ?", pid).Count(&n)
	if n >= 6 {
		return httpx.Conflict("you can save up to 6 commutes")
	}
	p := model.CommutePlan{PersonaID: pid, FromStationID: req.FromStationID, ToStationID: req.ToStationID, DepartWindowStart: a,
		DepartWindowEnd: b, Weekdays: weekdayMask(req.Weekdays), PathStationIDs: path, Active: true}
	if err := tx.Create(&p).Error; err != nil {
		return err
	}
	out, err := s.planDTOs(tx, []model.CommutePlan{p})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, out[0])
}

type planPatch struct {
	Active     *bool  `json:"active"`
	DepartFrom string `json:"depart_from"`
	DepartTo   string `json:"depart_to"`
	Weekdays   []int  `json:"weekdays" validate:"omitempty,min=1,max=7,unique,dive,min=1,max=7"`
}

func (s *Service) patchPlan(c echo.Context) error {
	req, err := httpx.Bind[planPatch](c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	tx := s.DB.WithContext(c.Request().Context())
	var p model.CommutePlan
	if err := tx.First(&p, "id = ? AND persona_id = ?", id, httpx.PrincipalFrom(c).PersonaID).Error; err != nil {
		return err
	}
	updates := map[string]any{}
	if req.Active != nil {
		updates["active"] = *req.Active
	}
	if req.DepartFrom != "" || req.DepartTo != "" {
		a, b, err := s.validatePlan(&planRequest{DepartFrom: req.DepartFrom, DepartTo: req.DepartTo})
		if err != nil {
			return err
		}
		updates["depart_window_start"], updates["depart_window_end"] = a, b
	}
	if len(req.Weekdays) > 0 {
		updates["weekdays"] = weekdayMask(req.Weekdays)
	}
	if err := tx.Model(&p).Updates(updates).Error; err != nil {
		return err
	}
	tx.First(&p, "id = ?", p.ID)
	out, err := s.planDTOs(tx, []model.CommutePlan{p})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out[0])
}

func (s *Service) deletePlan(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	if err := s.DB.WithContext(c.Request().Context()).Where("id = ? AND persona_id = ?", id, httpx.PrincipalFrom(c).PersonaID).
		Delete(&model.CommutePlan{}).Error; err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// --- Intents (F41) and the live index ---

func lineKey(line uuid.UUID) string { return "travel:line:" + line.String() }

// index puts an intent on every line it rides, scored by expiry; expired members are swept on read.
func (s *Service) index(ctx context.Context, in *model.TravelIntent) {
	pipe := s.Redis.Pipeline()
	for _, l := range in.LineIDs {
		pipe.ZAdd(ctx, lineKey(l), redis.Z{Score: float64(in.ExpiresAt.Unix()), Member: in.ID.String()})
		pipe.Expire(ctx, lineKey(l), 3*time.Hour)
	}
	_, _ = pipe.Exec(ctx)
}

func (s *Service) unindex(ctx context.Context, in *model.TravelIntent) {
	for _, l := range in.LineIDs {
		s.Redis.ZRem(ctx, lineKey(l), in.ID.String())
	}
}

func (s *Service) candidates(ctx context.Context, lines []uuid.UUID) []uuid.UUID {
	now := strconv.FormatInt(s.now().Unix(), 10)
	seen := map[uuid.UUID]bool{}
	var out []uuid.UUID
	for _, l := range lines {
		s.Redis.ZRemRangeByScore(ctx, lineKey(l), "-inf", now)
		ms, err := s.Redis.ZRangeByScore(ctx, lineKey(l), &redis.ZRangeBy{Min: "(" + now, Max: "+inf"}).Result()
		if err != nil {
			continue
		}
		for _, m := range ms {
			if id, err := uuid.Parse(m); err == nil && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

type intentRequest struct {
	FromStationID uuid.UUID        `json:"from_station_id" validate:"required"`
	ToStationID   uuid.UUID        `json:"to_station_id" validate:"required"`
	DepartInMin   int              `json:"depart_in_min" validate:"min=0,max=45"`
	Mood          model.TravelMood `json:"mood" validate:"omitempty,oneof=chatty quiet either"`
}

type IntentDTO struct {
	ID        uuid.UUID          `json:"id"`
	From      StationRef         `json:"from"`
	To        StationRef         `json:"to"`
	DepartAt  time.Time          `json:"depart_at"`
	ExpiresAt time.Time          `json:"expires_at"`
	Mood      model.TravelMood   `json:"mood"`
	Status    model.IntentStatus `json:"status"`
	FromPlan  bool               `json:"from_plan"`
}

func (s *Service) intentDTO(tx *gorm.DB, in *model.TravelIntent) (*IntentDTO, error) {
	refs, err := s.stationRefs(tx, in.FromStationID, in.ToStationID)
	if err != nil {
		return nil, err
	}
	return &IntentDTO{ID: in.ID, From: refs[in.FromStationID], To: refs[in.ToStationID], DepartAt: in.DepartAt, ExpiresAt: in.ExpiresAt,
		Mood: in.Mood, Status: in.Status, FromPlan: in.CommutePlanID != nil}, nil
}

// newIntent cancels the persona's other live intents: one trip at a time.
func (s *Service) newIntent(ctx context.Context, in *model.TravelIntent) error {
	path, lines, err := s.Metro.Route(ctx, in.FromStationID, in.ToStationID)
	if err != nil {
		return err
	}
	in.PathStationIDs, in.LineIDs = path, lines
	if in.Mood == "" {
		in.Mood = model.MoodEither
	}
	in.Status = model.IntentActive
	var old []model.TravelIntent
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("persona_id = ? AND status = ?", in.PersonaID, model.IntentActive).Find(&old).Error; err != nil {
			return err
		}
		if len(old) > 0 {
			if err := tx.Model(&model.TravelIntent{}).Where("persona_id = ? AND status = ?", in.PersonaID, model.IntentActive).
				Update("status", model.IntentCancelled).Error; err != nil {
				return err
			}
		}
		return tx.Create(in).Error
	})
	if err != nil {
		return err
	}
	for i := range old {
		s.unindex(ctx, &old[i])
	}
	s.index(ctx, in)
	return nil
}

// createIntent is "Travel Now": live for 45 minutes, then it disappears on its own.
func (s *Service) createIntent(c echo.Context) error {
	req, err := httpx.Bind[intentRequest](c)
	if err != nil {
		return err
	}
	if req.FromStationID == req.ToStationID {
		return ErrSameStation
	}
	now := s.now()
	in := model.TravelIntent{PersonaID: httpx.PrincipalFrom(c).PersonaID, FromStationID: req.FromStationID, ToStationID: req.ToStationID,
		DepartAt: now.Add(time.Duration(req.DepartInMin) * time.Minute), ExpiresAt: now.Add(IntentTTL), Mood: req.Mood}
	ctx := c.Request().Context()
	if err := s.newIntent(ctx, &in); err != nil {
		return err
	}
	if _, err := s.Match(ctx, &in); err != nil {
		return err
	}
	d, err := s.intentDTO(s.DB.WithContext(ctx), &in)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, d)
}

func (s *Service) liveIntent(tx *gorm.DB, persona uuid.UUID) (*model.TravelIntent, error) {
	var in model.TravelIntent
	if tx.Where("persona_id = ? AND status IN ? AND expires_at > ?", persona, []model.IntentStatus{model.IntentActive, model.IntentMatched}, s.now()).
		Order("created_at DESC").Limit(1).Find(&in).RowsAffected == 0 {
		return nil, nil
	}
	return &in, nil
}

func (s *Service) activeIntent(c echo.Context) error {
	tx := s.DB.WithContext(c.Request().Context())
	in, err := s.liveIntent(tx, httpx.PrincipalFrom(c).PersonaID)
	if err != nil || in == nil {
		return c.JSON(http.StatusOK, map[string]any{"intent": nil})
	}
	d, err := s.intentDTO(tx, in)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"intent": d})
}

func (s *Service) cancelIntent(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	ctx := c.Request().Context()
	var in model.TravelIntent
	if err := s.DB.WithContext(ctx).First(&in, "id = ? AND persona_id = ?", id, httpx.PrincipalFrom(c).PersonaID).Error; err != nil {
		return err
	}
	if err := s.DB.WithContext(ctx).Model(&in).Update("status", model.IntentCancelled).Error; err != nil {
		return err
	}
	s.unindex(ctx, &in)
	s.DB.WithContext(ctx).Model(&model.Match{}).Where("(intent_a_id = ? OR intent_b_id = ?) AND status = ?", in.ID, in.ID, model.MatchProposed).
		Update("status", model.MatchExpired)
	return c.NoContent(http.StatusNoContent)
}

// SpawnPlanIntents turns today's commute plans into live intents shortly before they depart.
func (s *Service) SpawnPlanIntents(ctx context.Context) error {
	now := s.now()
	local := now.In(ist)
	wd := int(local.Weekday())
	if wd == 0 {
		wd = 7
	}
	minute := int16(local.Hour()*60 + local.Minute())
	var plans []model.CommutePlan
	if err := s.DB.WithContext(ctx).Where("active = ? AND depart_window_start > ? AND depart_window_start <= ?", true, minute, minute+45).
		Find(&plans).Error; err != nil {
		return err
	}
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, ist)
	for _, p := range plans {
		if p.Weekdays&(1<<(wd-1)) == 0 {
			continue
		}
		var n int64
		s.DB.WithContext(ctx).Model(&model.TravelIntent{}).Where("commute_plan_id = ? AND created_at >= ?", p.ID, dayStart.UTC()).Count(&n)
		if n > 0 {
			continue
		}
		pid := p.ID
		in := model.TravelIntent{PersonaID: p.PersonaID, CommutePlanID: &pid, FromStationID: p.FromStationID, ToStationID: p.ToStationID,
			DepartAt:  dayStart.Add(time.Duration(p.DepartWindowStart) * time.Minute).UTC(),
			ExpiresAt: dayStart.Add(time.Duration(p.DepartWindowEnd)*time.Minute + 15*time.Minute).UTC()}
		if err := s.newIntent(ctx, &in); err != nil {
			continue
		}
		_, _ = s.Match(ctx, &in)
	}
	return s.DB.WithContext(ctx).Model(&model.TravelIntent{}).Where("status = ? AND expires_at <= ?", model.IntentActive, now).
		Update("status", model.IntentExpired).Error
}
