package games

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	mrand "math/rand/v2"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/ratelimit"
	"metro-buddy/server/internal/safety"
)

const (
	StatusLobby    = "lobby"
	StatusPlaying  = "playing"
	StatusFinished = "finished"
	StatusClosed   = "closed"

	// Spectators per room, on top of players.
	maxSpectators = 20
	// Idle lobbies and finished rooms close after this long.
	idleClose = 30 * time.Minute
)

type Publisher interface {
	Publish(ctx context.Context, personas []uuid.UUID, event string, data any)
}

type Service struct {
	DB      *gorm.DB
	Hub     Publisher
	Limiter *ratelimit.Limiter
	Now     func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

func (s *Service) ctx() Ctx {
	return Ctx{
		Now:  s.now(),
		Rand: mrand.New(mrand.NewPCG(mrand.Uint64(), mrand.Uint64())),
		Mask: func(t string) string { return pii.Mask(t, pii.Options{}).Text },
	}
}

func (s *Service) Register(g *echo.Group) {
	create := s.Limiter.Middleware(ratelimit.Rule{Name: "game_rooms", Limit: 30, Window: time.Hour})
	moves := s.Limiter.Middleware(ratelimit.Rule{Name: "game_moves", Limit: 900, Window: time.Minute})
	g.GET("/games", s.catalog)
	g.GET("/games/rooms", s.list)
	g.POST("/games/rooms", s.create, create)
	g.POST("/games/rooms/join", s.joinByCode)
	g.GET("/games/rooms/:id", s.get)
	g.POST("/games/rooms/:id/join", s.join)
	g.POST("/games/rooms/:id/leave", s.leave)
	g.POST("/games/rooms/:id/ready", s.ready)
	g.POST("/games/rooms/:id/start", s.start)
	g.POST("/games/rooms/:id/move", s.move, moves)
	g.POST("/games/rooms/:id/rematch", s.rematch)
}

type GameInfo struct {
	Name       string `json:"name"`
	MinPlayers int    `json:"min_players"`
	MaxPlayers int    `json:"max_players"`
}

func (s *Service) catalog(c echo.Context) error {
	out := []GameInfo{}
	for _, n := range Names() {
		e, _ := Lookup(n)
		lo, hi := e.Players()
		out = append(out, GameInfo{Name: n, MinPlayers: lo, MaxPlayers: hi})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

type PlayerDTO struct {
	Persona   identity.PersonaRef `json:"persona"`
	Spectator bool                `json:"spectator"`
	Ready     bool                `json:"ready"`
	Seat      int16               `json:"seat"`
	Score     int                 `json:"score"`
}

type RoomDTO struct {
	ID          uuid.UUID   `json:"id"`
	Game        string      `json:"game"`
	Visibility  string      `json:"visibility"`
	Code        string      `json:"code,omitempty"`
	HostID      uuid.UUID   `json:"host_id"`
	MinPlayers  int16       `json:"min_players"`
	MaxPlayers  int16       `json:"max_players"`
	Status      string      `json:"status"`
	Version     int64       `json:"version"`
	DeadlineAt  *time.Time  `json:"deadline_at"`
	Players     []PlayerDTO `json:"players"`
	PlayerCount int         `json:"player_count"`
	// Only in a single-room response.
	Me   *PlayerDTO `json:"me,omitempty"`
	View any        `json:"view,omitempty"`
}

func randomCode() string {
	const alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		b[i] = alphabet[n.Int64()]
	}
	return string(b)
}

func (s *Service) members(tx *gorm.DB, room uuid.UUID) ([]model.GamePlayer, error) {
	var ps []model.GamePlayer
	err := tx.Where("room_id = ?", room).Order("spectator, seat, joined_at").Find(&ps).Error
	return ps, err
}

func memberIDs(ps []model.GamePlayer) []uuid.UUID {
	out := make([]uuid.UUID, len(ps))
	for i, p := range ps {
		out[i] = p.PersonaID
	}
	return out
}

func players(ps []model.GamePlayer) []model.GamePlayer {
	out := []model.GamePlayer{}
	for _, p := range ps {
		if !p.Spectator {
			out = append(out, p)
		}
	}
	return out
}

func (s *Service) dto(tx *gorm.DB, r *model.GameRoom, ps []model.GamePlayer, viewer uuid.UUID, withView bool) (RoomDTO, error) {
	refs, err := identity.LoadRefs(tx, memberIDs(ps))
	if err != nil {
		return RoomDTO{}, err
	}
	out := RoomDTO{ID: r.ID, Game: r.Game, Visibility: r.Visibility, HostID: r.HostPersonaID, MinPlayers: r.MinPlayers,
		MaxPlayers: r.MaxPlayers, Status: r.Status, Version: r.Version, DeadlineAt: r.DeadlineAt, Players: []PlayerDTO{}}
	var me *model.GamePlayer
	for i, p := range ps {
		d := PlayerDTO{Persona: refs[p.PersonaID], Spectator: p.Spectator, Ready: p.Ready, Seat: p.Seat, Score: p.Score}
		out.Players = append(out.Players, d)
		if !p.Spectator {
			out.PlayerCount++
		}
		if p.PersonaID == viewer {
			me = &ps[i]
			out.Me = &d
		}
	}
	// Private codes are shared only with people already inside.
	if r.Visibility == "public" || me != nil {
		out.Code = r.Code
	}
	if withView && r.State != "" && (r.Status == StatusPlaying || r.Status == StatusFinished) {
		e, ok := Lookup(r.Game)
		if ok {
			st, err := e.Load([]byte(r.State))
			if err != nil {
				return out, err
			}
			out.View = st.View(viewer, me == nil || me.Spectator)
		}
	}
	return out, nil
}

type listQuery struct {
	Game string `query:"game"`
	// small: 2 players, group: 3–6, party: 7 or more.
	Size string `query:"size" validate:"omitempty,oneof=small group party"`
}

func (s *Service) list(c echo.Context) error {
	req, err := httpx.Bind[listQuery](c)
	if err != nil {
		return err
	}
	viewer := httpx.PrincipalFrom(c).PersonaID
	tx := s.DB.WithContext(c.Request().Context())
	q := tx.Model(&model.GameRoom{}).Where("visibility = ? AND status IN ?", "public", []string{StatusLobby, StatusPlaying}).
		Scopes(safety.ExcludeBlocked(viewer, "host_persona_id")).Order("last_active_at DESC").Limit(50)
	if req.Game != "" {
		q = q.Where("game = ?", req.Game)
	}
	switch req.Size {
	case "small":
		q = q.Where("max_players <= 2")
	case "group":
		q = q.Where("max_players BETWEEN 3 AND 6")
	case "party":
		q = q.Where("max_players >= 7")
	}
	var rooms []model.GameRoom
	if err := q.Find(&rooms).Error; err != nil {
		return err
	}
	out := make([]RoomDTO, 0, len(rooms))
	for i := range rooms {
		ps, err := s.members(tx, rooms[i].ID)
		if err != nil {
			return err
		}
		d, err := s.dto(tx, &rooms[i], ps, viewer, false)
		if err != nil {
			return err
		}
		out = append(out, d)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

type createBody struct {
	Game       string `json:"game" validate:"required"`
	Visibility string `json:"visibility" validate:"required,oneof=public private"`
	MaxPlayers int    `json:"max_players"`
}

func (s *Service) create(c echo.Context) error {
	req, err := httpx.Bind[createBody](c)
	if err != nil {
		return err
	}
	e, ok := Lookup(req.Game)
	if !ok {
		return httpx.Unprocessable("unknown_game", "that game isn't available")
	}
	lo, hi := e.Players()
	maxP := req.MaxPlayers
	if maxP == 0 {
		maxP = hi
	}
	if maxP < lo || maxP > hi {
		return httpx.Unprocessable("bad_size", "that many players doesn't fit this game")
	}
	p := httpx.PrincipalFrom(c)
	now := s.now()
	room := model.GameRoom{Game: req.Game, Visibility: req.Visibility, HostPersonaID: p.PersonaID, MinPlayers: int16(lo),
		MaxPlayers: int16(maxP), Status: StatusLobby, LastActiveAt: now}
	err = s.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		for range 5 {
			room.Code = randomCode()
			var n int64
			if err := tx.Model(&model.GameRoom{}).Where("code = ?", room.Code).Count(&n).Error; err != nil {
				return err
			}
			if n == 0 {
				break
			}
		}
		if err := tx.Create(&room).Error; err != nil {
			return err
		}
		return tx.Create(&model.GamePlayer{RoomID: room.ID, PersonaID: p.PersonaID, Seat: 0, JoinedAt: now}).Error
	})
	if err != nil {
		return err
	}
	return s.respond(c, room.ID, http.StatusCreated)
}

func (s *Service) respond(c echo.Context, id uuid.UUID, status int) error {
	tx := s.DB.WithContext(c.Request().Context())
	var r model.GameRoom
	if err := tx.First(&r, "id = ?", id).Error; err != nil {
		return httpx.NotFound()
	}
	ps, err := s.members(tx, r.ID)
	if err != nil {
		return err
	}
	d, err := s.dto(tx, &r, ps, httpx.PrincipalFrom(c).PersonaID, true)
	if err != nil {
		return err
	}
	return c.JSON(status, d)
}

func (s *Service) get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	var r model.GameRoom
	if err := s.DB.WithContext(c.Request().Context()).First(&r, "id = ?", id).Error; err != nil {
		return httpx.NotFound()
	}
	viewer := httpx.PrincipalFrom(c).PersonaID
	if r.Visibility == "private" {
		var n int64
		s.DB.WithContext(c.Request().Context()).Model(&model.GamePlayer{}).Where("room_id = ? AND persona_id = ?", id, viewer).Count(&n)
		if n == 0 {
			return httpx.NotFound()
		}
	}
	// Timers also advance here, so a room never looks stuck while the ticker catches up.
	if r.Status == StatusPlaying && r.DeadlineAt != nil && !r.DeadlineAt.After(s.now()) {
		if err := s.tickRoom(c.Request().Context(), id); err != nil {
			return err
		}
	}
	return s.respond(c, id, http.StatusOK)
}

type joinBody struct {
	Code      string `json:"code"`
	Spectator bool   `json:"spectator"`
}

func (s *Service) joinByCode(c echo.Context) error {
	req, err := httpx.Bind[joinBody](c)
	if err != nil {
		return err
	}
	var r model.GameRoom
	if err := s.DB.WithContext(c.Request().Context()).First(&r, "code = ? AND status <> ?", normalizeCode(req.Code), StatusClosed).Error; err != nil {
		return httpx.E(http.StatusNotFound, "bad_code", "no room with that code")
	}
	return s.doJoin(c, r.ID, req.Spectator)
}

func normalizeCode(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch >= 'a' && ch <= 'z' {
			ch -= 32
		}
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			out = append(out, ch)
		}
	}
	return string(out)
}

