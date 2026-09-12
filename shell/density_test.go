package shell_test

import (
	"errors"
	"fmt"
	"image"
	"testing"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"
	"github.com/VinceLewis/gio-kit/diagnostic"
	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/shell"
	"github.com/VinceLewis/gio-kit/theme"
)

// HUF-05 test 1: the same model renders a measurably different bar height and
// title size between compact and comfortable, while every accessible label,
// role and state stays identical.
func TestCompactAndComfortableBarDifferMeasurablyWithSameSemantics(t *testing.T) {
	model := shell.Model{
		Title: "Application", DrawerTitle: "Destinations", OpenNavigationLabel: "Open navigation",
		Navigation: []shell.Item{{ID: "home", Label: "Home", Selected: true, Enabled: true}},
		TopBar:     []shell.Control{{ID: "scope", Label: "Scope control", Icon: shellIcon(t), Enabled: true}},
	}
	th := shellTheme()
	heights := make(map[string]int)
	for _, density := range []string{theme.DensityCompact, theme.DensityComfortable} {
		model.Density = density
		w, err := shell.NewWidget(model)
		if err != nil {
			t.Fatal(err)
		}
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
			return w.Layout(gtx, th, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
		}, guitest.Size(360, 640))
		if err != nil {
			t.Fatal(err)
		}
		heights[density] = w.DebugSnapshot(diagnostic.Request{}).State["barHeight"].(int)
		if _, err := d.Find(buttonNamed("Scope control")); err != nil {
			t.Fatalf("%s lost control semantics: %v", density, err)
		}
		if _, err := d.Find(guitest.Description("Open navigation")); err != nil {
			t.Fatalf("%s lost navigation button semantics: %v", density, err)
		}
		if err := d.Tap(guitest.Description("Open navigation")); err != nil {
			t.Fatalf("%s: %v", density, err)
		}
		if _, err := d.Find(guitest.All(buttonNamed("Home"), guitest.Selected(true))); err != nil {
			t.Fatalf("%s lost navigation semantics: %v", density, err)
		}
		_ = d.Close()
	}
	if heights[theme.DensityCompact] != 56 || heights[theme.DensityComfortable] != 64 {
		t.Fatalf("bar heights = compact:%d comfortable:%d, want 56 and 64", heights[theme.DensityCompact], heights[theme.DensityComfortable])
	}
}

// HUF-05 test 3: navigation and context-action touch targets stay at least
// 48dp, without overlapping, in every profile.
func TestTouchTargetsRemainAtLeastMinimumAcrossProfiles(t *testing.T) {
	for _, density := range []string{theme.DensityCompact, theme.DensityComfortable, theme.DensitySpacious} {
		model := shell.Model{
			Title: "Application", OpenNavigationLabel: "Open navigation", Density: density,
			TopBar: []shell.Control{{ID: "scope", Label: "Scope control", Icon: shellIcon(t), Enabled: true}},
		}
		w, err := shell.NewWidget(model)
		if err != nil {
			t.Fatal(err)
		}
		th := shellTheme()
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
			return w.Layout(gtx, th, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
		}, guitest.Size(360, 640))
		if err != nil {
			t.Fatal(err)
		}
		nav, err := d.Find(guitest.Description("Open navigation"))
		if err != nil {
			t.Fatal(err)
		}
		control, err := d.Find(buttonNamed("Scope control"))
		if err != nil {
			t.Fatal(err)
		}
		for name, bounds := range map[string]image.Rectangle{"navigation": nav.Desc.Bounds, "control": control.Desc.Bounds} {
			if bounds.Dx() < 48 || bounds.Dy() < 48 {
				t.Fatalf("%s %s touch target below 48dp: %v", density, name, bounds)
			}
		}
		if nav.Desc.Bounds.Overlaps(control.Desc.Bounds) {
			t.Fatalf("%s navigation and control targets overlap: %v %v", density, nav.Desc.Bounds, control.Desc.Bounds)
		}
		_ = d.Close()
	}
}

