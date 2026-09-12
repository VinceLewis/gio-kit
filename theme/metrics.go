// Package theme provides application-neutral visual metrics shared by the
// reusable Gio widgets in this module. It holds no application vocabulary, no
// colour palette, and no layout code; consumers map their own validated
// configuration onto these value types.
package theme

import (
	"fmt"

	"gioui.org/unit"
)

// Density names the built-in metric profiles. They are deliberately generic:
// a consumer maps its own declared vocabulary onto these names.
const (
	DensityCompact     = "compact"
	DensityComfortable = "comfortable"
	DensitySpacious    = "spacious"
)

// MinTouchTarget is the interactive minimum every control retains. Density
// reduces painted size, padding, and gaps; it never reduces this value.
const MinTouchTarget = unit.Dp(48)

// Metrics describes application-neutral visual geometry for one density.
// Every field is a painted measurement. Interactive minima are held separately
// in MinTouchTarget and are never derived from these values.
//
// The zero Metrics means "unspecified": Resolve and WithDefaults substitute the
// comfortable profile field by field, so an existing caller that never sets
// metrics keeps the geometry it had before this type existed.
type Metrics struct {
	// PageGap separates whole sections in a page.
	PageGap unit.Dp
	// SectionGap separates a section's own blocks: heading, comment,
	// controls, and each list, calendar, or matrix.
	SectionGap unit.Dp
	// ListGap separates adjacent rows in a list.
	ListGap unit.Dp
	// RowPaddingY and RowPaddingX pad a row's content inside its surface.
	RowPaddingY unit.Dp
	RowPaddingX unit.Dp
	// ControlGap separates whole controls in a wrapping flow.
	ControlGap unit.Dp
	// InlineGap separates inline fragments, icons, and their labels.
	InlineGap unit.Dp
	// ButtonPaddingY and ButtonPaddingX pad a painted action's label inside
	// its visible surface, independent of its touch target.
	ButtonPaddingY unit.Dp
	ButtonPaddingX unit.Dp
	// IconSize is the painted square size of a decorative or semantic icon.
	IconSize unit.Dp
	// SurfaceRadius rounds painted surfaces.
	SurfaceRadius unit.Dp
	// BarHeight is a top bar's content height, excluding system insets.
	BarHeight unit.Dp
	// TitleSize is a top bar or page title.
	TitleSize unit.Sp
	// HeadingSize is a section or list heading.
	HeadingSize unit.Sp
	// BodySize is primary body text.
	BodySize unit.Sp
	// SecondarySize is supporting or muted text.
	SecondarySize unit.Sp
}

// Bounds are the inclusive limits every metric is clamped into. They are wide
// enough for deliberate tuning and narrow enough that a configuration mistake
// cannot produce a zero-size control, a negative gap, or unbounded text.
const (
	minGap  = unit.Dp(0)
	maxGap  = unit.Dp(64)
	minSize = unit.Dp(8)
	maxSize = unit.Dp(96)
	minText = unit.Sp(10)
	maxText = unit.Sp(40)
)

// Comfortable is the default profile. Its values reproduce the geometry the
// reusable widgets used before metrics existed, so adopting this package
// changes nothing for a consumer that supplies no configuration.
func Comfortable() Metrics {
	return Metrics{
		PageGap:        12,
		SectionGap:     6,
		ListGap:        6,
		RowPaddingY:    6,
		RowPaddingX:    8,
		ControlGap:     8,
		InlineGap:      6,
		ButtonPaddingY: 6,
		ButtonPaddingX: 10,
		IconSize:       18,
		SurfaceRadius:  5,
		BarHeight:      64,
		TitleSize:      24,
		HeadingSize:    16,
		BodySize:       14,
		SecondarySize:  12,
	}
}

// Compact reduces painted size, padding, and gaps so more content is legible
// above the fold. Touch targets and font scaling are unaffected.
func Compact() Metrics {
	return Metrics{
		PageGap:        8,
		SectionGap:     4,
		ListGap:        2,
		RowPaddingY:    8,
		RowPaddingX:    8,
		ControlGap:     4,
		InlineGap:      4,
		ButtonPaddingY: 4,
		ButtonPaddingX: 8,
		IconSize:       16,
		SurfaceRadius:  4,
		BarHeight:      56,
		TitleSize:      20,
		HeadingSize:    15,
		BodySize:       13,
		SecondarySize:  11,
	}
}

