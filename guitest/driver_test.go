package guitest_test

import (
	"context"
	"errors"
	"image"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/guitest"
)

func theme() *material.Theme {
	theme := material.NewTheme()
	theme.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	return theme
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestDriverPointerEditorBackAndResize(t *testing.T) {
	var button widget.Clickable
	var editor widget.Editor
	var backTag struct{}
	clicks, backs := 0, 0
	th := theme()
	var dimensions image.Point
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		dimensions = gtx.Constraints.Max
		for button.Clicked(gtx) {
			clicks++
		}
		event.Op(gtx.Ops, &backTag)
		for {
			e, ok := gtx.Event(key.Filter{Name: key.NameBack})
			if !ok {
				break
			}
			if e, ok := e.(key.Event); ok && e.State == key.Press {
				backs++
			}
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.Button(th, &button, "Save").Layout),
			layout.Rigid(material.Editor(th, &editor, "Description").Layout),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Tap(guitest.Label("Save")); err != nil {
		t.Fatal(err)
	}
	if clicks != 1 {
		t.Fatalf("clicks = %d", clicks)
	}
	if err := d.Type(guitest.Role(semantic.Editor), "città 🌍"); err != nil {
		t.Fatalf("%v; nodes=%+v", err, d.Nodes())
	}
	if editor.Text() != "città 🌍" {
		t.Fatalf("editor = %q", editor.Text())
	}
	if err := d.Back(); err != nil {
		t.Fatal(err)
	}
	if backs != 1 {
		t.Fatalf("back events = %d", backs)
	}
	if err := d.Resize(820, 420, unit.Metric{PxPerDp: 2, PxPerSp: 2}); err != nil {
		t.Fatal(err)
	}
	if dimensions != image.Pt(820, 420) {
		t.Fatalf("viewport = %v", dimensions)
	}
	if err := d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
}

func TestSelectorErrorsDoNotActivateWidgets(t *testing.T) {
	var first, second widget.Clickable
	th := theme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.Button(th, &first, "Duplicate").Layout),
			layout.Rigid(material.Button(th, &second, "Duplicate").Layout))
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Tap(guitest.Label("Duplicate")); !errors.Is(err, guitest.ErrAmbiguous) {
		t.Fatalf("duplicate: %v", err)
	}
	if err := d.Tap(guitest.Label("Missing")); !errors.Is(err, guitest.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if err := d.Type(guitest.Role(semantic.Button), "bad"); !errors.Is(err, guitest.ErrAmbiguous) {
		t.Fatalf("ambiguous edit: %v", err)
	}
}

func TestDisabledAndOutsideViewportTargets(t *testing.T) {
	for _, test := range []struct {
		name     string
		disabled bool
		offset   image.Point
	}{
		{"disabled", true, image.Point{}}, {"offscreen", false, image.Pt(900, 0)},
	} {
		t.Run(test.name, func(t *testing.T) {
			var button widget.Clickable
			th := theme()
			d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
				defer op.Offset(test.offset).Push(gtx.Ops).Pop()
				if test.disabled {
					gtx = gtx.Disabled()
				}
				return material.Button(th, &button, "Blocked").Layout(gtx)
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = d.Close() })
			if err := d.Tap(guitest.Label("Blocked")); !errors.Is(err, guitest.ErrNotInteractable) {
				t.Fatalf("tap: %v", err)
			}
		})
	}
}

func TestVirtualTimeAsyncIdleAndCleanup(t *testing.T) {
	var env guitest.Environment
	done := make(chan struct{})
	results := make(chan error, 1)
	applied, cleaned := false, false
	d, err := guitest.NewApp(func(e guitest.Environment) (guitest.Harness, error) {
		env = e
		go func() {
			defer close(done)
			results <- e.Clock.Sleep(e.Context, time.Second)
			e.Invalidate()
		}()
		return guitest.Harness{
			Layout: func(layout.Context) layout.Dimensions {
				select {
				case err := <-results:
					if err != nil {
						t.Error(err)
					}
					applied = true
				default:
				}
				return layout.Dimensions{}
			},
			Idle:  func() bool { return applied },
			Close: func() error { <-done; cleaned = true; return nil },
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.WaitFor(testContext(t), func() bool { return env.Clock.Pending() == 1 }); err != nil {
		t.Fatal(err)
	}
	if applied {
		t.Fatal("timer completed before virtual time advanced")
	}
	if err := d.Advance(time.Second); err != nil {
		t.Fatal(err)
	}
	if err := d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if !applied {
		t.Fatal("idle accepted before the queued result was applied")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if !cleaned {
		t.Fatal("cleanup was not called")
	}
	env.Invalidate()
	if err := d.Frame(); !errors.Is(err, guitest.ErrClosed) {
		t.Fatalf("closed frame: %v", err)
	}
}

func TestCloseCancelsVirtualSleeperAndIsIdempotent(t *testing.T) {
	done := make(chan error, 1)
	var closes atomic.Int32
	d, err := guitest.NewApp(func(e guitest.Environment) (guitest.Harness, error) {
		go func() { done <- e.Clock.Sleep(e.Context, time.Hour) }()
		return guitest.Harness{
			Layout: func(layout.Context) layout.Dimensions { return layout.Dimensions{} },
			Close: func() error {
				closes.Add(1)
				if err := <-done; !errors.Is(err, context.Canceled) {
					return err
				}
				return nil
			},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if closes.Load() != 1 || d.Clock().Pending() != 0 {
		t.Fatal("close did not clean up exactly once")
	}
}

func TestWaitCancellationFrameLimitAndFutureRedraw(t *testing.T) {
	empty := func(layout.Context) layout.Dimensions { return layout.Dimensions{} }
	d, err := guitest.New(empty)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := d.WaitFor(ctx, func() bool { return false }); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}

	loop, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		gtx.Execute(op.InvalidateCmd{})
		return layout.Dimensions{}
	}, guitest.MaxFrames(4))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = loop.Close() })
	if err := loop.Settle(testContext(t)); !errors.Is(err, guitest.ErrFrameLimit) {
		t.Fatalf("unbounded animation: %v", err)
	}

	future, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Hour)})
		return layout.Dimensions{}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = future.Close() })
	start := future.Clock().Now()
	if err := future.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if !future.Clock().Now().Equal(start) {
		t.Fatal("settle advanced a future-only redraw")
	}
}

func TestFactoryFailureCleansPartialApplication(t *testing.T) {
	want := errors.New("startup failed")
	closed := false
	d, err := guitest.NewApp(func(e guitest.Environment) (guitest.Harness, error) {
		return guitest.Harness{Close: func() error { closed = e.Context.Err() != nil; return nil }}, want
	})
	if d != nil || !errors.Is(err, want) || !closed {
		t.Fatalf("factory cleanup: driver=%v err=%v closed=%t", d, err, closed)
	}
}
