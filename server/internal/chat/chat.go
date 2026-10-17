// Package chat is the one messaging engine behind DMs, groups, crews, travel chats and
// discussion rooms. Every send re-checks membership on the server: consent to contact is
// established when someone becomes a member, never assumed.
package chat

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metro-buddy/server/internal/db"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/media"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/ratelimit"
	"metro-buddy/server/internal/safety"
	"metro-buddy/server/internal/secure"
)

var (
	ErrNotMember   = httpx.NotFound()
	ErrBlocked     = httpx.Forbidden("you can't message this person")
	ErrSlowMode    = httpx.E(http.StatusTooManyRequests, "slow_mode", "slow mode is on, wait a moment")
	ErrClosed      = httpx.Conflict("this conversation is closed")
	ErrBadClientID = httpx.Unprocessable("invalid_message_id", "message id must be a fresh UUIDv7")
)

type Publisher interface {
	Publish(ctx context.Context, personas []uuid.UUID, typ string, data any)
}

type Service struct {
	DB      *gorm.DB
	PII     *pii.Pipeline
	Media   *media.Service
	Hub     Publisher
	Limiter *ratelimit.Limiter
	// MaskOptions returns PII masking exemptions for a conversation (the reveal ladder lifts some).
	MaskOptions func(tx *gorm.DB, convID uuid.UUID) pii.Options
	Box         *secure.Box
	// Audit records identity-boundary crossings (reveals); nil-safe.
	Audit func(tx *gorm.DB, userID uuid.UUID, verb string, conv uuid.UUID, meta model.JSONMap)
	// OnMessage lets push notifications fire for offline members.
	OnMessage func(ctx context.Context, conv *model.Conversation, msg *model.Message, recipients []uuid.UUID)
	// OnRequest and OnAccepted notify about DM requests.
	OnRequest  func(ctx context.Context, to, requestID uuid.UUID)
	OnAccepted func(ctx context.Context, requester, convID uuid.UUID)
	Now        func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

func (s *Service) Register(g *echo.Group) {
	send := s.Limiter.Middleware(ratelimit.Rule{Name: "messages", Limit: 60, Window: time.Minute, ByTier: true})
	g.GET("/conversations", s.list)
	g.GET("/conversations/:id", s.get)
	g.PATCH("/conversations/:id", s.patch)
	g.PUT("/conversations/:id/disappearing", s.setDisappearing)
	s.registerReveal(g)
	g.POST("/conversations/:id/leave", s.leave)
	g.GET("/conversations/:id/messages", s.history)
	g.POST("/conversations/:id/messages", s.send, send)
	g.DELETE("/messages/:id", s.deleteMessage)
	g.PATCH("/messages/:id", s.editMessage, send)
	g.POST("/messages/:id/view", s.viewOnce)
	s.registerReceipts(g)
	s.registerRequests(g)
	s.registerGroups(g)
	s.registerRooms(g)
}

// member returns the caller's active membership or ErrNotMember. Non-members always see 404.
func member(tx *gorm.DB, convID, persona uuid.UUID) (*model.Conversation, *model.ConversationMember, error) {
	var m model.ConversationMember
	err := tx.Where("conversation_id = ? AND persona_id = ? AND left_at IS NULL", convID, persona).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, ErrNotMember
	}
	if err != nil {
		return nil, nil, err
	}
	var c model.Conversation
	if err := tx.First(&c, "id = ?", convID).Error; err != nil {
		return nil, nil, ErrNotMember
	}
	return &c, &m, nil
}

func memberIDs(tx *gorm.DB, convID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND left_at IS NULL", convID).Pluck("persona_id", &ids).Error
	return ids, err
}

