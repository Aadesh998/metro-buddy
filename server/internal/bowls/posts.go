package bowls

import (
	"context"
	"math"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/identity"
	"metro-buddy/server/internal/media"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
	"metro-buddy/server/internal/ratelimit"
	"metro-buddy/server/internal/safety"
)

var Flairs = []string{"Rant", "Advice", "Question", "Wholesome", "Funny", "Lost & Found", "PSA", "Daily Prompt"}

var (
	ErrMuted     = httpx.Forbidden("you're muted in this bowl for now")
	ErrPollShape = httpx.Unprocessable("invalid_poll", "polls need 2 to 6 options")
	ErrMedia     = httpx.Unprocessable("invalid_media", "image posts need 1 to 4 images")
	ErrBadFlair  = httpx.Unprocessable("invalid_flair", "unknown flair")
)

func (h *Handlers) registerPosts(g *echo.Group) {
	posts := h.Limiter.Middleware(ratelimit.Rule{Name: "posts", Limit: 5, Window: time.Hour, ByTier: true})
	g.POST("/posts", h.createPost, posts)
	g.GET("/posts/:id", h.getPost)
	g.DELETE("/posts/:id", h.deletePost)
	g.POST("/posts/:id/poll/vote", h.votePoll)
	g.GET("/bowls/:id/posts", h.feed)
	g.POST("/posts/:id/comments", h.createComment,
		h.Limiter.Middleware(ratelimit.Rule{Name: "comments", Limit: 30, Window: time.Hour, ByTier: true}))
	g.GET("/posts/:id/comments", h.listComments)
	g.DELETE("/comments/:id", h.deleteComment)
	g.POST("/votes", h.vote, h.Limiter.Middleware(ratelimit.Rule{Name: "votes", Limit: 300, Window: time.Hour}))
}

type pollInput struct {
	Options       []string `json:"options" validate:"required,dive,required,max=60"`
	ClosesInHours int      `json:"closes_in_hours" validate:"required,min=1,max=168"`
	Multi         bool     `json:"multi"`
}

type createPostRequest struct {
	BowlID    uuid.UUID      `json:"bowl_id" validate:"required"`
	Type      model.PostType `json:"type" validate:"required,oneof=text image poll"`
	Title     string         `json:"title" validate:"required,max=140"`
	Body      string         `json:"body" validate:"max=5000"`
	Flair     *string        `json:"flair"`
	MediaKeys []string       `json:"media_keys" validate:"max=4,dive,max=200"`
	Poll      *pollInput     `json:"poll"`
}

func hotRank(score int, created time.Time) float64 {
	order := math.Log10(math.Max(math.Abs(float64(score)), 1))
	sign := 0.0
	if score > 0 {
		sign = 1
	} else if score < 0 {
		sign = -1
	}
	return math.Round((sign*order+float64(created.Unix()-1_700_000_000)/45000)*1e7) / 1e7
}

