// Package studio is Vent Studio: approved hosts run live audio shows and listeners call in.
// Nothing is ever recorded, and on-air permission is granted and revoked server-side.
package studio

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metro-buddy/server/internal/ai"
	"metro-buddy/server/internal/audit"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/notify"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/ratelimit"
)

const (
	MaxOnAir       = 8 * time.Minute
	MaxExtend      = 5 * time.Minute
	reminderLead   = 10 * time.Minute
	spikeWindow    = 2 * time.Minute
	SpikeThreshold = 3
	topicMax       = 80
	startEarly     = 15 * time.Minute
)

var Reactions = []string{"heart", "hug", "clap", "laugh", "fire", "same"}

var (
	ErrNotHost     = httpx.Forbidden("only approved hosts can do that")
	ErrNotLive     = httpx.Conflict("this episode isn't live")
	ErrNotYourShow = httpx.Forbidden("this isn't your show")
)

type Publisher interface {
	Publish(ctx context.Context, personas []uuid.UUID, typ string, data any)
}

type Service struct {
	DB      *gorm.DB
	Live    Live
	Hub     Publisher
	Notify  *notify.Service
	PII     *pii.Pipeline
	Audit   *audit.Recorder
	Limiter *ratelimit.Limiter
	AI      *ai.Client
	Redis   *redis.Client
	Log     *slog.Logger
	Now     func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

func (s *Service) Register(g *echo.Group, e *echo.Echo) {
	g.POST("/studio/host-application", s.apply)
	g.GET("/studio/host-application", s.myApplication)
	g.GET("/studio/topic-ideas", s.topicIdeas, s.Limiter.Middleware(ratelimit.Rule{Name: "topic_ideas", Limit: 20, Window: time.Hour}))
	g.GET("/shows", s.listShows)
	g.POST("/shows", s.createShow)
	g.GET("/shows/:id", s.getShow)
	g.POST("/shows/:id/follow", s.follow)
	g.DELETE("/shows/:id/follow", s.unfollow)
	g.POST("/shows/:id/episodes", s.createEpisode)
	g.GET("/episodes", s.schedule)
	g.GET("/episodes/:id", s.getEpisode)
	g.POST("/episodes/:id/start", s.start)
	g.POST("/episodes/:id/end", s.end)
	g.POST("/episodes/:id/join", s.join)
	g.POST("/episodes/:id/leave", s.leave)
	g.POST("/episodes/:id/call-in", s.callIn, s.Limiter.Middleware(ratelimit.Rule{Name: "call_in", Limit: 10, Window: time.Hour}))
	g.DELETE("/episodes/:id/call-in", s.withdraw)
	g.GET("/episodes/:id/queue", s.queue)
	g.POST("/episodes/:id/stage", s.stage)
	g.POST("/episodes/:id/unstage", s.unstage)
	g.POST("/episodes/:id/extend", s.extend)
	g.POST("/episodes/:id/mute", s.muteCaller)
	g.POST("/episodes/:id/remove", s.removeListener)
	g.POST("/episodes/:id/reactions", s.react, s.Limiter.Middleware(ratelimit.Rule{Name: "reactions", Limit: 60, Window: time.Minute}))
	g.POST("/episodes/:id/questions", s.ask, s.Limiter.Middleware(ratelimit.Rule{Name: "questions", Limit: 5, Window: 10 * time.Minute}))
	g.GET("/episodes/:id/questions", s.questions)
	g.POST("/questions/:id/answered", s.answered)
	e.POST("/v1/livekit/webhook", s.webhook)
}

// --- Hosts (F30) ---

type applyRequest struct {
	Tagline string `json:"tagline" validate:"required,min=5,max=80"`
	Pitch   string `json:"pitch" validate:"required,min=20,max=500"`
}

type HostDTO struct {
	Status    model.HostStatus `json:"status"`
	Tagline   string           `json:"tagline"`
	CreatedAt time.Time        `json:"created_at"`
}

func (s *Service) apply(c echo.Context) error {
	req, err := httpx.Bind[applyRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	var hp model.HostProfile
	if tx.Where("persona_id = ?", p.PersonaID).Limit(1).Find(&hp).RowsAffected == 1 {
		if hp.Status != model.HostRejected {
			return httpx.Conflict("you've already applied")
		}
		hp.Status = model.HostApplied
	}
	hp.PersonaID = p.PersonaID
	hp.Tagline = pii.Mask(req.Tagline, pii.Options{}).Text
	hp.Pitch = pii.Mask(req.Pitch, pii.Options{}).Text
	if hp.Status == "" {
		hp.Status = model.HostApplied
	}
	if err := tx.Save(&hp).Error; err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, HostDTO{Status: hp.Status, Tagline: hp.Tagline, CreatedAt: hp.CreatedAt})
}

func (s *Service) myApplication(c echo.Context) error {
	var hp model.HostProfile
	if s.DB.WithContext(c.Request().Context()).Where("persona_id = ?", httpx.PrincipalFrom(c).PersonaID).Limit(1).Find(&hp).RowsAffected == 0 {
		return c.JSON(http.StatusOK, map[string]any{"application": nil})
	}
	return c.JSON(http.StatusOK, map[string]any{"application": HostDTO{Status: hp.Status, Tagline: hp.Tagline, CreatedAt: hp.CreatedAt}})
}

func (s *Service) requireHost(tx *gorm.DB, persona uuid.UUID) error {
	var n int64
	tx.Model(&model.HostProfile{}).Where("persona_id = ? AND status = ?", persona, model.HostApproved).Count(&n)
	if n == 0 {
		return ErrNotHost
	}
	return nil
}

// --- Shows and episodes (F31) ---

type showRequest struct {
	Title       string   `json:"title" validate:"required,min=3,max=60"`
	Description string   `json:"description" validate:"max=300"`
	TopicTags   []string `json:"topic_tags" validate:"max=5,dive,min=2,max=24"`
	Schedule    string   `json:"schedule" validate:"max=120"`
}

type ShowDTO struct {
	ID            uuid.UUID           `json:"id"`
	Host          identity.PersonaRef `json:"host"`
	Title         string              `json:"title"`
	Description   string              `json:"description"`
	TopicTags     []string            `json:"topic_tags"`
	Schedule      *string             `json:"schedule"`
	FollowerCount int                 `json:"follower_count"`
	Following     bool                `json:"following"`
	IsMine        bool                `json:"is_mine"`
	Next          *EpisodeDTO         `json:"next_episode"`
}

type EpisodeDTO struct {
	ID            uuid.UUID           `json:"id"`
	ShowID        uuid.UUID           `json:"show_id"`
	ShowTitle     string              `json:"show_title"`
	Host          identity.PersonaRef `json:"host"`
	Title         string              `json:"title"`
	ScheduledAt   time.Time           `json:"scheduled_at"`
	Status        model.EpisodeStatus `json:"status"`
	Listeners     int64               `json:"listeners"`
	PeakListeners int                 `json:"peak_listeners"`
	MyRole        *model.StageRole    `json:"my_role"`
	OnAir         *CallInDTO          `json:"on_air"`
	MyCallIn      *CallInDTO          `json:"my_call_in"`
	QueueLength   int64               `json:"queue_length"`
	AutoMuted     bool                `json:"auto_muted"`
}

func (s *Service) createShow(c echo.Context) error {
	req, err := httpx.Bind[showRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	if err := s.requireHost(tx, p.PersonaID); err != nil {
		return err
	}
	var per model.Persona
	tx.Select("city_id").First(&per, "id = ?", p.PersonaID)
	show := model.Show{HostPersonaID: p.PersonaID, CityID: &per.CityID, Title: pii.Mask(req.Title, pii.Options{}).Text,
		Description: pii.Mask(req.Description, pii.Options{}).Text, TopicTags: req.TopicTags}
	if req.Schedule != "" {
		sc := req.Schedule
		show.ScheduleRRule = &sc
	}
	if show.TopicTags == nil {
		show.TopicTags = model.StringList{}
	}
	if err := tx.Create(&show).Error; err != nil {
		return err
	}
	return s.respondShow(c, &show)
}

func (s *Service) showDTOs(tx *gorm.DB, viewer uuid.UUID, shows []model.Show) ([]ShowDTO, error) {
	var hosts, ids []uuid.UUID
	for _, sh := range shows {
		hosts = append(hosts, sh.HostPersonaID)
		ids = append(ids, sh.ID)
	}
	refs, err := identity.LoadRefs(tx, hosts)
	if err != nil {
		return nil, err
	}
	following := map[uuid.UUID]bool{}
	if len(ids) > 0 {
		var fs []model.ShowFollower
		tx.Where("persona_id = ? AND show_id IN ?", viewer, ids).Find(&fs)
		for _, f := range fs {
			following[f.ShowID] = true
		}
	}
	out := make([]ShowDTO, len(shows))
	for i, sh := range shows {
		d := ShowDTO{ID: sh.ID, Host: refs[sh.HostPersonaID], Title: sh.Title, Description: sh.Description, TopicTags: sh.TopicTags,
			Schedule: sh.ScheduleRRule, FollowerCount: sh.FollowerCount, Following: following[sh.ID], IsMine: sh.HostPersonaID == viewer}
		if d.TopicTags == nil {
			d.TopicTags = []string{}
		}
		var ep model.ShowEpisode
		if tx.Where("show_id = ? AND status IN ?", sh.ID, []model.EpisodeStatus{model.EpisodeScheduled, model.EpisodeLive}).Order("scheduled_at").Limit(1).Find(&ep).RowsAffected == 1 {
			eps, err := s.episodeDTOs(tx, viewer, []model.ShowEpisode{ep})
			if err != nil {
				return nil, err
			}
			d.Next = &eps[0]
		}
		out[i] = d
	}
	return out, nil
}

func (s *Service) respondShow(c echo.Context, sh *model.Show) error {
	out, err := s.showDTOs(s.DB.WithContext(c.Request().Context()), httpx.PrincipalFrom(c).PersonaID, []model.Show{*sh})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out[0])
}

func (s *Service) listShows(c echo.Context) error {
	tx := s.DB.WithContext(c.Request().Context())
	q := tx.Model(&model.Show{}).Where("host_persona_id IN (SELECT persona_id FROM host_profiles WHERE status = ?)", model.HostApproved)
	if c.QueryParam("following") == "1" {
		q = q.Where("id IN (SELECT show_id FROM show_followers WHERE persona_id = ?)", httpx.PrincipalFrom(c).PersonaID)
	}
	if c.QueryParam("mine") == "1" {
		q = q.Where("host_persona_id = ?", httpx.PrincipalFrom(c).PersonaID)
	}
	var shows []model.Show
	if err := q.Order("follower_count DESC").Limit(100).Find(&shows).Error; err != nil {
		return err
	}
	out, err := s.showDTOs(tx, httpx.PrincipalFrom(c).PersonaID, shows)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (s *Service) loadShow(tx *gorm.DB, idParam string) (*model.Show, error) {
	id, err := uuid.Parse(idParam)
	if err != nil {
		return nil, httpx.NotFound()
	}
	var sh model.Show
	return &sh, tx.First(&sh, "id = ?", id).Error
}

func (s *Service) getShow(c echo.Context) error {
	sh, err := s.loadShow(s.DB.WithContext(c.Request().Context()), c.Param("id"))
	if err != nil {
		return err
	}
	return s.respondShow(c, sh)
}

func (s *Service) follow(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	var sh *model.Show
	err := s.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var err error
		if sh, err = s.loadShow(tx, c.Param("id")); err != nil {
			return err
		}
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.ShowFollower{ShowID: sh.ID, PersonaID: p.PersonaID})
		if res.Error != nil || res.RowsAffected == 0 {
			return res.Error
		}
		sh.FollowerCount++
		return tx.Model(sh).Update("follower_count", gorm.Expr("follower_count + 1")).Error
	})
	if err != nil {
		return err
	}
	return s.respondShow(c, sh)
}

