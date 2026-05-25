// Package imdiff renders a per-pixel highlight overlay PNG comparing a
// baseline image to a current image. Results are cached in-process by
// (sha256(baseline), sha256(current)).
package imdiff

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sync"
)

// Differ produces highlight-overlay PNGs.
type Differ struct {
	// HighlightColor is the color drawn over pixels that differ between
	// baseline and current.
	HighlightColor color.RGBA

	// FadeAlpha is how much of the current image to keep visible under
	// the highlights (0-255, where 255 = fully opaque, 64 = mostly faded).
	FadeAlpha uint8

	mu    sync.RWMutex
	cache map[[2][32]byte][]byte
}

// New returns a Differ with sensible defaults: red highlights on a faded
// current image.
func New() *Differ {
	return &Differ{
		HighlightColor: color.RGBA{255, 0, 0, 255},
		FadeAlpha:      80,
		cache:          map[[2][32]byte][]byte{},
	}
}

// Diff returns a PNG showing changed pixels highlighted on a faded copy
// of `current`. When `baseline` is empty (added snapshot), every pixel
// in `current` is highlighted. When `current` is empty (deleted snapshot),
// every pixel in `baseline` is highlighted.
func (d *Differ) Diff(baseline, current []byte) ([]byte, error) {
	key := [2][32]byte{sha256.Sum256(baseline), sha256.Sum256(current)}

	d.mu.RLock()
	if cached, ok := d.cache[key]; ok {
		d.mu.RUnlock()
		return cached, nil
	}
	d.mu.RUnlock()

	out, err := d.compute(baseline, current)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	d.cache[key] = out
	d.mu.Unlock()
	return out, nil
}

func (d *Differ) compute(baseline, current []byte) ([]byte, error) {
	switch {
	case len(current) == 0 && len(baseline) == 0:
		return nil, fmt.Errorf("imdiff: both baseline and current are empty")
	case len(current) == 0:
		// Deleted: highlight everything in baseline.
		return d.fullHighlight(baseline)
	case len(baseline) == 0:
		// Added: highlight everything in current.
		return d.fullHighlight(current)
	}

	bImg, err := png.Decode(bytes.NewReader(baseline))
	if err != nil {
		return nil, fmt.Errorf("decode baseline: %w", err)
	}
	cImg, err := png.Decode(bytes.NewReader(current))
	if err != nil {
		return nil, fmt.Errorf("decode current: %w", err)
	}

	if bImg.Bounds().Size() != cImg.Bounds().Size() {
		return nil, fmt.Errorf("imdiff: size mismatch (baseline %v, current %v)", bImg.Bounds().Size(), cImg.Bounds().Size())
	}

	w, h := cImg.Bounds().Dx(), cImg.Bounds().Dy()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	d.drawFaded(out, cImg)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !sameRGBA(bImg.At(x, y), cImg.At(x, y)) {
				out.SetRGBA(x, y, d.HighlightColor)
			}
		}
	}

	return encodePNG(out)
}

func (d *Differ) fullHighlight(src []byte) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	d.drawFaded(out, img)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out.SetRGBA(x, y, d.HighlightColor)
		}
	}
	return encodePNG(out)
}

// drawFaded copies src into dst at FadeAlpha, on a white background.
func (d *Differ) drawFaded(dst *image.RGBA, src image.Image) {
	white := image.NewUniform(color.RGBA{255, 255, 255, 255})
	draw.Draw(dst, dst.Bounds(), white, image.Point{}, draw.Src)
	mask := image.NewUniform(color.Alpha{A: d.FadeAlpha})
	draw.DrawMask(dst, dst.Bounds(), src, image.Point{}, mask, image.Point{}, draw.Over)
}

func sameRGBA(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return buf.Bytes(), nil
}