// Create makes a conversation with initial members inside the caller's transaction.
func Create(tx *gorm.DB, now time.Time, conv *model.Conversation, members map[uuid.UUID]model.ConvRole) error {
	if conv.ID == uuid.Nil {
		conv.ID = model.NewID()
	}
	conv.LastMessageAt = &now
	if err := tx.Create(conv).Error; err != nil {
		return err
	}
	for pid, role := range members {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.ConversationMember{
			ConversationID: conv.ID, PersonaID: pid, Role: role, JoinedAt: now,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// SystemMessage posts an unauthored notice ("You matched!", meet point) into a conversation.
func (s *Service) SystemMessage(ctx context.Context, tx *gorm.DB, convID uuid.UUID, kind model.MessageKind, body string) (*model.Message, error) {
	m := model.Message{ID: model.NewID(), ConversationID: convID, Kind: kind, BodyMasked: body, Status: model.ContentVisible, CreatedAt: s.now()}
	if err := tx.Create(&m).Error; err != nil {
		return nil, err
	}
	if err := tx.Model(&model.Conversation{}).Where("id = ?", convID).Update("last_message_at", m.CreatedAt).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// PostSystem adds a system message outside any request and delivers it to every member.
func (s *Service) PostSystem(ctx context.Context, convID uuid.UUID, kind model.MessageKind, body string) error {
	tx := s.DB.WithContext(ctx)
	var conv model.Conversation
	if err := tx.First(&conv, "id = ?", convID).Error; err != nil {
		return err
	}
	msg, err := s.SystemMessage(ctx, tx, convID, kind, body)
	if err != nil {
		return err
	}
	var members []uuid.UUID
	tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND left_at IS NULL", convID).Pluck("persona_id", &members)
	s.fanOut(ctx, &conv, *msg, members)
	return nil
}

type MessageDTO struct {
	ID             uuid.UUID            `json:"id"`
	ConversationID uuid.UUID            `json:"conversation_id"`
	Sender         *identity.PersonaRef `json:"sender"`
	IsMine         bool                 `json:"is_mine"`
	Kind           model.MessageKind    `json:"kind"`
	Body           string               `json:"body"`
	Media          []media.MediaDTO     `json:"media"`
	ReplyToID      *uuid.UUID           `json:"reply_to_id"`
	Deleted        bool                 `json:"deleted"`
	PIIHidden      bool                 `json:"pii_hidden"`
	CreatedAt      time.Time            `json:"created_at"`
	ExpiresAt      *time.Time           `json:"expires_at"`
	EditedAt       *time.Time           `json:"edited_at"`
	// View-once photos never carry a URL here; recipients open them through POST /messages/:id/view.
	ViewOnce    bool `json:"view_once"`
	ViewedByMe  bool `json:"viewed_by_me"`
	ShowSupport bool `json:"show_support,omitempty"`
}

func (s *Service) messageDTOs(ctx context.Context, tx *gorm.DB, viewer uuid.UUID, msgs []model.Message) ([]MessageDTO, error) {
	var senders []uuid.UUID
	for _, m := range msgs {
		if m.SenderPersonaID != nil {
			senders = append(senders, *m.SenderPersonaID)
		}
	}
	refs, err := identity.LoadRefs(tx, senders)
	if err != nil {
		return nil, err
	}
	viewed := s.viewedByMe(tx, viewer, msgs)
	out := make([]MessageDTO, len(msgs))
	for i, m := range msgs {
		d := MessageDTO{ID: m.ID, ConversationID: m.ConversationID, Kind: m.Kind, Body: m.BodyMasked, ReplyToID: m.ReplyToID,
			PIIHidden: m.PIIFlags != 0, CreatedAt: m.CreatedAt, ExpiresAt: m.ExpiresAt, EditedAt: m.EditedAt, ViewOnce: m.ViewOnce}
		if m.ViewOnce {
			d.Media = viewOncePlaceholders(m.Media)
			d.ViewedByMe = viewed[m.ID]
		} else {
			d.Media = s.Media.DTOs(ctx, m.Media)
		}
		if m.SenderPersonaID != nil {
			r := refs[*m.SenderPersonaID]
			d.Sender, d.IsMine = &r, *m.SenderPersonaID == viewer
		}
		if m.Status != model.ContentVisible {
			d.Deleted, d.Body, d.Media = true, "", []media.MediaDTO{}
		}
		out[i] = d
	}
	return out, nil
}

type ConversationDTO struct {
	ID            uuid.UUID              `json:"id"`
	Kind          model.ConversationKind `json:"kind"`
	Title         *string                `json:"title"`
	Topic         *string                `json:"topic,omitempty"`
	Members       []identity.PersonaRef  `json:"members"`
	MemberCount   int                    `json:"member_count"`
	MyRole        model.ConvRole         `json:"my_role"`
	LastMessage   *MessageDTO            `json:"last_message"`
	LastMessageAt *time.Time             `json:"last_message_at"`
	Unread        int64                  `json:"unread"`
	MutedUntil    *time.Time             `json:"muted_until"`
	StartsAt      *time.Time             `json:"starts_at,omitempty"`
	EndsAt        *time.Time             `json:"ends_at,omitempty"`
	Closed        bool                   `json:"closed"`
	SlowModeSec   int16                  `json:"slow_mode_sec"`
	Disappear     *int                   `json:"disappear_after_sec"`
	InviteCode    *string                `json:"invite_code,omitempty"`
	BowlID        *uuid.UUID             `json:"bowl_id,omitempty"`
	MatchID       *uuid.UUID             `json:"match_id,omitempty"`
	CrewID        *uuid.UUID             `json:"crew_id,omitempty"`
	RevealLevels  map[string]string      `json:"reveal_levels,omitempty"`
}

func (s *Service) conversationDTO(ctx context.Context, tx *gorm.DB, viewer uuid.UUID, c *model.Conversation, me *model.ConversationMember) (*ConversationDTO, error) {
	var members []model.ConversationMember
	if err := tx.Where("conversation_id = ? AND left_at IS NULL", c.ID).Find(&members).Error; err != nil {
		return nil, err
	}
	blocked, err := safety.BlockedSet(ctx, tx, viewer)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	for _, m := range members {
		if !blocked[m.PersonaID] {
			ids = append(ids, m.PersonaID)
		}
	}
	refs, err := identity.LoadRefs(tx, ids)
	if err != nil {
		return nil, err
	}
	d := &ConversationDTO{ID: c.ID, Kind: c.Kind, Title: c.Title, Topic: c.Topic, MemberCount: len(members), MyRole: me.Role,
		LastMessageAt: c.LastMessageAt, MutedUntil: me.MutedUntil, StartsAt: c.StartsAt, EndsAt: c.EndsAt,
		Closed: c.ClosedAt != nil, SlowModeSec: c.SlowModeSec, Disappear: c.DisappearAfterSec, BowlID: c.BowlID, MatchID: c.MatchID, CrewID: c.CrewID}
	if me.Role != model.ConvMember {
		d.InviteCode = c.InviteCode
	}
	// Big rooms list a sample of members; small chats list everyone.
	for i, id := range ids {
		if i >= 20 {
			break
		}
		d.Members = append(d.Members, refs[id])
	}
	if d.Members == nil {
		d.Members = []identity.PersonaRef{}
	}
	if c.Kind == model.ConvDM || c.Kind == model.ConvTravel {
		d.RevealLevels = map[string]string{}
		for _, m := range members {
			d.RevealLevels[m.PersonaID.String()] = string(m.RevealLevel)
		}
	}
	var last model.Message
	q := tx.Where("conversation_id = ? AND (expires_at IS NULL OR expires_at > ?)", c.ID, s.now()).
		Scopes(safety.ExcludeBlocked(viewer, "COALESCE(sender_persona_id, '00000000-0000-0000-0000-000000000000')"))
	if q.Order("created_at DESC").Limit(1).Find(&last).RowsAffected == 1 {
		ds, err := s.messageDTOs(ctx, tx, viewer, []model.Message{last})
		if err != nil {
			return nil, err
		}
		d.LastMessage = &ds[0]
	}
	d.Unread, err = unreadCount(tx, c.ID, me)
	return d, err
}

func unreadCount(tx *gorm.DB, convID uuid.UUID, me *model.ConversationMember) (int64, error) {
	q := tx.Model(&model.Message{}).Where("conversation_id = ? AND (sender_persona_id IS NULL OR sender_persona_id <> ?) AND status = ?",
		convID, me.PersonaID, model.ContentVisible)
	if me.LastReadMessageID != nil {
		q = q.Where("created_at > (SELECT created_at FROM messages WHERE id = ?)", *me.LastReadMessageID)
	} else {
		q = q.Where("created_at >= ?", me.JoinedAt)
	}
	var n int64
	return n, q.Count(&n).Error
}

type listQuery struct {
	Kind   string `query:"kind" validate:"omitempty,oneof=dm group crew travel discussion"`
	Cursor string `query:"cursor"`
	Limit  int    `query:"limit"`
}

func (s *Service) list(c echo.Context) error {
	req, err := httpx.Bind[listQuery](c)
	if err != nil {
		return err
	}
	viewer := httpx.PrincipalFrom(c).PersonaID
	ctx := c.Request().Context()
	tx := s.DB.WithContext(ctx)
	q := tx.Model(&model.Conversation{}).
		Joins("JOIN conversation_members cm ON cm.conversation_id = conversations.id AND cm.persona_id = ? AND cm.left_at IS NULL", viewer)
	if req.Kind != "" {
		q = q.Where("conversations.kind = ?", req.Kind)
	}
	// A DM with someone you blocked (or who blocked you) disappears from your list.
	q = q.Where(`NOT (conversations.kind = 'dm' AND EXISTS (
		SELECT 1 FROM conversation_members o JOIN blocks b ON (b.blocker_persona_id = ? AND b.blocked_persona_id = o.persona_id) OR (b.blocked_persona_id = ? AND b.blocker_persona_id = o.persona_id)
		WHERE o.conversation_id = conversations.id AND o.persona_id <> ?))`, viewer, viewer, viewer)
	limit := db.ClampLimit(req.Limit)
	if req.Cursor != "" {
		cur, err := db.DecodeCursor(req.Cursor)
		if err != nil {
			return err
		}
		q = q.Where("(conversations.last_message_at < ? OR (conversations.last_message_at = ? AND conversations.id < ?))", cur.CreatedAt, cur.CreatedAt, cur.ID)
	}
	var convs []model.Conversation
	if err := q.Order("conversations.last_message_at DESC").Order("conversations.id DESC").Limit(limit + 1).Find(&convs).Error; err != nil {
		return err
	}
	next := ""
	if len(convs) > limit {
		convs = convs[:limit]
		l := convs[len(convs)-1]
		next = db.Cursor{CreatedAt: *l.LastMessageAt, ID: l.ID}.Encode()
	}
	out := make([]*ConversationDTO, 0, len(convs))
	for i := range convs {
		_, me, err := member(tx, convs[i].ID, viewer)
		if err != nil {
			return err
		}
		d, err := s.conversationDTO(ctx, tx, viewer, &convs[i], me)
		if err != nil {
			return err
		}
		out = append(out, d)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out, "next_cursor": next})
}

func (s *Service) get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return ErrNotMember
	}
	viewer := httpx.PrincipalFrom(c).PersonaID
	ctx := c.Request().Context()
	tx := s.DB.WithContext(ctx)
	conv, me, err := member(tx, id, viewer)
	if err != nil {
		return err
	}
	d, err := s.conversationDTO(ctx, tx, viewer, conv, me)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, d)
}

