package guitest_test

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// TestTermuxRealWidgetInput is the step-1 feasibility proof. It deliberately
// uses public Gio APIs and real widgets, without a window, GPU, or callbacks
// that bypass input routing. It also pins fonts instead of reading device fonts.
func TestTermuxRealWidgetInput(t *testing.T) {
	var router input.Router
	var operations op.Ops
	var button widget.Clickable
	var editor widget.Editor
	editor.SingleLine = true
	theme := material.NewTheme()
	theme.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	clicks := 0
	now := time.Unix(0, 0)
	frame := func() {
		operations.Reset()
		gtx := layout.Context{
			Ops: &operations, Source: router.Source(), Now: now,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(420, 820)),
		}
		viewport := clip.Rect{Max: gtx.Constraints.Max}.Push(&operations)
		for button.Clicked(gtx) {
			clicks++
		}
		layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.Button(theme, &button, "Continue").Layout),
			layout.Rigid(material.Editor(theme, &editor, "Description").Layout),
		)
		viewport.Pop()
		router.Frame(&operations)
	}
	frame()
	findRole := func(role semantic.ClassOp) input.SemanticNode {
		t.Helper()
		for _, node := range router.AppendSemantics(nil) {
			if node.Desc.Class == role {
				return node
			}
		}
		t.Fatalf("missing semantic role %v", role)
		return input.SemanticNode{}
	}
	tap := func(node input.SemanticNode) {
		t.Helper()
		bounds := node.Desc.Bounds.Intersect(image.Rect(0, 0, 420, 820))
		if bounds.Empty() {
			t.Fatal("widget has no targetable bounds")
		}
		p := bounds.Min.Add(bounds.Size().Div(2))
		e := pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 1,
			Position: f32.Pt(float32(p.X), float32(p.Y)), Time: now.Sub(time.Unix(0, 0))}
		router.Queue(e)
		frame()
		now = now.Add(50 * time.Millisecond)
		e.Kind, e.Time = pointer.Release, now.Sub(time.Unix(0, 0))
		router.Queue(e)
		frame()
		frame() // Consume focus/commands deferred by the router.
	}
	tap(findRole(semantic.Button))
	if clicks != 1 {
		t.Fatalf("pointer action produced %d clicks, want 1", clicks)
	}
	tap(findRole(semantic.Editor))
	if !router.Source().Focused(&editor) {
		t.Fatal("pointer input did not focus the editor")
	}
	router.Queue(key.EditEvent{Range: router.EditorState().Selection.Range, Text: "Touch → text"})
	frame()
	frame()
	if got := editor.Text(); got != "Touch → text" {
		t.Fatalf("edit input produced %q", got)
	}
}