// HUF-05 test 4: at a narrow width the context action is bounded to roughly
// 45% of the bar, its visible text is elided, and its accessible name stays
// the complete label rather than the truncated visible text.
func TestNarrowContextActionBoundedEllipsizedWithFullAccessibleName(t *testing.T) {
	const fullLabel = "Choose workspace: Engineering Productivity Analytics Program"
	const visible = "Engineering Productivity Analytics Program display name"
	model := shell.Model{
		Title: "Application", OpenNavigationLabel: "Open navigation",
		TopBar: []shell.Control{{ID: "scope", Label: fullLabel, CompactLabel: visible, Icon: shellIcon(t), Enabled: true}},
	}
	w, err := shell.NewWidget(model)
	if err != nil {
		t.Fatal(err)
	}
	th := shellTheme()
	width := 400
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return w.Layout(gtx, th, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
	}, guitest.Size(width, 640))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	node, err := d.Find(buttonNamed(fullLabel))
	if err != nil {
		t.Fatal(err)
	}
	if node.Desc.Label != fullLabel {
		t.Fatalf("accessible name lost: %q", node.Desc.Label)
	}
	if node.Desc.Label == visible {
		t.Fatal("semantic label collapsed to the truncated visible text")
	}
	got := node.Desc.Bounds.Dx()
	upper := int(0.45*float32(width)) + 24 // tolerance for the bar's own edge margin and gaps
	if got < 48 || got > upper {
		t.Fatalf("context action width %d not bounded to ~45%% of a %d wide bar (upper=%d)", got, width, upper)
	}
}

// HUF-05 test 5: SetModel rejects an unknown density and accepts every known one.
func TestSetModelValidatesDensity(t *testing.T) {
	w, err := shell.NewWidget(shell.Model{Title: "App"})
	if err != nil {
		t.Fatal(err)
	}
	for _, density := range []string{"", theme.DensityCompact, theme.DensityComfortable, theme.DensitySpacious} {
		if err := w.SetModel(shell.Model{Title: "App", Density: density}); err != nil {
			t.Fatalf("SetModel rejected valid density %q: %v", density, err)
		}
	}
	if err := w.SetModel(shell.Model{Title: "App", Density: "cosy"}); err == nil {
		t.Fatal("SetModel accepted an unknown density")
	}
	if _, err := shell.NewWidget(shell.Model{Title: "App", Density: "cosy"}); err == nil {
		t.Fatal("NewWidget accepted an unknown density")
	}
}

// HUF-05 test 6: raising the font scale grows the title and neither clips it
// nor makes it overlap the bar's controls.
func TestFontScaleGrowsTitleWithoutClippingOrOverlappingControls(t *testing.T) {
	model := shell.Model{
		Title: "Overview", OpenNavigationLabel: "Open navigation",
		TopBar: []shell.Control{{ID: "scope", Label: "Choose workspace", Icon: shellIcon(t), Enabled: true}},
	}
	w, err := shell.NewWidget(model)
	if err != nil {
		t.Fatal(err)
	}
	th := shellTheme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return w.Layout(gtx, th, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
	}, guitest.Size(400, 640))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	baseline, err := d.Find(guitest.Description("Overview"))
	if err != nil {
		t.Fatal(err)
	}
	baseHeight := baseline.Desc.Bounds.Dy()
	if err := d.Resize(400, 640, unit.Metric{PxPerDp: 1, PxPerSp: 3}); err != nil {
		t.Fatal(err)
	}
	scaled, err := d.Find(guitest.Description("Overview"))
	if err != nil {
		t.Fatal(err)
	}
	if scaled.Desc.Bounds.Dy() <= baseHeight {
		t.Fatalf("title did not grow with font scale: base=%d scaled=%d", baseHeight, scaled.Desc.Bounds.Dy())
	}
	control, err := d.Find(buttonNamed("Choose workspace"))
	if err != nil {
		t.Fatal(err)
	}
	if control.Desc.Bounds.Dx() < 48 || control.Desc.Bounds.Dy() < 48 {
		t.Fatalf("control touch target shrank under font scale: %v", control.Desc.Bounds)
	}
	if scaled.Desc.Bounds.Overlaps(control.Desc.Bounds) {
		t.Fatalf("title overlapped bar control at large font scale: title=%v control=%v", scaled.Desc.Bounds, control.Desc.Bounds)
	}
	if !scaled.Desc.Bounds.In(image.Rect(0, 0, 400, 640)) {
		t.Fatalf("title clipped out of the viewport at large font scale: %v", scaled.Desc.Bounds)
	}
}

