// Package avatar turns an uploaded picture into the profile picture the hub
// stores: a new 512×512 JPEG, centre-cropped to a square. Only the pixels
// survive; metadata (location, camera) and anything else in the upload are
// gone, because the output is encoded from scratch.
package avatar

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // registers PNG with image.Decode
	"io"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // registers WebP
)

const (
	// MaxUpload is the largest file accepted.
	MaxUpload = 5 << 20
	// Size is the width and height of the result.
	Size = 512
	// maxPixels keeps a small file that claims huge dimensions (a
	// decompression bomb) from being decoded at all.
	maxPixels = 40_000_000
	maxSide   = 12_000
)

var (
	ErrTooLarge = errors.New("picture larger than 5 MB")
	ErrFormat   = errors.New("not a JPEG, PNG or WebP picture")
	ErrTooBig   = errors.New("picture has too many pixels")
	ErrTooSmall = errors.New("picture is too small")
)

// Process reads at most MaxUpload bytes and returns the new JPEG.
func Process(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxUpload+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxUpload {
		return nil, ErrTooLarge
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ErrFormat
	}
	switch format {
	case "jpeg", "png", "webp":
	default:
		return nil, ErrFormat
	}
	if cfg.Width < 16 || cfg.Height < 16 {
		return nil, ErrTooSmall
	}
	if cfg.Width > maxSide || cfg.Height > maxSide || cfg.Width*cfg.Height > maxPixels {
		return nil, ErrTooBig
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormat, err)
	}

	// The largest centred square.
	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	x0 := b.Min.X + (b.Dx()-side)/2
	y0 := b.Min.Y + (b.Dy()-side)/2
	square := image.Rect(x0, y0, x0+side, y0+side)

	// Transparent parts become white: JPEG has no alpha.
	dst := image.NewRGBA(image.Rect(0, 0, Size, Size))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, square, draw.Over, nil)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