func (s *Service) unfollow(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	var sh *model.Show
	err := s.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var err error
		if sh, err = s.loadShow(tx, c.Param("id")); err != nil {
			return err
		}
		res := tx.Where("show_id = ? AND persona_id = ?", sh.ID, p.PersonaID).Delete(&model.ShowFollower{})
		if res.Error != nil || res.RowsAffected == 0 {
			return res.Error
		}
		sh.FollowerCount--
		return tx.Model(sh).Update("follower_count", gorm.Expr("follower_count - 1")).Error
	})
	if err != nil {
		return err
	}
	return s.respondShow(c, sh)
}

type episodeRequest struct {
	Title       string    `json:"title" validate:"required,min=3,max=80"`
	ScheduledAt time.Time `json:"scheduled_at" validate:"required"`
}

func (s *Service) createEpisode(c echo.Context) error {
	req, err := httpx.Bind[episodeRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	sh, err := s.loadShow(tx, c.Param("id"))
	if err != nil {
		return err
	}
	if sh.HostPersonaID != p.PersonaID {
		return ErrNotYourShow
	}
	if err := s.requireHost(tx, p.PersonaID); err != nil {
		return err
	}
	if req.ScheduledAt.Before(s.now().Add(-time.Minute)) || req.ScheduledAt.After(s.now().Add(60*24*time.Hour)) {
		return httpx.Unprocessable("invalid_time", "schedule within the next 60 days")
	}
	ep := model.ShowEpisode{ShowID: sh.ID, Title: pii.Mask(req.Title, pii.Options{}).Text, ScheduledAt: req.ScheduledAt.UTC(), Status: model.EpisodeScheduled}
	ep.ID = model.NewID()
	ep.LiveKitRoom = "ep_" + ep.ID.String()
	if err := tx.Create(&ep).Error; err != nil {
		return err
	}
	return s.respondEpisode(c, &ep)
}

func (s *Service) episodeDTOs(tx *gorm.DB, viewer uuid.UUID, eps []model.ShowEpisode) ([]EpisodeDTO, error) {
	out := make([]EpisodeDTO, len(eps))
	for i, ep := range eps {
		var sh model.Show
		tx.Unscoped().First(&sh, "id = ?", ep.ShowID)
		refs, err := identity.LoadRefs(tx, []uuid.UUID{sh.HostPersonaID})
		if err != nil {
			return nil, err
		}
		d := EpisodeDTO{ID: ep.ID, ShowID: sh.ID, ShowTitle: sh.Title, Host: refs[sh.HostPersonaID], Title: ep.Title, ScheduledAt: ep.ScheduledAt,
			Status: ep.Status, PeakListeners: ep.PeakListeners, AutoMuted: ep.AutoMutedAt != nil}
		tx.Model(&model.EpisodeParticipant{}).Where("episode_id = ? AND left_at IS NULL", ep.ID).Count(&d.Listeners)
		var me model.EpisodeParticipant
		if tx.Where("episode_id = ? AND persona_id = ? AND left_at IS NULL", ep.ID, viewer).Limit(1).Find(&me).RowsAffected == 1 {
			d.MyRole = &me.Role
		}
		var ci model.CallInRequest
		if tx.Where("episode_id = ? AND status = ?", ep.ID, model.CallInOnAir).Limit(1).Find(&ci).RowsAffected == 1 {
			cd, err := s.callInDTO(tx, &ci, false)
			if err != nil {
				return nil, err
			}
			d.OnAir = cd
		}
		var mine model.CallInRequest
		if tx.Where("episode_id = ? AND persona_id = ? AND status IN ?", ep.ID, viewer, []model.CallInStatus{model.CallInQueued, model.CallInOnAir}).Limit(1).Find(&mine).RowsAffected == 1 {
			cd, err := s.callInDTO(tx, &mine, true)
			if err != nil {
				return nil, err
			}
			d.MyCallIn = cd
		}
		tx.Model(&model.CallInRequest{}).Where("episode_id = ? AND status = ?", ep.ID, model.CallInQueued).Count(&d.QueueLength)
		out[i] = d
	}
	return out, nil
}

func (s *Service) respondEpisode(c echo.Context, ep *model.ShowEpisode) error {
	out, err := s.episodeDTOs(s.DB.WithContext(c.Request().Context()), httpx.PrincipalFrom(c).PersonaID, []model.ShowEpisode{*ep})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out[0])
}

