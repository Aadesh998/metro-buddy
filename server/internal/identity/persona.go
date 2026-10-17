package identity

import (
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/pii"
)

const AliasChangeInterval = 30 * 24 * time.Hour

var (
	ErrBadAlias       = httpx.Unprocessable("invalid_alias", "pick one of the suggested aliases")
	ErrAliasTaken     = httpx.Conflict("that alias was just taken, re-roll for another")
	ErrAliasTooSoon   = httpx.E(http.StatusTooManyRequests, "alias_change_too_soon", "you can change your alias once every 30 days")
	ErrPersonaExists  = httpx.Conflict("you already have a persona")
	ErrUnknownCity    = httpx.Unprocessable("invalid_city", "unknown city")
	ErrUnknownStation = httpx.Unprocessable("invalid_station", "unknown station")
)

type suggestRequest struct {
	CityID uuid.UUID `json:"city_id" validate:"required"`
}

type Suggestion struct {
	Alias      string `json:"alias"`
	AvatarSeed string `json:"avatar_seed"`
}

func (h *Handlers) suggest(c echo.Context) error {
	req, err := httpx.Bind[suggestRequest](c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	out := make([]Suggestion, 0, 3)
	seen := map[string]bool{}
	// Clean two-word names first; a number is added only once those keep colliding.
	for tries := 0; len(out) < 3 && tries < 40; tries++ {
		a := NewAlias(tries >= 15)
		if seen[a] || h.Aliases.mightBeTaken(ctx, req.CityID, a) {
			continue
		}
		seen[a] = true
		if taken, err := h.aliasTaken(h.DB.WithContext(ctx), req.CityID, a); err != nil {
			return err
		} else if !taken {
			out = append(out, Suggestion{Alias: a, AvatarSeed: NewAvatarSeed()})
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"suggestions": out})
}

func (h *Handlers) aliasTaken(tx *gorm.DB, cityID uuid.UUID, alias string) (bool, error) {
	var n int64
	err := tx.Unscoped().Model(&model.Persona{}).Where("city_id = ? AND alias_key = ?", cityID, AliasKey(alias)).Count(&n).Error
	if err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	// Retired aliases stay reserved so nobody can impersonate a persona's old name.
	err = tx.Model(&model.PersonaAliasHistory{}).Where("alias = ?", alias).Count(&n).Error
	return n > 0, err
}

type createPersonaRequest struct {
	CityID     uuid.UUID `json:"city_id" validate:"required"`
	Alias      string    `json:"alias" validate:"required,max=40"`
	AvatarSeed string    `json:"avatar_seed" validate:"required"`
	Bio        string    `json:"bio" validate:"max=160"`
	Intents    []string  `json:"intents" validate:"max=5,dive,oneof=vent listen travel_buddy friends discuss"`
}

func (h *Handlers) createPersona(c echo.Context) error {
	p := httpx.PrincipalFrom(c)
	if p.PersonaID != uuid.Nil {
		return ErrPersonaExists
	}
	req, err := httpx.Bind[createPersonaRequest](c)
	if err != nil {
		return err
	}
	if !ValidAlias(req.Alias) {
		return ErrBadAlias
	}
	if !ValidSeed(req.AvatarSeed) {
		return httpx.Unprocessable("invalid_avatar", "invalid avatar seed")
	}
	ctx := c.Request().Context()
	var persona model.Persona
	err = h.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var city model.City
		if err := tx.First(&city, "id = ?", req.CityID).Error; err != nil {
			return ErrUnknownCity
		}
		if taken, err := h.aliasTaken(tx, req.CityID, req.Alias); err != nil {
			return err
		} else if taken {
			return ErrAliasTaken
		}
		persona = model.Persona{
			UserID: p.UserID, CityID: req.CityID, Alias: req.Alias, AliasKey: AliasKey(req.Alias),
			AvatarSeed: req.AvatarSeed, Intents: dedupe(req.Intents),
		}
		persona.ID = model.NewID()
		if req.Bio != "" {
			r, err := h.PII.Process(tx, model.CTPersona, persona.ID, req.Bio, pii.Options{})
			if err != nil {
				return err
			}
			persona.Bio = r.Text
		}
		if err := tx.Create(&persona).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return ErrAliasTaken
			}
			return err
		}
		return tx.Model(&model.User{}).Where("id = ?", p.UserID).Update("city_id", req.CityID).Error
	})
	if err != nil {
		return err
	}
	h.Aliases.add(ctx, cityKey(req.CityID, req.Alias))
	return h.ownPersona(c, persona.ID)
}

type OwnPersona struct {
	ID                   uuid.UUID  `json:"id"`
	Alias                string     `json:"alias"`
	AvatarSeed           string     `json:"avatar_seed"`
	Bio                  string     `json:"bio"`
	CityID               uuid.UUID  `json:"city_id"`
	Intents              []string   `json:"intents"`
	IsHost               bool       `json:"is_host"`
	Verified             bool       `json:"verified"`
	HomeStationID        *uuid.UUID `json:"home_station_id"`
	WorkStationID        *uuid.UUID `json:"work_station_id"`
	AliasChangeAvailable time.Time  `json:"alias_change_available_at"`
	Vibes                []VibeDTO  `json:"vibes"`
}

func (h *Handlers) getPersona(c echo.Context) error {
	return h.ownPersona(c, httpx.PrincipalFrom(c).PersonaID)
}

