package screenshot_test

import (
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/guitest/screenshot"
)

func TestOptionalCapturePreservesJSON(t *testing.T) {
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		paint.Fill(gtx.Ops, color.NRGBA{R: 255, A: 255})
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}, guitest.Size(32, 32))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	base := filepath.Join(t.TempDir(), "frame")
	err = screenshot.Save(d, base, guitest.DumpOptions{})
	if _, statErr := os.Stat(base + ".json"); statErr != nil {
		t.Fatal(statErr)
	}
	if errors.Is(err, screenshot.ErrUnavailable) {
		t.Log(err)
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(base + ".png")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(16, 16).RGBA()
	if r < 60000 || g > 1000 || b > 1000 {
		t.Fatalf("unexpected screenshot pixel %d %d %d", r, g, b)
	}
}

func TestGoldenDifference(t *testing.T) {
	a, b := image.NewRGBA(image.Rect(0, 0, 2, 2)), image.NewRGBA(image.Rect(0, 0, 2, 2))
	b.SetRGBA(0, 0, color.RGBA{R: 10})
	if diff, err := screenshot.Difference(a, b, 0); err != nil || diff != .25 {
		t.Fatalf("%v %v", diff, err)
	}
	if diff, err := screenshot.Difference(a, b, 10); err != nil || diff != 0 {
		t.Fatalf("%v %v", diff, err)
	}
}
