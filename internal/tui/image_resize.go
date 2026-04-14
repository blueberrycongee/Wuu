package tui

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
	"image/png"

	// Register decoders for formats the clipboard may produce.
	_ "image/gif"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"github.com/blueberrycongee/wuu/internal/providers"
)

const (
	// apiImageMaxBase64Size is the Anthropic API limit on base64-encoded
	// image size. Other providers have similar or larger limits.
	apiImageMaxBase64Size = 5 * 1024 * 1024 // 5 MB

	// imageTargetRawSize accounts for the ~33% base64 expansion.
	imageTargetRawSize = apiImageMaxBase64Size * 3 / 4 // ~3.75 MB

	// imageMaxDim is the maximum width or height before we downscale.
	// The Anthropic API internally resizes at 1568px; we use 2000 as a
	// comfortable client-side ceiling (same as Claude Code).
	imageMaxDim = 2000

	// imageLastResortDim is the aggressive fallback dimension.
	imageLastResortDim = 1000
)

// maybeResizeImage downsizes an image so it stays within the API's
// 5 MB base64 / 2000x2000 pixel limits. Returns the original image
// unchanged when it already fits.
func maybeResizeImage(img providers.InputImage) (providers.InputImage, error) {
	raw, err := base64.StdEncoding.DecodeString(img.Data)
	if err != nil {
		return img, nil // can't decode base64 — pass through
	}
	if len(raw) == 0 {
		return img, nil
	}

	decoded, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return img, nil // unknown format — pass through
	}

	w, h := decoded.Bounds().Dx(), decoded.Bounds().Dy()

	// Fast path: already within both limits.
	if len(raw) <= imageTargetRawSize && w <= imageMaxDim && h <= imageMaxDim {
		return img, nil
	}

	// Clamp dimensions to 2000x2000.
	newW, newH := clampImageDimensions(w, h, imageMaxDim)
	if newW != w || newH != h {
		decoded = scaleImage(decoded, newW, newH)
	}

	// Try progressive compression strategies.
	strategies := []struct {
		encode func(image.Image) ([]byte, error)
		media  string
	}{
		{encodePNG, "image/png"},
		{encodeJPEG(80), "image/jpeg"},
		{encodeJPEG(60), "image/jpeg"},
		{encodeJPEG(40), "image/jpeg"},
		{encodeJPEG(20), "image/jpeg"},
	}
	for _, s := range strategies {
		buf, err := s.encode(decoded)
		if err != nil {
			continue
		}
		if len(buf) <= imageTargetRawSize {
			return providers.InputImage{
				MediaType: s.media,
				Data:      base64.StdEncoding.EncodeToString(buf),
			}, nil
		}
	}

	// Last resort: aggressive downscale + low-quality JPEG.
	smallW, smallH := clampImageDimensions(
		decoded.Bounds().Dx(), decoded.Bounds().Dy(), imageLastResortDim,
	)
	small := scaleImage(decoded, smallW, smallH)
	buf, err := encodeJPEG(20)(small)
	if err != nil {
		return img, nil // give up, return original
	}
	return providers.InputImage{
		MediaType: "image/jpeg",
		Data:      base64.StdEncoding.EncodeToString(buf),
	}, nil
}

// clampImageDimensions scales (w, h) so neither exceeds maxDim,
// preserving aspect ratio.
func clampImageDimensions(w, h, maxDim int) (int, int) {
	if w <= maxDim && h <= maxDim {
		return w, h
	}
	if w >= h {
		return maxDim, max(1, h*maxDim/w)
	}
	return max(1, w*maxDim/h), maxDim
}

// scaleImage resamples src to (w, h) using CatmullRom interpolation.
func scaleImage(src image.Image, w, h int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	enc := &png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeJPEG(quality int) func(image.Image) ([]byte, error) {
	return func(img image.Image) ([]byte, error) {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
}
