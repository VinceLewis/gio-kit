package grid

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// layoutRow uses separate sibling hit targets for selection and key-cell opening.
func (w *Widget) layoutRow(gtx layout.Context, theme *material.Theme, row Row, columns []Column, selected bool, index int) layout.Dimensions {
	rowClick := w.row(row.ID)
	bg := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	if index%2 == 1 {
		bg = color.NRGBA{R: 247, G: 249, B: 253, A: 255}
	}
	if selected {
		bg = color.NRGBA{R: 221, G: 235, B: 255, A: 255}
	}
	openColumn := w.OpenColumn
	if openColumn == "" && len(columns) > 0 {
		openColumn = columns[0].ID
	}
	dims := surface(gtx, bg, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(columns)+1)
		if w.EnableSelection {
			check := w.rowCheck(row.ID)
			check.Value = selected
			if check.Update(gtx) {
				w.Controller.ToggleSelection(row.ID)
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedCell(gtx, unit.Dp(80), material.CheckBox(theme, check, "").Layout)
			}))
		}
		for _, column := range columns {
			column := column
			children = append(children, columnChild(column, func(gtx layout.Context) layout.Dimensions {
				cell := func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(theme, row.Cells[column.ID])
					label.MaxLines = 2
					if column.ID == openColumn {
						label.Color = color.NRGBA{R: 39, G: 92, B: 225, A: 255}
					}
					switch column.Align {
					case AlignMiddle:
						label.Alignment = text.Middle
					case AlignEnd:
						label.Alignment = text.End
					}
					return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12), Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, label.Layout)
				}
				if column.ID == openColumn {
					return rowClick.Layout(gtx, cell)
				}
				return cell(gtx)
			}))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
	for _, press := range rowClick.History() {
		if !press.Cancelled && !press.End.IsZero() && press.End.After(w.rowOpens[row.ID]) {
			w.rowOpens[row.ID] = press.End
			if w.OnRow != nil {
				w.OnRow(row)
			}
		}
	}
	return dims
}
