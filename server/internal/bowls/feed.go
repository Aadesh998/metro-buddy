package bowls

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/db"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/safety"
)

type feedQuery struct {
	Sort   string `query:"sort" validate:"omitempty,oneof=hot new top"`
	Window string `query:"t" validate:"omitempty,oneof=day week month all"`
	Cursor string `query:"cursor"`
	Limit  int    `query:"limit"`
}

var windows = map[string]time.Duration{"day": 24 * time.Hour, "week": 7 * 24 * time.Hour, "month": 30 * 24 * time.Hour}

// Keyset cursor over (sort value, id). Hot and top pages stay stable while new posts arrive.
func encodeKey(v float64, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatFloat(v, 'g', -1, 64) + "|" + id.String()))
}

func decodeKey(s string) (float64, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return 0, uuid.Nil, db.ErrBadCursor
	}
	vs, ids, ok := strings.Cut(string(b), "|")
	if !ok {
		return 0, uuid.Nil, db.ErrBadCursor
	}
	v, err := strconv.ParseFloat(vs, 64)
	if err != nil {
		return 0, uuid.Nil, db.ErrBadCursor
	}
	id, err := uuid.Parse(ids)
	if err != nil {
		return 0, uuid.Nil, db.ErrBadCursor
	}
	return v, id, nil
}

func encodeID(id uuid.UUID) string { return base64.RawURLEncoding.EncodeToString(id[:]) }

func decodeID(s string) (uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(b) != 16 {
		return uuid.Nil, db.ErrBadCursor
	}
	return uuid.UUID(b), nil
}

func sortSlice[T any](xs []T, less func(a, b T) bool) {
	sort.SliceStable(xs, func(i, j int) bool { return less(xs[i], xs[j]) })
}

func (h *Handlers) feed(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	viewer := httpx.PrincipalFrom(c).PersonaID
	tx := h.DB.WithContext(c.Request().Context())
	var b model.Bowl
	if err := tx.First(&b, "id = ?", id).Error; err != nil {
		return err
	}
	if ok, err := CanView(tx, &b, viewer); err != nil {
		return err
	} else if !ok {
		return ErrNotMember
	}
	var pinned *model.Post
	if b.Kind == model.BowlCity && b.CityID != nil && c.QueryParam("cursor") == "" {
		var err error
		if pinned, err = h.pinnedPrompt(c.Request().Context(), *b.CityID); err != nil {
			return err
		}
	}
	if pinned == nil && b.PinnedPostID != nil && c.QueryParam("cursor") == "" {
		var p model.Post
		if tx.Where("id = ? AND status = ?", *b.PinnedPostID, model.ContentVisible).Limit(1).Find(&p).RowsAffected == 1 {
			pinned = &p
		}
	}
	return h.servePosts(c, tx.Model(&model.Post{}).Where("posts.bowl_id = ?", b.ID), pinned)
}

// home is the merged feed of every bowl the viewer joined.
func (h *Handlers) home(c echo.Context) error {
	viewer := httpx.PrincipalFrom(c).PersonaID
	q := h.DB.WithContext(c.Request().Context()).Model(&model.Post{}).
		Where("posts.bowl_id IN (SELECT bowl_id FROM bowl_members WHERE persona_id = ?)", viewer)
	var pinned *model.Post
	if c.QueryParam("cursor") == "" {
		var persona model.Persona
		if err := h.DB.WithContext(c.Request().Context()).Select("city_id").First(&persona, "id = ?", viewer).Error; err != nil {
			return err
		}
		var err error
		if pinned, err = h.pinnedPrompt(c.Request().Context(), persona.CityID); err != nil {
			return err
		}
	}
	return h.servePosts(c, q, pinned)
}