type patchRequest struct {
	MutedUntil *time.Time `json:"muted_until"`
	Unmute     bool       `json:"unmute"`
}

func (s *Service) patch(c echo.Context) error {
	req, err := httpx.Bind[patchRequest](c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return ErrNotMember
	}
	viewer := httpx.PrincipalFrom(c).PersonaID
	tx := s.DB.WithContext(c.Request().Context())
	if _, _, err := member(tx, id, viewer); err != nil {
		return err
	}
	var until any = req.MutedUntil
	if req.Unmute {
		until = nil
	}
	if err := tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND persona_id = ?", id, viewer).Update("muted_until", until).Error; err != nil {
		return err
	}
	return s.get(c)
}

func (s *Service) leave(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return ErrNotMember
	}
	viewer := httpx.PrincipalFrom(c).PersonaID
	ctx := c.Request().Context()
	var others []uuid.UUID
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		conv, me, err := member(tx, id, viewer)
		if err != nil {
			return err
		}
		if err := tx.Model(me).Where("conversation_id = ? AND persona_id = ?", id, viewer).Update("left_at", s.now()).Error; err != nil {
			return err
		}
		// An owner leaving hands the group to the longest-standing member.
		if me.Role == model.ConvOwner && conv.Kind == model.ConvGroup {
			var next model.ConversationMember
			if tx.Where("conversation_id = ? AND left_at IS NULL", id).Order("joined_at").Limit(1).Find(&next).RowsAffected == 1 {
				if err := tx.Model(&next).Where("conversation_id = ? AND persona_id = ?", id, next.PersonaID).Update("role", model.ConvOwner).Error; err != nil {
					return err
				}
			}
		}
		others, err = memberIDs(tx, id)
		return err
	})
	if err != nil {
		return err
	}
	s.Hub.Publish(ctx, others, "conversation.member_left", map[string]any{"conversation_id": id, "persona_id": viewer})
	return c.NoContent(http.StatusNoContent)
}

