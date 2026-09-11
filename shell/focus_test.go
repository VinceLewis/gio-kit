package shell

import (
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/guitest"
)

func TestKeyboardFocusRemainsDistinctFromSelection(t *testing.T) {
	w, err := NewWidget(Model{Title: "Application", DrawerTitle: "Destinations", Navigation: []Item{{ID: "one", Label: "One", Enabled: true}}})
	if err != nil {
		t.Fatal(err)
	}
	w.drawerOpen = true
	theme := material.NewTheme()
	theme.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	requestFocus := false
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		if requestFocus {
			gtx.Execute(key.FocusCmd{Tag: w.clickable(w.navigation, "one")})
			requestFocus = false
		}
		return w.Layout(gtx, theme, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
	}, guitest.Size(360, 640))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	requestFocus = true
	if err := d.Frame(); err != nil {
		t.Fatal(err)
	}
	if err := d.Frame(); err != nil {
		t.Fatal(err)
	}
	if !w.navigationFocus["one"] || w.Model.Navigation[0].Selected {
		t.Fatal("keyboard focus was lost or became selection")
	}
	if err := d.ClearFocus(); err != nil {
		t.Fatal(err)
	}
	if w.navigationFocus["one"] {
		t.Fatal("cleared keyboard focus remained active")
	}
}
