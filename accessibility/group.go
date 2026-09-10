// Package accessibility associates meaningful names and state with composed
// Gio controls. It adds no input handlers and contains no test identifiers.
package accessibility

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// Group scopes semantics to the measured bounds of content. Use it to name
// controls whose visible caption is otherwise a sibling of their input node.
// Description is for human-readable help or validation, never machine IDs.
type Group struct {
	Label, Description string
	Disabled, Selected bool
}

func (g Group) Layout(gtx layout.Context, content layout.Widget) layout.Dimensions {
	record := op.Record(gtx.Ops)
	dims := content(gtx)
	call := record.Stop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	semantic.LabelOp(g.Label).Add(gtx.Ops)
	semantic.DescriptionOp(g.Description).Add(gtx.Ops)
	semantic.EnabledOp(gtx.Enabled() && !g.Disabled).Add(gtx.Ops)
	semantic.SelectedOp(g.Selected).Add(gtx.Ops)
	call.Add(gtx.Ops)
	return dims
}