type historyQuery struct {
	Cursor string `query:"cursor"`
	Limit  int    `query:"limit"`
}

// history pages newest-first; blocked senders' messages are hidden, expired ones never return.
func (s *Service) history(c echo.Context) error {
	req, err := httpx.Bind[historyQuery](c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return ErrNotMember
	}
	viewer := httpx.PrincipalFrom(c).PersonaID
	ctx := c.Request().Context()
	tx := s.DB.WithContext(ctx)
	_, me, err := member(tx, id, viewer)
	if err != nil {
		return err
	}
	q := tx.Model(&model.Message{}).Where("conversation_id = ? AND (expires_at IS NULL OR expires_at > ?) AND created_at >= ?", id, s.now(), me.JoinedAt).
		Scopes(safety.ExcludeBlocked(viewer, "COALESCE(sender_persona_id, '00000000-0000-0000-0000-000000000000')"))
	q, limit, err := db.PageDesc(q, "messages", req.Cursor, req.Limit)
	if err != nil {
		return err
	}
	var rows []model.Message
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	rows, next := db.Trim(rows, limit, func(m model.Message) db.Cursor { return db.Cursor{CreatedAt: m.CreatedAt, ID: m.ID} })
	out, err := s.messageDTOs(ctx, tx, viewer, rows)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out, "next_cursor": next})
}

