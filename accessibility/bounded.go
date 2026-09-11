package accessibility

import (
	"image"

	"gioui.org/layout"
)

const unboundedLayoutExtent = 1_000_000

// BoundedText lays out a stateless text widget once with an explicit finite
// maximum. It clears the inherited height minimum so text retains its
// intrinsic height while preserving an assigned width (for example, from a
// Flexed child) and can wrap within max. The surrounding layout remains
// responsible for any minimum touch target.
//
// Both max coordinates must be non-negative and less than Gio's internal
// one-million-pixel unbounded layout extent. This
// helper neither truncates text nor clips recorded operations. Public Gio APIs
// can bound the layout context, but cannot prove how a platform accessibility
// bridge clips descendant semantics; callers must keep the full accessible
// name on the enclosing control and verify platform behavior separately.
func BoundedText(gtx layout.Context, max image.Point, text layout.Widget) layout.Dimensions {
	if max.X < 0 || max.Y < 0 || max.X >= unboundedLayoutExtent || max.Y >= unboundedLayoutExtent {
		panic("accessibility: BoundedText requires finite non-negative constraints")
	}
	if text == nil {
		return layout.Dimensions{}
	}
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, max.X)
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, max.Y)
	gtx.Constraints.Min.X = min(gtx.Constraints.Min.X, gtx.Constraints.Max.X)
	gtx.Constraints.Min.Y = 0
	return text(gtx)
}
