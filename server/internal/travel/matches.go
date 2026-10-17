package travel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/chat"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/notify"
	"metro-buddy/server/internal/safety"
)

type side struct {
	intent  model.TravelIntent
	persona model.Persona
	person  Person
	vibes   []string
	labels  map[string]string
	trust   int
}

func (s *Service) loadSides(tx *gorm.DB, intents []model.TravelIntent) (map[uuid.UUID]*side, error) {
	if len(intents) == 0 {
		return map[uuid.UUID]*side{}, nil
	}
	pids := make([]uuid.UUID, len(intents))
	for i, in := range intents {
		pids[i] = in.PersonaID
	}
	var personas []model.Persona
	if err := tx.Where("id IN ?", pids).Find(&personas).Error; err != nil {
		return nil, err
	}
	byPersona := map[uuid.UUID]model.Persona{}
	var uids []uuid.UUID
	for _, p := range personas {
		byPersona[p.ID] = p
		uids = append(uids, p.UserID)
	}
	var users []model.User
	if err := tx.Where("id IN ?", uids).Find(&users).Error; err != nil {
		return nil, err
	}
	byUser := map[uuid.UUID]model.User{}
	for _, u := range users {
		byUser[u.ID] = u
	}
	var settings []model.UserSettings
	tx.Where("user_id IN ?", uids).Find(&settings)
	byUserSettings := map[uuid.UUID]model.UserSettings{}
	for _, st := range settings {
		byUserSettings[st.UserID] = st
	}
	vibes, err := identity.LoadVibes(tx, pids)
	if err != nil {
		return nil, err
	}
	year := s.now().In(ist).Year()
	out := map[uuid.UUID]*side{}
	for _, in := range intents {
		p, ok := byPersona[in.PersonaID]
		if !ok {
			continue
		}
		u := byUser[p.UserID]
		if u.Status != model.UserActive {
			continue
		}
		st := byUserSettings[u.ID]
		person := Person{Verified: u.VerifiedAt != nil, SameGender: st.MatchSameGenderOnly, VerifiedOnly: st.MatchVerifiedOnly}
		if u.Gender != nil {
			person.Gender = string(*u.Gender)
		}
		if u.BirthYear != nil {
			person.Age = year - int(*u.BirthYear)
		}
		if st.MatchAgeMin != nil {
			person.AgeMin = int(*st.MatchAgeMin)
		}
		if st.MatchAgeMax != nil {
			person.AgeMax = int(*st.MatchAgeMax)
		}
		sd := &side{intent: in, persona: p, person: person, trust: u.TrustScore, labels: map[string]string{}}
		for _, v := range vibes[p.ID] {
			sd.vibes = append(sd.vibes, v.Slug)
			sd.labels[v.Slug] = v.LabelEN
		}
		out[in.ID] = sd
	}
	return out, nil
}

type scored struct {
	other   *side
	score   float64
	segment []uuid.UUID
	why     model.MatchWhy
}