// followingFeed shows posts by people the viewer follows, from public bowls or bowls they're in.
func (h *Handlers) followingFeed(c echo.Context) error {
	viewer := httpx.PrincipalFrom(c).PersonaID
	q := h.DB.WithContext(c.Request().Context()).Model(&model.Post{}).
		Where("posts.author_persona_id IN (SELECT followee_persona_id FROM follows WHERE follower_persona_id = ?)", viewer).
		Where("(posts.bowl_id IN (SELECT id FROM bowls WHERE visibility = ? AND deleted_at IS NULL) OR posts.bowl_id IN (SELECT bowl_id FROM bowl_members WHERE persona_id = ?))",
			model.BowlPublic, viewer)
	return h.servePosts(c, q, nil)
}

func (h *Handlers) servePosts(c echo.Context, q *gorm.DB, pinned *model.Post) error {
	req, err := httpx.Bind[feedQuery](c)
	if err != nil {
		return err
	}
	viewer := httpx.PrincipalFrom(c).PersonaID
	q = q.Where("posts.status = ?", model.ContentVisible).Scopes(safety.Hidden(viewer, "posts.author_persona_id"))
	if pinned != nil {
		q = q.Where("posts.id <> ?", pinned.ID)
	}
	limit := db.ClampLimit(req.Limit)
	var rows []model.Post
	var key func(p model.Post) string

	switch req.Sort {
	case "new":
		pq, l, err := db.PageDesc(q, "posts", req.Cursor, limit)
		if err != nil {
			return err
		}
		if err := pq.Find(&rows).Error; err != nil {
			return err
		}
		var next string
		rows, next = db.Trim(rows, l, func(p model.Post) db.Cursor { return db.Cursor{CreatedAt: p.CreatedAt, ID: p.ID} })
		return h.respondPosts(c, rows, next, pinned)
	case "top":
		if d, ok := windows[req.Window]; ok {
			q = q.Where("posts.created_at > ?", h.now().Add(-d))
		}
		q, err = keyset(q, "posts.score", req.Cursor)
		key = func(p model.Post) string { return encodeKey(float64(p.Score), p.ID) }
		q = q.Order("posts.score DESC").Order("posts.id DESC")
	default:
		q, err = keyset(q, "posts.hot_rank", req.Cursor)
		key = func(p model.Post) string { return encodeKey(p.HotRank, p.ID) }
		q = q.Order("posts.hot_rank DESC").Order("posts.id DESC")
	}
	if err != nil {
		return err
	}
	if err := q.Limit(limit + 1).Find(&rows).Error; err != nil {
		return err
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = key(rows[len(rows)-1])
	}
	return h.respondPosts(c, rows, next, pinned)
}

func keyset(q *gorm.DB, col, cursor string) (*gorm.DB, error) {
	if cursor == "" {
		return q, nil
	}
	v, id, err := decodeKey(cursor)
	if err != nil {
		return nil, err
	}
	return q.Where(fmt.Sprintf("(%s < ? OR (%s = ? AND posts.id < ?))", col, col), v, v, id), nil
}

func (h *Handlers) respondPosts(c echo.Context, rows []model.Post, next string, pinned *model.Post) error {
	ctx, viewer := c.Request().Context(), httpx.PrincipalFrom(c).PersonaID
	out, err := h.PostDTOs(ctx, viewer, rows)
	if err != nil {
		return err
	}
	var pin *PostDTO
	if pinned != nil {
		ps, err := h.PostDTOs(ctx, viewer, []model.Post{*pinned})
		if err != nil {
			return err
		}
		pin = &ps[0]
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out, "next_cursor": next, "pinned": pin})
}

// RefreshHotRanks recomputes hot_rank for recent posts; older posts have decayed out of hot anyway.
func (h *Handlers) RefreshHotRanks(ctx context.Context) error {
	since := h.now().Add(-7 * 24 * time.Hour)
	var rows []struct {
		ID        uuid.UUID
		Score     int
		CreatedAt time.Time
		HotRank   float64
	}
	if err := h.DB.WithContext(ctx).Model(&model.Post{}).Select("id, score, created_at, hot_rank").
		Where("created_at > ?", since).Scan(&rows).Error; err != nil {
		return err
	}
	return h.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			if hr := hotRank(r.Score, r.CreatedAt); hr != r.HotRank {
				if err := tx.Model(&model.Post{}).Where("id = ?", r.ID).Update("hot_rank", hr).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
