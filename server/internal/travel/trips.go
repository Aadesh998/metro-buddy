package travel

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/trust"
	"metro-buddy/server/internal/notify"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/safety"
)

const (
	noShowAfter   = 20 * time.Minute
	completeAfter = 3 * time.Hour
	remindBefore  = 15 * time.Minute
)

func (s *Service) createTrip(ctx context.Context, tx *gorm.DB, m *model.Match) (*model.Trip, error) {
	var a, b model.TravelIntent
	if err := tx.First(&a, "id = ?", m.IntentAID).Error; err != nil {
		return nil, err
	}
	if err := tx.First(&b, "id = ?", m.IntentBID).Error; err != nil {
		return nil, err
	}
	station, platform, coach, at := s.meetPoint(tx, m, &a, &b)
	mid := m.ID
	t := model.Trip{MatchID: &mid, MeetStationID: station, MeetPlatform: platform, MeetCoach: coach, MeetAt: at, Status: model.TripPlanned}
	if err := tx.Create(&t).Error; err != nil {
		return nil, err
	}
	for _, in := range []model.TravelIntent{a, b} {
		if err := tx.Create(&model.TripParticipant{TripID: t.ID, PersonaID: in.PersonaID, ToStationID: in.ToStationID}).Error; err != nil {
			return nil, err
		}
	}
	return &t, nil
}

func (s *Service) registerTrips(g *echo.Group) {
	g.GET("/trips/active", s.activeTrip)
	g.GET("/trips/:id", s.getTrip)
	g.POST("/trips/:id/checkin", s.checkin)
	g.POST("/trips/:id/progress", s.progress)
	g.POST("/trips/:id/cancel", s.cancelTrip)
	g.POST("/trips/:id/feedback", s.feedback)
}

type ParticipantDTO struct {
	Persona   identity.PersonaRef `json:"persona"`
	CheckedIn bool                `json:"checked_in"`
	NoShow    bool                `json:"no_show"`
	IsMe      bool                `json:"is_me"`
}

type TripDTO struct {
	ID             uuid.UUID        `json:"id"`
	Status         model.TripStatus `json:"status"`
	MeetStation    StationRef       `json:"meet_station"`
	MeetPlatform   string           `json:"meet_platform"`
	MeetCoach      string           `json:"meet_coach"`
	MeetAt         time.Time        `json:"meet_at"`
	Participants   []ParticipantDTO `json:"participants"`
	ConversationID *uuid.UUID       `json:"conversation_id"`
	Shared         bool             `json:"shared"`
	ShareURL       string           `json:"share_url,omitempty"`
	FeedbackGiven  bool             `json:"feedback_given"`
}

func (s *Service) tripDTO(tx *gorm.DB, viewer uuid.UUID, t *model.Trip) (*TripDTO, error) {
	var parts []model.TripParticipant
	if err := tx.Where("trip_id = ?", t.ID).Find(&parts).Error; err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(parts))
	for i, p := range parts {
		ids[i] = p.PersonaID
	}
	refs, err := identity.LoadRefs(tx, ids)
	if err != nil {
		return nil, err
	}
	st, err := s.stationRefs(tx, t.MeetStationID)
	if err != nil {
		return nil, err
	}
	d := &TripDTO{ID: t.ID, Status: t.Status, MeetStation: st[t.MeetStationID], MeetPlatform: t.MeetPlatform, MeetCoach: t.MeetCoach, MeetAt: t.MeetAt}
	for _, p := range parts {
		d.Participants = append(d.Participants, ParticipantDTO{Persona: refs[p.PersonaID], CheckedIn: p.CheckedInAt != nil, NoShow: p.NoShow, IsMe: p.PersonaID == viewer})
	}
	if t.MatchID != nil {
		var conv model.Conversation
		if tx.Select("id").Where("match_id = ?", *t.MatchID).Limit(1).Find(&conv).RowsAffected == 1 {
			d.ConversationID = &conv.ID
		}
	}
	if t.ShareToken != nil && t.ShareExpiresAt != nil && s.now().Before(*t.ShareExpiresAt) {
		d.Shared, d.ShareURL = true, s.BaseURL+"/trip/"+*t.ShareToken
	}
	var n int64
	tx.Model(&model.TripFeedback{}).Where("trip_id = ? AND from_persona_id = ?", t.ID, viewer).Count(&n)
	d.FeedbackGiven = n > 0
	return d, nil
}

