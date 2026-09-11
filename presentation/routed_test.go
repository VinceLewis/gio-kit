package presentation_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

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
