package shell

import (
	"image"

	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Action is an intrinsically sized primary action with a minimum 48dp target.
// Its label wraps to at most two lines, retaining its full accessible name.
// The caller owns the clickable, placement and selected state; disable input
// with gtx.Disabled(). Selected actions add weight and an underline.
func Action(gtx layout.Context, theme *material.Theme, click *widget.Clickable, label string, selected bool) layout.Dimensions {
	if theme == nil || click == nil {
		return layout.Dimensions{}
	}
	minimum := gtx.Dp(unit.Dp(48))
	gtx.Constraints.Min = image.Pt(min(minimum, gtx.Constraints.Max.X), min(minimum, gtx.Constraints.Max.Y))
	dims := material.ButtonLayout(theme, click).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.SelectedOp(selected).Add(gtx.Ops)
		return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			text := material.Body2(theme, label)
			text.Color, text.MaxLines, text.Truncator = theme.Palette.ContrastFg, 2, "…"
			if selected {
				text.Font.Weight = font.Bold
			}
			return boundedText(gtx, unit.Dp(44), text.Layout)
		})
	})
	if selected {
		selectionMark(gtx, dims, theme.Palette.ContrastFg, false)
	}
	return dims
}
