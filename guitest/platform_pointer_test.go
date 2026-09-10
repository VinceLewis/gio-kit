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
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/guitest"
)

func TestPlatformPointerSharedEdgeArbitration(t *testing.T) {
	var buttons [2]widget.Clickable
	clicks := [2]int{}
	th := theme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		for i := range buttons {
			for buttons[i].Clicked(gtx) {
				clicks[i]++
			}
		}
		fixed := func(i int, label string) layout.FlexChild {
			return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints = layout.Exact(image.Pt(100, 60))
				return material.Button(th, &buttons[i], label).Layout(gtx)
			})
		}
		return layout.Flex{}.Layout(gtx, fixed(0, "Left"), fixed(1, "Right"))
	}, guitest.Size(200, 60))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	for _, tc := range []struct {
		name string
		pos  f32.Point
		want [2]int
	}{
		{"inside_left_edge", f32.Pt(95, 30), [2]int{1, 0}},
		{"inside_right_edge", f32.Pt(105, 30), [2]int{1, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := d.PressAt(tc.pos); err != nil {
				t.Fatal(err)
			}
			if err := d.Advance(20 * time.Millisecond); err != nil {
				t.Fatal(err)
			}
			if err := d.Release(); err != nil {
				t.Fatal(err)
			}
			if clicks != tc.want {
				t.Fatalf("clicks = %v, want %v", clicks, tc.want)
			}
		})
	}
}

func TestPlatformLongPressThresholdBoundary(t *testing.T) {
	const threshold = 500 * time.Millisecond
	var click gesture.Click
	var down time.Time
	longPresses := 0
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
				if gtx.Now.Sub(down) >= threshold {
					longPresses++
				}
			}
		}
		defer clip.Rect(image.Rect(0, 0, 120, 80)).Push(gtx.Ops).Pop()
		semantic.LabelOp("Threshold target").Add(gtx.Ops)
		click.Add(gtx.Ops)
		return layout.Dimensions{Size: image.Pt(120, 80)}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.LongPress(guitest.Label("Threshold target"), threshold-time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if longPresses != 0 {
		t.Fatal("sub-threshold hold was classified as long press")
	}
	if err := d.LongPress(guitest.Label("Threshold target"), threshold); err != nil {
		t.Fatal(err)
	}
	if longPresses != 1 {
		t.Fatalf("threshold hold count = %d", longPresses)
	}
}

func TestPlatformHeldPointerCloseAndLargeListRecovery(t *testing.T) {
	t.Run("held_pointer_close", func(t *testing.T) {
		var button widget.Clickable
		closed := false
		th := theme()
		d, err := guitest.NewApp(func(guitest.Environment) (guitest.Harness, error) {
			return guitest.Harness{
				Layout: func(gtx layout.Context) layout.Dimensions {
					return material.Button(th, &button, "Dispose").Layout(gtx)
				},
				Close: func() error { closed = true; return nil },
			}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Press(guitest.Label("Dispose")); err != nil {
			t.Fatal(err)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
		if !closed {
			t.Fatal("application did not close with held pointer")
		}
		if err := d.Release(); !errors.Is(err, guitest.ErrClosed) {
			t.Fatalf("release after disposal = %v", err)
		}
	})

	t.Run("cancelled_drag_recovers", func(t *testing.T) {
		var list widget.List
		list.Axis = layout.Vertical
		buttons := make(map[int]*widget.Clickable)
		th := theme()
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
			return list.Layout(gtx, 10000, func(gtx layout.Context, i int) layout.Dimensions {
				if buttons[i] == nil {
					buttons[i] = new(widget.Clickable)
				}
				return material.Button(th, buttons[i], fmt.Sprintf("Recovery row %d", i)).Layout(gtx)
			})
		}, guitest.Size(300, 400))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = d.Close() })
		if err := d.Press(guitest.Label("Recovery row 1")); err != nil {
			t.Fatal(err)
		}
		if err := d.Advance(20 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
		if err := d.Move(f32.Pt(150, 0)); err != nil {
			t.Fatal(err)
		}
		if err := d.CancelPointer(); err != nil {
			t.Fatal(err)
		}
		before := list.Position.First
		target := guitest.Label(fmt.Sprintf("Recovery row %d", before+2))
		if err := d.Scroll(target, f32.Pt(0, 700)); err != nil {
			t.Fatal(err)
		}
		if err := d.Settle(testContext(t)); err != nil {
			t.Fatal(err)
		}
		if list.Position.First <= before {
			t.Fatalf("list did not recover after cancellation: %d -> %d", before, list.Position.First)
		}
		if len(d.Nodes()) > 100 {
			t.Fatalf("recovery broke virtualization: %d semantic nodes", len(d.Nodes()))
		}
	})
}