func (s *Service) join(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	req, err := httpx.Bind[joinBody](c)
	if err != nil {
		return err
	}
	var r model.GameRoom
	if err := s.DB.WithContext(c.Request().Context()).First(&r, "id = ?", id).Error; err != nil || r.Visibility == "private" {
		// Private rooms are joined only by code.
		return httpx.NotFound()
	}
	return s.doJoin(c, id, req.Spectator)
}

var ErrRoomFull = httpx.E(http.StatusConflict, "room_full", "this room is full")

func (s *Service) doJoin(c echo.Context, id uuid.UUID, spectator bool) error {
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var notify []uuid.UUID
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var r model.GameRoom
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&r, "id = ?", id).Error; err != nil {
			return httpx.NotFound()
		}
		if r.Status == StatusClosed {
			return httpx.E(http.StatusGone, "room_closed", "this room has closed")
		}
		ps, err := s.members(tx, id)
		if err != nil {
			return err
		}
		blocked, err := safety.BlockedSet(ctx, tx, p.PersonaID)
		if err != nil {
			return err
		}
		for _, m := range ps {
			if m.PersonaID == p.PersonaID {
				return nil
			}
			if blocked[m.PersonaID] {
				return httpx.E(http.StatusForbidden, "cant_join", "you can't join this room")
			}
		}
		// Once a game is running, newcomers can only watch.
		if r.Status != StatusLobby {
			spectator = true
		}
		var nPlayers, nWatch int
		used := map[int16]bool{}
		for _, m := range ps {
			if m.Spectator {
				nWatch++
			} else {
				nPlayers++
				used[m.Seat] = true
			}
		}
		seat := int16(0)
		if !spectator {
			if nPlayers >= int(r.MaxPlayers) {
				return ErrRoomFull
			}
			for used[seat] {
				seat++
			}
		} else if nWatch >= maxSpectators {
			return ErrRoomFull
		}
		notify = memberIDs(ps)
		if err := tx.Create(&model.GamePlayer{RoomID: id, PersonaID: p.PersonaID, Spectator: spectator, Seat: seat, JoinedAt: s.now()}).Error; err != nil {
			return err
		}
		return tx.Model(&r).Updates(map[string]any{"version": gorm.Expr("version + 1"), "last_active_at": s.now()}).Error
	})
	if err != nil {
		return err
	}
	s.ping(ctx, id, notify)
	return s.respond(c, id, http.StatusOK)
}

