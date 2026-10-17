package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"metro-buddy/server/internal/config"
)

type Deps struct {
	Cfg   *config.Config
	DB    *gorm.DB
	Redis *redis.Client
	Log   *slog.Logger
}

func NewServer(d *Deps) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())

	e.GET("/healthz", health(d))
	return e
}

func health(d *Deps) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()

		status := map[string]string{"db": "ok", "redis": "ok"}
		code := http.StatusOK
		if sqlDB, err := d.DB.DB(); err != nil || sqlDB.PingContext(ctx) != nil {
			status["db"] = "down"
			code = http.StatusServiceUnavailable
		}
		if d.Redis.Ping(ctx).Err() != nil {
			status["redis"] = "down"
			code = http.StatusServiceUnavailable
		}
		return c.JSON(code, status)
	}
}