// Match runs the engine for one intent: hard filters first, then the weighted score.
func (s *Service) Match(ctx context.Context, in *model.TravelIntent) ([]model.Match, error) {
	tx := s.DB.WithContext(ctx)
	now := s.now()
	ids := s.candidates(ctx, in.LineIDs)
	var cands []model.TravelIntent
	if len(ids) > 0 {
		// The DB re-check is what guarantees expired or cancelled intents are never matched.
		if err := tx.Where("id IN ? AND id <> ? AND persona_id <> ? AND status = ? AND expires_at > ?", ids, in.ID, in.PersonaID, model.IntentActive, now).
			Where("persona_id IN (SELECT personas.id FROM personas JOIN consents ON consents.user_id = personas.user_id WHERE consents.purpose = ?)", model.ConsentTravel).
			Find(&cands).Error; err != nil {
			return nil, err
		}
	}
	sides, err := s.loadSides(tx, append(cands, *in))
	if err != nil {
		return nil, err
	}
	me := sides[in.ID]
	if me == nil {
		return nil, nil
	}
	blocked, err := safety.BlockedSet(ctx, tx, in.PersonaID)
	if err != nil {
		return nil, err
	}
	lineNames := s.lineNames(tx, in.LineIDs)

	var results []scored
	for _, c := range cands {
		o := sides[c.ID]
		if o == nil || blocked[o.persona.ID] || o.persona.UserID == me.persona.UserID {
			continue
		}
		if d := c.DepartAt.Sub(in.DepartAt); d > MaxDelta || d < -MaxDelta {
			continue
		}
		if !Compatible(me.person, o.person) {
			continue
		}
		if !consented(tx, o.persona.UserID) {
			continue
		}
		seg := Overlap(in.PathStationIDs, c.PathStationIDs)
		ratio := OverlapRatio(in.PathStationIDs, c.PathStationIDs, seg)
		if ratio < MinOverlap {
			continue
		}
		tf := TimeFit(in.DepartAt, c.DepartAt)
		vj, shared := Jaccard(me.vibes, o.vibes)
		mins := int(abs(c.DepartAt.Sub(in.DepartAt)).Minutes())
		why := model.MatchWhy{OverlapCount: len(seg), MinutesApart: mins}
		for _, l := range lineNames {
			why.Chips = append(why.Chips, "Same "+l)
			break
		}
		why.Chips = append(why.Chips, fmt.Sprintf("%d stops together", len(seg)-1))
		if mins == 0 {
			why.Chips = append(why.Chips, "Same time")
		} else {
			why.Chips = append(why.Chips, fmt.Sprintf("%d min apart", mins))
		}
		for i, sh := range shared {
			why.SharedVibes = append(why.SharedVibes, me.labels[sh])
			if i < 2 {
				why.Chips = append(why.Chips, "Both into "+me.labels[sh])
			}
		}
		if in.Mood == c.Mood && in.Mood != model.MoodEither {
			why.Chips = append(why.Chips, "Both feel "+string(in.Mood))
		}
		results = append(results, scored{other: o, score: Score(ratio, tf, vj, o.trust), segment: seg, why: why})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if len(results) > TopN {
		results = results[:TopN]
	}

	var out []model.Match
	for _, r := range results {
		var existing int64
		tx.Model(&model.Match{}).Where("(intent_a_id = ? AND intent_b_id = ?) OR (intent_a_id = ? AND intent_b_id = ?)",
			in.ID, r.other.intent.ID, r.other.intent.ID, in.ID).Count(&existing)
		if existing > 0 {
			continue
		}
		exp := in.ExpiresAt
		if r.other.intent.ExpiresAt.Before(exp) {
			exp = r.other.intent.ExpiresAt
		}
		m := model.Match{IntentAID: in.ID, IntentBID: r.other.intent.ID, PersonaAID: in.PersonaID, PersonaBID: r.other.persona.ID,
			Score: r.score, Why: r.why, OverlapFromStationID: r.segment[0], OverlapToStationID: r.segment[len(r.segment)-1],
			AStatus: model.SidePending, BStatus: model.SidePending, Status: model.MatchProposed, ExpiresAt: exp}
		if err := tx.Create(&m).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				continue
			}
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func consented(tx *gorm.DB, userID uuid.UUID) bool {
	ok, err := identity.HasConsent(tx, userID.String(), model.ConsentTravel)
	return err == nil && ok
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func (s *Service) lineNames(tx *gorm.DB, ids []uuid.UUID) []string {
	if len(ids) == 0 {
		return nil
	}
	var lines []model.MetroLine
	tx.Where("id IN ?", ids).Find(&lines)
	byID := map[uuid.UUID]string{}
	for _, l := range lines {
		byID[l.ID] = l.Name
	}
	var out []string
	for _, id := range ids {
		if n, ok := byID[id]; ok {
			out = append(out, n)
		}
	}
	return out
}

// MatchDTO shows only the shared stretch of the journey, never where either person starts or ends.
type MatchDTO struct {
	ID             uuid.UUID             `json:"id"`
	Other          identity.PersonaRef   `json:"other"`
	OtherVibes     []identity.VibeDTO    `json:"other_vibes"`
	Score          float64               `json:"score"`
	Why            model.MatchWhy        `json:"why"`
	SegmentFrom    StationRef            `json:"segment_from"`
	SegmentTo      StationRef            `json:"segment_to"`
	OtherMood      model.TravelMood      `json:"other_mood"`
	MyStatus       model.MatchSideStatus `json:"my_status"`
	TheyAccepted   bool                  `json:"they_accepted"`
	Status         model.MatchStatus     `json:"status"`
	ExpiresAt      time.Time             `json:"expires_at"`
	ConversationID *uuid.UUID            `json:"conversation_id"`
	TripID         *uuid.UUID            `json:"trip_id"`
}

func (s *Service) matchDTOs(tx *gorm.DB, viewer uuid.UUID, ms []model.Match) ([]MatchDTO, error) {
	var others, stations, intents []uuid.UUID
	for _, m := range ms {
		o, oi := m.PersonaBID, m.IntentBID
		if m.PersonaBID == viewer {
			o, oi = m.PersonaAID, m.IntentAID
		}
		others = append(others, o)
		intents = append(intents, oi)
		stations = append(stations, m.OverlapFromStationID, m.OverlapToStationID)
	}
	refs, err := identity.LoadRefs(tx, others)
	if err != nil {
		return nil, err
	}
	vibes, err := identity.LoadVibes(tx, others)
	if err != nil {
		return nil, err
	}
	srefs, err := s.stationRefs(tx, stations...)
	if err != nil {
		return nil, err
	}
	moods := map[uuid.UUID]model.TravelMood{}
	if len(intents) > 0 {
		var ins []model.TravelIntent
		tx.Select("id", "mood").Where("id IN ?", intents).Find(&ins)
		for _, i := range ins {
			moods[i.ID] = i.Mood
		}
	}
	out := make([]MatchDTO, len(ms))
	for i, m := range ms {
		mine, theirs, other, oi := m.AStatus, m.BStatus, m.PersonaBID, m.IntentBID
		if m.PersonaBID == viewer {
			mine, theirs, other, oi = m.BStatus, m.AStatus, m.PersonaAID, m.IntentAID
		}
		d := MatchDTO{ID: m.ID, Other: refs[other], OtherVibes: vibes[other], Score: m.Score, Why: m.Why,
			SegmentFrom: srefs[m.OverlapFromStationID], SegmentTo: srefs[m.OverlapToStationID], OtherMood: moods[oi],
			MyStatus: mine, Status: m.Status, ExpiresAt: m.ExpiresAt,
			// Revealing a pending acceptance would let someone probe who wants them; only mutual counts.
			TheyAccepted: theirs == model.SideAccepted && mine == model.SideAccepted}
		if d.OtherVibes == nil {
			d.OtherVibes = []identity.VibeDTO{}
		}
		if m.Status == model.MatchAccepted {
			var conv model.Conversation
			if tx.Select("id").Where("match_id = ?", m.ID).Limit(1).Find(&conv).RowsAffected == 1 {
				d.ConversationID = &conv.ID
			}
			var trip model.Trip
			if tx.Select("id").Where("match_id = ?", m.ID).Limit(1).Find(&trip).RowsAffected == 1 {
				d.TripID = &trip.ID
			}
		}
		out[i] = d
	}
	return out, nil
}

func (s *Service) listMatches(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	var ms []model.Match
	if err := tx.Where("(persona_a_id = ? OR persona_b_id = ?) AND ((status = ? AND expires_at > ?) OR (status = ? AND updated_at > ?))",
		p.PersonaID, p.PersonaID, model.MatchProposed, s.now(), model.MatchAccepted, s.now().Add(-12*time.Hour)).
		Scopes(safety.ExcludeBlocked(p.PersonaID, "persona_a_id"), safety.ExcludeBlocked(p.PersonaID, "persona_b_id")).
		Order("score DESC").Limit(TopN * 2).Find(&ms).Error; err != nil {
		return err
	}
	var visible []model.Match
	for _, m := range ms {
		mine := m.AStatus
		if m.PersonaBID == p.PersonaID {
			mine = m.BStatus
		}
		if mine != model.SideDeclined {
			visible = append(visible, m)
		}
	}
	out, err := s.matchDTOs(tx, p.PersonaID, visible)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (s *Service) refreshMatches(c echo.Context) error {
	ctx := c.Request().Context()
	in, err := s.liveIntent(s.DB.WithContext(ctx), httpx.PrincipalFrom(c).PersonaID)
	if err != nil {
		return err
	}
	if in == nil {
		return httpx.Conflict("start Travel Now or save a commute first")
	}
	if _, err := s.Match(ctx, in); err != nil {
		return err
	}
	return s.listMatches(c)
}

func (s *Service) loadMyMatch(tx *gorm.DB, idParam string, viewer uuid.UUID) (*model.Match, bool, error) {
	id, err := uuid.Parse(idParam)
	if err != nil {
		return nil, false, httpx.NotFound()
	}
	var m model.Match
	if err := tx.First(&m, "id = ? AND (persona_a_id = ? OR persona_b_id = ?)", id, viewer, viewer).Error; err != nil {
		return nil, false, err
	}
	return &m, m.PersonaAID == viewer, nil
}

// acceptMatch: the chat and trip exist only once both sides have said yes.
func (s *Service) acceptMatch(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var m *model.Match
	var conv *model.Conversation
	var trip *model.Trip
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var isA bool
		var err error
		if m, isA, err = s.loadMyMatch(tx, c.Param("id"), p.PersonaID); err != nil {
			return err
		}
		if m.Status != model.MatchProposed || !s.now().Before(m.ExpiresAt) {
			return httpx.Conflict("this match is no longer available")
		}
		if b, err := safety.IsBlocked(ctx, tx, m.PersonaAID, m.PersonaBID); err != nil || b {
			return httpx.NotFound()
		}
		col := "b_status"
		if isA {
			col, m.AStatus = "a_status", model.SideAccepted
		} else {
			m.BStatus = model.SideAccepted
		}
		if err := tx.Model(m).Update(col, model.SideAccepted).Error; err != nil {
			return err
		}
		if m.AStatus != model.SideAccepted || m.BStatus != model.SideAccepted {
			return nil
		}
		m.Status = model.MatchAccepted
		if err := tx.Model(m).Update("status", model.MatchAccepted).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.TravelIntent{}).Where("id IN ?", []uuid.UUID{m.IntentAID, m.IntentBID}).Update("status", model.IntentMatched).Error; err != nil {
			return err
		}
		// Each person can only travel with one buddy per trip; their other proposals lapse.
		if err := tx.Model(&model.Match{}).Where("id <> ? AND status = ? AND (intent_a_id IN ? OR intent_b_id IN ?)",
			m.ID, model.MatchProposed, []uuid.UUID{m.IntentAID, m.IntentBID}, []uuid.UUID{m.IntentAID, m.IntentBID}).
			Update("status", model.MatchExpired).Error; err != nil {
			return err
		}
		mid := m.ID
		conv = &model.Conversation{Kind: model.ConvTravel, MatchID: &mid, MemberCap: 2}
		if err := chat.Create(tx, s.now(), conv, map[uuid.UUID]model.ConvRole{m.PersonaAID: model.ConvMember, m.PersonaBID: model.ConvMember}); err != nil {
			return err
		}
		if trip, err = s.createTrip(ctx, tx, m); err != nil {
			return err
		}
		if _, err := s.Chat.SystemMessage(ctx, tx, conv.ID, model.MsgSystem, "You matched! Keep it inside the station. Share your trip from the trip screen."); err != nil {
			return err
		}
		_, err = s.Chat.SystemMessage(ctx, tx, conv.ID, model.MsgMeetPoint, meetText(tx, trip))
		return err
	})
	if err != nil {
		return err
	}
	s.afterAccept(ctx, p.PersonaID, m, conv)
	out, err := s.matchDTOs(s.DB.WithContext(ctx), p.PersonaID, []model.Match{*m})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out[0])
}

