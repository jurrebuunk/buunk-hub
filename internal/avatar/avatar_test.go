package avatar

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func encodePNG(t *testing.T, img image.Image) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// A JPEG with an EXIF block (APP1) carrying a location-like marker.
func jpegWithEXIF(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	for x := 0; x < 800; x++ {
		for y := 0; y < 600; y++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 100, 255})
		}
	}
	var b bytes.Buffer
	jpeg.Encode(&b, img, nil)
	raw := b.Bytes()
	exif := append([]byte("Exif\x00\x00"), []byte("GPS-SECRET-52.37N-4.89E")...)
	app1 := append([]byte{0xFF, 0xE1, byte((len(exif) + 2) >> 8), byte(len(exif) + 2)}, exif...)
	return append(append(append([]byte{}, raw[:2]...), app1...), raw[2:]...)
}

func TestProcessJPEGDropsMetadata(t *testing.T) {
	in := jpegWithEXIF(t)
	if !bytes.Contains(in, []byte("GPS-SECRET")) {
		t.Fatal("test input has no marker")
	}
	out, err := Process(bytes.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("GPS-SECRET")) || bytes.Contains(out, []byte("Exif")) {
		t.Error("metadata survived")
	}
	img, format, err := image.Decode(bytes.NewReader(out))
	if err != nil || format != "jpeg" {
		t.Fatalf("output: %v %s", err, format)
	}
	if b := img.Bounds(); b.Dx() != Size || b.Dy() != Size {
		t.Errorf("output %dx%d, want %dx%d", b.Dx(), b.Dy(), Size, Size)
	}
}

// A tall picture is cropped to its middle; transparency becomes white.
func TestProcessCropsAndFlattens(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 100; x++ {
			c := color.NRGBA{0, 0, 0, 0} // transparent middle
			if y < 100 || y >= 200 {
				c = color.NRGBA{255, 0, 0, 255} // red top and bottom
			}
			img.Set(x, y, c)
		}
	}
	out, err := Process(bytes.NewReader(encodePNG(t, img)))
	if err != nil {
		t.Fatal(err)
	}
	res, _, _ := image.Decode(bytes.NewReader(out))
	r, g, b, _ := res.At(Size/2, Size/2).RGBA()
	if r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Errorf("centre %d,%d,%d: want white (the middle, flattened)", r>>8, g>>8, b>>8)
	}
}

func TestProcessRefuses(t *testing.T) {
	var g bytes.Buffer
	gif.Encode(&g, image.NewPaletted(image.Rect(0, 0, 64, 64), []color.Color{color.Black}), nil)

	// A PNG header claiming 20000×20000 pixels: refused before decoding.
	bomb := encodePNG(t, image.NewGray(image.Rect(0, 0, 64, 64)))
	bomb = append([]byte{}, bomb...)
	copy(bomb[16:24], []byte{0, 0, 0x4e, 0x20, 0, 0, 0x4e, 0x20})
	binary.BigEndian.PutUint32(bomb[29:33], crc32.ChecksumIEEE(bomb[12:29])) // IHDR checksum

	for name, tc := range map[string]struct {
		in   []byte
		want error
	}{
		"gif":       {g.Bytes(), ErrFormat},
		"text":      {[]byte("<svg xmlns='http://www.w3.org/2000/svg'/>"), ErrFormat},
		"too large": {bytes.Repeat([]byte{0}, MaxUpload+1), ErrTooLarge},
		"tiny":      {encodePNG(t, image.NewGray(image.Rect(0, 0, 8, 8))), ErrTooSmall},
		"bomb":      {bomb, ErrTooBig},
	} {
		if _, err := Process(bytes.NewReader(tc.in)); !errors.Is(err, tc.want) {
			t.Errorf("%s: err %v, want %v", name, err, tc.want)
		}
	}
	if _, err := Process(strings.NewReader("")); !errors.Is(err, ErrFormat) {
		t.Errorf("empty: %v", err)
	}
}
