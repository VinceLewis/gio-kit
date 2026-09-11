package shell

import (
	"image/color"
	"testing"

	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestResponsiveMode(t *testing.T) {
	if ResolveMode(ModeAuto, 719, 720) != ModeCompact || ResolveMode(ModeAuto, 720, 720) != ModeWide {
		t.Fatal("automatic shell mode did not honor breakpoint")
	}
	if ResolveMode(ModeWide, 320, 720) != ModeWide {
		t.Fatal("explicit shell mode was ignored")
	}
	if ResolveMode(ModeAuto, unit.Dp(719), 0) != ModeCompact {
		t.Fatal("default breakpoint was not applied")
	}
}

func TestBarSurfaceColorsPreservePrimaryActionsAndDefaults(t *testing.T) {
	theme := material.NewTheme()
	original := theme.Palette
	w := &Widget{}
	if got := w.barTheme(theme); got.Palette != original {
		t.Fatal("default app bar no longer uses the supplied theme")
	}
	w.BarBackground = color.NRGBA{R: 12, G: 18, B: 28, A: 255}
	w.BarForeground = color.NRGBA{R: 240, G: 244, B: 250, A: 255}
	bar := w.barTheme(theme)
	if bar.Palette.ContrastBg != w.BarBackground || bar.Palette.ContrastFg != w.BarForeground {
		t.Fatal("bar did not receive both surface contrast colors")
	}
	if theme.Palette != original || bar.Palette.Fg != original.Fg || bar.Palette.Bg != original.Bg || bar.Shaper != theme.Shaper {
		t.Fatal("bar palette overrides changed content, primary actions or typography")
	}
	w.BarForeground = color.NRGBA{}
	if got := w.barTheme(theme); got.Palette.ContrastFg != original.ContrastFg || got.Palette.ContrastBg != w.BarBackground {
		t.Fatal("unset foreground did not independently preserve the theme default")
	}
}

func TestModelValidation(t *testing.T) {
	valid := Model{Title: "App", Navigation: []Item{{ID: "home", Label: "Home", Enabled: true}}, TopBar: []Control{{ID: "scope", Label: "Scope", Enabled: true}}}
	if _, err := NewWidget(valid); err != nil {
		t.Fatal(err)
	}
	valid.Drawer = []Control{{ID: "scope", Label: "Duplicate"}}
	if _, err := NewWidget(valid); err == nil {
		t.Fatal("duplicate control was accepted")
	}
}