func (s *Service) afterAccept(ctx context.Context, actor uuid.UUID, m *model.Match, conv *model.Conversation) {
	other := m.PersonaAID
	if other == actor {
		other = m.PersonaBID
	}
	if conv == nil {
		s.Notify.Notify(ctx, other, model.NotifMatch, model.JSONMap{"match_id": m.ID.String(), "text": "Someone on your route wants to travel together"},
			notify.Content{Title: "Travel buddy request", Body: "Someone on your route wants to travel together", Link: "metrobuddy://travel"}, true)
		s.Chat.Hub.Publish(ctx, []uuid.UUID{other}, "match.new", map[string]any{"match_id": m.ID})
		return
	}
	for _, p := range []uuid.UUID{m.PersonaAID, m.PersonaBID} {
		s.Notify.Notify(ctx, p, model.NotifMatch, model.JSONMap{"match_id": m.ID.String(), "conversation_id": conv.ID.String(), "text": "It's a match! Your meet point is ready"},
			notify.Content{Title: "It's a match", Body: "Your meet point is ready", Link: "metrobuddy://chat/" + conv.ID.String()}, true)
	}
	s.Chat.Hub.Publish(ctx, []uuid.UUID{m.PersonaAID, m.PersonaBID}, "conversation.new", map[string]any{"id": conv.ID, "kind": conv.Kind})
	s.Chat.Hub.Publish(ctx, []uuid.UUID{m.PersonaAID, m.PersonaBID}, "match.new", map[string]any{"match_id": m.ID})
}

