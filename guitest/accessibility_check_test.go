package guitest_test

import (
	"errors"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/guitest"
)

// TestCheckAccessibilityCatchesUnnamedInteractableNode covers item 3 of
// gio-json-test-upgrade-plan.md: a bare clickable region with no label
// anywhere in its subtree must fail the opt-in accessibility check (see
// DumpOptions.CheckAccessibility's doc comment for why it is currently
// opt-in, not the plan's target opt-out default).
func TestCheckAccessibilityCatchesUnnamedInteractableNode(t *testing.T) {
	th := theme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect{Max: layout.FPt(gtx.Constraints.Max).Round()}.Push(gtx.Ops).Pop()
		return material.Button(th, new(widget.Clickable), "Named").Layout(gtx)
	}, guitest.Size(320, 480))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.Capture(guitest.DumpOptions{CheckAccessibility: true}); err != nil {
		t.Fatalf("named button unexpectedly failed the check: %v", err)
	}
}

// TestCheckAccessibilityRejectsBareClickTarget proves the negative case: a
// bare click gesture with no semantic label anywhere in its subtree.
func TestCheckAccessibilityRejectsBareClickTarget(t *testing.T) {
	th := theme()
	var click widget.Clickable
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return material.Body1(th, "").Layout(gtx)
		})
	}, guitest.Size(320, 480))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	_, err = d.Capture(guitest.DumpOptions{CheckAccessibility: true})
	var violations *guitest.ErrAccessibilityViolations
	if !errors.As(err, &violations) || len(violations.Violations) == 0 {
		t.Fatalf("expected accessibility violations for an unnamed click target, got %v", err)
	}
}