// Spacious increases padding and gaps for low-density presentation.
func Spacious() Metrics {
	return Metrics{
		PageGap:        20,
		SectionGap:     10,
		ListGap:        10,
		RowPaddingY:    12,
		RowPaddingX:    14,
		ControlGap:     12,
		InlineGap:      8,
		ButtonPaddingY: 10,
		ButtonPaddingX: 16,
		IconSize:       22,
		SurfaceRadius:  8,
		BarHeight:      72,
		TitleSize:      26,
		HeadingSize:    18,
		BodySize:       16,
		SecondarySize:  14,
	}
}

// ValidateDensity reports whether name is a known density. An empty name is
// valid and means "inherit from a less specific declaration".
func ValidateDensity(name string) error {
	switch name {
	case "", DensityCompact, DensityComfortable, DensitySpacious:
		return nil
	}
	return fmt.Errorf("theme: unknown density %q", name)
}

// Profile returns the built-in profile for a density name. An empty name
// returns the comfortable profile. ok is false for an unknown name, and the
// returned profile is then the comfortable profile so a caller that chooses to
// continue still renders a usable page.
func Profile(name string) (Metrics, bool) {
	switch name {
	case DensityCompact:
		return Compact(), true
	case DensitySpacious:
		return Spacious(), true
	case "", DensityComfortable:
		return Comfortable(), true
	}
	return Comfortable(), false
}

// WithDefaults substitutes fallback for every unspecified field, then clamps
// each field into its safe bound. A zero fallback field falls through to the
// comfortable profile, so the result is always complete and usable.
func (m Metrics) WithDefaults(fallback Metrics) Metrics {
	base := Comfortable()
	pickDp := func(value, alternative, builtin unit.Dp) unit.Dp {
		if value > 0 {
			return value
		}
		if alternative > 0 {
			return alternative
		}
		return builtin
	}
	pickSp := func(value, alternative, builtin unit.Sp) unit.Sp {
		if value > 0 {
			return value
		}
		if alternative > 0 {
			return alternative
		}
		return builtin
	}
	// A zero gap is a legitimate declared value, so gaps distinguish "unset"
	// only through the fallback chain rather than by treating zero as absent.
	result := Metrics{
		PageGap:        pickDp(m.PageGap, fallback.PageGap, base.PageGap),
		SectionGap:     pickDp(m.SectionGap, fallback.SectionGap, base.SectionGap),
		ListGap:        pickDp(m.ListGap, fallback.ListGap, base.ListGap),
		RowPaddingY:    pickDp(m.RowPaddingY, fallback.RowPaddingY, base.RowPaddingY),
		RowPaddingX:    pickDp(m.RowPaddingX, fallback.RowPaddingX, base.RowPaddingX),
		ControlGap:     pickDp(m.ControlGap, fallback.ControlGap, base.ControlGap),
		InlineGap:      pickDp(m.InlineGap, fallback.InlineGap, base.InlineGap),
		ButtonPaddingY: pickDp(m.ButtonPaddingY, fallback.ButtonPaddingY, base.ButtonPaddingY),
		ButtonPaddingX: pickDp(m.ButtonPaddingX, fallback.ButtonPaddingX, base.ButtonPaddingX),
		IconSize:       pickDp(m.IconSize, fallback.IconSize, base.IconSize),
		SurfaceRadius:  pickDp(m.SurfaceRadius, fallback.SurfaceRadius, base.SurfaceRadius),
		BarHeight:      pickDp(m.BarHeight, fallback.BarHeight, base.BarHeight),
		TitleSize:      pickSp(m.TitleSize, fallback.TitleSize, base.TitleSize),
		HeadingSize:    pickSp(m.HeadingSize, fallback.HeadingSize, base.HeadingSize),
		BodySize:       pickSp(m.BodySize, fallback.BodySize, base.BodySize),
		SecondarySize:  pickSp(m.SecondarySize, fallback.SecondarySize, base.SecondarySize),
	}
	return result.clamp()
}

