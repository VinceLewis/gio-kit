package screenshot_test

import (
	"image/color"
	"path/filepath"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/guitest/screenshot"
)

func TestCompareGoldenCreatesAndMatches(t *testing.T) {
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		paint.Fill(gtx.Ops, color.NRGBA{R: 255, A: 255})
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}, guitest.Size(32, 32))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	path := filepath.Join(t.TempDir(), "checkpoint.png")
	t.Setenv("GUITEST_UPDATE_GOLDEN", "1")
	screenshot.CompareGolden(t, d, path, 0, 0)
	t.Setenv("GUITEST_UPDATE_GOLDEN", "")
	screenshot.CompareGolden(t, d, path, 0, 0)
}
