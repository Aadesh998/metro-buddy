package admin

import (
	"net/http"
	"regexp"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
)

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,46}$`)

func (h *Handlers) listBowls(c echo.Context) error {
	var rows []model.Bowl
	q := h.DB.WithContext(c.Request().Context()).Order("kind, name")
	if k := c.QueryParam("kind"); k != "" {
		q = q.Where("kind = ?", k)
	}
	if err := q.Limit(500).Find(&rows).Error; err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": rows})
}

type bowlRequest struct {
	Kind        model.BowlKind       `json:"kind" validate:"required,oneof=city line station office_hub college topic"`
	Slug        string               `json:"slug" validate:"required"`
	Name        string               `json:"name" validate:"required,max=60"`
	Description string               `json:"description" validate:"max=300"`
	CityID      *uuid.UUID           `json:"city_id"`
	LineID      *uuid.UUID           `json:"line_id"`
	StationID   *uuid.UUID           `json:"station_id"`
	Visibility  model.BowlVisibility `json:"visibility" validate:"required,oneof=public members_only"`
	RulesMD     string               `json:"rules_md" validate:"max=4000"`
}

func (h *Handlers) createBowl(c echo.Context) error {
	req, err := httpx.Bind[bowlRequest](c)
	if err != nil {
		return err
	}
	if !slugRe.MatchString(req.Slug) {
		return httpx.Unprocessable("invalid_slug", "lowercase letters, digits and dashes")
	}
	b := model.Bowl{Kind: req.Kind, Slug: req.Slug, Name: req.Name, Description: req.Description, CityID: req.CityID,
		LineID: req.LineID, StationID: req.StationID, Visibility: req.Visibility, RulesMD: req.RulesMD}
	tx := h.DB.WithContext(c.Request().Context())
	if err := tx.Create(&b).Error; err != nil {
		return err
	}
	actor := PrincipalFrom(c).ID
	h.Audit.Log(tx, model.ActorAdmin, &actor, "bowl.create", "bowl", &b.ID, model.JSONMap{"slug": b.Slug})
	return c.JSON(http.StatusCreated, b)
}

type bowlPatch struct {
	Name        *string               `json:"name" validate:"omitempty,max=60"`
	Description *string               `json:"description" validate:"omitempty,max=300"`
	Visibility  *model.BowlVisibility `json:"visibility" validate:"omitempty,oneof=public members_only"`
	RulesMD     *string               `json:"rules_md" validate:"omitempty,max=4000"`
}

func (h *Handlers) updateBowl(c echo.Context) error {
	req, err := httpx.Bind[bowlPatch](c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Visibility != nil {
		updates["visibility"] = *req.Visibility
	}
	if req.RulesMD != nil {
		updates["rules_md"] = *req.RulesMD
	}
	tx := h.DB.WithContext(c.Request().Context())
	res := tx.Model(&model.Bowl{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return httpx.NotFound()
	}
	actor := PrincipalFrom(c).ID
	h.Audit.Log(tx, model.ActorAdmin, &actor, "bowl.update", "bowl", &id, model.JSONMap(updates))
	var b model.Bowl
	tx.First(&b, "id = ?", id)
	return c.JSON(http.StatusOK, b)
}

func (h *Handlers) deleteBowl(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	tx := h.DB.WithContext(c.Request().Context())
	if err := tx.Delete(&model.Bowl{}, "id = ?", id).Error; err != nil {
		return err
	}
	actor := PrincipalFrom(c).ID
	h.Audit.Log(tx, model.ActorAdmin, &actor, "bowl.delete", "bowl", &id, nil)
	return c.NoContent(http.StatusNoContent)
}