func (m Metrics) clamp() Metrics {
	gap := func(value unit.Dp) unit.Dp { return clampDp(value, minGap, maxGap) }
	size := func(value unit.Dp) unit.Dp { return clampDp(value, minSize, maxSize) }
	text := func(value unit.Sp) unit.Sp { return clampSp(value, minText, maxText) }
	return Metrics{
		PageGap:        gap(m.PageGap),
		SectionGap:     gap(m.SectionGap),
		ListGap:        gap(m.ListGap),
		RowPaddingY:    gap(m.RowPaddingY),
		RowPaddingX:    gap(m.RowPaddingX),
		ControlGap:     gap(m.ControlGap),
		InlineGap:      gap(m.InlineGap),
		ButtonPaddingY: gap(m.ButtonPaddingY),
		ButtonPaddingX: gap(m.ButtonPaddingX),
		IconSize:       size(m.IconSize),
		SurfaceRadius:  clampDp(m.SurfaceRadius, 0, maxSize),
		BarHeight:      clampDp(m.BarHeight, MinTouchTarget, maxSize),
		TitleSize:      text(m.TitleSize),
		HeadingSize:    text(m.HeadingSize),
		BodySize:       text(m.BodySize),
		SecondarySize:  text(m.SecondarySize),
	}
}

func clampDp(value, low, high unit.Dp) unit.Dp {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func clampSp(value, low, high unit.Sp) unit.Sp {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// Set holds one profile per density plus the profile used when nothing is
// declared. A consumer builds a Set from its own validated configuration and
// passes it to a widget by value; the widget never mutates it.
//
// The zero Set resolves entirely to the built-in profiles.
type Set struct {
	Compact     Metrics
	Comfortable Metrics
	Spacious    Metrics
	// Default names the density used when no declaration supplies one.
	// An empty Default means comfortable.
	Default string
}

// DefaultSet returns the built-in profiles with a comfortable default.
func DefaultSet() Set {
	return Set{Compact: Compact(), Comfortable: Comfortable(), Spacious: Spacious(), Default: DensityComfortable}
}

// Profile returns the completed profile for one density name. Unspecified
// fields fall back to the matching built-in profile and are clamped. An
// unknown name resolves as the Set's default, matching Resolve.
func (s Set) Profile(name string) Metrics {
	switch name {
	case DensityCompact:
		return s.Compact.WithDefaults(Compact())
	case DensitySpacious:
		return s.Spacious.WithDefaults(Spacious())
	case DensityComfortable:
		return s.Comfortable.WithDefaults(Comfortable())
	}
	// An empty or unknown name resolves through the Set's own default, which
	// defaultName has already reduced to a known density.
	return s.Profile(s.defaultName())
}

func (s Set) defaultName() string {
	switch s.Default {
	case DensityCompact, DensityComfortable, DensitySpacious:
		return s.Default
	}
	return DensityComfortable
}

// Resolve applies the precedence rule shared by every consumer of this
// package: the first non-empty name wins, from most specific to least. Callers
// pass their declarations in that order, ending with any configured default;
// an empty or exhausted list resolves to the Set's own default.
//
// An unknown name is not silently ignored: it resolves to the Set's default
// rather than falling through to the next, weaker declaration, so a consumer
// that validates its input sees the same profile its validation described.
func (s Set) Resolve(names ...string) Metrics {
	for _, name := range names {
		if name == "" {
			continue
		}
		return s.Profile(name)
	}
	return s.Profile(s.defaultName())
}

// ResolveName returns the density name Resolve would use. Consumers use it for
// diagnostics and tests without re-implementing the precedence rule.
func (s Set) ResolveName(names ...string) string {
	for _, name := range names {
		if name == "" {
			continue
		}
		if err := ValidateDensity(name); err != nil {
			return s.defaultName()
		}
		return name
	}
	return s.defaultName()
}
