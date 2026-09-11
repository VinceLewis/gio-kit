package shell_test

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/diagnostic"
	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/shell"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

func shellTheme() *material.Theme {
	theme := material.NewTheme()
	theme.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	return theme
}

func shellIcon(t *testing.T) *widget.Icon {
	t.Helper()
	icon, err := widget.NewIcon(icons.ActionSettings)
	if err != nil {
		t.Fatal(err)
	}
	return icon
}

func buttonNamed(label string) guitest.Selector {
	return guitest.All(guitest.Role(semantic.Button), guitest.Name(label))
}

// N1/N6: the permanent drawer scrolls through grouped navigation while the
// content and app bar remain independently usable at short landscape heights.
func TestShortLandscapeDrawerRoutedReachability(t *testing.T) {
	model := shell.Model{Title: "Application", DrawerTitle: "Destinations", TopBar: []shell.Control{{ID: "scope", Label: "Choose workspace", Icon: shellIcon(t), Enabled: true}}}
	for i := 0; i < 18; i++ {
		model.Navigation = append(model.Navigation, shell.Item{ID: fmt.Sprint(i), Label: fmt.Sprintf("Destination %02d", i), Group: fmt.Sprintf("Group %d", i/6), Selected: i == 0, Enabled: true})
	}
	w, err := shell.NewWidget(model)
	if err != nil {
		t.Fatal(err)
	}
	selected, controls, contentClicks := "", 0, 0
	w.OnNavigate = func(id string) {
		selected = id
		for i := range w.Model.Navigation {
			w.Model.Navigation[i].Selected = w.Model.Navigation[i].ID == id
		}
	}
	w.OnControl = func(string) { controls++ }
	var content widget.Clickable
	theme := shellTheme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		for content.Clicked(gtx) {
			contentClicks++
		}
		return w.Layout(gtx, theme, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return shell.Action(gtx, theme, &content, "Content action", false)
			})
		})
	}, guitest.Size(840, 240))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.Find(buttonNamed("Destination 17")); !errors.Is(err, guitest.ErrNotFound) {
		t.Fatalf("last destination should initially be virtualized: %v", err)
	}
	if err := d.Scroll(buttonNamed("Destination 00"), f32.Pt(0, 2000)); err != nil {
		t.Fatal(err)
	}
	if err := d.Settle(context.Background()); err != nil {
		t.Fatal(err)
	}
	last, err := d.Find(buttonNamed("Destination 17"))
	if err != nil {
		t.Fatal(err)
	}
	if !last.Desc.Bounds.In(image.Rect(0, 0, 240, 240)) || last.Desc.Bounds.Dy() < 48 {
		t.Fatalf("last destination target is clipped or compressed: %v", last.Desc.Bounds)
	}
	if err := d.Tap(buttonNamed("Destination 17")); err != nil {
		t.Fatal(err)
	}
	if selected != "17" {
		t.Fatal("scrolling did not expose a working navigation target")
	}
	if _, err := d.Find(guitest.All(buttonNamed("Destination 17"), guitest.Selected(true))); err != nil {
		t.Fatal(err)
	}
	if err := d.Tap(buttonNamed("Choose workspace")); err != nil {
		t.Fatal(err)
	}
	if err := d.Tap(buttonNamed("Content action")); err != nil {
		t.Fatal(err)
	}
	if controls != 1 || contentClicks != 1 || w.DebugSnapshot(diagnostic.Request{}).State["drawerFirst"].(int) == 0 {
		t.Fatal("drawer scrolling interfered with app bar, content or its own position")
	}
}

