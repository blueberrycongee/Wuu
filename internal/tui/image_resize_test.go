package tui

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"testing"

	"github.com/blueberrycongee/wuu/internal/providers"
)

func makePNG(w, h int) string {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestMaybeResizeImage_SmallPassthrough(t *testing.T) {
	data := makePNG(100, 100)
	img := providers.InputImage{MediaType: "image/png", Data: data}
	out, err := maybeResizeImage(img)
	if err != nil {
		t.Fatal(err)
	}
	if out.Data != data {
		t.Fatal("small image should pass through unchanged")
	}
}

func TestMaybeResizeImage_LargeDimensionsClamped(t *testing.T) {
	data := makePNG(4000, 2000)
	img := providers.InputImage{MediaType: "image/png", Data: data}
	out, err := maybeResizeImage(img)
	if err != nil {
		t.Fatal(err)
	}

	raw, _ := base64.StdEncoding.DecodeString(out.Data)
	decoded, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() > imageMaxDim || bounds.Dy() > imageMaxDim {
		t.Fatalf("dimensions %dx%d exceed limit %d", bounds.Dx(), bounds.Dy(), imageMaxDim)
	}
	// Aspect ratio should be preserved: 4000:2000 = 2:1 → 2000:1000.
	if bounds.Dx() != 2000 || bounds.Dy() != 1000 {
		t.Fatalf("expected 2000x1000, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestClampImageDimensions(t *testing.T) {
	tests := []struct {
		w, h, max   int
		wantW, wantH int
	}{
		{800, 600, 2000, 800, 600},       // within limits
		{4000, 2000, 2000, 2000, 1000},   // width exceeds
		{1000, 3000, 2000, 666, 2000},    // height exceeds
		{5000, 5000, 2000, 2000, 2000},   // both exceed, square
	}
	for _, tt := range tests {
		gotW, gotH := clampImageDimensions(tt.w, tt.h, tt.max)
		if gotW != tt.wantW || gotH != tt.wantH {
			t.Errorf("clampImageDimensions(%d, %d, %d) = (%d, %d), want (%d, %d)",
				tt.w, tt.h, tt.max, gotW, gotH, tt.wantW, tt.wantH)
		}
	}
}

func TestMaybeResizeImage_InvalidBase64Passthrough(t *testing.T) {
	img := providers.InputImage{MediaType: "image/png", Data: "!!!invalid!!!"}
	out, err := maybeResizeImage(img)
	if err != nil {
		t.Fatal(err)
	}
	if out.Data != img.Data {
		t.Fatal("invalid base64 should pass through unchanged")
	}
}
