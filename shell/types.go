// Package shell provides a responsive application frame with navigation and
// declarative control slots. It has no knowledge of application models.
package shell

import (
	"errors"
	"fmt"

	"gioui.org/unit"
	"gioui.org/widget"
)

type Mode uint8

const (
	ModeAuto Mode = iota
	ModeCompact
	ModeWide
)

type Item struct {
	ID, Label, Group, DisabledReason string
	Icon                             *widget.Icon
	Selected, Enabled                bool
}

type Control struct {
	ID, Label, DisabledReason string
	// CompactLabel is optional visible text beside the top-bar icon. Label
	// remains the full accessible name, including when this text is elided.
	CompactLabel      string
	Icon              *widget.Icon
	Selected, Enabled bool
}

type Model struct {
	Title, DrawerTitle string
	// PageTitle identifies the current destination or transient view. When
	// supplied it is the visible app-bar title; Title remains accessible.
	PageTitle      string
	Navigation     []Item
	TopBar, Drawer []Control
}

func (m Model) Validate() error {
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
