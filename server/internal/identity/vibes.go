package identity

import (
	_ "embed"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
)

const (
	MinVibes = 3
	MaxVibes = 10
)

//go:embed vibe_seed.json
var vibeSeed []byte

func SeedVibes(db *gorm.DB) error {
	var tags []struct{ Slug, En, Hi, Cat string }
	if err := json.Unmarshal(vibeSeed, &tags); err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, t := range tags {
			var v model.VibeTag
			if err := tx.Where(model.VibeTag{Slug: t.Slug}).
				Assign(model.VibeTag{LabelEN: t.En, LabelHI: t.Hi, Category: t.Cat}).FirstOrCreate(&v).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (h *Handlers) vibeCatalog(c echo.Context) error {
	var tags []model.VibeTag
	if err := h.DB.WithContext(c.Request().Context()).Order("category, label_en").Find(&tags).Error; err != nil {
		return err
	}
	out := make([]VibeDTO, len(tags))
	for i, t := range tags {
		out[i] = VibeDTO{Slug: t.Slug, LabelEN: t.LabelEN, LabelHI: t.LabelHI, Category: t.Category}
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=3600")
	return c.JSON(http.StatusOK, map[string]any{"items": out, "min": MinVibes, "max": MaxVibes})
}

func (h *Handlers) myVibes(c echo.Context) error {
	pid := httpx.PrincipalFrom(c).PersonaID
	m, err := LoadVibes(h.DB.WithContext(c.Request().Context()), []uuid.UUID{pid})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": nonNilVibes(m[pid])})
}

type setVibesRequest struct {
	Slugs []string `json:"slugs" validate:"required,min=3,max=10,unique,dive,max=32"`
}

func (h *Handlers) setVibes(c echo.Context) error {
	req, err := httpx.Bind[setVibesRequest](c)
	if err != nil {
		return err
	}
	pid := httpx.PrincipalFrom(c).PersonaID
	err = h.DB.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var tags []model.VibeTag
		if err := tx.Where("slug IN ?", req.Slugs).Find(&tags).Error; err != nil {
			return err
		}
		if len(tags) != len(req.Slugs) {
			return httpx.Unprocessable("unknown_vibe", "one or more vibes don't exist")
		}
		if err := tx.Where("persona_id = ?", pid).Delete(&model.PersonaVibe{}).Error; err != nil {
			return err
		}
		rows := make([]model.PersonaVibe, len(tags))
		for i, t := range tags {
			rows[i] = model.PersonaVibe{PersonaID: pid, VibeTagID: t.ID}
		}
		return tx.Create(&rows).Error
	})
	if err != nil {
		return err
	}
	return h.myVibes(c)
}

type setIntentsRequest struct {
	Intents []string `json:"intents" validate:"required,min=1,max=5,unique,dive,oneof=vent listen travel_buddy friends discuss"`
}

func (h *Handlers) setIntents(c echo.Context) error {
	req, err := httpx.Bind[setIntentsRequest](c)
	if err != nil {
		return err
	}
	pid := httpx.PrincipalFrom(c).PersonaID
	if err := h.DB.WithContext(c.Request().Context()).Model(&model.Persona{}).Where("id = ?", pid).
		Update("intents", model.StringList(req.Intents)).Error; err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"intents": req.Intents})
}