func (s *Service) ping(ctx context.Context, room uuid.UUID, to []uuid.UUID) {
	if len(to) == 0 {
		ps, err := s.members(s.DB.WithContext(ctx), room)
		if err != nil {
			return
		}
		to = memberIDs(ps)
	}
	var r model.GameRoom
	if s.DB.WithContext(ctx).Select("version").First(&r, "id = ?", room).Error != nil {
		return
	}
	s.Hub.Publish(ctx, to, "game.updated", map[string]any{"room_id": room, "version": r.Version})
}

func (s *Service) leave(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var r model.GameRoom
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&r, "id = ?", id).Error; err != nil {
			return httpx.NotFound()
		}
		var me model.GamePlayer
		if err := tx.First(&me, "room_id = ? AND persona_id = ?", id, p.PersonaID).Error; err != nil {
			return nil
		}
		if err := tx.Delete(&me).Error; err != nil {
			return err
		}
		updates := map[string]any{"version": gorm.Expr("version + 1"), "last_active_at": s.now()}
		rest, err := s.members(tx, id)
		if err != nil {
			return err
		}
		left := players(rest)
		if len(left) == 0 {
			updates["status"] = StatusClosed
		} else if r.HostPersonaID == p.PersonaID {
			updates["host_persona_id"] = left[0].PersonaID
		}
		if r.Status == StatusPlaying && !me.Spectator {
			st, e, err := s.load(&r)
			if err != nil {
				return err
			}
			c := s.ctx()
			if st.Leave(c, p.PersonaID) || len(left) < int(r.MinPlayers) {
				if err := s.save(tx, &r, e, st, updates, len(left) < int(r.MinPlayers)); err != nil {
					return err
				}
				return nil
			}
		}
		return tx.Model(&r).Updates(updates).Error
	})
	if err != nil {
		return err
	}
	s.ping(ctx, id, nil)
	return c.NoContent(http.StatusNoContent)
}

