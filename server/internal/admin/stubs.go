package admin

import "github.com/labstack/echo/v4"

func (h *Handlers) registerModeration(*echo.Group, echo.MiddlewareFunc) {}
func (h *Handlers) registerGrievance(*echo.Group, echo.MiddlewareFunc)  {}
func (h *Handlers) registerStudio(*echo.Group)                          {}
func (h *Handlers) registerMetrics(*echo.Group)                         {}
