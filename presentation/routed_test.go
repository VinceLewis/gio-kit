package presentation_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"
	"unicode"

	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/presentation"
)

func presentationTheme() *material.Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	return th
}

// N7 and composed-control acceptance: real Bool/Clickable input survives
// repeated viewport changes; complete labelled switch targets never overlap.
func TestRoutedSwitchesWrapAndRetainSelectionAcrossResize(t *testing.T) {
	page := presentation.Page{Sections: []presentation.Section{{
		Heading: "Filters",
		Controls: []presentation.Control{
			{ID: "active", Kind: "toggle", Label: "Active", Icon: "check", Enabled: true},
			{ID: "reserved", Kind: "toggle", Label: "Reserved", Icon: "calendar", Enabled: true},
			{ID: "unassigned", Kind: "toggle", Label: "Unassigned", Icon: "users", Enabled: true},
			{ID: "deferred", Kind: "toggle", Label: "Deferred", Value: "true", DisabledReason: "Requires permission"},
		},
		Actions: []presentation.Action{{ID: "create", Label: "Create", Icon: "add", Enabled: true}},
	}}}
	w := presentation.NewWidget(page)
	var events []presentation.Event
	w.OnEvent = func(event presentation.Event) { events = append(events, event) }
	th := presentationTheme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) }, guitest.Size(320, 640))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	selected := false
	for _, viewport := range []image.Point{{320, 640}, {720, 280}, {440, 500}, {240, 420}, {320, 640}} {
		if err := d.Resize(viewport.X, viewport.Y, unit.Metric{PxPerDp: 1, PxPerSp: 1}); err != nil {
			t.Fatal(err)
		}
		var bounds []image.Rectangle
		for _, label := range []string{"Active", "Reserved", "Unassigned", "Deferred"} {
			node, err := d.Find(guitest.All(guitest.Role(semantic.Switch), guitest.Name(label)))
			if err != nil {
				t.Fatalf("%s switch at %v: %v; nodes: %+v", label, viewport, err, d.Nodes())
			}
			if node.Desc.Bounds.Dy() < 48 || node.Desc.Bounds.Dx() < 48 || !node.Desc.Bounds.In(image.Rectangle{Max: viewport}) {
				t.Fatalf("%s switch has clipped or undersized target at %v: %v", label, viewport, node.Desc.Bounds)
			}
			for _, previous := range bounds {
				if !node.Desc.Bounds.Intersect(previous).Empty() {
					t.Fatalf("switch targets overlap at %v", viewport)
				}
			}
			bounds = append(bounds, node.Desc.Bounds)
		}
		selected = !selected
		for _, control := range page.Sections[0].Controls[:3] {
			target := guitest.All(guitest.Role(semantic.Switch), guitest.Name(control.Label))
			before := len(events)
			if err := d.Tap(target); err != nil {
				t.Fatal(err)
			}
			node, err := d.Find(guitest.All(target, guitest.Selected(selected), guitest.Enabled(true)))
			if err != nil || node.Desc.Gestures&input.ClickGesture == 0 {
				t.Fatalf("switch selection/click semantics after resize: %v", err)
			}
			if len(events) != before+1 || events[before].ID != control.ID || events[before].Value != map[bool]string{true: "true", false: "false"}[selected] {
				t.Fatalf("switch did not emit exactly one identified change: %#v", events[before:])
			}
		}
		before := len(events)
		disabled := guitest.All(guitest.Role(semantic.Switch), guitest.Name("Deferred"), guitest.Selected(true), guitest.Enabled(false))
		if _, err := d.Find(disabled); err != nil {
			t.Fatal(err)
		}
		if err := d.Tap(disabled); !errors.Is(err, guitest.ErrNotInteractable) {
			t.Fatalf("disabled switch accepted input: %v", err)
		}
		if len(events) != before {
			t.Fatal("disabled switch emitted an event")
		}
		if err := d.Tap(guitest.All(guitest.Role(semantic.Button), guitest.Name("Create"))); err != nil {
			t.Fatal(err)
		}
		if events[len(events)-1].ID != "create" || events[len(events)-1].Kind != presentation.EventAction {
			t.Fatal("compact primary action did not route")
		}
		button, err := d.Find(guitest.All(guitest.Role(semantic.Button), guitest.Name("Create")))
		if err != nil || button.Desc.Bounds.Dy() != 48 || button.Desc.Bounds.Dx() > 130 {
			t.Fatalf("primary action lost compact accessible bounds: %v; %v", button.Desc.Bounds, err)
		}
	}
	w.SetPage(presentation.Page{Sections: []presentation.Section{{Controls: []presentation.Control{{ID: "active", Kind: "toggle", Label: "Active", Value: "true", Enabled: true}}}}})
	if err := d.Frame(); err != nil {
		t.Fatal(err)
	}
	w.SetPage(page)
	if err := d.Frame(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Find(guitest.All(guitest.Role(semantic.Switch), guitest.Name("Active"), guitest.Selected(false))); err != nil {
		t.Fatalf("authoritative page refresh did not reset existing switch: %v", err)
	}
}

