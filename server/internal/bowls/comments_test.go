package bowls_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"metro-buddy/server/internal/bowls"
)

func TestThreadedCommentsDepthOPBadgeAndDeletion(t *testing.T) {
	f := setup(t)
	var p bowls.PostDTO
	f.post(f.a, map[string]any{"type": "text", "title": "Seat wars"}).Want(http.StatusCreated).JSON(&p)
	path := "/v1/posts/" + p.ID.String() + "/comments"
	var c0, c1, c2, c3 bowls.CommentDTO
	f.env.Do(http.MethodPost, path, f.b.AccessToken, map[string]any{"body": "Same here"}).Want(http.StatusCreated).JSON(&c0)
	f.env.Do(http.MethodPost, path, f.a.AccessToken, map[string]any{"body": "OP reply", "parent_id": c0.ID}).Want(http.StatusCreated).JSON(&c1)
	f.env.Do(http.MethodPost, path, f.b.AccessToken, map[string]any{"body": "deeper", "parent_id": c1.ID}).Want(http.StatusCreated).JSON(&c2)
	f.env.Do(http.MethodPost, path, f.a.AccessToken, map[string]any{"body": "deepest", "parent_id": c2.ID}).Want(http.StatusCreated).JSON(&c3)
	if c1.Depth != 1 || !c1.IsOP || c2.Depth != 2 || c3.Depth != 2 || *c3.ParentID != c1.ID {
		t.Fatalf("depths %d %d %d parent %v", c1.Depth, c2.Depth, c3.Depth, c3.ParentID)
	}
	f.env.Do(http.MethodDelete, "/v1/comments/"+c0.ID.String(), f.b.AccessToken, nil).Want(http.StatusNoContent)

	var list struct{ Items []bowls.CommentDTO }
	f.env.Do(http.MethodGet, path, f.b.AccessToken, nil).Want(http.StatusOK).JSON(&list)
	if len(list.Items) != 4 || list.Items[0].ID != c0.ID || !list.Items[0].Deleted || list.Items[0].Body != "" || list.Items[1].ID != c1.ID {
		t.Fatalf("%+v", list.Items)
	}
	var got bowls.PostDTO
	f.env.Do(http.MethodGet, "/v1/posts/"+p.ID.String(), f.a.AccessToken, nil).Want(http.StatusOK).JSON(&got)
	if got.CommentCount != 3 {
		t.Fatalf("comment_count %d", got.CommentCount)
	}
}

func TestVotesAreIdempotentPerPersona(t *testing.T) {
	f := setup(t)
	var p bowls.PostDTO
	f.post(f.a, map[string]any{"type": "text", "title": "vote me"}).Want(http.StatusCreated).JSON(&p)
	vote := func(v int) int {
		var r struct{ Score int }
		f.env.Do(http.MethodPost, "/v1/votes", f.b.AccessToken, map[string]any{"target_type": "post", "target_id": p.ID, "value": v}).Want(http.StatusOK).JSON(&r)
		return r.Score
	}
	for _, step := range []struct{ v, want int }{{1, 1}, {1, 1}, {-1, -1}, {-1, -1}, {0, 0}, {0, 0}, {1, 1}} {
		if got := vote(step.v); got != step.want {
			t.Fatalf("vote %d -> score %d, want %d", step.v, got, step.want)
		}
	}
	f.env.Do(http.MethodPost, "/v1/votes", f.b.AccessToken, map[string]any{"target_type": "post", "target_id": uuid.New(), "value": 1}).Want(http.StatusNotFound)
}

func TestFeedSortsPaginatesAndHidesBlockedAuthors(t *testing.T) {
	f := setup(t)
	ids := []uuid.UUID{}
	for i := range 7 {
		var p bowls.PostDTO
		u := f.a
		if i%2 == 1 {
			u = f.b
		}
		f.post(u, map[string]any{"type": "text", "title": "post"}).Want(http.StatusCreated).JSON(&p)
		ids = append(ids, p.ID)
	}
	f.env.Do(http.MethodPost, "/v1/votes", f.b.AccessToken, map[string]any{"target_type": "post", "target_id": ids[0], "value": 1}).Want(http.StatusOK)
	if err := f.env.App.Bowls.RefreshHotRanks(t.Context()); err != nil {
		t.Fatal(err)
	}
	walk := func(sort string, tok string) []uuid.UUID {
		var out []uuid.UUID
		cursor := ""
		for {
			var page struct {
				Items      []bowls.PostDTO
				NextCursor string `json:"next_cursor"`
			}
			f.env.Do(http.MethodGet, "/v1/bowls/"+f.bowl+"/posts?limit=3&sort="+sort+"&cursor="+cursor, tok, nil).Want(http.StatusOK).JSON(&page)
			for _, p := range page.Items {
				out = append(out, p.ID)
			}
			if page.NextCursor == "" {
				return out
			}
			cursor = page.NextCursor
		}
	}
	for _, s := range []string{"new", "hot", "top"} {
		got := walk(s, f.a.AccessToken)
		if len(got) != 7 {
			t.Fatalf("%s: %d posts", s, len(got))
		}
	}
	if top := walk("top", f.a.AccessToken); top[0] != ids[0] {
		t.Fatal("top feed should start with the upvoted post")
	}
	if n := walk("new", f.a.AccessToken); n[0] != ids[6] {
		t.Fatal("new feed should start with the latest post")
	}
	f.env.Do(http.MethodPost, "/v1/blocks", f.a.AccessToken, map[string]any{"persona_id": f.b.PersonaID}).Want(http.StatusNoContent)
	if got := walk("hot", f.a.AccessToken); len(got) != 4 {
		t.Fatalf("blocked author's posts still visible: %d", len(got))
	}
	if got := walk("hot", f.b.AccessToken); len(got) != 3 {
		t.Fatalf("blocker's posts visible to blocked user: %d", len(got))
	}
	f.env.Do(http.MethodGet, "/v1/posts/"+ids[0].String(), f.b.AccessToken, nil).Want(http.StatusNotFound)
	var home struct{ Items []bowls.PostDTO }
	f.env.Do(http.MethodGet, "/v1/feed?sort=new", f.a.AccessToken, nil).Want(http.StatusOK).JSON(&home)
	if len(home.Items) != 4 {
		t.Fatalf("home feed %d", len(home.Items))
	}
}
