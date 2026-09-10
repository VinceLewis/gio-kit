// Package screenshot optionally renders a guitest frame. The default build is
// graphics-free; the guitestgpu tag explicitly selects Gio's headless backend.
package screenshot

import (
	"errors"
	"image"
	"image/png"
	"io"
	"os"

	"github.com/VinceLewis/gio-kit/guitest"
)

var ErrUnavailable = errors.New("guitest: headless screenshot unavailable")

// WritePNG renders the last frame. Pixels are not redacted: use synthetic data.
func WritePNG(d *guitest.Driver, writer io.Writer) error {
	if writer == nil {
		return errors.New("guitest: nil PNG writer")
	}
	img, err := capture(d)
	if err != nil {
		return err
	}
	return png.Encode(writer, img)
}

// Save pairs a redacted JSON dump with an optional PNG. On an unavailable GPU,
// JSON is preserved and ErrUnavailable is returned. base is a path without an
// extension; callers own its directory and artifact lifecycle.
func Save(d *guitest.Driver, base string, options guitest.DumpOptions) error {
	jsonFile, err := os.OpenFile(base+".json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	err = errors.Join(d.DumpJSON(jsonFile, options), jsonFile.Close())
	if err != nil {
		return err
	}
	img, err := capture(d)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(base+".png", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	return errors.Join(png.Encode(file, img), file.Close())
}

type FailureReporter interface {
	Cleanup(func())
	Failed() bool
	Logf(string, ...any)
}

// OnFailure registers artifact capture. Register it after Driver.Close cleanup
// so Go's last-in-first-out cleanup captures the frame before closing the UI.
func OnFailure(t FailureReporter, d *guitest.Driver, base string, options guitest.DumpOptions) {
	t.Cleanup(func() {
		if t.Failed() {
			if err := Save(d, base, options); err != nil {
				t.Logf("guitest artifacts: %v", err)
			}
		}
	})
}

// Difference returns the fraction of pixels with any RGBA channel difference
// above tolerance. Geometry mismatches are errors, not scaled comparisons.
func Difference(a, b image.Image, tolerance uint8) (float64, error) {
	if a == nil || b == nil || a.Bounds() != b.Bounds() {
		return 0, errors.New("guitest: image bounds differ")
	}
	r := a.Bounds()
	total := r.Dx() * r.Dy()
	if total == 0 {
		return 0, nil
	}
	different := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			for _, pair := range [][2]uint32{{ar, br}, {ag, bg}, {ab, bb}, {aa, ba}} {
				delta := int64(pair[0]) - int64(pair[1])
				if delta < 0 {
					delta = -delta
				}
				if delta > int64(tolerance)*257 {
					different++
					break
				}
			}
		}
	}
	return float64(different) / float64(total), nil
}