func (h *Handlers) createPost(c echo.Context) error {
	req, err := httpx.Bind[createPostRequest](c)
	if err != nil {
		return err
	}
	if req.Flair != nil && !slices.Contains(Flairs, *req.Flair) {
		return ErrBadFlair
	}
	switch req.Type {
	case model.PostPoll:
		if req.Poll == nil || len(req.Poll.Options) < 2 || len(req.Poll.Options) > 6 {
			return ErrPollShape
		}
	case model.PostImage:
		if len(req.MediaKeys) == 0 {
			return ErrMedia
		}
	}
	if req.Type != model.PostImage && len(req.MediaKeys) > 0 {
		return ErrMedia
	}
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	if err := h.checkCanPost(ctx, req.BowlID, p.PersonaID); err != nil {
		return err
	}

	var medias model.MediaList
	needsReview := false
	for _, k := range req.MediaKeys {
		m, verdict, err := h.Media.Finalize(ctx, p.PersonaID, k)
		if err != nil {
			return err
		}
		needsReview = needsReview || verdict == media.Review
		medias = append(medias, m)
	}

	now := h.now()
	post := model.Post{BowlID: req.BowlID, AuthorPersonaID: p.PersonaID, Type: req.Type, Flair: req.Flair, Media: medias, Status: model.ContentVisible}
	post.ID = model.NewID()
	post.CreatedAt = now
	err = h.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		raw := req.Title + "\n\n" + req.Body
		var labels []string
		if req.Poll != nil {
			labels = req.Poll.Options
			for _, o := range labels {
				raw += "\n- " + o
			}
		}
		r, err := h.PII.Process(tx, model.CTPost, post.ID, raw, pii.Options{})
		if err != nil {
			return err
		}
		post.Title = pii.Mask(req.Title, pii.Options{}).Text
		post.BodyMasked = pii.Mask(req.Body, pii.Options{}).Text
		post.PIIFlags = r.Flags
		post.HotRank = hotRank(0, now)
		if err := tx.Create(&post).Error; err != nil {
			return err
		}
		if req.Poll != nil {
			if err := tx.Create(&model.Poll{PostID: post.ID, ClosesAt: now.Add(time.Duration(req.Poll.ClosesInHours) * time.Hour), Multi: req.Poll.Multi}).Error; err != nil {
				return err
			}
			for i, o := range labels {
				if err := tx.Create(&model.PollOption{PostID: post.ID, Label: pii.Mask(o, pii.Options{}).Text, Position: int16(i)}).Error; err != nil {
					return err
				}
			}
		}
		if needsReview {
			_, err := safety.OpenCase(tx, now, model.CTPost, post.ID, &p.PersonaID, model.SevMedium, "image_review")
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	h.PII.AfterCommit(ctx, model.CTPost, post.ID)
	if h.OnUGC != nil {
		h.OnUGC(ctx, p.PersonaID, req.Title+"\n"+req.Body)
	}
	out, err := h.PostDTOs(ctx, p.PersonaID, []model.Post{post})
	if err != nil {
		return err
	}
	out[0].ShowSupport = safety.Distress(req.Title + "\n" + req.Body)
	return c.JSON(http.StatusCreated, out[0])
}

func (h *Handlers) checkCanPost(ctx context.Context, bowlID, persona uuid.UUID) error {
	tx := h.DB.WithContext(ctx)
	var b model.Bowl
	if err := tx.First(&b, "id = ?", bowlID).Error; err != nil {
		return err
	}
	m, err := membership(tx, b.ID, persona)
	if err != nil {
		return err
	}
	if m == nil {
		return ErrNotMember
	}
	if m.MutedUntil != nil && h.now().Before(*m.MutedUntil) {
		return ErrMuted
	}
	return nil
}

type PollOptionDTO struct {
	ID    uuid.UUID `json:"id"`
	Label string    `json:"label"`
	Votes int       `json:"votes"`
	Mine  bool      `json:"mine"`
}

type PollDTO struct {
	ClosesAt time.Time       `json:"closes_at"`
	Closed   bool            `json:"closed"`
	Multi    bool            `json:"multi"`
	Total    int             `json:"total"`
	Options  []PollOptionDTO `json:"options"`
}

type BowlRef struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
	Name string    `json:"name"`
}

type PostDTO struct {
	ID           uuid.UUID           `json:"id"`
	Bowl         BowlRef             `json:"bowl"`
	Author       identity.PersonaRef `json:"author"`
	IsMine       bool                `json:"is_mine"`
	Type         model.PostType      `json:"type"`
	Title        string              `json:"title"`
	Body         string              `json:"body"`
	Flair        *string             `json:"flair"`
	Media        []media.MediaDTO    `json:"media"`
	Poll         *PollDTO            `json:"poll"`
	Score        int                 `json:"score"`
	MyVote       int16               `json:"my_vote"`
	CommentCount int                 `json:"comment_count"`
	PIIHidden    bool                `json:"pii_hidden"`
	IsPrompt     bool                `json:"is_prompt"`
	CreatedAt    time.Time           `json:"created_at"`
	// Set only on the author's own create response when the text suggests they may be at risk.
	ShowSupport bool `json:"show_support,omitempty"`
}