// schedule lists live episodes first, then what's coming up.
func (s *Service) schedule(c echo.Context) error {
	tx := s.DB.WithContext(c.Request().Context())
	q := tx.Where("status = ? OR (status = ? AND scheduled_at > ?)", model.EpisodeLive, model.EpisodeScheduled, s.now().Add(-time.Hour))
	if c.QueryParam("live") == "1" {
		q = tx.Where("status = ?", model.EpisodeLive)
	}
	var eps []model.ShowEpisode
	if err := q.Order("CASE WHEN status = 'live' THEN 0 ELSE 1 END").Order("scheduled_at").Limit(50).Find(&eps).Error; err != nil {
		return err
	}
	out, err := s.episodeDTOs(tx, httpx.PrincipalFrom(c).PersonaID, eps)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (s *Service) loadEpisode(tx *gorm.DB, idParam string) (*model.ShowEpisode, *model.Show, error) {
	id, err := uuid.Parse(idParam)
	if err != nil {
		return nil, nil, httpx.NotFound()
	}
	var ep model.ShowEpisode
	if err := tx.First(&ep, "id = ?", id).Error; err != nil {
		return nil, nil, err
	}
	var sh model.Show
	if err := tx.Unscoped().First(&sh, "id = ?", ep.ShowID).Error; err != nil {
		return nil, nil, err
	}
	return &ep, &sh, nil
}

func (s *Service) getEpisode(c echo.Context) error {
	ep, _, err := s.loadEpisode(s.DB.WithContext(c.Request().Context()), c.Param("id"))
	if err != nil {
		return err
	}
	return s.respondEpisode(c, ep)
}

// SendReminders pushes followers 10 minutes before an episode starts.
func (s *Service) SendReminders(ctx context.Context) error {
	now := s.now()
	tx := s.DB.WithContext(ctx)
	var eps []model.ShowEpisode
	tx.Where("status = ? AND reminder_sent_at IS NULL AND scheduled_at > ? AND scheduled_at <= ?", model.EpisodeScheduled, now, now.Add(reminderLead)).Find(&eps)
	for _, ep := range eps {
		res := tx.Model(&model.ShowEpisode{}).Where("id = ? AND reminder_sent_at IS NULL", ep.ID).Update("reminder_sent_at", now)
		if res.RowsAffected == 0 {
			continue
		}
		var sh model.Show
		tx.First(&sh, "id = ?", ep.ShowID)
		var followers []uuid.UUID
		tx.Model(&model.ShowFollower{}).Where("show_id = ?", ep.ShowID).Pluck("persona_id", &followers)
		for _, f := range followers {
			s.Notify.Notify(ctx, f, model.NotifEpisodeLive, model.JSONMap{"episode_id": ep.ID.String(), "show": sh.Title, "soon": true},
				notify.Content{Title: sh.Title + " starts in 10 minutes", Body: ep.Title, Link: "metrobuddy://episode/" + ep.ID.String()}, true)
		}
	}
	return nil
}

// --- Live rooms (F32) ---

func (s *Service) role(tx *gorm.DB, ep *model.ShowEpisode, sh *model.Show, persona uuid.UUID) model.StageRole {
	if sh.HostPersonaID == persona {
		return model.StageHost
	}
	var p model.EpisodeParticipant
	if tx.Where("episode_id = ? AND persona_id = ?", ep.ID, persona).Limit(1).Find(&p).RowsAffected == 1 && p.Role == model.StageCoHost {
		return model.StageCoHost
	}
	return model.StageListener
}

func (s *Service) start(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var ep *model.ShowEpisode
	var followers []uuid.UUID
	var title string
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sh *model.Show
		var err error
		if ep, sh, err = s.loadEpisode(tx, c.Param("id")); err != nil {
			return err
		}
		if sh.HostPersonaID != p.PersonaID {
			return ErrNotYourShow
		}
		if err := s.requireHost(tx, p.PersonaID); err != nil {
			return err
		}
		if ep.Status != model.EpisodeScheduled {
			return httpx.Conflict("this episode already started or ended")
		}
		if s.now().Before(ep.ScheduledAt.Add(-startEarly)) {
			return httpx.Conflict("you can go live 15 minutes before the scheduled time")
		}
		now := s.now()
		ep.Status, ep.StartedAt = model.EpisodeLive, &now
		if err := tx.Model(ep).Updates(map[string]any{"status": model.EpisodeLive, "started_at": now}).Error; err != nil {
			return err
		}
		title = sh.Title
		return tx.Model(&model.ShowFollower{}).Where("show_id = ?", sh.ID).Pluck("persona_id", &followers).Error
	})
	if err != nil {
		return err
	}
	for _, f := range followers {
		s.Notify.Notify(ctx, f, model.NotifEpisodeLive, model.JSONMap{"episode_id": ep.ID.String(), "show": title},
			notify.Content{Title: title + " is live", Body: "Tap to listen in", Link: "metrobuddy://episode/" + ep.ID.String()}, true)
	}
	return s.respondEpisode(c, ep)
}

