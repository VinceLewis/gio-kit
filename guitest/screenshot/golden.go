package screenshot

import (
	"errors"
	"image"
	"image/png"
	"os"

	"github.com/VinceLewis/gio-kit/guitest"
)

// goldenUpdateEnv mirrors guitest.AssertGoldenDump's env-var convention (see
// its doc comment for why an env var, not a flag): the two golden mechanisms
// are meant to be regenerated together with one `go test` invocation.
const goldenUpdateEnv = "GUITEST_UPDATE_GOLDEN"

// GoldenReporter is the subset of *testing.T golden-checkpoint assertions need.
type GoldenReporter interface {
	Helper()
	Fatalf(format string, args ...any)
	Skipf(format string, args ...any)
}

// CompareGolden renders d's current frame and compares it against the
// checkpoint PNG at path, failing if more than tolerance fraction of pixels
// differ by more than pixelTolerance per channel (see Difference). Per
// gio-json-test-upgrade-plan.md item 9's resolved decision, this is scoped to
// a handful of explicitly named checkpoints, not a general
// screenshot-everything mode — callers pick which screens are worth pinning
// a reference image for.
//
// On a build without the guitestgpu tag, rendering is unavailable
// (ErrUnavailable) and this call is skipped rather than failed, matching
// Save/OnFailure's existing behavior elsewhere in this package: a golden
// pixel checkpoint is an extra guard, never the only signal a screen is
// correct, so an environment that cannot render must not block the rest of
// the suite.
func CompareGolden(t GoldenReporter, d *guitest.Driver, path string, pixelTolerance uint8, tolerance float64) {
	t.Helper()
	img, err := capture(d)
	if errors.Is(err, ErrUnavailable) {
		t.Skipf("guitest: golden screenshot comparison unavailable: %v", err)
		return
	}
	if err != nil {
		t.Fatalf("guitest: render frame for golden comparison: %v", err)
		return
	}
	if os.Getenv(goldenUpdateEnv) != "" {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			t.Fatalf("guitest: write golden checkpoint %s: %v", path, err)
			return
		}
		err = errors.Join(png.Encode(file, img), file.Close())
		if err != nil {
			t.Fatalf("guitest: write golden checkpoint %s: %v", path, err)
		}
		return
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("guitest: golden checkpoint %s does not exist; run with %s=1 to create it", path, goldenUpdateEnv)
		return
	}
	if err != nil {
		t.Fatalf("guitest: read golden checkpoint %s: %v", path, err)
		return
	}
	defer file.Close()
	want, _, err := image.Decode(file)
	if err != nil {
		t.Fatalf("guitest: decode golden checkpoint %s: %v", path, err)
		return
	}
	diff, err := Difference(want, img, pixelTolerance)
	if err != nil {
		t.Fatalf("guitest: compare against golden checkpoint %s: %v (re-run with %s=1 after confirming the change, e.g. a size/layout change, is intended)", path, err, goldenUpdateEnv)
		return
	}
	if diff > tolerance {
		t.Fatalf("guitest: golden checkpoint %s differs in %.2f%% of pixels, want <= %.2f%% (re-run with %s=1 after confirming the change is intended)", path, diff*100, tolerance*100, goldenUpdateEnv)
	}
}
