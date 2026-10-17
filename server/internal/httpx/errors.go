package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"metro-buddy/server/internal/db"
)

type Error struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func E(status int, code, msg string) *Error { return &Error{Status: status, Code: code, Message: msg} }

func BadRequest(msg string) *Error { return E(http.StatusBadRequest, "bad_request", msg) }
func Unauthorized() *Error {
	return E(http.StatusUnauthorized, "unauthorized", "authentication required")
}
func Forbidden(msg string) *Error { return E(http.StatusForbidden, "forbidden", msg) }
func NotFound() *Error            { return E(http.StatusNotFound, "not_found", "not found") }
func Conflict(msg string) *Error  { return E(http.StatusConflict, "conflict", msg) }
func Unprocessable(code, msg string) *Error {
	return E(http.StatusUnprocessableEntity, code, msg)
}
func TooMany() *Error {
	return E(http.StatusTooManyRequests, "rate_limited", "too many requests, slow down")
}

type envelope struct {
	Error     *Error `json:"error"`
	RequestID string `json:"request_id,omitempty"`
}

func ErrorHandler(log *slog.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		// The client hung up; there's nobody to answer and nothing went wrong on our side.
		if errors.Is(err, context.Canceled) {
			c.Response().WriteHeader(499)
			return
		}
		he := toError(err)
		if he.Status >= 500 {
			log.Error("request failed", "err", err.Error(), "route", c.Path(), "request_id", RequestID(c))
		}
		var werr error
		if c.Request().Method == http.MethodHead {
			werr = c.NoContent(he.Status)
		} else {
			werr = c.JSON(he.Status, envelope{Error: he, RequestID: RequestID(c)})
		}
		if werr != nil {
			log.Error("write error response", "err", werr.Error())
		}
	}
}

func toError(err error) *Error {
	var he *Error
	if errors.As(err, &he) {
		return he
	}
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fields := map[string]string{}
		for _, fe := range ve {
			fields[jsonName(fe)] = fe.Tag()
		}
		return &Error{Status: http.StatusUnprocessableEntity, Code: "validation_failed", Message: "some fields are invalid", Fields: fields}
	}
	var ee *echo.HTTPError
	if errors.As(err, &ee) {
		msg := http.StatusText(ee.Code)
		if ee.Code < 500 {
			if s, ok := ee.Message.(string); ok {
				msg = s
			}
		}
		return E(ee.Code, codeFor(ee.Code), msg)
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return NotFound()
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return Conflict("already exists")
	case errors.Is(err, db.ErrBadCursor):
		return BadRequest("invalid cursor")
	}
	return E(http.StatusInternalServerError, "internal", "something went wrong")
}

func codeFor(status int) string {
	switch status {
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusRequestEntityTooLarge:
		return "too_large"
	case http.StatusTooManyRequests:
		return "rate_limited"
	}
	if status >= 500 {
		return "internal"
	}
	return "bad_request"
}

func jsonName(fe validator.FieldError) string {
	return fe.Field()
}