type readyBody struct {
	Ready bool `json:"ready"`
}

func (s *Service) ready(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	req, err := httpx.Bind[readyBody](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	res := s.DB.WithContext(ctx).Model(&model.GamePlayer{}).
		Where("room_id = ? AND persona_id = ? AND spectator = ? AND room_id IN (SELECT id FROM game_rooms WHERE status = ?)", id, p.PersonaID, false, StatusLobby).
		Update("ready", req.Ready)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return httpx.Conflict("you can only get ready in a lobby you're playing in")
	}
	s.DB.WithContext(ctx).Model(&model.GameRoom{}).Where("id = ?", id).Updates(map[string]any{"version": gorm.Expr("version + 1"), "last_active_at": s.now()})
	s.ping(ctx, id, nil)
	return s.respond(c, id, http.StatusOK)
}

func (s *Service) load(r *model.GameRoom) (State, Engine, error) {
	e, ok := Lookup(r.Game)
	if !ok {
		return nil, nil, httpx.Unprocessable("unknown_game", "that game isn't available")
	}
	st, err := e.Load([]byte(r.State))
	return st, e, err
}

// save writes the new state; when the game is over (or forced) it records scores and ends it.
func (s *Service) save(tx *gorm.DB, r *model.GameRoom, _ Engine, st State, updates map[string]any, forceEnd bool) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if updates == nil {
		updates = map[string]any{}
	}
	updates["state"] = string(b)
	updates["version"] = gorm.Expr("version + 1")
	updates["deadline_at"] = st.Deadline()
	updates["last_active_at"] = s.now()
	if st.Over() || forceEnd {
		now := s.now()
		if _, ok := updates["status"]; !ok {
			updates["status"] = StatusFinished
		}
		updates["ended_at"] = now
		updates["deadline_at"] = nil
		for pid, score := range st.Scores() {
			if err := tx.Model(&model.GamePlayer{}).Where("room_id = ? AND persona_id = ?", r.ID, pid).Update("score", score).Error; err != nil {
				return err
			}
		}
	}
	return tx.Model(r).Updates(updates).Error
}

func (s *Service) start(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var r model.GameRoom
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&r, "id = ?", id).Error; err != nil {
			return httpx.NotFound()
		}
		if r.HostPersonaID != p.PersonaID {
			return httpx.Forbidden("only the host can start")
		}
		if r.Status != StatusLobby {
			return httpx.Conflict("this game has already started")
		}
		ps, err := s.members(tx, id)
		if err != nil {
			return err
		}
		pl := players(ps)
		if len(pl) < int(r.MinPlayers) {
			return httpx.Unprocessable("not_enough_players", "waiting for more players")
		}
		ids := []uuid.UUID{}
		for _, m := range pl {
			if !m.Ready && m.PersonaID != r.HostPersonaID {
				return httpx.Unprocessable("not_ready", "everyone needs to be ready")
			}
			ids = append(ids, m.PersonaID)
		}
		e, ok := Lookup(r.Game)
		if !ok {
			return httpx.Unprocessable("unknown_game", "that game isn't available")
		}
		st, err := e.Start(s.ctx(), ids)
		if err != nil {
			return err
		}
		if err := tx.Model(&model.GamePlayer{}).Where("room_id = ?", id).Update("score", 0).Error; err != nil {
			return err
		}
		return s.save(tx, &r, e, st, map[string]any{"status": StatusPlaying, "started_at": s.now(), "ended_at": nil}, false)
	})
	if err != nil {
		return err
	}
	s.ping(ctx, id, nil)
	return s.respond(c, id, http.StatusOK)
}