func (h *Handlers) ownPersona(c echo.Context, id uuid.UUID) error {
	ctx := c.Request().Context()
	var p model.Persona
	if err := h.DB.WithContext(ctx).First(&p, "id = ?", id).Error; err != nil {
		return err
	}
	var u model.User
	if err := h.DB.WithContext(ctx).Select("verified_at").First(&u, "id = ?", p.UserID).Error; err != nil {
		return err
	}
	vibes, err := LoadVibes(h.DB.WithContext(ctx), []uuid.UUID{p.ID})
	if err != nil {
		return err
	}
	avail := p.CreatedAt
	if p.AliasChangedAt != nil {
		avail = p.AliasChangedAt.Add(AliasChangeInterval)
	}
	return c.JSON(http.StatusOK, OwnPersona{
		ID: p.ID, Alias: p.Alias, AvatarSeed: p.AvatarSeed, Bio: p.Bio, CityID: p.CityID,
		Intents: nonNil(p.Intents), IsHost: p.IsHost, Verified: u.VerifiedAt != nil,
		HomeStationID: p.HomeStationID, WorkStationID: p.WorkStationID,
		AliasChangeAvailable: avail, Vibes: nonNilVibes(vibes[p.ID]),
	})
}

type patchPersonaRequest struct {
	Bio           *string    `json:"bio" validate:"omitempty,max=160"`
	AvatarSeed    *string    `json:"avatar_seed"`
	Intents       *[]string  `json:"intents" validate:"omitempty,max=5,dive,oneof=vent listen travel_buddy friends discuss"`
	HomeStationID *uuid.UUID `json:"home_station_id"`
	WorkStationID *uuid.UUID `json:"work_station_id"`
}

func (h *Handlers) patchPersona(c echo.Context) error {
	req, err := httpx.Bind[patchPersonaRequest](c)
	if err != nil {
		return err
	}
	pid := httpx.PrincipalFrom(c).PersonaID
	err = h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var p model.Persona
		if err := tx.First(&p, "id = ?", pid).Error; err != nil {
			return err
		}
		updates := map[string]any{}
		if req.Bio != nil {
			r, err := h.PII.Process(tx, model.CTPersona, pid, *req.Bio, pii.Options{})
			if err != nil {
				return err
			}
			updates["bio"] = r.Text
		}
		if req.AvatarSeed != nil {
			if !ValidSeed(*req.AvatarSeed) {
				return httpx.Unprocessable("invalid_avatar", "invalid avatar seed")
			}
			updates["avatar_seed"] = *req.AvatarSeed
		}
		if req.Intents != nil {
			updates["intents"] = model.StringList(dedupe(*req.Intents))
		}
		for col, sid := range map[string]*uuid.UUID{"home_station_id": req.HomeStationID, "work_station_id": req.WorkStationID} {
			if sid == nil {
				continue
			}
			var n int64
			if err := tx.Model(&model.Station{}).Where("id = ? AND city_id = ?", *sid, p.CityID).Count(&n).Error; err != nil {
				return err
			}
			if n == 0 {
				return ErrUnknownStation
			}
			updates[col] = *sid
		}
		if len(updates) == 0 {
			return nil
		}
		return tx.Model(&p).Updates(updates).Error
	})
	if err != nil {
		return err
	}
	return h.ownPersona(c, pid)
}

type aliasRequest struct {
	Alias string `json:"alias" validate:"required,max=40"`
}

func (h *Handlers) changeAlias(c echo.Context) error {
	req, err := httpx.Bind[aliasRequest](c)
	if err != nil {
		return err
	}
	if !ValidAlias(req.Alias) {
		return ErrBadAlias
	}
	pid := httpx.PrincipalFrom(c).PersonaID
	now := h.now()
	var p model.Persona
	var oldAlias string
	err = h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&p, "id = ?", pid).Error; err != nil {
			return err
		}
		if p.AliasChangedAt != nil && now.Before(p.AliasChangedAt.Add(AliasChangeInterval)) {
			return ErrAliasTooSoon
		}
		if taken, err := h.aliasTaken(tx, p.CityID, req.Alias); err != nil {
			return err
		} else if taken {
			return ErrAliasTaken
		}
		from := p.CreatedAt
		if p.AliasChangedAt != nil {
			from = *p.AliasChangedAt
		}
		oldAlias = p.Alias
		if err := tx.Create(&model.PersonaAliasHistory{PersonaID: p.ID, Alias: p.Alias, UsedFrom: from, UsedTo: now}).Error; err != nil {
			return err
		}
		return tx.Model(&p).Updates(map[string]any{"alias": req.Alias, "alias_key": AliasKey(req.Alias), "alias_changed_at": now}).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrAliasTaken
		}
		return err
	}
	h.Aliases.add(c.Request().Context(), cityKey(p.CityID, req.Alias), retiredKey(oldAlias))
	return h.ownPersona(c, pid)
}

func (h *Handlers) getPublicPersona(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	me := httpx.PrincipalFrom(c).PersonaID
	if h.Visible != nil {
		if ok, err := h.Visible(c.Request().Context(), me, id); err != nil {
			return err
		} else if !ok {
			return httpx.NotFound()
		}
	}
	m, err := LoadPublic(h.DB.WithContext(c.Request().Context()), []uuid.UUID{id})
	if err != nil {
		return err
	}
	pp, ok := m[id]
	if !ok || pp.Deleted {
		return httpx.NotFound()
	}
	return c.JSON(http.StatusOK, pp)
}

func dedupe(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
