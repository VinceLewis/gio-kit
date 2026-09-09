package form

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestFormActionsStackOnNarrowViewport(t *testing.T) {
	var operations op.Ops
	gtx := layout.Context{Ops: &operations, Constraints: layout.Exact(image.Pt(519, 800)), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1.5}}
	if !stackFormActions(gtx) {
		t.Fatal("narrow large-text form did not use stacked actions")
	}
	gtx.Constraints = layout.Exact(image.Pt(520, 800))
	if stackFormActions(gtx) {
		t.Fatal("wide form unexpectedly used stacked actions")
	}
}
