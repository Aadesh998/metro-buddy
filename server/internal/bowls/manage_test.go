package bowls_test

import (
	"net/http"
	"testing"

	"metro-buddy/server/internal/apitest"
	"metro-buddy/server/internal/bowls"
	"metro-buddy/server/internal/model"
)

func setTier(env *apitest.Env, u apitest.User, tier int) {
	env.App.DB.Model(&model.User{}).Where("id = (SELECT user_id FROM personas WHERE id = ?)", u.PersonaID).Update("trust_tier", tier)
}

func TestCreatingBowlsNeedsTierTwoAndOwnerRunsMods(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	owner, member, outsider := env.NewUser("9000000001", city), env.NewUser("9000000002", city), env.NewUser("9000000003", city)
	body := map[string]any{"name": "Night Shift Nurses", "kind": "topic", "description": "For anyone on the late shift", "rules_md": "Be kind. No real names."}
	env.Do(http.MethodPost, "/v1/bowls", owner.AccessToken, body).Want(http.StatusForbidden)
	setTier(env, owner, 2)
	var b bowls.BowlDTO
	env.Do(http.MethodPost, "/v1/bowls", owner.AccessToken, body).Want(http.StatusCreated).JSON(&b)
	if b.Role == nil || *b.Role != "owner" || b.MemberCount != 1 || b.RulesMD == "" {
		t.Fatalf("%+v", b)
	}
	env.Do(http.MethodPost, "/v1/bowls", owner.AccessToken, body).Want(http.StatusConflict)
	id := b.ID.String()

	env.Do(http.MethodPost, "/v1/bowls/"+id+"/moderators", owner.AccessToken, map[string]any{"persona_id": member.PersonaID}).Want(http.StatusUnprocessableEntity)
	env.Do(http.MethodPost, "/v1/bowls/"+id+"/join", member.AccessToken, nil).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/bowls/"+id+"/join", outsider.AccessToken, nil).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/bowls/"+id+"/moderators", outsider.AccessToken, map[string]any{"persona_id": outsider.PersonaID}).Want(http.StatusForbidden)
	env.Do(http.MethodPost, "/v1/bowls/"+id+"/moderators", owner.AccessToken, map[string]any{"persona_id": member.PersonaID}).Want(http.StatusNoContent)
	var mods struct{ Items []bowls.ModDTO }
	env.Do(http.MethodGet, "/v1/bowls/"+id+"/moderators", outsider.AccessToken, nil).Want(http.StatusOK).JSON(&mods)
	if len(mods.Items) != 2 {
		t.Fatalf("%+v", mods.Items)
	}
	// A mod can't change visibility or appoint mods; only the owner can.
	env.Do(http.MethodPatch, "/v1/bowls/"+id, member.AccessToken, map[string]any{"visibility": "members_only"}).Want(http.StatusForbidden)
	env.Do(http.MethodPatch, "/v1/bowls/"+id, member.AccessToken, map[string]any{"rules_md": "Be kind. Call 9876543210 for help"}).Want(http.StatusOK).JSON(&b)
	if b.RulesMD == "" || contains(b.RulesMD, "9876543210") {
		t.Fatalf("rules %q", b.RulesMD)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestBowlModsRemoveLockPinAndMute(t *testing.T) {
	env := apitest.New(t)
	city := env.City("DEL")
	owner, troll := env.NewUser("9000000001", city), env.NewUser("9000000002", city)
	setTier(env, owner, 2)
	var b bowls.BowlDTO
	env.Do(http.MethodPost, "/v1/bowls", owner.AccessToken, map[string]any{"name": "Gurgaon Gym Rats", "kind": "topic"}).Want(http.StatusCreated).JSON(&b)
	id := b.ID.String()
	env.Do(http.MethodPost, "/v1/bowls/"+id+"/join", troll.AccessToken, nil).Want(http.StatusOK)
	var welcome, spam bowls.PostDTO
	env.Do(http.MethodPost, "/v1/posts", owner.AccessToken, map[string]any{"bowl_id": id, "type": "text", "title": "Welcome, read the rules", "body": "Be nice"}).Want(http.StatusCreated).JSON(&welcome)
	env.Do(http.MethodPost, "/v1/posts", troll.AccessToken, map[string]any{"bowl_id": id, "type": "text", "title": "Buy my supplements", "body": "cheap"}).Want(http.StatusCreated).JSON(&spam)

	env.Do(http.MethodPost, "/v1/posts/"+spam.ID.String()+"/mod", troll.AccessToken, map[string]any{"action": "remove"}).Want(http.StatusForbidden)
	env.Do(http.MethodPost, "/v1/posts/"+spam.ID.String()+"/mod", owner.AccessToken, map[string]any{"action": "remove", "reason": "spam"}).Want(http.StatusNoContent)
	env.Do(http.MethodPatch, "/v1/bowls/"+id, owner.AccessToken, map[string]any{"pinned_post_id": welcome.ID}).Want(http.StatusOK)
	env.Do(http.MethodPost, "/v1/posts/"+welcome.ID.String()+"/mod", owner.AccessToken, map[string]any{"action": "lock"}).Want(http.StatusNoContent)

	var feed struct {
		Items  []bowls.PostDTO
		Pinned *bowls.PostDTO
	}
	env.Do(http.MethodGet, "/v1/bowls/"+id+"/posts?sort=new", troll.AccessToken, nil).Want(http.StatusOK).JSON(&feed)
	if feed.Pinned == nil || feed.Pinned.ID != welcome.ID || !feed.Pinned.Locked || len(feed.Items) != 0 {
		t.Fatalf("pinned %+v items %d", feed.Pinned, len(feed.Items))
	}
	env.Do(http.MethodPost, "/v1/posts/"+welcome.ID.String()+"/comments", troll.AccessToken, map[string]any{"body": "lol"}).Want(http.StatusForbidden)
	env.Do(http.MethodPost, "/v1/posts/"+welcome.ID.String()+"/mod", owner.AccessToken, map[string]any{"action": "unlock"}).Want(http.StatusNoContent)

	env.Do(http.MethodPost, "/v1/bowls/"+id+"/mutes", owner.AccessToken, map[string]any{"persona_id": owner.PersonaID, "days": 1}).Want(http.StatusUnprocessableEntity)
	env.Do(http.MethodPost, "/v1/bowls/"+id+"/mutes", owner.AccessToken, map[string]any{"persona_id": troll.PersonaID, "days": 3, "reason": "spam"}).Want(http.StatusNoContent)
	env.Do(http.MethodPost, "/v1/posts/"+welcome.ID.String()+"/comments", troll.AccessToken, map[string]any{"body": "still here"}).Want(http.StatusForbidden)
	env.Do(http.MethodPost, "/v1/posts", troll.AccessToken, map[string]any{"bowl_id": id, "type": "text", "title": "Again", "body": "x"}).Want(http.StatusForbidden)

	var log struct{ Items []bowls.ModLogDTO }
	env.Do(http.MethodGet, "/v1/bowls/"+id+"/modlog", troll.AccessToken, nil).Want(http.StatusForbidden)
	env.Do(http.MethodGet, "/v1/bowls/"+id+"/modlog", owner.AccessToken, nil).Want(http.StatusOK).JSON(&log)
	if len(log.Items) != 5 || log.Items[0].Verb != "mute" || log.Items[0].Actor.ID != owner.PersonaID {
		t.Fatalf("%+v", log.Items)
	}
	var n int64
	env.App.DB.Model(&model.Notification{}).Where("persona_id = ? AND kind = ?", troll.PersonaID, model.NotifModNotice).Count(&n)
	if n != 2 {
		t.Fatalf("troll notices %d", n)
	}
}
