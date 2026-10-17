package httpx

import (
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const requestIDKey = "request_id"

func RequestIDMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Always generated server-side: a client-chosen ID could be used to forge log correlation.
			id := uuid.NewString()
			c.Set(requestIDKey, id)
			c.Response().Header().Set(echo.HeaderXRequestID, id)
			return next(c)
		}
	}
}

func RequestID(c echo.Context) string {
	s, _ := c.Get(requestIDKey).(string)
	return s
}

// Logs the route template, never the raw URL: paths and query strings can carry IDs or search text.
func AccessLog(log *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			if err != nil {
				c.Error(err)
			}
			log.Info("http",
				"method", c.Request().Method,
				"route", c.Path(),
				"status", c.Response().Status,
				"dur_ms", time.Since(start).Milliseconds(),
				"request_id", RequestID(c),
			)
			return nil
		}
	}
}

func Recover(log *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					buf := make([]byte, 8<<10)
					buf = buf[:runtime.Stack(buf, false)]
					log.Error("panic", "panic", fmt.Sprint(r), "stack", string(buf), "route", c.Path(), "request_id", RequestID(c))
					err = fmt.Errorf("panic recovered")
				}
			}()
			return next(c)
		}
	}
}