// PostDTOs batch-loads authors, bowls, votes and polls so a feed page is a fixed number of queries.
func (h *Handlers) PostDTOs(ctx context.Context, viewer uuid.UUID, posts []model.Post) ([]PostDTO, error) {
	tx := h.DB.WithContext(ctx)
	out := make([]PostDTO, 0, len(posts))
	if len(posts) == 0 {
		return out, nil
	}
	var authorIDs, bowlIDs, postIDs, pollIDs []uuid.UUID
	for _, p := range posts {
		authorIDs = append(authorIDs, p.AuthorPersonaID)
		bowlIDs = append(bowlIDs, p.BowlID)
		postIDs = append(postIDs, p.ID)
		if p.Type == model.PostPoll {
			pollIDs = append(pollIDs, p.ID)
		}
	}
	refs, err := identity.LoadRefs(tx, authorIDs)
	if err != nil {
		return nil, err
	}
	var bowls []model.Bowl
	if err := tx.Unscoped().Where("id IN ?", bowlIDs).Find(&bowls).Error; err != nil {
		return nil, err
	}
	bowlByID := map[uuid.UUID]model.Bowl{}
	for _, b := range bowls {
		bowlByID[b.ID] = b
	}
	var votes []model.Vote
	if err := tx.Where("persona_id = ? AND target_type = ? AND target_id IN ?", viewer, model.VotePost, postIDs).Find(&votes).Error; err != nil {
		return nil, err
	}
	myVote := map[uuid.UUID]int16{}
	for _, v := range votes {
		myVote[v.TargetID] = v.Value
	}
	polls, err := h.loadPolls(tx, viewer, pollIDs)
	if err != nil {
		return nil, err
	}
	for _, p := range posts {
		b := bowlByID[p.BowlID]
		out = append(out, PostDTO{
			ID: p.ID, Bowl: BowlRef{ID: b.ID, Slug: b.Slug, Name: b.Name}, Author: refs[p.AuthorPersonaID],
			IsMine: p.AuthorPersonaID == viewer, Type: p.Type, Title: p.Title, Body: p.BodyMasked, Flair: p.Flair,
			Media: h.Media.DTOs(ctx, p.Media), Poll: polls[p.ID], Score: p.Score, MyVote: myVote[p.ID],
			CommentCount: p.CommentCount, PIIHidden: p.PIIFlags != 0, IsPrompt: p.IsPrompt, CreatedAt: p.CreatedAt,
		})
	}
	return out, nil
}

func (h *Handlers) loadPolls(tx *gorm.DB, viewer uuid.UUID, postIDs []uuid.UUID) (map[uuid.UUID]*PollDTO, error) {
	out := map[uuid.UUID]*PollDTO{}
	if len(postIDs) == 0 {
		return out, nil
	}
	var polls []model.Poll
	if err := tx.Where("post_id IN ?", postIDs).Find(&polls).Error; err != nil {
		return nil, err
	}
	var opts []model.PollOption
	if err := tx.Where("post_id IN ?", postIDs).Order("position").Find(&opts).Error; err != nil {
		return nil, err
	}
	var mine []model.PollVote
	if err := tx.Where("persona_id = ? AND post_id IN ?", viewer, postIDs).Find(&mine).Error; err != nil {
		return nil, err
	}
	voted := map[uuid.UUID]bool{}
	for _, v := range mine {
		voted[v.OptionID] = true
	}
	now := h.now()
	for _, p := range polls {
		out[p.PostID] = &PollDTO{ClosesAt: p.ClosesAt, Closed: now.After(p.ClosesAt), Multi: p.Multi, Options: []PollOptionDTO{}}
	}
	for _, o := range opts {
		d := out[o.PostID]
		if d == nil {
			continue
		}
		d.Options = append(d.Options, PollOptionDTO{ID: o.ID, Label: o.Label, Votes: o.VoteCount, Mine: voted[o.ID]})
		d.Total += o.VoteCount
	}
	return out, nil
}

