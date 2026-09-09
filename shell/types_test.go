package shell

import (
	"testing"

	"gioui.org/unit"
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
