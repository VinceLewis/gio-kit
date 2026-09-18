package guitest_test

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/guitest"
)

// TestCoverageDetectsOverlappingPaintOrder exercises item 1 of
// gio-json-test-upgrade-plan.md: Coverage must reflect real occlusion, not a
// hardcoded "unknown". Bottom and Top fully overlap inside a layout.Stack
// (Top is painted later, per Gio's document order, so it sits on top of
// Bottom); Clear sits outside the stack and never overlaps anything.
func TestCoverageDetectsOverlappingPaintOrder(t *testing.T) {
	var bottom, top, clear widget.Clickable
	th := theme()
	fixed := func(gtx layout.Context) layout.Context {
		gtx.Constraints = layout.Exact(image.Pt(200, 60))
		return gtx
	}
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{}.Layout(gtx,
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return material.Button(th, &bottom, "Bottom").Layout(fixed(gtx))
					}),
					layout.Expanded(func(gtx layout.Context) layout.Dimensions {
						return material.Button(th, &top, "Top").Layout(fixed(gtx))
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(th, &clear, "Clear").Layout(fixed(gtx))
			}),
		)
	}, guitest.Size(320, 480))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })

	dump, err := d.Capture(guitest.DumpOptions{})
	if err != nil {
		t.Fatal(err)
	}
	byLabel := map[string]guitest.FrameNode{}
	for _, n := range dump.Nodes {
		byLabel[n.Label] = n
	}
	bottomNode, ok := byLabel["Bottom"]
	if !ok {
		t.Fatal("missing Bottom node")
	}
	// material.Button's label sub-node bounds can extend past its own
	// clickable box (label measurement isn't clip-aware in this synthetic
	// layout), so the overlap with Top's box lands as "covered" or
	// "partially_covered" depending on exact text metrics; either result
	// proves occlusion was detected, which is what this test checks for.
	if bottomNode.Coverage == "uncovered" {
		t.Fatalf("Bottom coverage = %q, want covered or partially_covered (Top overlaps it)", bottomNode.Coverage)
	}
	topNode, ok := byLabel["Top"]
	if !ok {
		t.Fatal("missing Top node")
	}
	if topNode.Coverage != "uncovered" {
		t.Fatalf("Top coverage = %q, want uncovered (nothing paints over it)", topNode.Coverage)
	}
	clearNode, ok := byLabel["Clear"]
	if !ok {
		t.Fatal("missing Clear node")
	}
	if clearNode.Coverage != "uncovered" {
		t.Fatalf("Clear coverage = %q, want uncovered (does not overlap Bottom/Top)", clearNode.Coverage)
	}
}
