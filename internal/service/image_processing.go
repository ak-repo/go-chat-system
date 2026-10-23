package service

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"
)

const MaxImageBytes int64 = 10 << 20
const MaxImagePixels int64 = 25_000_000

type ProcessedImage struct {
	Full, Thumbnail []byte
	MIME            string
	Width, Height   int
}

func ProcessImage(data []byte, declared string) (*ProcessedImage, error) {
	if len(data) == 0 || int64(len(data)) > MaxImageBytes {
		return nil, fmt.Errorf("image exceeds size limit")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid image")
	}
	allowed := map[string]string{"jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp"}
	if allowed[format] == "" || allowed[format] != declared {
		return nil, fmt.Errorf("unsupported or mismatched image type")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > MaxImagePixels || cfg.Width > 16384 || cfg.Height > 16384 {
		return nil, fmt.Errorf("image dimensions exceed limit")
	}
	if animatedImage(data, format) {
		return nil, fmt.Errorf("animated images are not supported")
	}
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, fmt.Errorf("corrupt image")
	}
	full := imaging.Fit(img, 2048, 2048, imaging.Lanczos)
	thumb := imaging.Fit(full, 320, 320, imaging.Lanczos)
	opaque := full.Opaque()
	mime := "image/png"
	encode := func(img image.Image) ([]byte, error) {
		var out bytes.Buffer
		var err error
		if opaque {
			err = jpeg.Encode(&out, img, &jpeg.Options{Quality: 85})
		} else {
			err = png.Encode(&out, img)
		}
		return out.Bytes(), err
	}
	if opaque {
		mime = "image/jpeg"
	}
	f, err := encode(full)
	if err != nil {
		return nil, err
	}
	t, err := encode(thumb)
	if err != nil {
		return nil, err
	}
	return &ProcessedImage{Full: f, Thumbnail: t, MIME: mime, Width: full.Bounds().Dx(), Height: full.Bounds().Dy()}, nil
}

// Check container chunks rather than searching pixel bytes for magic strings.
func animatedImage(data []byte, format string) bool {
	switch format {
	case "png":
		for pos := 8; pos+12 <= len(data); {
			n := int64(binary.BigEndian.Uint32(data[pos : pos+4]))
			if n > int64(len(data)-pos-12) {
				return true
			}
			if string(data[pos+4:pos+8]) == "acTL" {
				return true
			}
			pos += 12 + int(n)
		}
	case "webp":
		for pos := 12; pos+8 <= len(data); {
			n := int64(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
			if n > int64(len(data)-pos-8) {
				return true
			}
			kind := string(data[pos : pos+4])
			if kind == "ANIM" || kind == "ANMF" {
				return true
			}
			pos += 8 + int(n) + (int(n) % 2)
		}
	}
	return false
}
