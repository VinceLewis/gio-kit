package guitest_test

import (
	"image"
	"testing"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/guitest"
)

func TestPlatformViewportMatrixAndKeyboardContraction(t *testing.T) {
	var got image.Point
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		got = gtx.Constraints.Max
		return layout.Dimensions{Size: got}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	metric := unit.Metric{PxPerDp: 2, PxPerSp: 2}
	for _, tc := range []struct {
		name string
		size image.Point
	}{
		{"compact_phone", image.Pt(360, 640)},
		{"tall_phone", image.Pt(412, 915)},
		{"landscape", image.Pt(915, 412)},
		{"tablet", image.Pt(1280, 800)},
		{"narrow_split_screen", image.Pt(280, 800)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := d.Resize(tc.size.X, tc.size.Y, metric); err != nil {
				t.Fatal(err)
			}
			if got != tc.size {
				t.Fatalf("constraints %v, want %v", got, tc.size)
			}
		})
	}

	phone := image.Pt(412, 915)
	contracted := image.Pt(412, 515)
	for _, size := range []image.Point{phone, contracted, phone} {
		if err := d.Resize(size.X, size.Y, metric); err != nil {
			t.Fatal(err)
		}
		if got != size {
			t.Fatalf("keyboard-like resize %v, want %v", got, size)
		}
	}
}

func TestPlatformFocusSelectionCompositionAndImmediateBack(t *testing.T) {
	var editor widget.Editor
	var backTag struct{}
	var focused bool
	backs := 0
	th := theme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		focused = gtx.Focused(&editor)
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
		return material.Editor(th, &editor, "Input").Layout(gtx)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	target := guitest.Role(semantic.Editor)
	if err := d.Focus(target); err != nil {
		t.Fatal(err)
	}
	if !focused {
		t.Fatal("editor did not acquire focus")
	}
	if err := d.Edit(key.Range{}, "ab😀cd"); err != nil {
		t.Fatal(err)
	}
	selection := key.Range{Start: 2, End: 3}
	if err := d.SetSelection(selection); err != nil {
		t.Fatal(err)
	}
	if start, end := editor.Selection(); start != 2 || end != 3 {
		t.Fatalf("selection = %d:%d", start, end)
	}
	if err := d.SetComposition(selection); err != nil {
		t.Fatal(err)
	}
	if err := d.Edit(selection, "漢字"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetComposition(key.Range{Start: -1, End: -1}); err != nil {
		t.Fatal(err)
	}
	if got := editor.Text(); got != "ab漢字cd" {
		t.Fatalf("composition-shaped replacement = %q", got)
	}
	if err := d.Back(); err != nil {
		t.Fatal(err)
	}
	if backs != 1 {
		t.Fatalf("immediate Back events = %d", backs)
	}
	if err := d.ClearFocus(); err != nil {
		t.Fatal(err)
	}
	if focused {
		t.Fatal("editor retained focus after explicit platform-like focus loss")
	}
}