func (s *Service) end(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	ep, sh, err := s.loadEpisode(s.DB.WithContext(ctx), c.Param("id"))
	if err != nil {
		return err
	}
	if s.role(s.DB.WithContext(ctx), ep, sh, p.PersonaID) == model.StageListener {
		return ErrNotYourShow
	}
	if err := s.finish(ctx, ep); err != nil {
		return err
	}
	return s.respondEpisode(c, ep)
}

func (s *Service) finish(ctx context.Context, ep *model.ShowEpisode) error {
	now := s.now()
	audience := s.listeners(s.DB.WithContext(ctx), ep.ID)
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.ShowEpisode{}).Where("id = ? AND status = ?", ep.ID, model.EpisodeLive).Updates(map[string]any{"status": model.EpisodeEnded, "ended_at": now})
		if res.Error != nil || res.RowsAffected == 0 {
			return res.Error
		}
		if err := tx.Model(&model.CallInRequest{}).Where("episode_id = ? AND status IN ?", ep.ID, []model.CallInStatus{model.CallInQueued, model.CallInOnAir}).
			Updates(map[string]any{"status": model.CallInDone, "off_air_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&model.EpisodeParticipant{}).Where("episode_id = ? AND left_at IS NULL", ep.ID).Update("left_at", now).Error
	})
	if err != nil {
		return err
	}
	ep.Status = model.EpisodeEnded
	s.Hub.Publish(ctx, audience, "episode.ended", map[string]any{"episode_id": ep.ID})
	return nil
}

// ForceEnd is the admin kill switch: it ends the episode and closes the LiveKit room.
func (s *Service) ForceEnd(ctx context.Context, id uuid.UUID) error {
	var ep model.ShowEpisode
	if err := s.DB.WithContext(ctx).First(&ep, "id = ?", id).Error; err != nil {
		return err
	}
	if ep.Status != model.EpisodeLive {
		return ErrNotLive
	}
	if err := s.finish(ctx, &ep); err != nil {
		return err
	}
	_ = s.Live.CloseRoom(ctx, ep.LiveKitRoom)
	return nil
}

// EndStale closes episodes left live with nobody in them, and marks no-shows as cancelled.
func (s *Service) EndStale(ctx context.Context) error {
	tx := s.DB.WithContext(ctx)
	now := s.now()
	var eps []model.ShowEpisode
	tx.Where("status = ? AND started_at < ?", model.EpisodeLive, now.Add(-30*time.Minute)).Find(&eps)
	for i := range eps {
		var n int64
		tx.Model(&model.EpisodeParticipant{}).Where("episode_id = ? AND left_at IS NULL", eps[i].ID).Count(&n)
		if n == 0 || now.Sub(*eps[i].StartedAt) > 6*time.Hour {
			_ = s.finish(ctx, &eps[i])
			_ = s.Live.CloseRoom(ctx, eps[i].LiveKitRoom)
		}
	}
	return tx.Model(&model.ShowEpisode{}).Where("status = ? AND scheduled_at < ?", model.EpisodeScheduled, now.Add(-2*time.Hour)).
		Update("status", model.EpisodeCancelled).Error
}