func TestRoutedRowsPreserveLongTextStatusAndActionSemantics(t *testing.T) {
	title := strings.Repeat("A descriptive record title ", 7)
	summary := strings.Repeat("A detailed location and supporting context ", 8)
	for _, viewport := range []image.Point{{320, 640}, {720, 280}, {240, 420}} {
		w := presentation.NewWidget(presentation.Page{Sections: []presentation.Section{{Lists: []presentation.List{{
			ID: "records", Heading: "Records",
			Rows: []presentation.Row{{
				ID: "record-one", AccessibleLabel: "Record details", Status: "ready", StatusLabel: "Ready", StatusAccessibleLabel: "Ready for review", StatusIcon: "check", ColorToken: "positive",
				Fragments: []presentation.Fragment{
					{Kind: "icon", Icon: "calendar", AccessibleLabel: "Scheduled record"},
					{Kind: "field", Text: title, Style: "bold"},
					{Kind: "field", Text: summary},
				},
				Actions: []presentation.Action{{ID: "inspect", Label: "Inspect", Icon: "list", Placement: "inline", Enabled: true}},
			}},
		}}}}})
		var event presentation.Event
		w.OnEvent = func(e presentation.Event) { event = e }
		resolved := 0
		w.ResolveColor = func(token string) color.NRGBA {
			if token != "positive" {
				t.Fatalf("unexpected token %q", token)
			}
			resolved++
			return color.NRGBA{G: 160, A: 255}
		}
		th := presentationTheme()
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) }, guitest.Size(viewport.X, viewport.Y))
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range []string{title, summary, "Ready", "Scheduled record"} {
			node, err := d.Find(guitest.Label(value))
			if err != nil {
				t.Fatal(err)
			}
			if node.Desc.Bounds.Min.X < 0 || node.Desc.Bounds.Max.X > viewport.X {
				t.Fatalf("long value exceeds viewport width: %v", node.Desc.Bounds)
			}
		}
		if _, err := d.Find(guitest.Description("Ready for review")); err != nil {
			t.Fatal(err)
		}
		if err := d.Tap(guitest.Within(guitest.All(guitest.Role(semantic.Button), guitest.Name("Inspect")), guitest.Label("Record details"))); err != nil {
			t.Fatal(err)
		}
		if event.RowID != "record-one" || event.ID != "inspect" {
			t.Fatal("wrapped row action lost its identity")
		}
		before := resolved
		th.Bg, th.Fg = color.NRGBA{R: 15, G: 23, B: 42, A: 255}, color.NRGBA{R: 240, G: 245, B: 250, A: 255}
		if err := d.Frame(); err != nil || resolved <= before {
			t.Fatalf("theme change did not resolve current colour token: %v", err)
		}
		var dump bytes.Buffer
		if err := d.DumpJSON(&dump, guitest.DumpOptions{}); err != nil {
			t.Fatal(err)
		}
		if !json.Valid(dump.Bytes()) || dump.Len() > 64*1024 {
			t.Fatal("invalid or unbounded semantic diagnostic")
		}
		if strings.Contains(dump.String(), "[calendar]") {
			t.Fatal("icon name rendered as placeholder text")
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSemanticIconVocabularyAndUnknownFallback(t *testing.T) {
	for _, name := range []string{"calendar", "check", "close", "dot", "home", "list", "log-out", "logout", "menu", "mic", "microphone", "music", "sync", "users", "x"} {
		if presentation.SemanticIcon(name) == nil {
			t.Errorf("declared semantic icon %q is unavailable", name)
		}
	}
	if presentation.SemanticIcon("unknown-future-icon") != nil {
		t.Fatal("unknown icon should defer to the text label")
	}
}

func compactToggleControlsPage() presentation.Page {
	return presentation.Page{Sections: []presentation.Section{{
		Controls: []presentation.Control{
			{ID: "one", Kind: "toggle", Label: "One", Icon: "check", Enabled: true},
			{ID: "two", Kind: "toggle", Label: "Two", Icon: "calendar", Enabled: true},
			{ID: "three", Kind: "toggle", Label: "Three", Icon: "users", Enabled: true},
		},
	}}}
}

// HUF-04 acceptance: at both a 412dp and a 360dp portrait width, three
// compact toggles occupy one visual row, stay inside the viewport, do not
// overlap, and each exposes a target of at least 48dp.
func TestCompactControlsOccupyOneRowAtPhoneWidths(t *testing.T) {
	th := presentationTheme()
	for _, width := range []int{412, 360} {
		w := presentation.NewWidget(compactToggleControlsPage())
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) }, guitest.Size(width, 800))
		if err != nil {
			t.Fatal(err)
		}
		var bounds []image.Rectangle
		var tops []int
		for _, label := range []string{"One", "Two", "Three"} {
			node, err := d.Find(guitest.All(guitest.Role(semantic.Switch), guitest.Name(label)))
			if err != nil {
				t.Fatalf("%s at width %d: %v", label, width, err)
			}
			b := node.Desc.Bounds
			if b.Dx() < 48 || b.Dy() < 48 {
				t.Fatalf("%s target under 48dp at width %d: %v", label, width, b)
			}
			if !b.In(image.Rectangle{Max: image.Pt(width, 800)}) {
				t.Fatalf("%s target outside the viewport at width %d: %v", label, width, b)
			}
			for _, previous := range bounds {
				if !b.Intersect(previous).Empty() {
					t.Fatalf("compact toggles overlap at width %d", width)
				}
			}
			bounds = append(bounds, b)
			tops = append(tops, b.Min.Y)
		}
		if tops[0] != tops[1] || tops[1] != tops[2] {
			t.Fatalf("three compact toggles did not share one visual row at width %d: tops=%v", width, tops)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// HUF-04 acceptance: when the equal-column arrangement no longer fits — a
// narrower width, or an enlarged font scale — the toggles wrap between
// complete controls: each remains non-overlapping and at least 48dp, but they
// no longer share one row.
func TestCompactControlsWrapBetweenCompleteControlsWhenTooNarrowOrFontEnlarged(t *testing.T) {
	th := presentationTheme()
	cases := []struct {
		name   string
		width  int
		metric unit.Metric
	}{
		{"narrow width", 180, unit.Metric{PxPerDp: 1, PxPerSp: 1}},
		{"enlarged font", 412, unit.Metric{PxPerDp: 1, PxPerSp: 2.6}},
	}
	for _, c := range cases {
		w := presentation.NewWidget(compactToggleControlsPage())
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) }, guitest.Size(c.width, 900), guitest.Metrics(c.metric))
		if err != nil {
			t.Fatal(err)
		}
		var bounds []image.Rectangle
		var tops []int
		for _, label := range []string{"One", "Two", "Three"} {
			node, err := d.Find(guitest.All(guitest.Role(semantic.Switch), guitest.Name(label)))
			if err != nil {
				t.Fatalf("%s (%s): %v", label, c.name, err)
			}
			b := node.Desc.Bounds
			if b.Dx() < 48 || b.Dy() < 48 {
				t.Fatalf("%s (%s) target under 48dp: %v", label, c.name, b)
			}
			for _, previous := range bounds {
				if !b.Intersect(previous).Empty() {
					t.Fatalf("controls overlap (%s): %v", c.name, b)
				}
			}
			bounds = append(bounds, b)
			tops = append(tops, b.Min.Y)
		}
		if tops[0] == tops[1] && tops[1] == tops[2] {
			t.Fatalf("controls did not wrap (%s): tops=%v", c.name, tops)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// Requirement 9: routed input still works in both list styles — tapping a
// row action and toggling a control still deliver the right Event.
func TestRoutedInputWorksInBothListStyles(t *testing.T) {
	for _, style := range []string{presentation.ListStyleDefault, presentation.ListStyleCompactFeed} {
		page := presentation.Page{Sections: []presentation.Section{{
			Controls: []presentation.Control{{ID: "flag", Kind: "toggle", Label: "Flag", Enabled: true}},
			Lists: []presentation.List{{ID: "list", Style: style, Rows: []presentation.Row{{
				ID: "row-1", StatusLabel: "Ready", StatusIcon: "check",
				Fragments: []presentation.Fragment{{Kind: "field", Text: "Title", Style: "bold"}},
				Actions:   []presentation.Action{{ID: "open", Label: "Open", Enabled: true}},
			}}}},
		}}}
		w := presentation.NewWidget(page)
		var events []presentation.Event
		w.OnEvent = func(e presentation.Event) { events = append(events, e) }
		th := presentationTheme()
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) }, guitest.Size(360, 640))
		if err != nil {
			t.Fatalf("%s: %v", style, err)
		}
		if err := d.Tap(guitest.All(guitest.Role(semantic.Switch), guitest.Name("Flag"))); err != nil {
			t.Fatalf("%s: toggle tap: %v", style, err)
		}
		if len(events) != 1 || events[0].ID != "flag" || events[0].Kind != presentation.EventControl {
			t.Fatalf("%s: toggle event = %#v", style, events)
		}
		if err := d.Tap(guitest.All(guitest.Role(semantic.Button), guitest.Name("Open"))); err != nil {
			t.Fatalf("%s: action tap: %v", style, err)
		}
		if len(events) != 2 || events[1].ID != "open" || events[1].RowID != "row-1" || events[1].Kind != presentation.EventAction {
			t.Fatalf("%s: action event = %#v", style, events)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// Requirement 6: an unknown icon name falls back to meaningful text, not a
// missing glyph, in the compactFeed rendering too.
func TestFeedRowUnknownIconFallsBackToAccessibleText(t *testing.T) {
	page := presentation.Page{Sections: []presentation.Section{{Lists: []presentation.List{{
		ID: "list", Style: presentation.ListStyleCompactFeed,
		Rows: []presentation.Row{{
			ID: "row-1",
			Fragments: []presentation.Fragment{
				{Kind: "icon", Icon: "not-a-real-icon", AccessibleLabel: "Reminder"},
				{Kind: "field", Text: "Weekly Sync", Style: "bold"},
			},
		}},
	}}}}}
	w := presentation.NewWidget(page)
	th := presentationTheme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) }, guitest.Size(360, 640))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Find(guitest.Label("Reminder")); err != nil {
		t.Fatalf("unknown icon did not fall back to its accessible text: %v; nodes: %+v", err, d.Nodes())
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

// Acceptance criterion 5, semantic side: inside a feed row, the ordered
// rich-text run presents as one accessible unit (the row group), not one
// semantic node per fragment. No node anywhere in the rendered page may carry
// a non-empty label with no letter or digit — the same global check the
// consumer's routed suite applies — covering both a separator that
// legitimately sits between two present values (the reported defect) and the
// pre-existing blank-value case. A paired positive assertion confirms the row
// group itself still announces every summary value and the status, so the
// fix is a suppression of duplicate nodes, not of content.
func TestFeedRowFragmentsPresentAsOneAccessibleUnit(t *testing.T) {
	rows := []presentation.Row{
		{
			ID: "between-present-values",
			Fragments: []presentation.Fragment{
				{Kind: "field", Text: "Weekly Sync", Style: "bold"},
				{Kind: "text", Text: " - ", Style: "muted"},
				{Kind: "field", Text: "Main Hall", Style: "muted"},
			},
			StatusLabel: "Confirmed", StatusIcon: "check",
		},
		{
			ID: "blank-middle-and-trailing",
			Fragments: []presentation.Fragment{
				{Kind: "field", Text: "Weekly Sync", Style: "bold"},
				{Kind: "text", Text: " - ", Style: "muted"},
				{Kind: "field", Text: "   ", Style: "muted"},
				{Kind: "text", Text: " - ", Style: "muted"},
				{Kind: "field", Text: "Main Hall", Style: "muted"},
				{Kind: "text", Text: " - ", Style: "muted"},
				{Kind: "field", Text: ""},
			},
		},
	}
	page := presentation.Page{Sections: []presentation.Section{{Lists: []presentation.List{{
		ID: "list", Style: presentation.ListStyleCompactFeed, Rows: rows,
	}}}}}
	w := presentation.NewWidget(page)
	th := presentationTheme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) }, guitest.Size(360, 900))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	// Positive: the row group still announces every summary value and the
	// status — the fix must not have suppressed everything.
	group, err := d.Find(func(n guitest.Node) bool {
		return containsAll(n.Desc.Label, "Weekly Sync", "Main Hall", "Confirmed")
	})
	if err != nil {
		t.Fatalf("row group no longer announces its summary values and status: %v; nodes: %+v", err, d.Nodes())
	}
	if group.Desc.Label == "" {
		t.Fatal("row group label is empty")
	}

	// Negative: no node anywhere — including the reported separator-between-
	// present-values case and the pre-existing blank-value case — carries a
	// non-empty label with no letter or digit, and none starts or ends on an
	// orphan separator.
	for _, node := range d.Nodes() {
		label := strings.TrimSpace(node.Desc.Label)
		if label == "" {
			continue
		}
		if !containsLetterOrDigit(label) {
			t.Fatalf("semantic node has a punctuation/whitespace-only label: %q", label)
		}
		if isSeparatorOnly(firstSeparatedToken(label, true)) || isSeparatorOnly(firstSeparatedToken(label, false)) {
			t.Fatalf("semantic node starts or ends on an orphan separator: %q", label)
		}
	}
}

func containsAll(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			return false
		}
	}
	return true
}

func containsLetterOrDigit(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// firstSeparatedToken returns the first (leading=true) or last (leading=false)
// whitespace-separated token of s.
func firstSeparatedToken(s string, leading bool) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	if leading {
		return fields[0]
	}
	return fields[len(fields)-1]
}

// isSeparatorOnly reports whether token is non-empty and contains no letter
// or digit — the same structural definition of a separator used by
// filterFeedFragments, applied here to a rendered/semantic token.
func isSeparatorOnly(token string) bool {
	return token != "" && !containsLetterOrDigit(token)
}
