package metro

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
)

type Handlers struct {
	Svc *Service
}

func (h *Handlers) Register(g *echo.Group) {
	g.GET("/cities", h.cities, cached)
	g.GET("/cities/:id/lines", h.lines, cached)
	g.GET("/cities/:id/stations", h.cityStations, cached)
	g.GET("/stations", h.search)
	g.GET("/route", h.route)
}

func cached(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "public, max-age=3600")
		return next(c)
	}
}

type CityDTO struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Code   string    `json:"code"`
	IsLive bool      `json:"is_live"`
}

func (h *Handlers) cities(c echo.Context) error {
	var rows []model.City
	if err := h.Svc.DB.WithContext(c.Request().Context()).Order("is_live DESC, name").Find(&rows).Error; err != nil {
		return err
	}
	out := make([]CityDTO, len(rows))
	for i, r := range rows {
		out[i] = CityDTO{ID: r.ID, Name: r.Name, Code: r.Code, IsLive: r.IsLive}
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

type LineDTO struct {
	ID       uuid.UUID        `json:"id"`
	Name     string           `json:"name"`
	Color    string           `json:"color"`
	Stations []LineStationDTO `json:"stations"`
}

type LineStationDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Seq  int16     `json:"seq"`
}

func (h *Handlers) lines(c echo.Context) error {
	cityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	db := h.Svc.DB.WithContext(c.Request().Context())
	var lines []model.MetroLine
	if err := db.Where("city_id = ?", cityID).Order("name").Find(&lines).Error; err != nil {
		return err
	}
	var rows []struct {
		LineID uuid.UUID
		ID     uuid.UUID
		Name   string
		Seq    int16
	}
	if err := db.Table("line_stations").Select("line_stations.line_id, stations.id, stations.name, line_stations.seq").
		Joins("JOIN stations ON stations.id = line_stations.station_id").
		Where("stations.city_id = ?", cityID).Order("line_stations.seq").Scan(&rows).Error; err != nil {
		return err
	}
	byLine := map[uuid.UUID][]LineStationDTO{}
	for _, r := range rows {
		byLine[r.LineID] = append(byLine[r.LineID], LineStationDTO{ID: r.ID, Name: r.Name, Seq: r.Seq})
	}
	out := make([]LineDTO, len(lines))
	for i, l := range lines {
		out[i] = LineDTO{ID: l.ID, Name: l.Name, Color: l.ColorHex, Stations: byLine[l.ID]}
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

type StationDTO struct {
	ID            uuid.UUID     `json:"id"`
	Name          string        `json:"name"`
	IsInterchange bool          `json:"is_interchange"`
	Lines         []LineChipDTO `json:"lines"`
	SearchText    string        `json:"search_text"`
}

type LineChipDTO struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Color string    `json:"color"`
}

func (h *Handlers) stationDTOs(c echo.Context, cityID *uuid.UUID, query string) ([]StationDTO, error) {
	db := h.Svc.DB.WithContext(c.Request().Context())
	tx := db.Model(&model.Station{}).Order("name")
	if cityID != nil {
		tx = tx.Where("city_id = ?", *cityID)
	}
	if query != "" {
		tx = tx.Where(`search_text LIKE ? ESCAPE '\'`, "%"+escapeLike(strings.ToLower(query))+"%").Limit(20)
	}
	var stations []model.Station
	if err := tx.Find(&stations).Error; err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(stations))
	for i, s := range stations {
		ids[i] = s.ID
	}
	var chips []struct {
		StationID uuid.UUID
		LineChipDTO
	}
	if len(ids) > 0 {
		if err := db.Table("line_stations").Select("line_stations.station_id, metro_lines.id, metro_lines.name, metro_lines.color_hex AS color").
			Joins("JOIN metro_lines ON metro_lines.id = line_stations.line_id").
			Where("line_stations.station_id IN ?", ids).Order("metro_lines.name").Scan(&chips).Error; err != nil {
			return nil, err
		}
	}
	byStation := map[uuid.UUID][]LineChipDTO{}
	for _, ch := range chips {
		byStation[ch.StationID] = append(byStation[ch.StationID], ch.LineChipDTO)
	}
	out := make([]StationDTO, len(stations))
	for i, s := range stations {
		out[i] = StationDTO{ID: s.ID, Name: s.Name, IsInterchange: s.IsInterchange, Lines: byStation[s.ID], SearchText: s.SearchText}
	}
	return out, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (h *Handlers) cityStations(c echo.Context) error {
	cityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound()
	}
	out, err := h.stationDTOs(c, &cityID, "")
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

type searchQuery struct {
	Q      string     `query:"q" validate:"required,min=2,max=60"`
	CityID *uuid.UUID `query:"city_id"`
}

func (h *Handlers) search(c echo.Context) error {
	req, err := httpx.Bind[searchQuery](c)
	if err != nil {
		return err
	}
	out, err := h.stationDTOs(c, req.CityID, req.Q)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": out})
}

type routeQuery struct {
	From uuid.UUID `query:"from" validate:"required"`
	To   uuid.UUID `query:"to" validate:"required"`
}

func (h *Handlers) route(c echo.Context) error {
	req, err := httpx.Bind[routeQuery](c)
	if err != nil {
		return err
	}
	path, lines, err := h.Svc.Route(c.Request().Context(), req.From, req.To)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"station_ids": path, "line_ids": lines})
}
