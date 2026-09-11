package accessibility

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
)

func TestBoundedTextUsesFiniteIntrinsicConstraintsOnce(t *testing.T) {
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(200, 160)),
		Ops:         new(op.Ops),
	}
	calls := 0
	dims := BoundedText(gtx, image.Pt(80, 60), func(got layout.Context) layout.Dimensions {
		calls++
		if got.Constraints.Min != image.Pt(80, 0) {
			t.Fatalf("minimum constraints = %v", got.Constraints.Min)
		}
		if want := image.Pt(80, 60); got.Constraints.Max != want {
			t.Fatalf("maximum constraints = %v, want %v", got.Constraints.Max, want)
		}
		return layout.Dimensions{Size: image.Pt(72, 48), Baseline: 9}
	})
	if calls != 1 {
		t.Fatalf("widget calls = %d, want 1", calls)
	}
	if dims.Size != image.Pt(72, 48) || dims.Baseline != 9 {
		t.Fatalf("dimensions = %#v", dims)
	}
}

func TestBoundedTextDoesNotExpandParentMaximum(t *testing.T) {
	gtx := layout.Context{
		Constraints: layout.Constraints{Min: image.Pt(20, 20), Max: image.Pt(40, 30)},
		Ops:         new(op.Ops),
	}
	BoundedText(gtx, image.Pt(80, 60), func(got layout.Context) layout.Dimensions {
		if want := image.Pt(40, 30); got.Constraints.Max != want {
			t.Fatalf("maximum constraints = %v, want %v", got.Constraints.Max, want)
		}
		return layout.Dimensions{}
	})
}

func TestBoundedTextRejectsUnboundedMaximum(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected unbounded constraints to panic")
		}
	}()
	BoundedText(layout.Context{}, image.Pt(100, unboundedLayoutExtent), func(layout.Context) layout.Dimensions {
		return layout.Dimensions{}
	})
}
