package bowls

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/safety"
)

const MaxCommentDepth = 2

type createCommentRequest struct {
	Body     string     `json:"body" validate:"required,max=2000"`
	ParentID *uuid.UUID `json:"parent_id"`
}

type CommentDTO struct {
	ID        uuid.UUID           `json:"id"`
	PostID    uuid.UUID           `json:"post_id"`
	ParentID  *uuid.UUID          `json:"parent_id"`
	Depth     int16               `json:"depth"`
	Author    identity.PersonaRef `json:"author"`
	IsOP      bool                `json:"is_op"`
	IsMine    bool                `json:"is_mine"`
	Body      string              `json:"body"`
	Deleted   bool                `json:"deleted"`
	Score     int                 `json:"score"`
	MyVote    int16               `json:"my_vote"`
	PIIHidden bool                `json:"pii_hidden"`
	CreatedAt time.Time           `json:"created_at"`
}

func (h *Handlers) createComment(c echo.Context) error {
	req, err := httpx.Bind[createCommentRequest](c)
	if err != nil {
		return err
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	post, err := h.loadVisiblePost(ctx, p.PersonaID, c.Param("id"))
	if err != nil {
		return err
	}
	cm := model.Comment{PostID: post.ID, AuthorPersonaID: p.PersonaID, Status: model.ContentVisible}
	cm.ID = model.NewID()
	var parentAuthor *uuid.UUID
	err = h.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if req.ParentID != nil {
			var parent model.Comment
			if err := tx.First(&parent, "id = ? AND post_id = ?", *req.ParentID, post.ID).Error; err != nil {
				return httpx.Unprocessable("invalid_parent", "that comment no longer exists")
			}
			if blocked, err := safety.IsBlocked(ctx, tx, p.PersonaID, parent.AuthorPersonaID); err != nil || blocked {
				return httpx.Unprocessable("invalid_parent", "that comment no longer exists")
			}
			// Replies deeper than the limit attach beside the comment instead of under it.
			if parent.Depth >= MaxCommentDepth {
				cm.ParentID, cm.Depth = parent.ParentID, parent.Depth
			} else {
				cm.ParentID, cm.Depth = &parent.ID, parent.Depth+1
			}
			parentAuthor = &parent.AuthorPersonaID
		}
		r, err := h.PII.Process(tx, model.CTComment, cm.ID, req.Body, pii.Options{})
		if err != nil {
			return err
		}
		cm.BodyMasked, cm.PIIFlags = r.Text, r.Flags
		if err := tx.Create(&cm).Error; err != nil {
			return err
		}
		return tx.Model(&model.Post{}).Where("id = ?", post.ID).Update("comment_count", gorm.Expr("comment_count + 1")).Error
	})
	if err != nil {
		return err
	}
	h.PII.AfterCommit(ctx, model.CTComment, cm.ID)
	if h.OnUGC != nil {
		h.OnUGC(ctx, p.PersonaID, req.Body)
	}
	if h.OnReply != nil {
		to := post.AuthorPersonaID
		if parentAuthor != nil {
			to = *parentAuthor
		}
		if to != p.PersonaID {
			h.OnReply(ctx, to, post.ID, cm.ID)
		}
	}
	out, err := h.commentDTOs(c, post, []model.Comment{cm})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, out[0])
}

func (h *Handlers) commentDTOs(c echo.Context, post *model.Post, cs []model.Comment) ([]CommentDTO, error) {
	viewer := httpx.PrincipalFrom(c).PersonaID
	tx := h.DB.WithContext(c.Request().Context())
	ids := make([]uuid.UUID, 0, len(cs))
	authors := make([]uuid.UUID, 0, len(cs))
	for _, cm := range cs {
		ids = append(ids, cm.ID)
		authors = append(authors, cm.AuthorPersonaID)
	}
	refs, err := identity.LoadRefs(tx, authors)
	if err != nil {
		return nil, err
	}
	my := map[uuid.UUID]int16{}
	if len(ids) > 0 {
		var votes []model.Vote
		if err := tx.Where("persona_id = ? AND target_type = ? AND target_id IN ?", viewer, model.VoteComment, ids).Find(&votes).Error; err != nil {
			return nil, err
		}
		for _, v := range votes {
			my[v.TargetID] = v.Value
		}
	}
	out := make([]CommentDTO, len(cs))
	for i, cm := range cs {
		d := CommentDTO{ID: cm.ID, PostID: cm.PostID, ParentID: cm.ParentID, Depth: cm.Depth, Author: refs[cm.AuthorPersonaID],
			IsOP: cm.AuthorPersonaID == post.AuthorPersonaID, IsMine: cm.AuthorPersonaID == viewer, Body: cm.BodyMasked,
			Score: cm.Score, MyVote: my[cm.ID], PIIHidden: cm.PIIFlags != 0, CreatedAt: cm.CreatedAt}
		// Removed comments keep their place in the thread so replies still make sense.
		if cm.DeletedAt.Valid || cm.Status != model.ContentVisible {
			d.Deleted, d.Body, d.Author, d.IsOP, d.Score = true, "", identity.DeletedRef(uuid.Nil), false, 0
		}
		out[i] = d
	}
	return out, nil
}

type commentsQuery struct {
	Sort   string `query:"sort" validate:"omitempty,oneof=top new"`
	Cursor string `query:"cursor"`
}

const commentPage = 500