// HUF-05 test 7: navigation and a bar control still deliver routed input
// callbacks in every profile.
func TestRoutedInputAcrossDensityProfiles(t *testing.T) {
	for _, density := range []string{theme.DensityCompact, theme.DensityComfortable, theme.DensitySpacious} {
		model := shell.Model{
			Title: "Application", DrawerTitle: "Destinations", OpenNavigationLabel: "Open navigation",
			Density:    density,
			Navigation: []shell.Item{{ID: "home", Label: "Home", Enabled: true}},
			TopBar:     []shell.Control{{ID: "scope", Label: "Scope control", Icon: shellIcon(t), Enabled: true}},
		}
		w, err := shell.NewWidget(model)
		if err != nil {
			t.Fatal(err)
		}
		navigated, activated := "", ""
		w.OnNavigate = func(id string) { navigated = id }
		w.OnControl = func(id string) { activated = id }
		th := shellTheme()
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
			return w.Layout(gtx, th, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
		}, guitest.Size(360, 640))
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Tap(buttonNamed("Scope control")); err != nil {
			t.Fatalf("%s: %v", density, err)
		}
		if activated != "scope" {
			t.Fatalf("%s: control activation was not routed", density)
		}
		if err := d.Tap(guitest.Description("Open navigation")); err != nil {
			t.Fatal(err)
		}
		if !w.DrawerOpen() {
			t.Fatalf("%s: navigation button did not open the drawer", density)
		}
		if err := d.Tap(buttonNamed("Home")); err != nil {
			t.Fatal(err)
		}
		if navigated != "home" {
			t.Fatalf("%s: navigation tap was not routed", density)
		}
		_ = d.Close()
	}
}

// HUF-05-followup test 1: two text-bearing controls (a context summary and a
// toggle) both fit on a wide bar, each at least 48dp, non-overlapping, and
// each bounded to roughly 45% of the bar width rather than starving its
// sibling.
func TestWideBarKeepsBothTextBearingControls(t *testing.T) {
	const fullLabel = "Contexts: Identity: Example Person"
	model := shell.Model{
		Title: "Application", OpenNavigationLabel: "Open navigation",
		TopBar: []shell.Control{
			{ID: "context", Label: fullLabel, CompactLabel: "Identity: Example Person", Icon: shellIcon(t), Enabled: true},
			{ID: "theme", Label: "Theme", Kind: shell.ControlToggle, ValueLabel: "Light", Enabled: true},
		},
	}
	w, err := shell.NewWidget(model)
	if err != nil {
		t.Fatal(err)
	}
	th := shellTheme()
	width := 1200
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return w.Layout(gtx, th, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
	}, guitest.Size(width, 700))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	context, err := d.Find(buttonNamed(fullLabel))
	if err != nil {
		t.Fatalf("context control was dropped: %v", err)
	}
	toggle, err := d.Find(guitest.All(guitest.Role(semantic.Switch), guitest.Name("Theme")))
	if err != nil {
		t.Fatalf("theme toggle was dropped: %v", err)
	}
	capWidth := int(0.45*float32(width)) + 1 // tolerance for rounding
	for name, bounds := range map[string]image.Rectangle{"context": context.Desc.Bounds, "theme": toggle.Desc.Bounds} {
		if bounds.Dx() < 48 || bounds.Dy() < 48 {
			t.Fatalf("%s touch target below 48dp: %v", name, bounds)
		}
		if bounds.Dx() > capWidth {
			t.Fatalf("%s width %d exceeds the ~45%% bar cap %d", name, bounds.Dx(), capWidth)
		}
	}
	if context.Desc.Bounds.Overlaps(toggle.Desc.Bounds) {
		t.Fatalf("context and theme controls overlap: %v %v", context.Desc.Bounds, toggle.Desc.Bounds)
	}
}

