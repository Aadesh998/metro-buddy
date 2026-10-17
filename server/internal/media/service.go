package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"metro-buddy/server/internal/httpx"
	"metro-buddy/server/internal/model"
)

var (
	ErrRejected   = httpx.Unprocessable("image_rejected", "that image isn't allowed on Metro Buddy")
	ErrBadUpload  = httpx.Unprocessable("invalid_upload", "upload the image again")
	ErrBadMIME    = httpx.Unprocessable("invalid_media_type", "only JPEG, PNG or WebP images")
	ErrUploadSize = httpx.Unprocessable("too_large", "images must be under 5 MB")
)

type Service struct {
	Store   Store
	Scanner Scanner
}

type uploadRequest struct {
	MIME string `json:"mime" validate:"required"`
	Size int64  `json:"size" validate:"required,min=1"`
}

type UploadTicket struct {
	Key       string            `json:"key"`
	UploadURL string            `json:"upload_url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresIn int               `json:"expires_in"`
}

func tmpPrefix(persona uuid.UUID) string { return "tmp/" + persona.String() + "/" }

func (s *Service) CreateUpload(c echo.Context) error {
	req, err := httpx.Bind[uploadRequest](c)
	if err != nil {
		return err
	}
	ext, ok := AllowedMIMEs[req.MIME]
	if !ok {
		return ErrBadMIME
	}
	if req.Size > MaxUploadBytes {
		return ErrUploadSize
	}
	key := tmpPrefix(httpx.PrincipalFrom(c).PersonaID) + model.NewID().String() + "." + ext
	ttl := 10 * time.Minute
	url, err := s.Store.PresignPut(c.Request().Context(), key, req.MIME, req.Size, ttl)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, UploadTicket{Key: key, UploadURL: url, Method: http.MethodPut,
		Headers: map[string]string{"Content-Type": req.MIME}, ExpiresIn: int(ttl.Seconds())})
}

// Finalize turns a client upload into a sanitized, scanned media object owned by the uploader.
func (s *Service) Finalize(ctx context.Context, persona uuid.UUID, tmpKey string) (model.Media, Verdict, error) {
	if !strings.HasPrefix(tmpKey, tmpPrefix(persona)) || strings.Contains(tmpKey, "..") {
		return model.Media{}, Review, ErrBadUpload
	}
	raw, err := s.Store.Get(ctx, tmpKey, MaxUploadBytes)
	if err != nil {
		if errors.Is(err, ErrTooLarge) {
			return model.Media{}, Review, ErrUploadSize
		}
		return model.Media{}, Review, ErrBadUpload
	}
	clean, w, h, err := Sanitize(raw)
	if err != nil {
		return model.Media{}, Review, ErrBadMIME
	}
	verdict, err := s.Scanner.Scan(ctx, clean)
	if err != nil {
		verdict = Review
	}
	_ = s.Store.Delete(ctx, tmpKey)
	if verdict == Blocked {
		return model.Media{}, Blocked, ErrRejected
	}
	key := "m/" + model.NewID().String() + ".jpg"
	if err := s.Store.Put(ctx, key, "image/jpeg", clean); err != nil {
		return model.Media{}, Review, err
	}
	return model.Media{Key: key, Width: w, Height: h, Mime: "image/jpeg", Cleared: verdict == Clear}, verdict, nil
}

type MediaDTO struct {
	URL     string `json:"url,omitempty"`
	Width   int    `json:"w"`
	Height  int    `json:"h"`
	Pending bool   `json:"pending"`
}

// DTOs only ever sign URLs for cleared images; pending ones render as a placeholder.
func (s *Service) DTOs(ctx context.Context, ms model.MediaList) []MediaDTO {
	out := make([]MediaDTO, 0, len(ms))
	for _, m := range ms {
		d := MediaDTO{Width: m.Width, Height: m.Height, Pending: !m.Cleared}
		if m.Cleared {
			if u, err := s.Store.PresignGet(ctx, m.Key, time.Hour); err == nil {
				d.URL = u
			}
		}
		out = append(out, d)
	}
	return out
}

// DevRoutes back the in-memory store so local clients can upload and view images without R2.
func (s *Service) DevRoutes(e *echo.Echo) {
	mem, ok := s.Store.(*Memory)
	if !ok {
		return
	}
	e.PUT("/dev-upload/*", func(c echo.Context) error {
		b, err := io.ReadAll(io.LimitReader(c.Request().Body, MaxUploadBytes+1))
		if err != nil || len(b) > MaxUploadBytes {
			return ErrUploadSize
		}
		return mem.Put(c.Request().Context(), c.Param("*"), "", b)
	}, bodyLimit)
	e.GET("/dev-media/*", func(c echo.Context) error {
		b, err := mem.Get(c.Request().Context(), c.Param("*"), MaxUploadBytes)
		if err != nil {
			return httpx.NotFound()
		}
		return c.Blob(http.StatusOK, "image/jpeg", b)
	})
}

func bodyLimit(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, MaxUploadBytes+1)
		return next(c)
	}
}
