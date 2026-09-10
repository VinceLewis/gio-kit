package guitest_test

import (
	"errors"
	"fmt"
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/accessibility"
	"github.com/VinceLewis/gio-kit/guitest"
)

func TestScopedSelectorsAndIDsFollowFrames(t *testing.T) {
	var buttons [2]widget.Clickable
	th := theme()
	clicked, reversed := -1, false
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		var children []layout.FlexChild
		for _, i := range []int{0, 1} {
			if reversed {
				i = 1 - i
			}
			if buttons[i].Clicked(gtx) {
				clicked = i
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return (accessibility.Group{Label: fmt.Sprintf("Group %d", i)}).Layout(gtx, material.Button(th, &buttons[i], "Save").Layout)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}, guitest.BindID("second.save", guitest.Within(guitest.Label("Save"), guitest.Label("Group 1"))))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Tap(guitest.Label("Save")); !errors.Is(err, guitest.ErrAmbiguous) {
		t.Fatal(err)
	}
	if _, err := d.Find(guitest.Containing(guitest.Label("Group 1"), guitest.Label("Save"))); err != nil {
		t.Fatal(err)
	}
	for _, reverse := range []bool{false, true} {
		reversed = reverse
		if err := d.Resize(640, 400, unit.Metric{PxPerDp: 1, PxPerSp: 1}); err != nil {
			t.Fatal(err)
		}
		if err := d.Tap(guitest.ID("second.save")); err != nil {
			t.Fatal(err)
		}
		if clicked != 1 {
			t.Fatalf("clicked %d", clicked)
		}
	}
	if err := d.Tap(guitest.Nth(guitest.Label("Save"), 1)); err != nil {
		t.Fatal(err)
	}
	if clicked != 0 {
		t.Fatal("occurrence did not follow frame order")
	}
	for _, n := range d.Nodes() {
		if n.Desc.Label == "second.save" || n.Desc.Description == "second.save" {
			t.Fatal("ID leaked into accessibility")
		}
	}
}

func TestPointerTimingCancellationAndDoubleTap(t *testing.T) {
	var click gesture.Click
	var counts []int
	var down time.Time
	var held time.Duration
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		for {
			e, ok := click.Update(gtx.Source)
			if !ok {
				break
			}
			switch e.Kind {
			case gesture.KindPress:
				down = gtx.Now
			case gesture.KindClick:
				counts = append(counts, e.NumClicks)
				held = gtx.Now.Sub(down)
			}
		}
		defer clip.Rect(image.Rect(0, 0, 120, 80)).Push(gtx.Ops).Pop()
		semantic.LabelOp("Touch area").Add(gtx.Ops)
		click.Add(gtx.Ops)
		return layout.Dimensions{Size: image.Pt(120, 80)}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Press(guitest.Label("Touch area")); err != nil {
		t.Fatal(err)
	}
	if err := d.Press(guitest.Label("Touch area")); err == nil {
		t.Fatal("nested press accepted")
	}
	if err := d.Move(f32.Pt(900, 900)); err != nil {
		t.Fatal(err)
	}
	if err := d.CancelPointer(); err != nil {
		t.Fatal(err)
	}
	if len(counts) != 0 {
		t.Fatal("cancel clicked")
	}
	if err := d.LongPress(guitest.Label("Touch area"), 600*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if held != 600*time.Millisecond {
		t.Fatalf("held %s", held)
	}
	if err := d.Advance(time.Second); err != nil {
		t.Fatal(err)
	}
	counts = nil
	if err := d.DoubleTap(guitest.Label("Touch area")); err != nil {
		t.Fatal(err)
	}
	if len(counts) != 2 || counts[0] != 1 || counts[1] != 2 {
		t.Fatalf("click counts %v", counts)
	}
}

func TestScrollAndTouchDragVirtualizedList(t *testing.T) {
	var list widget.List
	list.Axis = layout.Vertical
	th := theme()
	buttons := make(map[int]*widget.Clickable)
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return list.Layout(gtx, 10000, func(gtx layout.Context, i int) layout.Dimensions {
			if buttons[i] == nil {
				buttons[i] = new(widget.Clickable)
			}
			return material.Button(th, buttons[i], fmt.Sprintf("Row %d", i)).Layout(gtx)
		})
	}, guitest.Size(300, 400))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if len(d.Nodes()) > 100 {
		t.Fatal("list was not virtualized")
	}
	if err := d.Scroll(guitest.Label("Row 0"), f32.Pt(0, 500)); err != nil {
		t.Fatal(err)
	}
	if err := d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	before := list.Position.First
	if before == 0 {
		t.Fatal("wheel did not scroll")
	}
	// Start below the first partially clipped row so the touch can move upward.
	target := guitest.Label(fmt.Sprintf("Row %d", before+3))
	if err := d.Drag(target, f32.Pt(0, -140), 300*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if list.Position.First <= before {
		t.Fatalf("touch did not scroll: %d -> %d", before, list.Position.First)
	}
	if len(d.Nodes()) > 100 {
		t.Fatal("scroll broke virtualization")
	}
}