// HUF-05-followup test 2: the same two controls at a compact phone width pin
// the deliberate overflow shape — one control stays on the bar at the single-
// control cap, the other overflows into the drawer, rather than squeezing
// both below a readable width.
func TestCompactBarOverflowsSecondTextBearingControl(t *testing.T) {
	const fullLabel = "Contexts: Identity: Example Person"
	model := shell.Model{
		Title: "Application", OpenNavigationLabel: "Open navigation", DrawerTitle: "Destinations",
		TopBar: []shell.Control{
			{ID: "context", Label: fullLabel, CompactLabel: "Identity: Example Person", Icon: shellIcon(t), Enabled: true},
			{ID: "theme", Label: "Theme", Kind: shell.ControlToggle, ValueLabel: "Light", Enabled: true},
		},
	}
	w, err := shell.NewWidget(model)
	if err != nil {
		t.Fatal(err)
	}
	th := shellTheme()
	width := 412
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return w.Layout(gtx, th, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
	}, guitest.Size(width, 800))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	context, err := d.Find(buttonNamed(fullLabel))
	if err != nil {
		t.Fatalf("first-declared context control should stay on a compact bar: %v", err)
	}
	if context.Desc.Bounds.Dx() < 48 || context.Desc.Bounds.Dy() < 48 {
		t.Fatalf("context control below 48dp on a compact bar: %v", context.Desc.Bounds)
	}
	capWidth := int(0.45*float32(width)) + 1
	if context.Desc.Bounds.Dx() > capWidth {
		t.Fatalf("context control width %d exceeds the single-control cap %d on a compact bar", context.Desc.Bounds.Dx(), capWidth)
	}
	if _, err := d.Find(guitest.All(guitest.Role(semantic.Switch), guitest.Name("Theme"))); !errors.Is(err, guitest.ErrNotFound) {
		t.Fatalf("theme toggle should have overflowed off a compact bar, not been squeezed in: %v", err)
	}
	if err := d.Tap(guitest.Description("Open navigation")); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Find(guitest.All(guitest.Role(semantic.Switch), guitest.Name("Theme"))); err != nil {
		t.Fatalf("overflowed theme toggle should still be reachable in the drawer: %v", err)
	}
}

// HUF-05-followup test 3: regression guard for the reported defect — several
// text-bearing controls sharing space that genuinely exists must all stay on
// the bar; none may be dropped purely because a sibling was placed first and
// treated the ~45% figure as its own private reservation.
func TestNoTextBearingControlDroppedWhenSpaceExistsForAll(t *testing.T) {
	labels := []string{"Alpha context selection", "Beta context selection", "Gamma context selection"}
	model := shell.Model{Title: "Application", OpenNavigationLabel: "Open navigation"}
	for i, label := range labels {
		model.TopBar = append(model.TopBar, shell.Control{
			ID: fmt.Sprintf("ctl%d", i), Label: label, CompactLabel: label, Icon: shellIcon(t), Enabled: true,
		})
	}
	w, err := shell.NewWidget(model)
	if err != nil {
		t.Fatal(err)
	}
	th := shellTheme()
	width := 1400
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return w.Layout(gtx, th, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
	}, guitest.Size(width, 700))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	var previous *image.Rectangle
	for _, label := range labels {
		node, err := d.Find(buttonNamed(label))
		if err != nil {
			t.Fatalf("%q was dropped although room existed for all three controls: %v", label, err)
		}
		if node.Desc.Bounds.Dx() < 48 || node.Desc.Bounds.Dy() < 48 {
			t.Fatalf("%q below 48dp: %v", label, node.Desc.Bounds)
		}
		if previous != nil && previous.Overlaps(node.Desc.Bounds) {
			t.Fatalf("%q overlaps the previous control: %v %v", label, *previous, node.Desc.Bounds)
		}
		bounds := node.Desc.Bounds
		previous = &bounds
	}
}
