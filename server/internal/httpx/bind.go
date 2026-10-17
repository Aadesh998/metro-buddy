package httpx

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type Validator struct{ v *validator.Validate }

func NewValidator() *Validator {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		for _, tag := range []string{"json", "query", "param"} {
			if name, _, _ := strings.Cut(f.Tag.Get(tag), ","); name != "" && name != "-" {
				return name
			}
		}
		return f.Name
	})
	return &Validator{v: v}
}

func (cv *Validator) Validate(i any) error { return cv.v.Struct(i) }

func Bind[T any](c echo.Context) (*T, error) {
	req := new(T)
	if err := c.Bind(req); err != nil {
		return nil, BadRequest("malformed request")
	}
	if err := c.Validate(req); err != nil {
		return nil, err
	}
	return req, nil
}