type sendRequest struct {
	ID        uuid.UUID  `json:"id" validate:"required"`
	Body      string     `json:"body" validate:"max=4000"`
	ReplyToID *uuid.UUID `json:"reply_to_id"`
	MediaKey  string     `json:"media_key" validate:"max=200"`
	ViewOnce  bool       `json:"view_once"`
	Sticker   string     `json:"sticker" validate:"max=40"`
}

// validClientID accepts only UUIDv7 ids stamped near now, so a client can't backdate messages.
func (s *Service) validClientID(id uuid.UUID) bool {
	if id.Version() != 7 {
		return false
	}
	sec, nsec := id.Time().UnixTime()
	t := time.Unix(sec, nsec)
	d := s.now().Sub(t)
	return d < 10*time.Minute && d > -2*time.Minute
}

func (s *Service) send(c echo.Context) error {
	req, err := httpx.Bind[sendRequest](c)
	if err != nil {
		return err
	}
	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return ErrNotMember
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	tx := s.DB.WithContext(ctx)

	// Idempotency: a retried send with the same client id returns the stored message.
	var existing model.Message
	if tx.Where("id = ?", req.ID).Limit(1).Find(&existing).RowsAffected == 1 {
		if existing.ConversationID != convID || existing.SenderPersonaID == nil || *existing.SenderPersonaID != p.PersonaID {
			return httpx.Conflict("message id already used")
		}
		out, err := s.messageDTOs(ctx, tx, p.PersonaID, []model.Message{existing})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, out[0])
	}
	if !s.validClientID(req.ID) {
		return ErrBadClientID
	}
	if req.Body == "" && req.MediaKey == "" && req.Sticker == "" {
		return httpx.Unprocessable("empty_message", "say something")
	}
	if req.Sticker != "" && (req.Body != "" || req.MediaKey != "" || !slices.Contains(Stickers, req.Sticker)) {
		return httpx.Unprocessable("invalid_sticker", "unknown sticker")
	}
	if req.ViewOnce && req.MediaKey == "" {
		return httpx.Unprocessable("view_once_needs_photo", "only photos can be view once")
	}
	conv, me, err := member(tx, convID, p.PersonaID)
	if err != nil {
		return err
	}
	if err := s.canSend(ctx, tx, conv, me); err != nil {
		return err
	}
	var medias model.MediaList
	if req.MediaKey != "" {
		m, verdict, err := s.Media.Finalize(ctx, p.PersonaID, req.MediaKey)
		if err != nil {
			return err
		}
		medias = append(medias, m)
		if verdict == media.Review {
			defer func() {
				_, _ = safety.OpenCase(s.DB.WithContext(ctx), s.now(), model.CTMessage, req.ID, &p.PersonaID, model.SevMedium, "image_review")
			}()
		}
	}
	now := s.now()
	msg := model.Message{ID: req.ID, ConversationID: convID, SenderPersonaID: &p.PersonaID, Kind: model.MsgText, ReplyToID: req.ReplyToID,
		Media: medias, Status: model.ContentVisible, CreatedAt: now}
	if len(medias) > 0 {
		msg.Kind = model.MsgImage
		msg.ViewOnce = req.ViewOnce
	}
	if req.Sticker != "" {
		msg.Kind, msg.BodyMasked = model.MsgSticker, req.Sticker
	}
	if conv.DisappearAfterSec != nil {
		exp := now.Add(time.Duration(*conv.DisappearAfterSec) * time.Second)
		msg.ExpiresAt = &exp
	}
	var recipients []uuid.UUID
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		opts := pii.Options{}
		if s.MaskOptions != nil {
			opts = s.MaskOptions(tx, convID)
		}
		if req.Body != "" {
			r, err := s.PII.Process(tx, model.CTMessage, msg.ID, req.Body, opts)
			if err != nil {
				return err
			}
			msg.BodyMasked, msg.PIIFlags = r.Text, r.Flags
		}
		if err := tx.Create(&msg).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Conversation{}).Where("id = ?", convID).Update("last_message_at", now).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ConversationMember{}).Where("conversation_id = ? AND persona_id = ?", convID, p.PersonaID).
			Updates(map[string]any{"last_sent_at": now, "last_read_message_id": msg.ID}).Error; err != nil {
			return err
		}
		recipients, err = memberIDs(tx, convID)
		return err
	})
	if err != nil {
		return err
	}
	if req.Body != "" {
		s.PII.AfterCommit(ctx, model.CTMessage, msg.ID)
	}
	out, err := s.messageDTOs(ctx, s.DB.WithContext(ctx), p.PersonaID, []model.Message{msg})
	if err != nil {
		return err
	}
	s.fanOut(ctx, conv, msg, recipients)
	if s.OnMessage != nil {
		s.OnMessage(ctx, conv, &msg, recipients)
	}
	d := out[0]
	d.ShowSupport = safety.Distress(req.Body)
	return c.JSON(http.StatusCreated, d)
}