func (s *Service) move(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	var raw json.RawMessage
	if err := json.NewDecoder(http.MaxBytesReader(c.Response(), c.Request().Body, 64<<10)).Decode(&raw); err != nil {
		return httpx.BadRequest("invalid move")
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	var live *Live
	var members []uuid.UUID
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var r model.GameRoom
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&r, "id = ?", id).Error; err != nil {
			return httpx.NotFound()
		}
		var me model.GamePlayer
		if err := tx.First(&me, "room_id = ? AND persona_id = ?", id, p.PersonaID).Error; err != nil || me.Spectator {
			return httpx.Forbidden("only players can move")
		}
		if r.Status != StatusPlaying {
			return httpx.Conflict("the game isn't running")
		}
		st, e, err := s.load(&r)
		if err != nil {
			return err
		}
		gc := s.ctx()
		st.Tick(gc)
		if live, err = st.Move(gc, p.PersonaID, raw); err != nil {
			if m, ok := IsRefused(err); ok {
				return httpx.Unprocessable("bad_move", m.Msg)
			}
			return err
		}
		ps, err := s.members(tx, id)
		if err != nil {
			return err
		}
		members = memberIDs(ps)
		return s.save(tx, &r, e, st, nil, false)
	})
	if err != nil {
		return err
	}
	if live != nil {
		to := live.To
		if to == nil {
			to = members
		}
		s.Hub.Publish(ctx, to, "game.live", map[string]any{"room_id": id, "type": live.Type, "data": live.Data})
	}
	s.ping(ctx, id, members)
	return c.NoContent(http.StatusNoContent)
}

func (s *Service) rematch(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.GameRoom{}).Where("id = ? AND host_persona_id = ? AND status = ?", id, p.PersonaID, StatusFinished).
			Updates(map[string]any{"status": StatusLobby, "state": "", "deadline_at": nil, "version": gorm.Expr("version + 1"), "last_active_at": s.now()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return httpx.Conflict("only the host can start a rematch after the game ends")
		}
		return tx.Model(&model.GamePlayer{}).Where("room_id = ?", id).Update("ready", false).Error
	})
	if err != nil {
		return err
	}
	s.ping(ctx, id, nil)
	return s.respond(c, id, http.StatusOK)
}

func (s *Service) tickRoom(ctx context.Context, id uuid.UUID) error {
	changed := false
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var r model.GameRoom
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&r, "id = ?", id).Error; err != nil {
			return err
		}
		if r.Status != StatusPlaying {
			return nil
		}
		st, e, err := s.load(&r)
		if err != nil {
			return err
		}
		if !st.Tick(s.ctx()) {
			return nil
		}
		changed = true
		return s.save(tx, &r, e, st, nil, false)
	})
	if err == nil && changed {
		s.ping(ctx, id, nil)
	}
	return err
}

// Tick advances every room whose timer has run out. Run it every couple of seconds.
func (s *Service) Tick(ctx context.Context) error {
	var ids []uuid.UUID
	if err := s.DB.WithContext(ctx).Model(&model.GameRoom{}).Where("status = ? AND deadline_at <= ?", StatusPlaying, s.now()).
		Limit(200).Pluck("id", &ids).Error; err != nil {
		return err
	}
	var errs []error
	for _, id := range ids {
		errs = append(errs, s.tickRoom(ctx, id))
	}
	return errors.Join(errs...)
}

// CloseIdle closes lobbies and finished games nobody has touched for a while.
func (s *Service) CloseIdle(ctx context.Context) error {
	return s.DB.WithContext(ctx).Model(&model.GameRoom{}).
		Where("status IN ? AND last_active_at < ?", []string{StatusLobby, StatusFinished, StatusPlaying}, s.now().Add(-idleClose)).
		Updates(map[string]any{"status": StatusClosed, "deadline_at": nil}).Error
}