func (s *Service) declineMatch(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	err := s.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		m, isA, err := s.loadMyMatch(tx, c.Param("id"), p.PersonaID)
		if err != nil {
			return err
		}
		if m.Status != model.MatchProposed {
			return httpx.Conflict("this match is no longer available")
		}
		col := "b_status"
		if isA {
			col = "a_status"
		}
		return tx.Model(m).Updates(map[string]any{col: model.SideDeclined, "status": model.MatchDeclined}).Error
	})
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Service) ExpireMatches(ctx context.Context) error {
	return s.DB.WithContext(ctx).Model(&model.Match{}).Where("status = ? AND expires_at <= ?", model.MatchProposed, s.now()).
		Update("status", model.MatchExpired).Error
}

// --- Meet point (F44) ---

const womensCoach = "Coach 1 (women's coach)"

// meetPoint is always inside the station: the platform towards the shared direction, at a coach.
// The women's coach is only ever suggested when both travellers are women.
func (s *Service) meetPoint(tx *gorm.DB, m *model.Match, a, b *model.TravelIntent) (uuid.UUID, string, string, time.Time) {
	seg := Overlap(a.PathStationIDs, b.PathStationIDs)
	station := m.OverlapFromStationID
	platform := "Main concourse, then the platform for your line"
	if len(seg) >= 2 {
		var rows []struct {
			LineID       uuid.UUID
			Seq          int16
			PlatformHint *string
		}
		tx.Table("line_stations").Select("line_id, seq, platform_hint").Where("station_id = ?", seg[0]).Scan(&rows)
		for _, r := range rows {
			var next model.LineStation
			if tx.Where("line_id = ? AND station_id = ?", r.LineID, seg[1]).Limit(1).Find(&next).RowsAffected == 0 {
				continue
			}
			order := "seq DESC"
			if next.Seq < r.Seq {
				order = "seq ASC"
			}
			var end model.LineStation
			tx.Where("line_id = ?", r.LineID).Order(order).Limit(1).Find(&end)
			var term, line struct{ Name string }
			tx.Table("stations").Select("name").Where("id = ?", end.StationID).Scan(&term)
			tx.Table("metro_lines").Select("name").Where("id = ?", r.LineID).Scan(&line)
			platform = fmt.Sprintf("%s platform towards %s", line.Name, term.Name)
			if r.PlatformHint != nil {
				platform += " (" + *r.PlatformHint + ")"
			}
			break
		}
	}
	coach := "Coach 3, near Door 2"
	var genders []string
	tx.Table("users").Select("users.gender").Joins("JOIN personas ON personas.user_id = users.id").
		Where("personas.id IN ?", []uuid.UUID{m.PersonaAID, m.PersonaBID}).Pluck("users.gender", &genders)
	if len(genders) == 2 && genders[0] == string(model.GenderWoman) && genders[1] == string(model.GenderWoman) {
		coach = womensCoach
	}
	meetAt := a.DepartAt
	if b.DepartAt.After(meetAt) {
		meetAt = b.DepartAt
	}
	return station, platform, coach, meetAt
}

func meetText(tx *gorm.DB, t *model.Trip) string {
	var st struct{ Name string }
	tx.Table("stations").Select("name").Where("id = ?", t.MeetStationID).Scan(&st)
	return fmt.Sprintf("Meet point: %s station, %s, %s at %s. Stay inside the station.", st.Name, t.MeetPlatform, t.MeetCoach, t.MeetAt.In(ist).Format("3:04 PM"))
}