func (h *Handlers) listComments(c echo.Context) error {
	req, err := httpx.Bind[commentsQuery](c)
	if err != nil {
		return err
	}
	viewer := httpx.PrincipalFrom(c).PersonaID
	post, err := h.loadVisiblePost(c.Request().Context(), viewer, c.Param("id"))
	if err != nil {
		return err
	}
	q := h.DB.WithContext(c.Request().Context()).Unscoped().Model(&model.Comment{}).
		Where("post_id = ?", post.ID).Scopes(safety.Hidden(viewer, "author_persona_id"))
	if req.Cursor != "" {
		cur, err := decodeID(req.Cursor)
		if err != nil {
			return err
		}
		q = q.Where("id > ?", cur)
	}
	var rows []model.Comment
	if err := q.Order("id").Limit(commentPage + 1).Find(&rows).Error; err != nil {
		return err
	}
	next := ""
	if len(rows) > commentPage {
		rows = rows[:commentPage]
		next = encodeID(rows[len(rows)-1].ID)
	}
	out, err := h.commentDTOs(c, post, pruneDeleted(rows))
	if err != nil {
		return err
	}
	sortThread(out, req.Sort)
	return c.JSON(http.StatusOK, map[string]any{"items": out, "next_cursor": next})
}

// pruneDeleted drops removed comments that have no visible replies left.
func pruneDeleted(cs []model.Comment) []model.Comment {
	hasChild := map[uuid.UUID]bool{}
	for _, c := range cs {
		if c.ParentID != nil && !c.DeletedAt.Valid && c.Status == model.ContentVisible {
			hasChild[*c.ParentID] = true
		}
	}
	out := cs[:0]
	for _, c := range cs {
		if (c.DeletedAt.Valid || c.Status != model.ContentVisible) && !hasChild[c.ID] {
			continue
		}
		out = append(out, c)
	}
	return out
}

// sortThread orders comments depth-first: siblings by score (top) or time (new), children under parents.
func sortThread(cs []CommentDTO, mode string) {
	children := map[uuid.UUID][]CommentDTO{}
	var roots []CommentDTO
	present := map[uuid.UUID]bool{}
	for _, c := range cs {
		present[c.ID] = true
	}
	for _, c := range cs {
		if c.ParentID != nil && present[*c.ParentID] {
			children[*c.ParentID] = append(children[*c.ParentID], c)
		} else {
			roots = append(roots, c)
		}
	}
	less := func(a, b CommentDTO) bool {
		if mode != "new" && a.Score != b.Score {
			return a.Score > b.Score
		}
		return a.CreatedAt.Before(b.CreatedAt)
	}
	var walk func(list []CommentDTO)
	out := cs[:0]
	walk = func(list []CommentDTO) {
		sortSlice(list, less)
		for _, c := range list {
			out = append(out, c)
			walk(children[c.ID])
		}
	}
	walk(roots)
}

func (h *Handlers) deleteComment(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	viewer := httpx.PrincipalFrom(c).PersonaID
	err = h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var cm model.Comment
		if err := tx.First(&cm, "id = ? AND author_persona_id = ?", id, viewer).Error; err != nil {
			return err
		}
		if err := tx.Model(&cm).Update("status", model.ContentRemovedByAuthor).Error; err != nil {
			return err
		}
		if err := tx.Delete(&cm).Error; err != nil {
			return err
		}
		return tx.Model(&model.Post{}).Where("id = ? AND comment_count > 0", cm.PostID).Update("comment_count", gorm.Expr("comment_count - 1")).Error
	})
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

type voteRequest struct {
	TargetType model.VoteTarget `json:"target_type" validate:"required,oneof=post comment"`
	TargetID   uuid.UUID        `json:"target_id" validate:"required"`
	Value      int16            `json:"value" validate:"oneof=-1 0 1"`
}

// vote is idempotent per persona: repeating a vote is a no-op, changing it applies only the difference.
func (h *Handlers) vote(c echo.Context) error {
	req, err := httpx.Bind[voteRequest](c)
	if err != nil {
		return err
	}
	viewer := httpx.PrincipalFrom(c).PersonaID
	ctx := c.Request().Context()
	table := "posts"
	if req.TargetType == model.VoteComment {
		table = "comments"
		var cm model.Comment
		if err := h.DB.WithContext(ctx).First(&cm, "id = ?", req.TargetID).Error; err != nil {
			return err
		}
		if _, err := h.loadVisiblePost(ctx, viewer, cm.PostID.String()); err != nil {
			return err
		}
	} else if _, err := h.loadVisiblePost(ctx, viewer, req.TargetID.String()); err != nil {
		return err
	}
	var score int
	err = h.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var prev model.Vote
		found := tx.Where("persona_id = ? AND target_type = ? AND target_id = ?", viewer, req.TargetType, req.TargetID).Limit(1).Find(&prev).RowsAffected == 1
		delta := int(req.Value)
		if found {
			delta -= int(prev.Value)
		}
		switch {
		case req.Value == 0 && found:
			if err := tx.Delete(&prev).Error; err != nil {
				return err
			}
		case req.Value != 0 && found:
			if err := tx.Model(&prev).Where("persona_id = ? AND target_type = ? AND target_id = ?", viewer, req.TargetType, req.TargetID).
				Update("value", req.Value).Error; err != nil {
				return err
			}
		case req.Value != 0:
			if err := tx.Create(&model.Vote{PersonaID: viewer, TargetType: req.TargetType, TargetID: req.TargetID, Value: req.Value}).Error; err != nil {
				return err
			}
		}
		if delta != 0 {
			if err := tx.Table(table).Where("id = ?", req.TargetID).Update("score", gorm.Expr("score + ?", delta)).Error; err != nil {
				return err
			}
		}
		return tx.Table(table).Select("score").Where("id = ?", req.TargetID).Scan(&score).Error
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"score": score, "my_vote": req.Value})
}
