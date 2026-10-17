package admin_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/bowls"
	"metro-buddy/server/internal/model"
)

func decode(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
}

func TestReportToActionToAppealRoundTrip(t *testing.T) {
	env := apitest.New(t)
	c := console{env}
	city := env.Seeded()
	author, reporter := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	var list struct{ Items []bowls.BowlDTO }
	env.Do(http.MethodGet, "/v1/bowls?kind=topic", author.AccessToken, nil).Want(http.StatusOK).JSON(&list)
	bowl := list.Items[0].ID.String()
	env.Do(http.MethodPost, "/v1/bowls/"+bowl+"/join", author.AccessToken, nil).Want(http.StatusOK)
	var p bowls.PostDTO
	env.Do(http.MethodPost, "/v1/posts", author.AccessToken, map[string]any{"bowl_id": bowl, "type": "text", "title": "you are all idiots"}).Want(http.StatusCreated).JSON(&p)
	env.Do(http.MethodPost, "/v1/reports", reporter.AccessToken, map[string]any{"target_type": "post", "target_id": p.ID, "reason": "harassment"}).Want(http.StatusCreated)

	mod := cookieOf(c.login(t, c.newAdmin(t, "mod@m.in", model.AdminModerator), ""))
	lead := cookieOf(c.login(t, c.newAdmin(t, "lead@m.in", model.AdminTSLead), ""))

	var q struct {
		Items []struct {
			ID        uuid.UUID
			Severity  string
			TopReason string `json:"top_reason"`
		}
	}
	decode(t, c.do(http.MethodGet, "/admin/api/cases", mod, "", "").Body.Bytes(), &q)
	if len(q.Items) != 1 || q.Items[0].TopReason != "harassment" {
		t.Fatalf("%+v", q)
	}
	caseID := q.Items[0].ID.String()
	c.do(http.MethodPost, "/admin/api/cases/"+caseID+"/claim", mod, "{}", "application/json")
	if rec := c.do(http.MethodPost, "/admin/api/cases/"+caseID+"/actions", mod, `{"action":"ban","note":"repeat offender"}`, "application/json"); rec.Code != http.StatusForbidden {
		t.Fatalf("moderator banned: %d", rec.Code)
	}
	rec := c.do(http.MethodPost, "/admin/api/cases/"+caseID+"/actions", mod, `{"action":"remove_content","note":"insults other riders"}`, "application/json")
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var acts, audits int64
	env.App.DB.Model(&model.ModerationAction{}).Count(&acts)
	env.App.DB.Model(&model.AuditLog{}).Where("verb = ?", "case.action.remove_content").Count(&audits)
	if acts != 1 || audits != 1 {
		t.Fatalf("actions=%d audits=%d", acts, audits)
	}
	env.Do(http.MethodGet, "/v1/posts/"+p.ID.String(), reporter.AccessToken, nil).Want(http.StatusNotFound)

	var notices struct {
		Items []struct {
			ActionID  uuid.UUID `json:"action_id"`
			CanAppeal bool      `json:"can_appeal"`
		}
	}
	env.Do(http.MethodGet, "/v1/me/moderation", author.AccessToken, nil).Want(http.StatusOK).JSON(&notices)
	if len(notices.Items) != 1 || !notices.Items[0].CanAppeal {
		t.Fatalf("%+v", notices)
	}
	env.Do(http.MethodPost, "/v1/appeals", author.AccessToken, map[string]any{"action_id": notices.Items[0].ActionID, "text": "It was a joke among friends, sorry"}).Want(http.StatusCreated)
	env.Do(http.MethodPost, "/v1/appeals", author.AccessToken, map[string]any{"action_id": notices.Items[0].ActionID, "text": "again please review"}).Want(http.StatusConflict)

	var appeals struct{ Items []struct{ ID uuid.UUID } }
	decode(t, c.do(http.MethodGet, "/admin/api/appeals", mod, "", "").Body.Bytes(), &appeals)
	path := "/admin/api/appeals/" + appeals.Items[0].ID.String() + "/decide"
	if rec := c.do(http.MethodPost, path, mod, `{"decision":"overturned","note":"looks fine to me"}`, "application/json"); rec.Code != http.StatusForbidden {
		t.Fatalf("actor judged own appeal: %d", rec.Code)
	}
	if rec := c.do(http.MethodPost, path, lead, `{"decision":"overturned","note":"context was friendly banter"}`, "application/json"); rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	env.Do(http.MethodGet, "/v1/posts/"+p.ID.String(), reporter.AccessToken, nil).Want(http.StatusOK)

	env.Do(http.MethodPost, "/v1/reports", reporter.AccessToken, map[string]any{"target_type": "persona", "target_id": author.PersonaID, "reason": "threat"}).Want(http.StatusCreated)
	decode(t, c.do(http.MethodGet, "/admin/api/cases", lead, "", "").Body.Bytes(), &q)
	if rec := c.do(http.MethodPost, "/admin/api/cases/"+q.Items[0].ID.String()+"/actions", lead, `{"action":"ban","note":"credible threat"}`, "application/json"); rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	env.Do(http.MethodGet, "/v1/me", author.AccessToken, nil).Want(http.StatusUnauthorized)
}

func TestGrievanceDeskAckResolveAndOverdueAlert(t *testing.T) {
	env := apitest.New(t)
	c := console{env}
	u := env.NewUser("9000000001", env.City("DEL"))
	var g struct{ Ref string }
	env.Do(http.MethodPost, "/v1/grievances", u.AccessToken, map[string]any{"category": "privacy", "body": "Please delete the comment that mentions my office."}).Want(http.StatusCreated).JSON(&g)
	lead := cookieOf(c.login(t, c.newAdmin(t, "lead@m.in", model.AdminTSLead), ""))
	mod := cookieOf(c.login(t, c.newAdmin(t, "mod@m.in", model.AdminModerator), ""))
	if rec := c.do(http.MethodGet, "/admin/api/grievances", mod, "", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("moderator saw grievances: %d", rec.Code)
	}
	var list struct {
		Items []struct {
			ID   uuid.UUID
			Ref  string
			Body string
		}
	}
	decode(t, c.do(http.MethodGet, "/admin/api/grievances", lead, "", "").Body.Bytes(), &list)
	if len(list.Items) != 1 || list.Items[0].Ref != g.Ref || list.Items[0].Body == "" {
		t.Fatalf("%+v vs %s", list, g.Ref)
	}

	env.App.DB.Model(&model.GrievanceTicket{}).Where("1=1").Update("ack_due_at", time.Now().UTC().Add(-time.Hour))
	if err := env.App.Admin.AlertOverdue(t.Context()); err != nil {
		t.Fatal(err)
	}
	env.App.Admin.AlertOverdue(t.Context())
	var alerts int64
	env.App.DB.Model(&model.AuditLog{}).Where("verb = ?", "grievance.overdue").Count(&alerts)
	if alerts != 1 {
		t.Fatalf("overdue alerts %d", alerts)
	}
	id := list.Items[0].ID.String()
	c.do(http.MethodPost, "/admin/api/grievances/"+id+"/ack", lead, "{}", "application/json")
	rec := c.do(http.MethodPost, "/admin/api/grievances/"+id+"/resolve", lead, `{"resolution":"Comment removed and author warned."}`, "application/json")
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