type JoinDTO struct {
	URL   string          `json:"url"`
	Token string          `json:"token"`
	Room  string          `json:"room"`
	Role  model.StageRole `json:"role"`
}

// join issues a LiveKit token for the caller's role. Listeners get subscribe-only tokens.
func (s *Service) join(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	tx := s.DB.WithContext(ctx)
	ep, sh, err := s.loadEpisode(tx, c.Param("id"))
	if err != nil {
		return err
	}
	if ep.Status != model.EpisodeLive {
		return ErrNotLive
	}
	var removed int64
	tx.Model(&model.EpisodeParticipant{}).Where("episode_id = ? AND persona_id = ? AND left_at IS NOT NULL AND role = ?", ep.ID, p.PersonaID, "removed").Count(&removed)
	if removed > 0 {
		return httpx.Forbidden("the host removed you from this episode")
	}
	role := s.role(tx, ep, sh, p.PersonaID)
	refs, err := identity.LoadRefs(tx, []uuid.UUID{p.PersonaID})
	if err != nil {
		return err
	}
	publish := role == model.StageHost || role == model.StageCoHost
	tok, err := s.Live.Token(ep.LiveKitRoom, p.PersonaID.String(), refs[p.PersonaID].Alias, publish, role == model.StageHost)
	if err != nil {
		return err
	}
	if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "episode_id"}, {Name: "persona_id"}},
		DoUpdates: clause.Assignments(map[string]any{"left_at": nil, "joined_at": s.now()})}).
		Create(&model.EpisodeParticipant{EpisodeID: ep.ID, PersonaID: p.PersonaID, Role: role, JoinedAt: s.now()}).Error; err != nil {
		return err
	}
	s.updatePeak(tx, ep.ID)
	return c.JSON(http.StatusOK, JoinDTO{URL: s.Live.URL(), Token: tok, Room: ep.LiveKitRoom, Role: role})
}

func (s *Service) updatePeak(tx *gorm.DB, id uuid.UUID) {
	var n int64
	tx.Model(&model.EpisodeParticipant{}).Where("episode_id = ? AND left_at IS NULL", id).Count(&n)
	tx.Model(&model.ShowEpisode{}).Where("id = ? AND peak_listeners < ?", id, n).Update("peak_listeners", n)
}

func (s *Service) leave(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	tx.Model(&model.EpisodeParticipant{}).Where("episode_id = ? AND persona_id = ? AND left_at IS NULL", id, p.PersonaID).Update("left_at", s.now())
	var ci model.CallInRequest
	if tx.Where("episode_id = ? AND persona_id = ? AND status IN ?", id, p.PersonaID, []model.CallInStatus{model.CallInQueued, model.CallInOnAir}).Limit(1).Find(&ci).RowsAffected == 1 {
		s.dropCall(c.Request().Context(), &ci, model.CallInWithdrawn)
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Service) listeners(tx *gorm.DB, episode uuid.UUID) []uuid.UUID {
	var ids []uuid.UUID
	tx.Model(&model.EpisodeParticipant{}).Where("episode_id = ? AND left_at IS NULL", episode).Pluck("persona_id", &ids)
	return ids
}

func (s *Service) staff(tx *gorm.DB, ep *model.ShowEpisode) []uuid.UUID {
	var sh model.Show
	tx.Unscoped().First(&sh, "id = ?", ep.ShowID)
	ids := []uuid.UUID{sh.HostPersonaID}
	var co []uuid.UUID
	tx.Model(&model.EpisodeParticipant{}).Where("episode_id = ? AND role = ?", ep.ID, model.StageCoHost).Pluck("persona_id", &co)
	return append(ids, co...)
}

func (s *Service) broadcast(ctx context.Context, episode uuid.UUID, typ string, data any) {
	s.Hub.Publish(ctx, s.listeners(s.DB.WithContext(ctx), episode), typ, data)
}

// webhook keeps participant state in sync with LiveKit and stops any recording attempt.
func (s *Service) webhook(c echo.Context) error {
	ev, err := s.Live.Webhook(c.Request())
	if err != nil {
		return httpx.Unauthorized()
	}
	ctx := c.Request().Context()
	tx := s.DB.WithContext(ctx)
	var ep model.ShowEpisode
	if ev.Room != nil {
		tx.Where("live_kit_room = ?", ev.Room.Name).Limit(1).Find(&ep)
	}
	switch ev.Event {
	case "participant_left":
		if ep.ID != uuid.Nil && ev.Participant != nil {
			if pid, err := uuid.Parse(ev.Participant.Identity); err == nil {
				tx.Model(&model.EpisodeParticipant{}).Where("episode_id = ? AND persona_id = ? AND left_at IS NULL", ep.ID, pid).Update("left_at", s.now())
				var ci model.CallInRequest
				if tx.Where("episode_id = ? AND persona_id = ? AND status = ?", ep.ID, pid, model.CallInOnAir).Limit(1).Find(&ci).RowsAffected == 1 {
					s.dropCall(ctx, &ci, model.CallInDropped)
				}
			}
		}
	case "room_finished":
		if ep.ID != uuid.Nil {
			_ = s.finish(ctx, &ep)
		}
	case "egress_started":
		// Shows are never recorded. Any egress is stopped and logged.
		if ev.EgressInfo != nil {
			_ = s.Live.StopEgress(ctx, ev.EgressInfo.EgressId)
			s.Audit.Log(tx, model.ActorSystem, nil, "studio.egress_blocked", "episode", &ep.ID, model.JSONMap{"egress_id": ev.EgressInfo.EgressId})
		}
	}
	return c.NoContent(http.StatusOK)
}

// --- Call-in queue (F33) ---

type CallInDTO struct {
	ID        uuid.UUID           `json:"id"`
	Persona   identity.PersonaRef `json:"persona"`
	Topic     string              `json:"topic,omitempty"`
	Status    model.CallInStatus  `json:"status"`
	OnAirAt   *time.Time          `json:"on_air_at"`
	EndsAt    *time.Time          `json:"ends_at"`
	Masked    bool                `json:"voice_masked"`
	CreatedAt time.Time           `json:"created_at"`
}

// Topics are for the host's eyes; listeners only see who is on air.
func (s *Service) callInDTO(tx *gorm.DB, ci *model.CallInRequest, withTopic bool) (*CallInDTO, error) {
	refs, err := identity.LoadRefs(tx, []uuid.UUID{ci.PersonaID})
	if err != nil {
		return nil, err
	}
	d := &CallInDTO{ID: ci.ID, Persona: refs[ci.PersonaID], Status: ci.Status, OnAirAt: ci.OnAirAt, CreatedAt: ci.CreatedAt, Masked: ci.VoiceMasked}
	if withTopic {
		d.Topic = ci.TopicMasked
	}
	if ci.OnAirAt != nil {
		end := ci.OnAirAt.Add(time.Duration(ci.MaxSeconds) * time.Second)
		d.EndsAt = &end
	}
	return d, nil
}

type callInRequest struct {
	Topic       string `json:"topic" validate:"required,min=3,max=80"`
	VoiceMasked bool   `json:"voice_masked"`
}

func (s *Service) callIn(c echo.Context) error {
	req, err := httpx.Bind[callInRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var ci model.CallInRequest
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ep, sh, err := s.loadEpisode(tx, c.Param("id"))
		if err != nil {
			return err
		}
		if ep.Status != model.EpisodeLive {
			return ErrNotLive
		}
		if sh.HostPersonaID == p.PersonaID {
			return httpx.BadRequest("hosts don't call in to their own show")
		}
		var n int64
		tx.Model(&model.CallInRequest{}).Where("episode_id = ? AND persona_id = ? AND status IN ?", ep.ID, p.PersonaID,
			[]model.CallInStatus{model.CallInQueued, model.CallInOnAir}).Count(&n)
		if n > 0 {
			return httpx.Conflict("you're already in the queue")
		}
		ci = model.CallInRequest{EpisodeID: ep.ID, PersonaID: p.PersonaID, Status: model.CallInQueued, MaxSeconds: int(MaxOnAir.Seconds()), VoiceMasked: req.VoiceMasked}
		ci.ID = model.NewID()
		r, err := s.PII.Process(tx, model.CTCallIn, ci.ID, req.Topic, pii.Options{})
		if err != nil {
			return err
		}
		ci.TopicMasked = r.Text
		return tx.Create(&ci).Error
	})
	if err != nil {
		return err
	}
	tx := s.DB.WithContext(ctx)
	d, err := s.callInDTO(tx, &ci, true)
	if err != nil {
		return err
	}
	var ep model.ShowEpisode
	tx.First(&ep, "id = ?", ci.EpisodeID)
	s.Hub.Publish(ctx, s.staff(tx, &ep), "episode.queue", map[string]any{"episode_id": ep.ID, "call_in": d})
	return c.JSON(http.StatusCreated, d)
}