// fanOut delivers to every member except those who blocked (or were blocked by) the sender.
func (s *Service) fanOut(ctx context.Context, conv *model.Conversation, msg model.Message, members []uuid.UUID) {
	sender := uuid.Nil
	if msg.SenderPersonaID != nil {
		sender = *msg.SenderPersonaID
	}
	blocked := map[uuid.UUID]bool{}
	if sender != uuid.Nil {
		blocked, _ = safety.BlockedSet(ctx, s.DB, sender)
	}
	var to []uuid.UUID
	for _, m := range members {
		if !blocked[m] {
			to = append(to, m)
		}
	}
	dtos, err := s.messageDTOs(ctx, s.DB.WithContext(ctx), uuid.Nil, []model.Message{msg})
	if err != nil {
		return
	}
	s.Hub.Publish(ctx, to, "message.new", dtos[0])
}

func (s *Service) canSend(ctx context.Context, tx *gorm.DB, conv *model.Conversation, me *model.ConversationMember) error {
	now := s.now()
	if conv.ClosedAt != nil || (conv.EndsAt != nil && now.After(*conv.EndsAt)) {
		return ErrClosed
	}
	if conv.StartsAt != nil && now.Before(*conv.StartsAt) {
		return httpx.Conflict("this room hasn't opened yet")
	}
	if conv.SlowModeSec > 0 && me.Role == model.ConvMember && me.LastSentAt != nil &&
		now.Sub(*me.LastSentAt) < time.Duration(conv.SlowModeSec)*time.Second {
		return ErrSlowMode
	}
	if conv.Kind == model.ConvDM || conv.Kind == model.ConvTravel {
		others, err := memberIDs(tx, conv.ID)
		if err != nil {
			return err
		}
		for _, o := range others {
			if o == me.PersonaID {
				continue
			}
			if b, err := safety.IsBlocked(ctx, tx, me.PersonaID, o); err != nil || b {
				return ErrBlocked
			}
		}
		if len(others) < 2 {
			return httpx.Conflict("the other person left this chat")
		}
	}
	return nil
}

