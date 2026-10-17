package pii

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"metro-buddy/server/internal/httpx"
)

type previewRequest struct {
	Text string `json:"text" validate:"max=5000"`
}

// PreviewHandler lets the composer show exactly what others will see; nothing is stored.
func PreviewHandler(c echo.Context) error {
	req, err := httpx.Bind[previewRequest](c)
	if err != nil {
		return err
	}
	r := Mask(req.Text, Options{})
	kinds := make([]string, len(r.Kinds))
	for i, k := range r.Kinds {
		kinds[i] = string(k)
	}
	return c.JSON(http.StatusOK, map[string]any{"masked": r.Text, "hidden": kinds})
}
