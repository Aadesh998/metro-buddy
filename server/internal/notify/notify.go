// Package notify fans a notification out to the in-app inbox, the live socket and Expo push.
// Push text never contains message bodies or anything identifying: lock screens are public.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/db"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
)

var ist = time.FixedZone("IST", 5*3600+1800)

type Publisher interface {
	Publish(ctx context.Context, personas []uuid.UUID, typ string, data any)
}

type PushMessage struct {
	To        string         `json:"to"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Data      map[string]any `json:"data,omitempty"`
	Sound     string         `json:"sound,omitempty"`
	ChannelID string         `json:"channelId,omitempty"`
	Priority  string         `json:"priority,omitempty"`
}

type PushTicket struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Details struct {
		Error string `json:"error"`
	} `json:"details"`
}

type Pusher interface {
	Send(ctx context.Context, msgs []PushMessage) ([]PushTicket, error)
}

type Expo struct {
	AccessToken string
	Client      *http.Client
}

func (e Expo) Send(ctx context.Context, msgs []PushMessage) ([]PushTicket, error) {
	b, _ := json.Marshal(msgs)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://exp.host/--/api/v2/push/send", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if e.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+e.AccessToken)
	}
	c := e.Client
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expo push status %d", resp.StatusCode)
	}
	var out struct {
		Data []PushTicket `json:"data"`
	}
	return out.Data, json.NewDecoder(resp.Body).Decode(&out)
}

type Service struct {
	DB     *gorm.DB
	Hub    Publisher
	Pusher Pusher
	Log    *slog.Logger
	// Online reports whether the persona has a live socket; live users get no message pushes.
	Online func(ctx context.Context, persona uuid.UUID) bool
	Now    func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

// Push copy per kind. Only aliases appear, never real data or message content.
type Content struct {
	Title string
	Body  string
	Link  string
}

// Notify records an inbox item (unless inbox is false), emits it live, and pushes if allowed.
func (s *Service) Notify(ctx context.Context, persona uuid.UUID, kind model.NotificationKind, payload model.JSONMap, c Content, inbox bool) {
	if inbox {
		n := model.Notification{PersonaID: persona, Kind: kind, Payload: withLink(payload, c.Link)}
		if err := s.DB.WithContext(ctx).Create(&n).Error; err != nil {
			s.Log.Error("notification write failed", "err", err.Error())
			return
		}
		s.Hub.Publish(ctx, []uuid.UUID{persona}, "notification.new", notificationDTO(n))
	}
	s.push(ctx, persona, kind, c)
}

func withLink(p model.JSONMap, link string) model.JSONMap {
	if p == nil {
		p = model.JSONMap{}
	}
	if link != "" {
		p["link"] = link
	}
	return p
}

func (s *Service) push(ctx context.Context, persona uuid.UUID, kind model.NotificationKind, c Content) {
	if s.Pusher == nil {
		return
	}
	tx := s.DB.WithContext(ctx)
	var per model.Persona
	if err := tx.Select("user_id").First(&per, "id = ?", persona).Error; err != nil {
		return
	}
	var st model.UserSettings
	tx.Where("user_id = ?", per.UserID).Limit(1).Find(&st)
	if !s.allowed(st, kind) {
		return
	}
	var devices []model.Device
	if err := tx.Where("user_id = ? AND push_token IS NOT NULL", per.UserID).Find(&devices).Error; err != nil || len(devices) == 0 {
		return
	}
	msgs := make([]PushMessage, 0, len(devices))
	for _, d := range devices {
		m := PushMessage{To: *d.PushToken, Title: c.Title, Body: c.Body, Data: map[string]any{"link": c.Link, "kind": string(kind)}, Sound: "default", ChannelID: string(kind)}
		if kind == model.NotifSOS {
			m.Priority = "high"
			m.ChannelID = "sos"
		}
		msgs = append(msgs, m)
	}
	tickets, err := s.Pusher.Send(ctx, msgs)
	if err != nil {
		s.Log.Warn("push failed", "err", err.Error())
		return
	}
	for i, t := range tickets {
		// Uninstalled apps: forget the token so we stop sending to it.
		if i < len(devices) && t.Details.Error == "DeviceNotRegistered" {
			tx.Model(&model.Device{}).Where("id = ?", devices[i].ID).Update("push_token", nil)
		}
	}
}

// allowed applies per-kind preferences and quiet hours. SOS ignores both.
func (s *Service) allowed(st model.UserSettings, kind model.NotificationKind) bool {
	if kind == model.NotifSOS {
		return true
	}
	if on, ok := st.NotifPrefs[string(kind)]; ok && !on {
		return false
	}
	return !inQuietHours(st, s.now())
}

func inQuietHours(st model.UserSettings, now time.Time) bool {
	if st.QuietHoursStart == nil || st.QuietHoursEnd == nil {
		return false
	}
	t := now.In(ist)
	m := int16(t.Hour()*60 + t.Minute())
	a, b := *st.QuietHoursStart, *st.QuietHoursEnd
	if a <= b {
		return m >= a && m < b
	}
	return m >= a || m < b
}

func (s *Service) Register(g *echo.Group) {
	g.POST("/me/push-token", s.registerToken)
	g.DELETE("/me/push-token", s.removeToken)
	g.GET("/notifications", s.list)
	g.POST("/notifications/read", s.markRead)
}

type tokenRequest struct {
	Token      string         `json:"token" validate:"required,max=200,startswith=ExponentPushToken["`
	Platform   model.Platform `json:"platform" validate:"required,oneof=ios android"`
	AppVersion string         `json:"app_version" validate:"max=16"`
}

func (s *Service) registerToken(c echo.Context) error {
	req, err := httpx.Bind[tokenRequest](c)
	if err != nil {
		return err
	}
	uid := httpx.PrincipalFrom(c).UserID
	err = s.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		// A token belongs to one install; if it moved accounts on this phone, the old account loses it.
		if err := tx.Model(&model.Device{}).Where("push_token = ? AND user_id <> ?", req.Token, uid).Update("push_token", nil).Error; err != nil {
			return err
		}
		var d model.Device
		if tx.Where("user_id = ? AND push_token = ?", uid, req.Token).Limit(1).Find(&d).RowsAffected == 1 {
			return tx.Model(&d).Updates(map[string]any{"last_seen_at": s.now(), "app_version": req.AppVersion}).Error
		}
		tok := req.Token
		return tx.Create(&model.Device{UserID: uid, Platform: req.Platform, PushToken: &tok, AppVersion: req.AppVersion, LastSeenAt: s.now()}).Error
	})
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (s *Service) removeToken(c echo.Context) error {
	req, err := httpx.Bind[struct {
		Token string `json:"token" validate:"required,max=200"`
	}](c)
	if err != nil {
		return err
	}
	if err := s.DB.WithContext(c.Request().Context()).Model(&model.Device{}).
		Where("user_id = ? AND push_token = ?", httpx.PrincipalFrom(c).UserID, req.Token).Update("push_token", nil).Error; err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

type NotificationDTO struct {
	ID        uuid.UUID              `json:"id"`
	Kind      model.NotificationKind `json:"kind"`
	Payload   model.JSONMap          `json:"payload"`
	Read      bool                   `json:"read"`
	CreatedAt time.Time              `json:"created_at"`
}

func notificationDTO(n model.Notification) NotificationDTO {
	return NotificationDTO{ID: n.ID, Kind: n.Kind, Payload: n.Payload, Read: n.ReadAt != nil, CreatedAt: n.CreatedAt}
}

func (s *Service) list(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	tx := s.DB.WithContext(c.Request().Context())
	q, limit, err := db.PageDesc(tx.Model(&model.Notification{}).Where("persona_id = ?", p.PersonaID), "", c.QueryParam("cursor"), 30)
	if err != nil {
		return err
	}
	var rows []model.Notification
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	rows, next := db.Trim(rows, limit, func(n model.Notification) db.Cursor { return db.Cursor{CreatedAt: n.CreatedAt, ID: n.ID} })
	out := make([]NotificationDTO, len(rows))
	for i, r := range rows {
		out[i] = notificationDTO(r)
	}
	var unread int64
	tx.Model(&model.Notification{}).Where("persona_id = ? AND read_at IS NULL", p.PersonaID).Count(&unread)
	return c.JSON(http.StatusOK, map[string]any{"items": out, "next_cursor": next, "unread": unread})
}

type readRequest struct {
	IDs []uuid.UUID `json:"ids" validate:"max=100"`
	All bool        `json:"all"`
}

func (s *Service) markRead(c echo.Context) error {
	req, err := httpx.Bind[readRequest](c)
	if err != nil {
		return err
	}
	q := s.DB.WithContext(c.Request().Context()).Model(&model.Notification{}).
		Where("persona_id = ? AND read_at IS NULL", httpx.PrincipalFrom(c).PersonaID)
	if !req.All {
		if len(req.IDs) == 0 {
			return c.NoContent(http.StatusNoContent)
		}
		q = q.Where("id IN ?", req.IDs)
	}
	if err := q.Update("read_at", s.now()).Error; err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// Purge keeps the inbox to 60 days.
func (s *Service) Purge(ctx context.Context) error {
	return s.DB.WithContext(ctx).Where("created_at < ?", s.now().Add(-60*24*time.Hour)).Delete(&model.Notification{}).Error
}
