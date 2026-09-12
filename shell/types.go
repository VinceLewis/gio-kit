// Package shell provides a responsive application frame with navigation and
// declarative control slots. It has no knowledge of application models.
package shell

import (
	"errors"
	"fmt"

	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/VinceLewis/gio-kit/theme"
)

type Mode uint8

const (
	ModeAuto Mode = iota
	ModeCompact
	ModeWide
)

// ControlKind determines the control's interaction semantics.
type ControlKind uint8

const (
	ControlAction ControlKind = iota
	ControlToggle
)

// DrawerDismissal determines whether activating a control dismisses a compact
// drawer. The zero value preserves the historical close-on-activation policy.
type DrawerDismissal uint8

const (
	DismissDrawer DrawerDismissal = iota
	KeepDrawerOpen
)

type Item struct {
	ID, Label, Group, DisabledReason string
	Icon                             *widget.Icon
	Selected, Enabled                bool
}

type Control struct {
	ID, Label, DisabledReason, Availability string
	// CompactLabel is optional visible text beside the top-bar icon. Label
	// remains the full accessible name, including when this text is elided.
	CompactLabel string
	Icon         *widget.Icon
	Kind         ControlKind
	Dismissal    DrawerDismissal
	// Value and ValueLabel describe a controlled toggle's current boolean and
	// visible state. Consumers update Value from OnControl.
	Value, Selected, Enabled bool
	ValueLabel               string
}

type Model struct {
	Title, DrawerTitle                        string
	UtilityHeading, UnavailableSummary        string
	OpenNavigationLabel, CloseNavigationLabel string
	// PageTitle identifies the current destination or transient view. When
	// supplied it is the visible app-bar title; Title remains accessible.
	PageTitle      string
	Navigation     []Item
	TopBar, Drawer []Control
	// Metrics supplies application-neutral visual geometry. The zero value
	// uses the built-in theme profiles, preserving existing callers' geometry.
	Metrics theme.Set
	// Density selects the profile. An empty value uses the Set's own default.
	Density string
}

func (m Model) Validate() error {
	if err := theme.ValidateDensity(m.Density); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, item := range m.Navigation {
		if item.ID == "" || item.Label == "" {
			return errors.New("shell: navigation requires IDs and labels")
		}
		if seen["nav:"+item.ID] {
			return fmt.Errorf("shell: duplicate navigation ID %q", item.ID)
		}
		seen["nav:"+item.ID] = true
	}
	for _, controls := range [][]Control{m.TopBar, m.Drawer} {
		for _, control := range controls {
			if control.ID == "" || control.Label == "" {
				return errors.New("shell: controls require IDs and labels")
			}
			if seen["control:"+control.ID] {
				return fmt.Errorf("shell: duplicate control ID %q", control.ID)
			}
			if control.Kind > ControlToggle {
				return fmt.Errorf("shell: control %q has invalid kind", control.ID)
			}
			if control.Dismissal > KeepDrawerOpen {
				return fmt.Errorf("shell: control %q has invalid drawer dismissal", control.ID)
			}
			seen["control:"+control.ID] = true
		}
	}
	return nil
}

func ResolveMode(mode Mode, width, breakpoint unit.Dp) Mode {
	if mode == ModeCompact || mode == ModeWide {
		return mode
	}
	if breakpoint <= 0 {
		breakpoint = 720
	}
	if width < breakpoint {
		return ModeCompact
	}
	return ModeWide
}
