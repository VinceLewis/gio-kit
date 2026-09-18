package screenshot_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/guitest/screenshot"
)

func TestNodeColorsSeparatesForegroundFromBackground(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 20, 20))
	blue, red := color.NRGBA{B: 255, A: 255}, color.NRGBA{R: 255, A: 255}
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			img.SetNRGBA(x, y, blue)
		}
	}
	// A 4x4 red "glyph" centered in the node's interior.
	for y := 8; y < 12; y++ {
		for x := 8; x < 12; x++ {
			img.SetNRGBA(x, y, red)
		}
	}
	nodes := []guitest.FrameNode{{ID: "node/0", Bounds: guitest.Bounds{X: 0, Y: 0, Width: 20, Height: 20}}}
	colors, err := screenshot.NodeColors(img, nodes)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := colors["node/0"]
	if !ok {
		t.Fatal("expected a color estimate for node/0")
	}
	if got.Background != blue {
		t.Fatalf("background = %+v, want %+v", got.Background, blue)
	}
	if got.Foreground != red {
		t.Fatalf("foreground = %+v, want %+v", got.Foreground, red)
	}
}

func TestNodeColorsUniformFillHasEqualForegroundAndBackground(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	green := color.NRGBA{G: 255, A: 255}
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.SetNRGBA(x, y, green)
		}
	}
	nodes := []guitest.FrameNode{{ID: "node/0", Bounds: guitest.Bounds{X: 0, Y: 0, Width: 10, Height: 10}}}
	colors, err := screenshot.NodeColors(img, nodes)
	if err != nil {
		t.Fatal(err)
	}
	got := colors["node/0"]
	if got.Background != green || got.Foreground != green {
		t.Fatalf("uniform node = %+v, want both channels %+v", got, green)
	}
}

func TestNodeColorsSkipsOutOfBoundsNode(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 5, 5))
	nodes := []guitest.FrameNode{{ID: "node/0", Bounds: guitest.Bounds{X: 0, Y: 0, Width: 50, Height: 50}}}
	colors, err := screenshot.NodeColors(img, nodes)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := colors["node/0"]; ok {
		t.Fatal("expected an out-of-bounds node to be skipped, not estimated")
	}
}