// loadVisiblePost returns the post only if the viewer may see it: visible status, bowl
// visibility, and no block between viewer and author.
func (h *Handlers) loadVisiblePost(ctx context.Context, viewer uuid.UUID, idParam string) (*model.Post, error) {
	id, err := uuid.Parse(idParam)
	if err != nil {
		return nil, httpx.NotFound()
	}
	tx := h.DB.WithContext(ctx)
	var p model.Post
	if err := tx.First(&p, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if p.Status != model.ContentVisible && p.AuthorPersonaID != viewer {
		return nil, httpx.NotFound()
	}
	var b model.Bowl
	if err := tx.First(&b, "id = ?", p.BowlID).Error; err != nil {
		return nil, err
	}
	if ok, err := CanView(tx, &b, viewer); err != nil || !ok {
		return nil, httpx.NotFound()
	}
	if blocked, err := safety.IsBlocked(ctx, h.DB, viewer, p.AuthorPersonaID); err != nil || blocked {
		return nil, httpx.NotFound()
	}
	return &p, nil
}

func (h *Handlers) getPost(c echo.Context) error {
	viewer := httpx.PrincipalFrom(c).PersonaID
	p, err := h.loadVisiblePost(c.Request().Context(), viewer, c.Param("id"))
	if err != nil {
		return err
	}
	out, err := h.PostDTOs(c.Request().Context(), viewer, []model.Post{*p})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out[0])
}

func (h *Handlers) deletePost(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	viewer := httpx.PrincipalFrom(c).PersonaID
	err = h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var p model.Post
		if err := tx.First(&p, "id = ? AND author_persona_id = ?", id, viewer).Error; err != nil {
			return err
		}
		if err := tx.Model(&p).Update("status", model.ContentRemovedByAuthor).Error; err != nil {
			return err
		}
		return tx.Delete(&p).Error
	})
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

type pollVoteRequest struct {
	OptionIDs []uuid.UUID `json:"option_ids" validate:"required,min=1,max=6,unique"`
}

func (h *Handlers) votePoll(c echo.Context) error {
	req, err := httpx.Bind[pollVoteRequest](c)
	if err != nil {
		return err
	}
	viewer := httpx.PrincipalFrom(c).PersonaID
	ctx := c.Request().Context()
	post, err := h.loadVisiblePost(ctx, viewer, c.Param("id"))
	if err != nil {
		return err
	}
	err = h.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var poll model.Poll
		if err := tx.First(&poll, "post_id = ?", post.ID).Error; err != nil {
			return err
		}
		if h.now().After(poll.ClosesAt) {
			return httpx.Conflict("this poll has closed")
		}
		if !poll.Multi && len(req.OptionIDs) > 1 {
			return httpx.Unprocessable("single_choice", "pick one option")
		}
		var already int64
		if err := tx.Model(&model.PollVote{}).Where("post_id = ? AND persona_id = ?", post.ID, viewer).Count(&already).Error; err != nil {
			return err
		}
		if already > 0 {
			return httpx.Conflict("you already voted")
		}
		var n int64
		if err := tx.Model(&model.PollOption{}).Where("post_id = ? AND id IN ?", post.ID, req.OptionIDs).Count(&n).Error; err != nil {
			return err
		}
		if int(n) != len(req.OptionIDs) {
			return httpx.Unprocessable("invalid_option", "unknown option")
		}
		for _, oid := range req.OptionIDs {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.PollVote{OptionID: oid, PersonaID: viewer, PostID: post.ID}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.PollOption{}).Where("id = ?", oid).Update("vote_count", gorm.Expr("vote_count + 1")).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	out, err := h.PostDTOs(ctx, viewer, []model.Post{*post})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out[0])
}