func (s *Service) deleteMessage(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	viewer := httpx.PrincipalFrom(c).PersonaID
	ctx := c.Request().Context()
	var msg model.Message
	var members []uuid.UUID
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&msg, "id = ? AND sender_persona_id = ?", id, viewer).Error; err != nil {
			return err
		}
		if err := tx.Model(&msg).Update("status", model.ContentRemovedByAuthor).Error; err != nil {
			return err
		}
		members, err = memberIDs(tx, msg.ConversationID)
		return err
	})
	if err != nil {
		return err
	}
	s.Hub.Publish(ctx, members, "message.deleted", map[string]any{"id": id, "conversation_id": msg.ConversationID})
	return c.NoContent(http.StatusNoContent)
}

// OnMaskUpdated pushes the LLM pass's corrected text to everyone who already received the message.
func (s *Service) OnMaskUpdated(ct model.ContentType, id uuid.UUID, masked string) {
	if ct != model.CTMessage {
		return
	}
	ctx := context.Background()
	var msg model.Message
	if err := s.DB.WithContext(ctx).First(&msg, "id = ?", id).Error; err != nil {
		return
	}
	members, err := memberIDs(s.DB.WithContext(ctx), msg.ConversationID)
	if err != nil {
		return
	}
	s.Hub.Publish(ctx, members, "message.updated", map[string]any{"id": id, "conversation_id": msg.ConversationID, "body": masked, "pii_hidden": true})
}

// OnCaseClosed re-sends a message whose held image a moderator released.
func (s *Service) OnCaseClosed(caseID uuid.UUID) {
	ctx := context.Background()
	tx := s.DB.WithContext(ctx)
	var mc model.ModerationCase
	if tx.First(&mc, "id = ?", caseID).Error != nil || mc.TargetType != model.CTMessage {
		return
	}
	var msg model.Message
	if tx.First(&msg, "id = ?", mc.TargetID).Error != nil || len(msg.Media) == 0 {
		return
	}
	members, err := memberIDs(tx, msg.ConversationID)
	if err != nil {
		return
	}
	dtos, err := s.messageDTOs(ctx, tx, uuid.Nil, []model.Message{msg})
	if err != nil {
		return
	}
	s.Hub.Publish(ctx, members, "message.updated", map[string]any{"id": msg.ID, "conversation_id": msg.ConversationID, "media": dtos[0].Media})
}
