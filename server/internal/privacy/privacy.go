// Package privacy implements DPDP data rights: export and account deletion.
package privacy

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/audit"
	"metro-buddy/server/internal/auth"
	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/media"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/secure"
)

const (
	DeletionGrace = 30 * 24 * time.Hour
	ExportTTL     = 24 * time.Hour
)

type Service struct {
	DB     *gorm.DB
	Box    *secure.Box
	Store  media.Store
	Tokens *auth.Tokens
	Audit  *audit.Recorder
	Log    *slog.Logger
	Now    func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

func (s *Service) Register(authed *echo.Group) {
	authed.POST("/me/export", s.requestExport)
	authed.GET("/me/export", s.exportStatus)
	authed.DELETE("/me", s.requestDeletion)
	authed.POST("/me/restore", s.restore)
}

type exportDTO struct {
	Status    string     `json:"status"`
	URL       string     `json:"url,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func (s *Service) requestExport(c echo.Context) error {
	uid := httpx.PrincipalFrom(c).UserID
	db := s.DB.WithContext(c.Request().Context())
	var recent int64
	if err := db.Model(&model.ExportJob{}).Where("user_id = ? AND created_at > ?", uid, s.now().Add(-24*time.Hour)).Count(&recent).Error; err != nil {
		return err
	}
	if recent > 0 {
		return s.exportStatus(c)
	}
	if err := db.Create(&model.ExportJob{UserID: uid, Status: "pending"}).Error; err != nil {
		return err
	}
	return s.exportStatus(c)
}

func (s *Service) exportStatus(c echo.Context) error {
	var j model.ExportJob
	err := s.DB.WithContext(c.Request().Context()).Where("user_id = ?", httpx.PrincipalFrom(c).UserID).Order("created_at DESC").First(&j).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusOK, map[string]any{"export": nil})
	}
	if err != nil {
		return err
	}
	d := exportDTO{Status: j.Status, CreatedAt: j.CreatedAt, ExpiresAt: j.ExpiresAt}
	if j.Status == "ready" && j.Key != nil && j.ExpiresAt != nil && s.now().Before(*j.ExpiresAt) {
		if u, err := s.Store.PresignGet(c.Request().Context(), *j.Key, time.Until(*j.ExpiresAt)); err == nil {
			d.URL = u
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"export": d})
}

// RunExports builds pending exports. Everything a user can see about themselves goes in;
// secrets (hashes, ciphertexts, token hashes) never do.
func (s *Service) RunExports(ctx context.Context) error {
	var jobs []model.ExportJob
	if err := s.DB.WithContext(ctx).Where("status = ?", "pending").Limit(20).Find(&jobs).Error; err != nil {
		return err
	}
	for _, j := range jobs {
		data, err := s.collect(ctx, j.UserID)
		if err != nil {
			s.DB.Model(&j).Update("status", "failed")
			s.Log.Error("export failed", "err", err.Error())
			continue
		}
		key := "exports/" + model.NewID().String() + ".json"
		if err := s.Store.Put(ctx, key, "application/json", data); err != nil {
			return err
		}
		exp := s.now().Add(ExportTTL)
		if err := s.DB.WithContext(ctx).Model(&j).Updates(map[string]any{"status": "ready", "key": key, "expires_at": exp}).Error; err != nil {
			return err
		}
	}
	// Exports are personal data too: delete the file once the link has expired.
	var old []model.ExportJob
	if err := s.DB.WithContext(ctx).Where("status = ? AND expires_at < ?", "ready", s.now()).Find(&old).Error; err != nil {
		return err
	}
	for _, j := range old {
		if j.Key != nil {
			_ = s.Store.Delete(ctx, *j.Key)
		}
		s.DB.WithContext(ctx).Model(&j).Updates(map[string]any{"status": "expired", "key": nil})
	}
	return nil
}

func (s *Service) collect(ctx context.Context, uid uuid.UUID) ([]byte, error) {
	db := s.DB.WithContext(ctx)
	var u model.User
	if err := db.Unscoped().First(&u, "id = ?", uid).Error; err != nil {
		return nil, err
	}
	var phone, email string
	if len(u.PhoneEnc) > 0 {
		phone, _ = s.Box.OpenString(u.PhoneEnc)
	}
	if len(u.EmailEnc) > 0 {
		email, _ = s.Box.OpenString(u.EmailEnc)
	}
	out := map[string]any{
		"generated_at": s.now(),
		"account": map[string]any{
			"phone": phone, "email": email, "google_linked": u.GoogleSub != nil, "status": u.Status, "birth_year": u.BirthYear, "gender": u.Gender, "locale": u.Locale,
			"verified": u.VerifiedAt != nil, "created_at": u.CreatedAt,
		},
	}
	var persona model.Persona
	hasPersona := db.Unscoped().Where("user_id = ?", uid).Limit(1).Find(&persona).RowsAffected == 1
	type section struct {
		name string
		dest any
		q    *gorm.DB
	}
	var consents []model.Consent
	var settings []model.UserSettings
	var devices []model.Device
	var contacts []model.TrustedContact
	sections := []section{
		{"consents", &consents, db.Where("user_id = ?", uid).Order("created_at")},
		{"settings", &settings, db.Where("user_id = ?", uid)},
		{"devices", &devices, db.Where("user_id = ?", uid)},
		{"trusted_contacts", &contacts, db.Where("user_id = ?", uid)},
	}
	if hasPersona {
		pid := persona.ID
		out["persona"] = map[string]any{
			"alias": persona.Alias, "bio": persona.Bio, "intents": persona.Intents, "home_station_id": persona.HomeStationID,
			"work_station_id": persona.WorkStationID, "created_at": persona.CreatedAt,
		}
		add := func(name string, dest any, q *gorm.DB) { sections = append(sections, section{name, dest, q}) }
		add("alias_history", &[]model.PersonaAliasHistory{}, db.Where("persona_id = ?", pid))
		add("vibes", &[]model.PersonaVibe{}, db.Where("persona_id = ?", pid))
		add("bowls_joined", &[]model.BowlMember{}, db.Where("persona_id = ?", pid))
		add("posts", &[]model.Post{}, db.Unscoped().Where("author_persona_id = ?", pid))
		add("comments", &[]model.Comment{}, db.Unscoped().Where("author_persona_id = ?", pid))
		add("votes", &[]model.Vote{}, db.Where("persona_id = ?", pid))
		add("messages_sent", &[]model.Message{}, db.Where("sender_persona_id = ?", pid))
		add("dm_requests", &[]model.DMRequest{}, db.Where("from_persona_id = ? OR to_persona_id = ?", pid, pid))
		add("commute_plans", &[]model.CommutePlan{}, db.Unscoped().Where("persona_id = ?", pid))
		add("travel_intents", &[]model.TravelIntent{}, db.Where("persona_id = ?", pid))
		add("trip_feedback_given", &[]model.TripFeedback{}, db.Where("from_persona_id = ?", pid))
		add("reports_filed", &[]model.Report{}, db.Where("reporter_persona_id = ?", pid))
		add("blocks", &[]model.Block{}, db.Where("blocker_persona_id = ?", pid))
		add("mutes", &[]model.Mute{}, db.Where("muter_persona_id = ?", pid))
		add("notifications", &[]model.Notification{}, db.Where("persona_id = ?", pid))
	}
	for _, sec := range sections {
		if err := sec.q.Find(sec.dest).Error; err != nil {
			return nil, err
		}
		out[sec.name] = sec.dest
	}
	names := make([]map[string]any, 0, len(contacts))
	for _, tc := range contacts {
		p, _ := s.Box.OpenString(tc.PhoneEnc)
		names = append(names, map[string]any{"name": tc.Name, "phone": p, "added_at": tc.CreatedAt})
	}
	out["trusted_contacts"] = names
	return json.MarshalIndent(redact(out), "", "  ")
}

// redact strips internal-only fields before the export is written.
func redact(v map[string]any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	var walk func(x any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			for _, k := range []string{"PhoneHash", "PhoneEnc", "TokenHash", "BlockerUserID", "PushToken"} {
				delete(t, k)
			}
			for _, vv := range t {
				walk(vv)
			}
		case []any:
			for _, vv := range t {
				walk(vv)
			}
		}
	}
	walk(m)
	return m
}

func (s *Service) requestDeletion(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	ctx := c.Request().Context()
	now := s.now()
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", p.UserID).
			Updates(map[string]any{"status": model.UserPendingDeletion, "deletion_requested_at": now}).Error; err != nil {
			return err
		}
		// Hiding the persona immediately is what makes it show as [deleted] everywhere.
		if err := tx.Where("user_id = ?", p.UserID).Delete(&model.Persona{}).Error; err != nil {
			return err
		}
		if err := s.Tokens.RevokeAllForUser(ctx, tx, p.UserID); err != nil {
			return err
		}
		return s.Audit.Log(tx, model.ActorUser, &p.UserID, "account.deletion_requested", "user", &p.UserID, nil)
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusAccepted, map[string]any{"deletion_scheduled_at": now.Add(DeletionGrace)})
}

func (s *Service) restore(c echo.Context) error {
	uid := httpx.PrincipalFrom(c).UserID
	err := s.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var u model.User
		if err := tx.First(&u, "id = ?", uid).Error; err != nil {
			return err
		}
		if u.Status != model.UserPendingDeletion {
			return httpx.Conflict("account is not scheduled for deletion")
		}
		if err := tx.Model(&u).Updates(map[string]any{"status": model.UserActive, "deletion_requested_at": nil}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Model(&model.Persona{}).Where("user_id = ?", uid).Update("deleted_at", nil).Error; err != nil {
			return err
		}
		return s.Audit.Log(tx, model.ActorUser, &uid, "account.restored", "user", &uid, nil)
	})
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// PurgeDeleted erases personal data once the grace period ends. Content stays, attributed to
// the tombstoned [deleted] persona; anything that could identify the person is wiped.
func (s *Service) PurgeDeleted(ctx context.Context) error {
	var users []model.User
	if err := s.DB.WithContext(ctx).Where("status = ? AND deletion_requested_at < ?", model.UserPendingDeletion, s.now().Add(-DeletionGrace)).
		Limit(100).Find(&users).Error; err != nil {
		return err
	}
	for _, u := range users {
		err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			uid := u.ID
			for _, m := range []any{&model.Device{}, &model.RefreshToken{}, &model.TrustedContact{}, &model.UserSettings{}, &model.RevealProfile{}} {
				if err := tx.Where("user_id = ?", uid).Delete(m).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("blocker_user_id = ?", uid).Delete(&model.Block{}).Error; err != nil {
				return err
			}
			var pids []uuid.UUID
			if err := tx.Unscoped().Model(&model.Persona{}).Where("user_id = ?", uid).Pluck("id", &pids).Error; err != nil {
				return err
			}
			if len(pids) > 0 {
				if err := tx.Unscoped().Model(&model.Persona{}).Where("id IN ?", pids).Updates(map[string]any{
					"bio": "", "home_station_id": nil, "work_station_id": nil, "intents": model.StringList{},
				}).Error; err != nil {
					return err
				}
				for _, m := range []any{&model.PersonaVibe{}, &model.BowlMember{}, &model.CommutePlan{}, &model.TravelIntent{}, &model.Notification{}} {
					if err := tx.Unscoped().Where("persona_id IN ?", pids).Delete(m).Error; err != nil {
						return err
					}
				}
				if err := tx.Where("muter_persona_id IN ?", pids).Delete(&model.Mute{}).Error; err != nil {
					return err
				}
			}
			// Sign-in identifiers are cleared so the person can sign up fresh; nothing links back.
			return tx.Model(&model.User{}).Where("id = ?", uid).Updates(map[string]any{
				"status": model.UserDeleted, "phone_hash": nil, "phone_enc": nil,
				"email_hash": nil, "email_enc": nil, "email_verified_at": nil, "password_hash": nil, "google_sub": nil,
				"birth_year": nil, "gender": nil, "verified_at": nil, "city_id": nil,
			}).Error
		})
		if err != nil {
			return err
		}
		s.Audit.Log(nil, model.ActorSystem, nil, "account.purged", "user", &u.ID, nil)
	}
	return nil
}
