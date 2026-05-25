package imdiff

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"sync"
	"testing"
)

func solidPNG(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

// stripePNG returns a wxh image whose left half is colorA and right half colorB.
func stripePNG(t *testing.T, w, h int, a, b color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < w/2 {
				img.SetRGBA(x, y, a)
			} else {
				img.SetRGBA(x, y, b)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

func decode(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return img
}

func TestDiff_identicalImagesProduceNoHighlights(t *testing.T) {
	white := color.RGBA{255, 255, 255, 255}
	a := solidPNG(t, 4, 4, white)
	b := solidPNG(t, 4, 4, white)

	d := New()
	out, err := d.Diff(a, b)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	img := decode(t, out)
	hi := d.HighlightColor
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if uint8(r>>8) == hi.R && uint8(g>>8) == hi.G && uint8(bl>>8) == hi.B {
				t.Errorf("found highlight pixel at (%d,%d) in identical-image diff", x, y)
			}
		}
	}
}

func TestDiff_differentImagesHighlightChangedPixels(t *testing.T) {
	white := color.RGBA{255, 255, 255, 255}
	black := color.RGBA{0, 0, 0, 255}
	a := solidPNG(t, 4, 4, white)
	b := stripePNG(t, 4, 4, white, black)

	d := New()
	out, err := d.Diff(a, b)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	img := decode(t, out)
	hi := d.HighlightColor

	highlights := 0
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if uint8(r>>8) == hi.R && uint8(g>>8) == hi.G && uint8(bl>>8) == hi.B {
				highlights++
			}
		}
	}
	// Right half of the 4x4 (8 pixels) changed; require at least that many.
	if highlights < 8 {
		t.Errorf("expected >=8 highlight pixels, got %d", highlights)
	}
}

func TestDiff_sizeMismatchErrors(t *testing.T) {
	white := color.RGBA{255, 255, 255, 255}
	a := solidPNG(t, 2, 2, white)
	b := solidPNG(t, 3, 3, white)

	d := New()
	if _, err := d.Diff(a, b); err == nil {
		t.Fatal("expected size-mismatch error")
	}
}

func TestDiff_invalidPNGErrors(t *testing.T) {
	d := New()
	if _, err := d.Diff([]byte("not png"), []byte("nope")); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestDiff_caches(t *testing.T) {
	white := color.RGBA{255, 255, 255, 255}
	black := color.RGBA{0, 0, 0, 255}
	a := solidPNG(t, 4, 4, white)
	b := stripePNG(t, 4, 4, white, black)

	d := New()
	out1, err := d.Diff(a, b)
	if err != nil {
		t.Fatalf("Diff 1: %v", err)
	}
	out2, err := d.Diff(a, b)
	if err != nil {
		t.Fatalf("Diff 2: %v", err)
	}
	if !bytes.Equal(out1, out2) {
		t.Error("cached call returned different bytes")
	}
	// Implementation detail: cache hit returns the SAME byte slice header.
	if &out1[0] != &out2[0] {
		t.Error("expected cached call to return the same underlying slice")
	}
}

func TestDiff_concurrentSafe(t *testing.T) {
	white := color.RGBA{255, 255, 255, 255}
	black := color.RGBA{0, 0, 0, 255}
	a := solidPNG(t, 4, 4, white)
	b := stripePNG(t, 4, 4, white, black)

	d := New()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := d.Diff(a, b); err != nil {
				t.Errorf("Diff: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestDiff_addedSnapshotUsesEmptyBaseline(t *testing.T) {
	// Added files have empty baseline. Diff should still produce a valid PNG
	// (the whole current image is "new"), not an error.
	black := color.RGBA{0, 0, 0, 255}
	current := solidPNG(t, 4, 4, black)

	d := New()
	out, err := d.Diff(nil, current)
	if err != nil {
		t.Fatalf("Diff with nil baseline: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("output is not a valid PNG: %v", err)
	}
}

func TestDiff_deletedSnapshotUsesEmptyCurrent(t *testing.T) {
	// Deleted files have empty current. Diff should produce a valid PNG
	// (the whole baseline image was "removed").
	black := color.RGBA{0, 0, 0, 255}
	baseline := solidPNG(t, 4, 4, black)

	d := New()
	out, err := d.Diff(baseline, nil)
	if err != nil {
		t.Fatalf("Diff with nil current: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("output is not a valid PNG: %v", err)
	}
}
