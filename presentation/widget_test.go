package presentation

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestComposedWidgetLaysOutAtNarrowAndWideWidths(t *testing.T) {
	page := Page{Sections: []Section{{Heading: "Section", Controls: []Control{{ID: "toggle", Kind: "toggle", Label: "Toggle", Value: "true", Enabled: true}}, Lists: []List{{ID: "list", Rows: []Row{{ID: "row", Fragments: []Fragment{{Kind: "field", Text: "A useful row", Style: "bold"}}, Actions: []Action{{ID: "open", Label: "Open", Enabled: true}}}}}}, Calendars: []Calendar{{ID: "calendar", Month: "September 2026", Days: []CalendarDay{{Date: "2026-09-01", Label: "Tue 1 Sep"}}}}}}}
	for _, width := range []int{320, 900} {
		var operations op.Ops
		gtx := layout.Context{Ops: &operations, Constraints: layout.Exact(image.Pt(width, 720)), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Now: time.Now()}
		widget := NewWidget(page)
		dimensions := widget.Layout(gtx, material.NewTheme())
		if dimensions.Size.X > width || dimensions.Size.Y > 720 {
			t.Fatalf("%dpx layout = %v", width, dimensions.Size)
		}
	}
}