// myTrip loads a trip only for its participants; everyone else gets 404.
func (s *Service) myTrip(tx *gorm.DB, idParam string, viewer uuid.UUID) (*model.Trip, error) {
	id, err := uuid.Parse(idParam)
	if err != nil {
		return nil, httpx.NotFound()
	}
	var n int64
	if err := tx.Model(&model.TripParticipant{}).Where("trip_id = ? AND persona_id = ?", id, viewer).Count(&n).Error; err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, httpx.NotFound()
	}
	var t model.Trip
	return &t, tx.First(&t, "id = ?", id).Error
}

func (s *Service) respondTrip(c echo.Context, t *model.Trip) error {
	d, err := s.tripDTO(s.DB.WithContext(c.Request().Context()), httpx.PrincipalFrom(c).PersonaID, t)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, d)
}

func (s *Service) activeTrip(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	var t model.Trip
	if tx.Joins("JOIN trip_participants tp ON tp.trip_id = trips.id AND tp.persona_id = ?", p.PersonaID).
		Where("trips.status IN ? AND trips.meet_at > ?", []model.TripStatus{model.TripPlanned, model.TripMet}, s.now().Add(-completeAfter)).
		Order("trips.meet_at").Limit(1).Find(&t).RowsAffected == 0 {
		return c.JSON(http.StatusOK, map[string]any{"trip": nil})
	}
	d, err := s.tripDTO(tx, p.PersonaID, &t)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"trip": d})
}

func (s *Service) getTrip(c echo.Context) error {
	t, err := s.myTrip(s.DB.WithContext(c.Request().Context()), c.Param("id"), httpx.PrincipalFrom(c).PersonaID)
	if err != nil {
		return err
	}
	return s.respondTrip(c, t)
}