func (s *Service) withdraw(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	var ci model.CallInRequest
	if s.DB.WithContext(c.Request().Context()).Where("episode_id = ? AND persona_id = ? AND status IN ?", id, p.PersonaID,
		[]model.CallInStatus{model.CallInQueued, model.CallInOnAir}).Limit(1).Find(&ci).RowsAffected == 0 {
		return httpx.NotFound()
	}
	s.dropCall(c.Request().Context(), &ci, model.CallInWithdrawn)
	return c.NoContent(http.StatusNoContent)
}

func (s *Service) requireStaff(tx *gorm.DB, idParam string, persona uuid.UUID) (*model.ShowEpisode, error) {
	ep, sh, err := s.loadEpisode(tx, idParam)
	if err != nil {
		return nil, err
	}
	if s.role(tx, ep, sh, persona) == model.StageListener {
		return nil, ErrNotYourShow
	}
	if ep.Status != model.EpisodeLive {
		return nil, ErrNotLive
	}
	return ep, nil
}

func (s *Service) queue(c echo.Context) error {
	tx := s.DB.WithContext(c.Request().Context())
	ep, err := s.requireStaff(tx, c.Param("id"), httpx.PrincipalFrom(c).PersonaID)
	if err != nil {
		return err
	}
	var cis []model.CallInRequest
	tx.Where("episode_id = ? AND status IN ?", ep.ID, []model.CallInStatus{model.CallInQueued, model.CallInOnAir}).Order("created_at").Find(&cis)
	out := make([]*CallInDTO, 0, len(cis))
	for i := range cis {
		d, err := s.callInDTO(tx, &cis[i], true)
		if err != nil {
			return err
		}
		out = append(out, d)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

type stageRequest struct {
	CallInID uuid.UUID `json:"call_in_id" validate:"required"`
}

// stage puts one caller on air: anyone already on air is dropped first, and only then does
// the new caller get publish rights.
func (s *Service) stage(c echo.Context) error {
	req, err := httpx.Bind[stageRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	tx := s.DB.WithContext(ctx)
	ep, err := s.requireStaff(tx, c.Param("id"), p.PersonaID)
	if err != nil {
		return err
	}
	var ci model.CallInRequest
	if err := tx.First(&ci, "id = ? AND episode_id = ? AND status = ?", req.CallInID, ep.ID, model.CallInQueued).Error; err != nil {
		return httpx.NotFound()
	}
	var current []model.CallInRequest
	tx.Where("episode_id = ? AND status = ?", ep.ID, model.CallInOnAir).Find(&current)
	for i := range current {
		s.dropCall(ctx, &current[i], model.CallInDone)
	}
	if err := s.Live.SetPublish(ctx, ep.LiveKitRoom, ci.PersonaID.String(), true); err != nil {
		return httpx.E(http.StatusBadGateway, "live_unavailable", "couldn't bring the caller on air, try again")
	}
	now := s.now()
	tx.Model(&ci).Updates(map[string]any{"status": model.CallInOnAir, "on_air_at": now})
	ci.Status, ci.OnAirAt = model.CallInOnAir, &now
	tx.Model(&model.EpisodeParticipant{}).Where("episode_id = ? AND persona_id = ?", ep.ID, ci.PersonaID).Update("role", model.StageOnAir)
	d, _ := s.callInDTO(tx, &ci, false)
	s.broadcast(ctx, ep.ID, "episode.stage", map[string]any{"episode_id": ep.ID, "on_air": d})
	return c.JSON(http.StatusOK, d)
}

func (s *Service) unstage(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	tx := s.DB.WithContext(ctx)
	ep, err := s.requireStaff(tx, c.Param("id"), p.PersonaID)
	if err != nil {
		return err
	}
	var ci model.CallInRequest
	if tx.Where("episode_id = ? AND status = ?", ep.ID, model.CallInOnAir).Limit(1).Find(&ci).RowsAffected == 0 {
		return httpx.NotFound()
	}
	s.dropCall(ctx, &ci, model.CallInDone)
	return c.NoContent(http.StatusNoContent)
}

// dropCall revokes publish rights at once (the caller's mic goes dead) and records why.
func (s *Service) dropCall(ctx context.Context, ci *model.CallInRequest, status model.CallInStatus) {
	tx := s.DB.WithContext(ctx)
	wasOnAir := ci.Status == model.CallInOnAir
	var ep model.ShowEpisode
	if tx.First(&ep, "id = ?", ci.EpisodeID).Error != nil {
		return
	}
	if wasOnAir {
		if err := s.Live.SetPublish(ctx, ep.LiveKitRoom, ci.PersonaID.String(), false); err != nil && s.Log != nil {
			// Fall back to removing them from the room: an on-air mic must never stay open.
			s.Log.Warn("revoke publish failed, removing participant", "err", err.Error())
			_ = s.Live.Remove(ctx, ep.LiveKitRoom, ci.PersonaID.String())
		}
		tx.Model(&model.EpisodeParticipant{}).Where("episode_id = ? AND persona_id = ? AND role = ?", ep.ID, ci.PersonaID, model.StageOnAir).
			Update("role", model.StageListener)
	}
	tx.Model(&model.CallInRequest{}).Where("id = ?", ci.ID).Updates(map[string]any{"status": status, "off_air_at": s.now()})
	ci.Status = status
	if wasOnAir {
		s.broadcast(ctx, ep.ID, "episode.stage", map[string]any{"episode_id": ep.ID, "on_air": nil})
	} else {
		s.Hub.Publish(ctx, s.staff(tx, &ep), "episode.queue", map[string]any{"episode_id": ep.ID, "removed": ci.ID})
	}
}

type extendRequest struct {
	Minutes int `json:"minutes" validate:"required,min=1,max=5"`
}

func (s *Service) extend(c echo.Context) error {
	req, err := httpx.Bind[extendRequest](c)
	if err != nil {
		return err
	}
	tx := s.DB.WithContext(c.Request().Context())
	ep, err := s.requireStaff(tx, c.Param("id"), httpx.PrincipalFrom(c).PersonaID)
	if err != nil {
		return err
	}
	var ci model.CallInRequest
	if tx.Where("episode_id = ? AND status = ?", ep.ID, model.CallInOnAir).Limit(1).Find(&ci).RowsAffected == 0 {
		return httpx.NotFound()
	}
	if ci.MaxSeconds+req.Minutes*60 > int((MaxOnAir + MaxExtend).Seconds()) {
		return httpx.Unprocessable("max_extension", "a caller can be extended by 5 minutes in total")
	}
	tx.Model(&ci).Update("max_seconds", ci.MaxSeconds+req.Minutes*60)
	ci.MaxSeconds += req.Minutes * 60
	d, _ := s.callInDTO(tx, &ci, false)
	s.broadcast(c.Request().Context(), ep.ID, "episode.stage", map[string]any{"episode_id": ep.ID, "on_air": d})
	return c.JSON(http.StatusOK, d)
}

// EnforceOnAirCap drops callers who reach their time limit; runs every few seconds.
func (s *Service) EnforceOnAirCap(ctx context.Context) error {
	var cis []model.CallInRequest
	if err := s.DB.WithContext(ctx).Where("status = ?", model.CallInOnAir).Find(&cis).Error; err != nil {
		return err
	}
	now := s.now()
	for i := range cis {
		if cis[i].OnAirAt != nil && now.After(cis[i].OnAirAt.Add(time.Duration(cis[i].MaxSeconds)*time.Second)) {
			s.dropCall(ctx, &cis[i], model.CallInDone)
		}
	}
	return nil
}

// --- On-air safety (F36) ---

func (s *Service) muteCaller(c echo.Context) error {
	tx := s.DB.WithContext(c.Request().Context())
	ep, err := s.requireStaff(tx, c.Param("id"), httpx.PrincipalFrom(c).PersonaID)
	if err != nil {
		return err
	}
	var ci model.CallInRequest
	if tx.Where("episode_id = ? AND status = ?", ep.ID, model.CallInOnAir).Limit(1).Find(&ci).RowsAffected == 0 {
		return httpx.NotFound()
	}
	s.dropCall(c.Request().Context(), &ci, model.CallInDropped)
	return c.NoContent(http.StatusNoContent)
}

type removeRequest struct {
	PersonaID uuid.UUID `json:"persona_id" validate:"required"`
}

func (s *Service) removeListener(c echo.Context) error {
	req, err := httpx.Bind[removeRequest](c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	tx := s.DB.WithContext(ctx)
	p := httpx.PrincipalFrom(c)
	ep, err := s.requireStaff(tx, c.Param("id"), p.PersonaID)
	if err != nil {
		return err
	}
	if slices.Contains(s.staff(tx, ep), req.PersonaID) {
		return httpx.BadRequest("you can't remove a host")
	}
	var ci model.CallInRequest
	if tx.Where("episode_id = ? AND persona_id = ? AND status IN ?", ep.ID, req.PersonaID, []model.CallInStatus{model.CallInQueued, model.CallInOnAir}).Limit(1).Find(&ci).RowsAffected == 1 {
		s.dropCall(ctx, &ci, model.CallInDropped)
	}
	_ = s.Live.Remove(ctx, ep.LiveKitRoom, req.PersonaID.String())
	tx.Model(&model.EpisodeParticipant{}).Where("episode_id = ? AND persona_id = ?", ep.ID, req.PersonaID).
		Updates(map[string]any{"left_at": s.now(), "role": "removed"})
	s.Audit.Log(tx, model.ActorUser, &p.UserID, "studio.remove_listener", "episode", &ep.ID, model.JSONMap{"persona_id": req.PersonaID.String()})
	return c.NoContent(http.StatusNoContent)
}

// OnReport is called for every new report; a burst of reports against the live episode
// or the caller on air auto-mutes the caller until the host decides what to do.
func (s *Service) OnReport(ctx context.Context, ct model.ContentType, target uuid.UUID) {
	if ct != model.CTEpisode && ct != model.CTCallIn {
		return
	}
	tx := s.DB.WithContext(ctx)
	var ep model.ShowEpisode
	if ct == model.CTEpisode {
		if tx.First(&ep, "id = ?", target).Error != nil {
			return
		}
	} else {
		var ci model.CallInRequest
		if tx.First(&ci, "id = ?", target).Error != nil || tx.First(&ep, "id = ?", ci.EpisodeID).Error != nil {
			return
		}
	}
	if ep.Status != model.EpisodeLive {
		return
	}
	var onAir model.CallInRequest
	if tx.Where("episode_id = ? AND status = ?", ep.ID, model.CallInOnAir).Limit(1).Find(&onAir).RowsAffected == 0 {
		return
	}
	since := s.now().Add(-spikeWindow)
	if onAir.OnAirAt != nil && onAir.OnAirAt.After(since) {
		since = *onAir.OnAirAt
	}
	var n int64
	tx.Model(&model.Report{}).Where("created_at >= ? AND ((target_type = ? AND target_id = ?) OR (target_type = ? AND target_id = ?))",
		since, model.CTEpisode, ep.ID, model.CTCallIn, onAir.ID).Distinct("reporter_persona_id").Count(&n)
	if n < SpikeThreshold {
		return
	}
	s.dropCall(ctx, &onAir, model.CallInDropped)
	tx.Model(&model.ShowEpisode{}).Where("id = ?", ep.ID).Update("auto_muted_at", s.now())
	s.Audit.Log(tx, model.ActorSystem, nil, "studio.auto_mute", "episode", &ep.ID, model.JSONMap{"call_in_id": onAir.ID.String(), "reports": n})
	s.Hub.Publish(ctx, s.staff(tx, &ep), "episode.auto_muted", map[string]any{"episode_id": ep.ID, "reports": n})
}

// --- Reactions and questions (F35) ---

type reactRequest struct {
	Reaction string `json:"reaction" validate:"required"`
}

func (s *Service) react(c echo.Context) error {
	req, err := httpx.Bind[reactRequest](c)
	if err != nil {
		return err
	}
	if !slices.Contains(Reactions, req.Reaction) {
		return httpx.Unprocessable("invalid_reaction", "unknown reaction")
	}
	ctx := c.Request().Context()
	ep, _, err := s.loadEpisode(s.DB.WithContext(ctx), c.Param("id"))
	if err != nil {
		return err
	}
	if ep.Status != model.EpisodeLive {
		return ErrNotLive
	}
	s.broadcast(ctx, ep.ID, "episode.reaction", map[string]any{"episode_id": ep.ID, "reaction": req.Reaction})
	return c.NoContent(http.StatusNoContent)
}

type askRequest struct {
	Text string `json:"text" validate:"required,min=3,max=200"`
}

type QuestionDTO struct {
	ID        uuid.UUID           `json:"id"`
	Persona   identity.PersonaRef `json:"persona"`
	Text      string              `json:"text"`
	Answered  bool                `json:"answered"`
	CreatedAt time.Time           `json:"created_at"`
}

// ask: text questions go through the PII masker like every other piece of UGC.
func (s *Service) ask(c echo.Context) error {
	req, err := httpx.Bind[askRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var q model.EpisodeQuestion
	var ep *model.ShowEpisode
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		if ep, _, err = s.loadEpisode(tx, c.Param("id")); err != nil {
			return err
		}
		if ep.Status != model.EpisodeLive {
			return ErrNotLive
		}
		q = model.EpisodeQuestion{EpisodeID: ep.ID, PersonaID: p.PersonaID, Status: model.ContentVisible}
		q.ID = model.NewID()
		r, err := s.PII.Process(tx, model.CTEpisode, q.ID, req.Text, pii.Options{})
		if err != nil {
			return err
		}
		q.TextMasked, q.PIIFlags = r.Text, r.Flags
		return tx.Create(&q).Error
	})
	if err != nil {
		return err
	}
	tx := s.DB.WithContext(ctx)
	refs, _ := identity.LoadRefs(tx, []uuid.UUID{p.PersonaID})
	d := QuestionDTO{ID: q.ID, Persona: refs[p.PersonaID], Text: q.TextMasked, CreatedAt: q.CreatedAt}
	s.Hub.Publish(ctx, s.staff(tx, ep), "episode.question", map[string]any{"episode_id": ep.ID, "question": d})
	return c.JSON(http.StatusCreated, d)
}

func (s *Service) questions(c echo.Context) error {
	tx := s.DB.WithContext(c.Request().Context())
	ep, err := s.requireStaff(tx, c.Param("id"), httpx.PrincipalFrom(c).PersonaID)
	if err != nil {
		return err
	}
	var qs []model.EpisodeQuestion
	tx.Where("episode_id = ? AND status = ?", ep.ID, model.ContentVisible).Order("created_at DESC").Limit(200).Find(&qs)
	var ids []uuid.UUID
	for _, q := range qs {
		ids = append(ids, q.PersonaID)
	}
	refs, err := identity.LoadRefs(tx, ids)
	if err != nil {
		return err
	}
	out := make([]QuestionDTO, len(qs))
	for i, q := range qs {
		out[i] = QuestionDTO{ID: q.ID, Persona: refs[q.PersonaID], Text: q.TextMasked, Answered: q.Answered, CreatedAt: q.CreatedAt}
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

func (s *Service) answered(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	tx := s.DB.WithContext(c.Request().Context())
	var q model.EpisodeQuestion
	if err := tx.First(&q, "id = ?", id).Error; err != nil {
		return err
	}
	if _, err := s.requireStaff(tx, q.EpisodeID.String(), httpx.PrincipalFrom(c).PersonaID); err != nil {
		return err
	}
	tx.Model(&q).Update("answered", true)
	return c.NoContent(http.StatusNoContent)
}