// N1/N6: controls that cannot fit beside a bounded title remain reachable in
// the compact drawer, and activating one reveals the resulting content.
func TestCompactTopBarOverflowRoutedControl(t *testing.T) {
	model := shell.Model{Title: "Application with a long complete title", DrawerTitle: "Destinations", OpenNavigationLabel: "Navigation"}
	for i := 0; i < 10; i++ {
		model.TopBar = append(model.TopBar, shell.Control{ID: fmt.Sprint(i), Label: fmt.Sprintf("Control %02d", i), Icon: shellIcon(t), Enabled: true})
	}
	w, err := shell.NewWidget(model)
	if err != nil {
		t.Fatal(err)
	}
	activated := ""
	w.OnControl = func(id string) { activated = id }
	theme := shellTheme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return w.Layout(gtx, theme, func(gtx layout.Context) layout.Dimensions {
			return material.Body1(theme, "Current content").Layout(gtx)
		})
	}, guitest.Size(320, 280))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.Find(buttonNamed("Control 09")); !errors.Is(err, guitest.ErrNotFound) {
		t.Fatalf("control should overflow: %v", err)
	}
	if err := d.Tap(guitest.Description("Navigation")); err != nil {
		t.Fatal(err)
	}
	if !w.DrawerOpen() {
		t.Fatal("navigation button did not open drawer")
	}
	if err := d.Scroll(guitest.Label("Destinations"), f32.Pt(0, 1000)); err != nil {
		t.Fatal(err)
	}
	if err := d.Settle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := d.Tap(buttonNamed("Control 09")); err != nil {
		t.Fatal(err)
	}
	if activated != "9" || w.DrawerOpen() {
		t.Fatal("overflow control did not activate and reveal content")
	}
	if _, err := d.Find(guitest.Label("Current content")); err != nil {
		t.Fatal(err)
	}
}

func TestCompactDrawerToggleStateGroupingAndCloseAction(t *testing.T) {
	model := shell.Model{
		Title: "Application", DrawerTitle: "Destinations", UtilityHeading: "Utilities",
		UnavailableSummary: "Some utilities are unavailable", OpenNavigationLabel: "Open navigation", CloseNavigationLabel: "Close navigation",
		Navigation: []shell.Item{
			{ID: "one", Label: "One", Enabled: true},
			{ID: "two", Label: "Two", Enabled: true},
		},
		Drawer: []shell.Control{
			{ID: "theme", Label: "Theme", Kind: shell.ControlToggle, ValueLabel: "Light", Enabled: true, Dismissal: shell.KeepDrawerOpen},
			{ID: "sync", Label: "Sync", DisabledReason: "Connection required"},
			{ID: "share", Label: "Share", DisabledReason: "Connection required"},
			{ID: "admin", Label: "Administration", DisabledReason: "Administrator access required"},
		},
	}
	w, err := shell.NewWidget(model)
	if err != nil {
		t.Fatal(err)
	}
	activated := ""
	w.OnControl = func(id string) {
		activated = id
		if id == "theme" {
			w.Model.Drawer[0].Value = true
			w.Model.Drawer[0].ValueLabel = "Dark"
		}
	}
	theme := shellTheme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return w.Layout(gtx, theme, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
	}, guitest.Size(360, 640))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Tap(guitest.Description("Open navigation")); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Find(guitest.Label("Utilities")); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Find(guitest.Label("Connection required")); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Sync", "Share"} {
		if _, err := d.Find(guitest.All(buttonNamed(label), guitest.Description("Connection required"), guitest.Enabled(false))); err != nil {
			t.Fatalf("%s lost its detailed accessible explanation: %v", label, err)
		}
	}
	if _, err := d.Find(guitest.Label("Administrator access required")); err != nil {
		t.Fatal(err)
	}
	toggle := guitest.All(guitest.Role(semantic.Switch), guitest.Name("Theme"))
	if _, err := d.Find(guitest.All(toggle, guitest.Selected(false), guitest.Description("Light"))); err != nil {
		t.Fatal(err)
	}
	before := w.DebugSnapshot(diagnostic.Request{}).State["drawerOffset"]
	if err := d.Tap(toggle); err != nil {
		t.Fatal(err)
	}
	if activated != "theme" || !w.DrawerOpen() {
		t.Fatal("toggle activation did not preserve the compact drawer")
	}
	if _, err := d.Find(guitest.All(toggle, guitest.Selected(true), guitest.Description("Dark"))); err != nil {
		t.Fatal(err)
	}
	state := w.DebugSnapshot(diagnostic.Request{}).State
	if state["drawerOffset"] != before {
		t.Fatal("toggle activation changed drawer scroll position")
	}
	for _, key := range []string{"navigationRowHeight", "utilityRowHeight", "groupHeadingHeight", "helpHeight"} {
		if state[key].(int) <= 0 {
			t.Fatalf("missing measured %s diagnostic", key)
		}
	}
	if err := d.Tap(guitest.Description("Close navigation")); err != nil {
		t.Fatal(err)
	}
	if w.DrawerOpen() {
		t.Fatal("close action did not dismiss the compact drawer")
	}
}

