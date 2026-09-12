package form

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestFormActionsStackOnNarrowViewport(t *testing.T) {
	var operations op.Ops
	gtx := layout.Context{Ops: &operations, Constraints: layout.Exact(image.Pt(359, 800)), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1.5}}
	if !stackFormActions(gtx) {
		t.Fatal("very narrow form did not use stacked actions")
	}
	gtx.Constraints = layout.Exact(image.Pt(360, 800))
	if stackFormActions(gtx) {
		t.Fatal("compact form unexpectedly used full-width stacked actions")
	}
	if !compactFormActions(gtx) {
		t.Fatal("compact form did not use intrinsic action buttons")
	}
	gtx.Constraints = layout.Exact(image.Pt(520, 800))
	if compactFormActions(gtx) {
		t.Fatal("wide form unexpectedly used compact actions")
	}
}

func TestFormRendersContentAfterFieldsBeforeActions(t *testing.T) {
	form := testForm(t)
	widget := NewWidget(form)
	called := false
	widget.AfterFields = func(gtx layout.Context, _ *material.Theme) layout.Dimensions {
		called = true
		return layout.Spacer{Height: unit.Dp(24)}.Layout(gtx)
	}
	var operations op.Ops
	gtx := layout.Context{Ops: &operations, Constraints: layout.Exact(image.Pt(412, 800)), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	widget.Layout(gtx, material.NewTheme())
	if !called {
		t.Fatal("content after fields was not laid out")
	}
	if widget.saveCloseIcon == nil || widget.saveCloseExit == nil {
		t.Fatal("save and close does not have both icons")
	}
}
