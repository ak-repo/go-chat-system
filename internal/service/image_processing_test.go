package service

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"testing"
)

func testPNG(t *testing.T, w, h int, transparent bool) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if !transparent {
		draw.Draw(img, img.Bounds(), &image.Uniform{C: color.NRGBA{R: 220, G: 40, B: 60, A: 255}}, image.Point{}, draw.Src)
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestProcessImageBoundsAndTransparency(t *testing.T) {
	for _, transparent := range []bool{false, true} {
		p, err := ProcessImage(testPNG(t, 2400, 1200, transparent), "image/png")
		if err != nil {
			t.Fatal(err)
		}
		if p.Width != 2048 || p.Height != 1024 {
			t.Fatalf("dimensions: %dx%d", p.Width, p.Height)
		}
		want := "image/jpeg"
		if transparent {
			want = "image/png"
		}
		if p.MIME != want {
			t.Fatalf("mime %s", p.MIME)
		}
		thumb, _, err := image.DecodeConfig(bytes.NewReader(p.Thumbnail))
		if err != nil || thumb.Width != 320 || thumb.Height != 160 {
			t.Fatalf("thumbnail %#v %v", thumb, err)
		}
	}
	p, err := ProcessImage(testPNG(t, 8, 4, false), "image/png")
	if err != nil || p.Width != 8 || p.Height != 4 {
		t.Fatalf("small image upscaled: %#v %v", p, err)
	}
}
func TestProcessImageRejectsInvalidInputs(t *testing.T) {
	small := testPNG(t, 4, 2, false)
	largeHeader := append([]byte(nil), small...)
	binary.BigEndian.PutUint32(largeHeader[16:20], 10000)
	binary.BigEndian.PutUint32(largeHeader[20:24], 10000)
	binary.BigEndian.PutUint32(largeHeader[29:33], crc32.ChecksumIEEE(largeHeader[12:29]))
	animationChunk := make([]byte, 20)
	binary.BigEndian.PutUint32(animationChunk[:4], 8)
	copy(animationChunk[4:8], "acTL")
	binary.BigEndian.PutUint32(animationChunk[8:12], 2)
	binary.BigEndian.PutUint32(animationChunk[16:20], crc32.ChecksumIEEE(animationChunk[4:16]))
	animated := append(append(append([]byte{}, small[:33]...), animationChunk...), small[33:]...)
	for _, tc := range []struct {
		name string
		data []byte
		mime string
	}{{"empty", nil, "image/png"}, {"html", []byte("<svg onload='evil'/>"), "image/png"}, {"spoofed", small, "image/jpeg"}, {"size", make([]byte, MaxImageBytes+1), "image/png"}, {"pixels", largeHeader, "image/png"}, {"truncated", small[:35], "image/png"}, {"animated PNG", animated, "image/png"}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ProcessImage(tc.data, tc.mime); err == nil {
				t.Fatal("accepted invalid image")
			}
		})
	}
	webp := append([]byte("RIFFxxxxWEBP"), []byte("ANIM\x00\x00\x00\x00")...)
	if !animatedImage(webp, "webp") {
		t.Fatal("accepted animated WebP")
	}
}
func TestProcessImageNormalizesEXIFOrientation(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	// EXIF little-endian TIFF: orientation tag 0x112, value 6 (rotate 90 CW).
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	data := []byte{0xff, 0xd8, 0xff, 0xe1, 0, byte(len(exif) + 2)}
	data = append(data, exif...)
	data = append(data, b.Bytes()[2:]...)
	p, err := ProcessImage(data, "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if p.Width != 4 || p.Height != 8 {
		t.Fatalf("orientation not normalized: %dx%d", p.Width, p.Height)
	}
	if bytes.Contains(p.Full, []byte("Exif")) {
		t.Fatal("metadata retained")
	}
}

func TestProcessRealWebP(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString("UklGRh4AAABXRUJQVlA4TBEAAAAvA0AAEAdQmf7Xo4CBiOh/AAA=")
	if err != nil {
		t.Fatal(err)
	}
	processed, err := ProcessImage(data, "image/webp")
	if err != nil {
		t.Fatal(err)
	}
	if processed.Width != 4 || processed.Height != 2 || processed.MIME != "image/png" {
		t.Fatalf("WebP metadata: %#v", processed)
	}
}
