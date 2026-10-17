package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png"
	"net/http"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	MaxUploadBytes = 5 << 20
	maxSide        = 2048
)

var (
	ErrTooLarge  = errors.New("file too large")
	ErrNotFound  = errors.New("upload not found")
	ErrBadImage  = errors.New("not a supported image")
	AllowedMIMEs = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}
)

// Sanitize decodes and re-encodes as JPEG. Re-encoding drops every metadata block (EXIF GPS,
// camera serials, XMP); the EXIF orientation is applied to the pixels first so photos stay upright.
func Sanitize(raw []byte) ([]byte, int, int, error) {
	if _, ok := AllowedMIMEs[http.DetectContentType(raw)]; !ok {
		return nil, 0, 0, ErrBadImage
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width*cfg.Height > 40_000_000 {
		return nil, 0, 0, ErrBadImage
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, 0, 0, ErrBadImage
	}
	img = orient(img, exifOrientation(raw))
	img = fit(img, maxSide)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, 0, 0, err
	}
	b := img.Bounds()
	return out.Bytes(), b.Dx(), b.Dy(), nil
}

func fit(img image.Image, max int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= max && h <= max {
		return toRGBA(img)
	}
	if w > h {
		h, w = h*max/w, max
	} else {
		w, h = w*max/h, max
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}

func toRGBA(img image.Image) *image.RGBA {
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

// exifOrientation reads tag 0x0112 from a JPEG APP1 segment; 1 (upright) when absent.
func exifOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(b) {
		if b[i] != 0xFF {
			return 1
		}
		marker := b[i+1]
		size := int(binary.BigEndian.Uint16(b[i+2 : i+4]))
		if marker == 0xDA || size < 2 || i+2+size > len(b) {
			return 1
		}
		seg := b[i+4 : i+2+size]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return parseOrientation(seg[6:])
		}
		i += 2 + size
	}
	return 1
}

func parseOrientation(t []byte) int {
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:8]))
	if off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off : off+2]))
	for k := range n {
		e := off + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:e+2]) == 0x0112 {
			v := int(bo.Uint16(t[e+8 : e+10]))
			if v >= 1 && v <= 8 {
				return v
			}
		}
	}
	return 1
}

func orient(img image.Image, o int) image.Image {
	if o == 1 {
		return img
	}
	src := toRGBA(img)
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	var dst *image.RGBA
	if o >= 5 {
		dst = image.NewRGBA(image.Rect(0, 0, h, w))
	} else {
		dst = image.NewRGBA(image.Rect(0, 0, w, h))
	}
	for y := range h {
		for x := range w {
			var nx, ny int
			switch o {
			case 2:
				nx, ny = w-1-x, y
			case 3:
				nx, ny = w-1-x, h-1-y
			case 4:
				nx, ny = x, h-1-y
			case 5:
				nx, ny = y, x
			case 6:
				nx, ny = h-1-y, x
			case 7:
				nx, ny = h-1-y, w-1-x
			case 8:
				nx, ny = y, w-1-x
			}
			dst.Set(nx, ny, src.At(x, y))
		}
	}
	return dst
}
