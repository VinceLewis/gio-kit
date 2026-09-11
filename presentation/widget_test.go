package presentation

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
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

func TestSwitchIndicatorGeometryIgnoresTouchTargetMinimum(t *testing.T) {
	for _, density := range []float32{1, 1.5, 2} {
		for _, width := range []unit.Dp{320, 20} {
			for _, selected := range []bool{false, true} {
				var operations op.Ops
				gtx := layout.Context{Ops: &operations, Metric: unit.Metric{PxPerDp: density, PxPerSp: density}}
				gtx.Constraints = layout.Constraints{Min: image.Pt(0, gtx.Dp(48)), Max: image.Pt(gtx.Dp(width), gtx.Dp(100))}
				size, thumb := switchGeometry(gtx, selected)
				if size != image.Pt(gtx.Dp(min(width, 36)), gtx.Dp(22)) {
					t.Fatalf("indicator inherited touch height at density %v: %v", density, size)
				}
				if thumb.Empty() || !thumb.In(image.Rectangle{Max: size}) || thumb.Dx() != thumb.Dy() {
					t.Fatalf("thumb exceeds track at density %v, width %v, selected %v: %v in %v", density, width, selected, thumb, size)
				}
				if dims := switchIndicator(gtx, material.NewTheme(), selected, false); dims.Size != size {
					t.Fatalf("painted indicator differs from bounded geometry: %v", dims.Size)
				}
			}
		}
	}
}

func TestActionAndIconLabelKeepIntrinsicContentWithinTouchTarget(t *testing.T) {
	for _, density := range []float32{1, 1.5, 2} {
		var operations op.Ops
		gtx := layout.Context{Ops: &operations, Metric: unit.Metric{PxPerDp: density, PxPerSp: density}}
		gtx.Constraints = layout.Constraints{Min: image.Pt(0, gtx.Dp(48)), Max: image.Pt(gtx.Dp(320), gtx.Dp(100))}
		w, theme := NewWidget(Page{}), material.NewTheme()
		label := w.iconLabel(gtx, theme, "check", "Create", theme.Fg)
		if label.Size.Y >= gtx.Dp(48) {
			t.Fatalf("icon/label inherited touch-target height at density %v: %v", density, label.Size)
		}
		operations.Reset()
		button := w.actionButton(gtx, theme, new(widget.Clickable), Action{Label: "Create", Icon: "check", Enabled: true})
		if button.Size.Y != gtx.Dp(48) {
			t.Fatalf("action padding inflated 48dp touch target at density %v: %v", density, button.Size)
		}
	}
}
