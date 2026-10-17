package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/model"
)

func (h *Handlers) registerMetrics(g *echo.Group) {
	g.GET("/metrics", h.metrics)
}

type Point struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

type MetricsDTO struct {
	Days   int                `json:"days"`
	Series map[string][]Point `json:"series"`
	Now    map[string]int64   `json:"now"`
	AICost struct {
		TodayUSD float64            `json:"today_usd"`
		ByUse    map[string]float64 `json:"by_purpose_usd"`
	} `json:"ai_cost"`
}

// istDay buckets timestamps by Indian calendar day, in whichever SQL dialect is running.
func istDay(tx *gorm.DB, col string) string {
	if tx.Dialector.Name() == "postgres" {
		return "to_char(" + col + " AT TIME ZONE 'Asia/Kolkata', 'YYYY-MM-DD')"
	}
	return "date(" + col + ", '+330 minutes')"
}

func (h *Handlers) series(tx *gorm.DB, table, col, count string, since time.Time, where string, args ...any) []Point {
	var rows []Point
	q := tx.Table(table).Select(istDay(tx, col)+" AS day, "+count+" AS count").Where(col+" >= ?", since)
	if where != "" {
		q = q.Where(where, args...)
	}
	q.Group("day").Order("day").Scan(&rows)
	if rows == nil {
		rows = []Point{}
	}
	return rows
}

// metrics is aggregate-only: counts per day, never per-person data.
func (h *Handlers) metrics(c echo.Context) error {
	days, _ := strconv.Atoi(c.QueryParam("days"))
	if days <= 0 || days > 90 {
		days = 14
	}
	tx := h.DB.WithContext(c.Request().Context())
	now := h.now()
	since := now.AddDate(0, 0, -days)
	d := MetricsDTO{Days: days, Series: map[string][]Point{}, Now: map[string]int64{}}
	d.Series["signups"] = h.series(tx, "users", "created_at", "COUNT(*)", since, "")
	d.Series["active_sessions"] = h.series(tx, "analytics_events", "created_at", "COUNT(DISTINCT session_hash)", since, "")
	d.Series["posts"] = h.series(tx, "posts", "created_at", "COUNT(*)", since, "is_prompt = ?", false)
	d.Series["comments"] = h.series(tx, "comments", "created_at", "COUNT(*)", since, "")
	d.Series["messages"] = h.series(tx, "messages", "created_at", "COUNT(*)", since, "sender_persona_id IS NOT NULL")
	d.Series["matches"] = h.series(tx, "matches", "created_at", "COUNT(*)", since, "")
	d.Series["trips_met"] = h.series(tx, "trips", "meet_at", "COUNT(*)", since, "status IN ?", []model.TripStatus{model.TripMet, model.TripCompleted})
	d.Series["reports"] = h.series(tx, "reports", "created_at", "COUNT(*)", since, "")
	d.Series["sos"] = h.series(tx, "sos_events", "created_at", "COUNT(*)", since, "")
	d.Series["episodes"] = h.series(tx, "show_episodes", "started_at", "COUNT(*)", since, "started_at IS NOT NULL")

	count := func(m any, where string, args ...any) int64 {
		var n int64
		tx.Model(m).Where(where, args...).Count(&n)
		return n
	}
	d.Now["open_cases"] = count(&model.ModerationCase{}, "status IN ?", []model.CaseStatus{model.CaseOpen, model.CaseInReview, model.CaseEscalated})
	d.Now["overdue_cases"] = count(&model.ModerationCase{}, "status IN ? AND sla_due_at < ?", []model.CaseStatus{model.CaseOpen, model.CaseInReview}, now)
	d.Now["open_appeals"] = count(&model.Appeal{}, "status = ?", model.AppealOpen)
	d.Now["open_grievances"] = count(&model.GrievanceTicket{}, "resolved_at IS NULL")
	d.Now["host_applications"] = count(&model.HostProfile{}, "status = ?", model.HostApplied)
	d.Now["live_episodes"] = count(&model.ShowEpisode{}, "status = ?", model.EpisodeLive)
	d.Now["active_intents"] = count(&model.TravelIntent{}, "status = ? AND expires_at > ?", model.IntentActive, now)
	d.Now["users"] = count(&model.User{}, "status = ?", model.UserActive)

	start := now.Truncate(24 * time.Hour)
	var costs []struct {
		Purpose string
		Micros  int64
	}
	tx.Model(&model.AIUsage{}).Where("created_at >= ?", start).Select("purpose, SUM(cost_micros_usd) AS micros").Group("purpose").Scan(&costs)
	d.AICost.ByUse = map[string]float64{}
	for _, c := range costs {
		usd := float64(c.Micros) / 1e6
		d.AICost.ByUse[c.Purpose] = usd
		d.AICost.TodayUSD += usd
	}
	return c.JSON(http.StatusOK, d)
}
