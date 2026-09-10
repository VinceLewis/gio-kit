package grid

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/accessibility"
)

func (w *Widget) rowName(row Row, columns []Column) string {
	id := w.OpenColumn
	if id == "" && len(columns) > 0 {
		id = columns[0].ID
	}
	if name := row.Cells[id]; name != "" {
		return name
	}
	return row.ID
}

func selectionControl(gtx layout.Context, theme *material.Theme, check *widget.Bool, label string) layout.Dimensions {
	return (accessibility.Group{Label: label, Selected: check.Value}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return fixedCell(gtx, selectionColumnWidth, material.CheckBox(theme, check, "").Layout)
	})
}