func TestDrawerInteractionStatesClearWithoutBecomingSelection(t *testing.T) {
	model := shell.Model{Title: "Application", DrawerTitle: "Destinations", OpenNavigationLabel: "Open navigation", CloseNavigationLabel: "Close navigation", Navigation: []shell.Item{
		{ID: "one", Label: "One", Selected: true, Enabled: true},
		{ID: "two", Label: "Two", Enabled: true},
	}}
	w, err := shell.NewWidget(model)
	if err != nil {
		t.Fatal(err)
	}
	theme := shellTheme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return w.Layout(gtx, theme, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
	}, guitest.Size(360, 640))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Tap(guitest.Description("Open navigation")); err != nil {
		t.Fatal(err)
	}
	two := buttonNamed("Two")
	if err := d.Press(two); err != nil {
		t.Fatal(err)
	}
	if err := d.Frame(); err != nil {
		t.Fatal(err)
	}
	states := w.DebugSnapshot(diagnostic.Request{}).State["navigation"].([]any)
	if !states[1].(map[string]any)["pressed"].(bool) || states[1].(map[string]any)["selected"].(bool) {
		t.Fatal("active press was lost or became selection")
	}
	if err := d.CancelPointer(); err != nil {
		t.Fatal(err)
	}
	states = w.DebugSnapshot(diagnostic.Request{}).State["navigation"].([]any)
	if states[1].(map[string]any)["pressed"].(bool) || states[1].(map[string]any)["selected"].(bool) {
		t.Fatal("cancelled press remained active or became selection")
	}
	node, err := d.Find(two)
	if err != nil {
		t.Fatal(err)
	}
	center := f32.Pt(float32(node.Desc.Bounds.Min.X+node.Desc.Bounds.Max.X)/2, float32(node.Desc.Bounds.Min.Y+node.Desc.Bounds.Max.Y)/2)
	if err := d.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: center}); err != nil {
		t.Fatal(err)
	}
	if err := d.Frame(); err != nil {
		t.Fatal(err)
	}
	states = w.DebugSnapshot(diagnostic.Request{}).State["navigation"].([]any)
	if !states[1].(map[string]any)["hovered"].(bool) || states[1].(map[string]any)["selected"].(bool) {
		t.Fatal("mouse hover was lost or became selection")
	}
}

// N1/N6: dp metrics, repeated resize and transient selected state all pass
// through the same shell and real input.Router, preserving full text semantics.
func TestTopBarMetricsTitlesAndTransientSelection(t *testing.T) {
	const appTitle = "A complete application title that should remain accessible"
	const controlLabel = "Choose workspace: Workspace with a deliberately long display name"
	model := shell.Model{Title: appTitle, PageTitle: "Overview", Navigation: []shell.Item{{ID: "overview", Label: "Overview destination", Selected: true, Enabled: true}}, TopBar: []shell.Control{
		{ID: "scope", Label: controlLabel, CompactLabel: "Workspace with a deliberately long display name", Icon: shellIcon(t), Enabled: true},
	}}
	w, err := shell.NewWidget(model)
	if err != nil {
		t.Fatal(err)
	}
	w.BarBackground = color.NRGBA{R: 12, G: 18, B: 28, A: 255}
	w.BarForeground = color.NRGBA{R: 240, G: 244, B: 250, A: 255}
	w.OnControl = func(string) {
		w.Model.PageTitle = "Choose workspace"
		w.Model.TopBar[0].Selected = true
		w.Model.Navigation[0].Selected = false
	}
	theme := shellTheme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return w.Layout(gtx, theme, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
	}, guitest.Size(360, 640))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Tap(buttonNamed(controlLabel)); err != nil {
		t.Fatal(err)
	}
	for _, viewport := range []struct{ width, height, scale int }{{360, 640, 1}, {720, 1280, 2}, {800, 280, 1}, {1600, 560, 2}, {600, 360, 1}, {360, 640, 1}} {
		metric := unit.Metric{PxPerDp: float32(viewport.scale), PxPerSp: float32(viewport.scale)}
		if err := d.Resize(viewport.width, viewport.height, metric); err != nil {
			t.Fatal(err)
		}
		control, err := d.Find(guitest.All(buttonNamed(controlLabel), guitest.Selected(true)))
		if err != nil {
			t.Fatal(err)
		}
		if control.Desc.Bounds.Dx() < 48*viewport.scale || control.Desc.Bounds.Dy() < 48*viewport.scale || !control.Desc.Bounds.In(image.Rect(0, 0, viewport.width, viewport.height)) {
			t.Fatalf("%+v compressed or clipped control: %v", viewport, control.Desc.Bounds)
		}
		title, err := d.Find(guitest.Label("Choose workspace"))
		if err != nil {
			t.Fatal(err)
		}
		if title.Desc.Bounds.Dx() <= 0 || title.Desc.Bounds.Overlaps(control.Desc.Bounds) {
			t.Fatalf("%+v title/control collision: %v, %v", viewport, title.Desc.Bounds, control.Desc.Bounds)
		}
		if _, err := d.Find(guitest.Description(appTitle)); err != nil {
			t.Fatal("application title lost from accessible semantics", err)
		}
		if _, err := d.Find(guitest.All(buttonNamed("Overview destination"), guitest.Selected(true))); !errors.Is(err, guitest.ErrNotFound) {
			t.Fatal("transient view retained a selected destination", err)
		}
	}
}

