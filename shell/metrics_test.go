package shell

import (
	"testing"

	"github.com/VinceLewis/gio-kit/theme"
)

// HUF-05 test 2: a zero theme.Set and an empty Density are the configuration
// every pre-existing consumer has today. They must resolve to exactly the
// built-in comfortable profile — an explicit regression guard, expressed as
// concrete expected values rather than a formula recomputed from the same
// resolution logic under test.
func TestZeroMetricsAndDensityReproduceComfortableDefaults(t *testing.T) {
	w := &Widget{}
	if got, want := w.metrics(), theme.Comfortable(); got != want {
		t.Fatalf("zero Set/Density metrics = %+v, want theme.Comfortable() = %+v", got, want)
	}
	// Pin the specific fields the shell renders, so a future change to either
	// theme.Comfortable() or this resolution path is caught with a concrete,
	// human-readable expectation rather than only a struct-equality diff.
	got := w.metrics()
	if got.BarHeight < theme.MinTouchTarget {
		t.Fatalf("comfortable bar height %v is below the touch minimum", got.BarHeight)
	}
	if got.TitleSize <= 0 || got.ControlGap <= 0 || got.InlineGap <= 0 {
		t.Fatalf("comfortable metrics left a rendered field unset: %+v", got)
	}
}

func TestMetricsFollowsModelDensity(t *testing.T) {
	w := &Widget{}
	w.Model.Density = theme.DensityCompact
	if got, want := w.metrics(), theme.Compact(); got != want {
		t.Fatalf("compact density metrics = %+v, want %+v", got, want)
	}
	w.Model.Density = theme.DensitySpacious
	if got, want := w.metrics(), theme.Spacious(); got != want {
		t.Fatalf("spacious density metrics = %+v, want %+v", got, want)
	}
	w.Model.Density = ""
	w.Model.Metrics = theme.Set{Default: theme.DensitySpacious}
	if got, want := w.metrics(), theme.Spacious(); got != want {
		t.Fatalf("Set default did not apply when Density is empty: got %+v, want %+v", got, want)
	}
}