func (s *Service) checkin(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	var t *model.Trip
	err := s.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var err error
		if t, err = s.myTrip(tx, c.Param("id"), p.PersonaID); err != nil {
			return err
		}
		if t.Status != model.TripPlanned && t.Status != model.TripMet {
			return httpx.Conflict("this trip is over")
		}
		if err := tx.Model(&model.TripParticipant{}).Where("trip_id = ? AND persona_id = ? AND checked_in_at IS NULL", t.ID, p.PersonaID).
			Update("checked_in_at", s.now()).Error; err != nil {
			return err
		}
		var waiting int64
		tx.Model(&model.TripParticipant{}).Where("trip_id = ? AND checked_in_at IS NULL", t.ID).Count(&waiting)
		if waiting == 0 {
			t.Status = model.TripMet
			return tx.Model(t).Update("status", model.TripMet).Error
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.respondTrip(c, t)
}

type progressRequest struct {
	StationID uuid.UUID `json:"station_id" validate:"required"`
}

// progress records the last station passed. It's the only location the share page ever shows.
func (s *Service) progress(c echo.Context) error {
	req, err := httpx.Bind[progressRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	t, err := s.myTrip(tx, c.Param("id"), p.PersonaID)
	if err != nil {
		return err
	}
	var n int64
	tx.Model(&model.Station{}).Where("id = ?", req.StationID).Count(&n)
	if n == 0 {
		return httpx.Unprocessable("invalid_station", "unknown station")
	}
	if err := tx.Model(&model.TripParticipant{}).Where("trip_id = ? AND persona_id = ?", t.ID, p.PersonaID).Update("last_station_id", req.StationID).Error; err != nil {
		return err
	}
	return s.respondTrip(c, t)
}

func (s *Service) cancelTrip(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	t, err := s.myTrip(tx, c.Param("id"), p.PersonaID)
	if err != nil {
		return err
	}
	if t.Status != model.TripPlanned {
		return httpx.Conflict("this trip can't be cancelled now")
	}
	if err := tx.Model(t).Update("status", model.TripCancelled).Error; err != nil {
		return err
	}
	t.Status = model.TripCancelled
	if d, _ := s.tripDTO(tx, p.PersonaID, t); d != nil && d.ConversationID != nil {
		_, _ = s.Chat.SystemMessage(c.Request().Context(), tx, *d.ConversationID, model.MsgSystem, "The trip was cancelled.")
	}
	s.Chat.Hub.Publish(c.Request().Context(), participantIDs(tx, t.ID), "trip.cancelled", map[string]any{"trip_id": t.ID})
	return s.respondTrip(c, t)
}

func participantIDs(tx *gorm.DB, trip uuid.UUID) []uuid.UUID {
	var ids []uuid.UUID
	tx.Model(&model.TripParticipant{}).Where("trip_id = ?", trip).Pluck("persona_id", &ids)
	return ids
}

type feedbackRequest struct {
	Thumbs     int16  `json:"thumbs" validate:"oneof=-1 1"`
	FeltSafe   bool   `json:"felt_safe"`
	SafetyFlag bool   `json:"safety_flag"`
	Note       string `json:"note" validate:"max=300"`
}

// feedback is private: the other person never sees it. A safety flag goes straight to T&S.
func (s *Service) feedback(c echo.Context) error {
	req, err := httpx.Bind[feedbackRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var t *model.Trip
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		if t, err = s.myTrip(tx, c.Param("id"), p.PersonaID); err != nil {
			return err
		}
		var other model.TripParticipant
		if err := tx.Where("trip_id = ? AND persona_id <> ?", t.ID, p.PersonaID).First(&other).Error; err != nil {
			return err
		}
		fb := model.TripFeedback{TripID: t.ID, FromPersonaID: p.PersonaID, ToPersonaID: other.PersonaID, Thumbs: req.Thumbs,
			FeltSafe: req.FeltSafe, SafetyFlag: req.SafetyFlag || !req.FeltSafe, NoteMasked: pii.Mask(req.Note, pii.Options{}).Text}
		if err := tx.Create(&fb).Error; err != nil {
			return err
		}
		var otherUser model.Persona
		if err := tx.Select("user_id").First(&otherUser, "id = ?", other.PersonaID).Error; err != nil {
			return err
		}
		if fb.SafetyFlag {
			if _, err := safety.OpenCase(tx, s.now(), model.CTTrip, t.ID, &other.PersonaID, model.SevHigh, "safety_flag"); err != nil {
				return err
			}
			return trust.Record(tx, otherUser.UserID, model.TrustSafetyFlag, -10, &t.ID)
		}
		if req.Thumbs == 1 {
			return trust.Record(tx, otherUser.UserID, model.TrustPositiveFeedback, 2, &t.ID)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.respondTrip(c, t)
}

// TripLifecycle sends reminders, marks no-shows and closes finished trips.
func (s *Service) TripLifecycle(ctx context.Context) error {
	now := s.now()
	tx := s.DB.WithContext(ctx)
	var soon []model.Trip
	tx.Where("status = ? AND reminder_sent_at IS NULL AND meet_at > ? AND meet_at <= ?", model.TripPlanned, now, now.Add(remindBefore)).Find(&soon)
	for _, t := range soon {
		tx.Model(&t).Update("reminder_sent_at", now)
		for _, p := range participantIDs(tx, t.ID) {
			s.Notify.Notify(ctx, p, model.NotifTripReminder, model.JSONMap{"trip_id": t.ID.String(), "text": "Your travel buddy meet-up is in 15 minutes"},
				notify.Content{Title: "Meet-up soon", Body: "Head to your meet point. Share your trip with someone you trust.", Link: "metrobuddy://trip/" + t.ID.String()}, true)
		}
	}
	var late []model.Trip
	tx.Where("status = ? AND meet_at < ?", model.TripPlanned, now.Add(-noShowAfter)).Find(&late)
	for _, t := range late {
		var absent []model.TripParticipant
		tx.Where("trip_id = ? AND checked_in_at IS NULL", t.ID).Find(&absent)
		for _, a := range absent {
			tx.Model(&model.TripParticipant{}).Where("trip_id = ? AND persona_id = ?", t.ID, a.PersonaID).Update("no_show", true)
			var per model.Persona
			if tx.Select("user_id").First(&per, "id = ?", a.PersonaID).Error == nil {
				trust.Record(tx, per.UserID, model.TrustNoShow, -5, &t.ID)
			}
		}
		tx.Model(&t).Update("status", model.TripNoShow)
	}
	return tx.Model(&model.Trip{}).Where("status = ? AND meet_at < ?", model.TripMet, now.Add(-completeAfter)).Update("status", model.TripCompleted).Error
}