func TestCompactActionRoutedSelectionAndDisabledState(t *testing.T) {
	var click widget.Clickable
	selected, disabled := false, false
	theme := shellTheme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		for click.Clicked(gtx) {
			selected = true
		}
		if disabled {
			gtx = gtx.Disabled()
		}
		return shell.Action(gtx, theme, &click, "Choose", selected)
	}, guitest.Size(360, 640))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	target, err := d.Find(buttonNamed("Choose"))
	if err != nil {
		t.Fatal(err)
	}
	if target.Desc.Bounds.Dx() < 48 || target.Desc.Bounds.Dx() > 160 || target.Desc.Bounds.Dy() < 48 || target.Desc.Bounds.Dy() > 64 {
		t.Fatalf("action was not compact with an accessible target: %v", target.Desc.Bounds)
	}
	if err := d.Tap(buttonNamed("Choose")); err != nil {
		t.Fatal(err)
	}
	if !selected {
		t.Fatal("action did not receive routed input")
	}
	if _, err := d.Find(guitest.All(buttonNamed("Choose"), guitest.Selected(true))); err != nil {
		t.Fatal(err)
	}
	disabled = true
	if err := d.Frame(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Find(guitest.All(buttonNamed("Choose"), guitest.Selected(true), guitest.Enabled(false))); err != nil {
		t.Fatal("disabled action lost role, name or state", err)
	}
	if err := d.Tap(buttonNamed("Choose")); !errors.Is(err, guitest.ErrNotInteractable) {
		t.Fatal("disabled action should reject input", err)
	}
}

func TestDisabledNavigationAndControlsRetainAccessibleRoles(t *testing.T) {
	w, err := shell.NewWidget(shell.Model{
		Title: "Application", Navigation: []shell.Item{{ID: "archive", Label: "Archive", Selected: true, DisabledReason: "Requires access"}},
		TopBar: []shell.Control{{ID: "network", Label: "Network", Icon: shellIcon(t), DisabledReason: "Unavailable offline"}},
		Drawer: []shell.Control{{ID: "settings", Label: "Settings", DisabledReason: "Requires access"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	activated := 0
	w.OnNavigate = func(string) { activated++ }
	w.OnControl = func(string) { activated++ }
	theme := shellTheme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return w.Layout(gtx, theme, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
	}, guitest.Size(840, 480))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	for _, label := range []string{"Archive", "Network", "Settings"} {
		node, err := d.Find(guitest.All(buttonNamed(label), guitest.Enabled(false)))
		if err != nil {
			t.Fatalf("disabled %s lost button role, name or state: %v", label, err)
		}
		if node.Desc.Description == "" || node.Desc.Bounds.Dy() < 48 {
			t.Fatalf("disabled %s lost its reason or target bounds: %+v", label, node.Desc)
		}
		if err := d.Tap(buttonNamed(label)); !errors.Is(err, guitest.ErrNotInteractable) {
			t.Fatalf("disabled %s should reject input: %v", label, err)
		}
	}
	if activated != 0 {
		t.Fatal("disabled shell item activated")
	}
	if _, err := d.Find(guitest.All(buttonNamed("Archive"), guitest.Selected(true))); err != nil {
		t.Fatal("disabled selected destination lost selected state", err)
	}
}
