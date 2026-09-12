package theme_test

import (
	"testing"

	"gioui.org/unit"
	"github.com/VinceLewis/gio-kit/theme"
)

func TestProfilesDifferMeasurably(t *testing.T) {
	compact, comfortable, spacious := theme.Compact(), theme.Comfortable(), theme.Spacious()
	if !(compact.PageGap < comfortable.PageGap && comfortable.PageGap < spacious.PageGap) {
		t.Fatalf("page gaps are not ordered: %v %v %v", compact.PageGap, comfortable.PageGap, spacious.PageGap)
	}
	if !(compact.ListGap < comfortable.ListGap && comfortable.ListGap < spacious.ListGap) {
		t.Fatalf("list gaps are not ordered: %v %v %v", compact.ListGap, comfortable.ListGap, spacious.ListGap)
	}
	if !(compact.BodySize < comfortable.BodySize && comfortable.BodySize < spacious.BodySize) {
		t.Fatalf("body sizes are not ordered: %v %v %v", compact.BodySize, comfortable.BodySize, spacious.BodySize)
	}
	for name, metrics := range map[string]theme.Metrics{"compact": compact, "comfortable": comfortable, "spacious": spacious} {
		if metrics.BarHeight < theme.MinTouchTarget {
			t.Fatalf("%s bar height %v is below the touch minimum", name, metrics.BarHeight)
		}
	}
}

func TestZeroMetricsKeepComfortableDefaults(t *testing.T) {
	if got, want := (theme.Metrics{}).WithDefaults(theme.Metrics{}), theme.Comfortable(); got != want {
		t.Fatalf("zero metrics resolved to %+v, want %+v", got, want)
	}
	if got, want := (theme.Set{}).Resolve(), theme.Comfortable(); got != want {
		t.Fatalf("zero set resolved to %+v, want %+v", got, want)
	}
	if got, want := (theme.Set{}).Resolve(theme.DensityCompact), theme.Compact(); got != want {
		t.Fatalf("zero set compact resolved to %+v, want %+v", got, want)
	}
}

func TestWithDefaultsClampsUnsafeValues(t *testing.T) {
	unsafe := theme.Metrics{PageGap: -10, IconSize: 1, BarHeight: 12, BodySize: 400, SurfaceRadius: 900}
	got := unsafe.WithDefaults(theme.Metrics{})
	if got.PageGap < 0 {
		t.Fatalf("negative gap survived: %v", got.PageGap)
	}
	if got.IconSize < unit.Dp(8) {
		t.Fatalf("icon size %v is below the safe minimum", got.IconSize)
	}
	if got.BarHeight < theme.MinTouchTarget {
		t.Fatalf("bar height %v is below the touch minimum", got.BarHeight)
	}
	if got.BodySize > unit.Sp(40) {
		t.Fatalf("body size %v exceeds the safe maximum", got.BodySize)
	}
}

func TestPrecedenceUsesFirstDeclaredName(t *testing.T) {
	set := theme.DefaultSet()
	cases := []struct {
		names []string
		want  string
	}{
		{[]string{"compact", "spacious", "comfortable"}, "compact"},
		{[]string{"", "spacious", "comfortable"}, "spacious"},
		{[]string{"", "", ""}, "comfortable"},
		{nil, "comfortable"},
		{[]string{"", "unknown", "compact"}, "comfortable"},
	}
	for _, test := range cases {
		if got := set.ResolveName(test.names...); got != test.want {
			t.Fatalf("ResolveName(%v) = %q, want %q", test.names, got, test.want)
		}
		want, _ := theme.Profile(test.want)
		if got := set.Resolve(test.names...); got != want {
			t.Fatalf("Resolve(%v) = %+v, want %+v", test.names, got, want)
		}
	}
}

func TestSetDefaultAppliesWhenNothingIsDeclared(t *testing.T) {
	set := theme.DefaultSet()
	set.Default = theme.DensityCompact
	if got, want := set.Resolve(), theme.Compact(); got != want {
		t.Fatalf("set default resolved to %+v, want %+v", got, want)
	}
	set.Default = "nonsense"
	if got, want := set.Resolve(), theme.Comfortable(); got != want {
		t.Fatalf("invalid set default resolved to %+v, want %+v", got, want)
	}
}

func TestValidateDensityRejectsUnknownNames(t *testing.T) {
	for _, name := range []string{"", "compact", "comfortable", "spacious"} {
		if err := theme.ValidateDensity(name); err != nil {
			t.Fatalf("ValidateDensity(%q) = %v, want nil", name, err)
		}
	}
	if err := theme.ValidateDensity("cosy"); err == nil {
		t.Fatal("ValidateDensity accepted an unknown name")
	}
	if _, ok := theme.Profile("cosy"); ok {
		t.Fatal("Profile reported an unknown name as known")
	}
}

func TestSetProfileCompletesPartialConfiguration(t *testing.T) {
	set := theme.Set{Compact: theme.Metrics{ListGap: 1}, Default: theme.DensityCompact}
	got := set.Profile(theme.DensityCompact)
	if got.ListGap != 1 {
		t.Fatalf("declared list gap was lost: %v", got.ListGap)
	}
	if got.IconSize != theme.Compact().IconSize {
		t.Fatalf("unset icon size %v did not fall back to the built-in compact profile", got.IconSize)
	}
}
