package grid

import (
	"fmt"
	"image/color"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

const selectionColumnWidth unit.Dp = 56

func tableMinimumWidth(columns []Column, selection bool) unit.Dp {
	var width unit.Dp
	if selection {
		width += selectionColumnWidth
	}
	for _, column := range columns {
		if column.Width > 0 {
			width += column.Width
			continue
		}
		flex := column.Flex
		if flex <= 0 {
			flex = 1
		}
		width += unit.Dp(140 * flex)
	}
	return width
}

func fixedTableColumns(columns []Column) []Column {
	fixed := append([]Column(nil), columns...)
	for index := range fixed {
		if fixed[index].Width > 0 {
			continue
		}
		flex := fixed[index].Flex
		if flex <= 0 {
			flex = 1
		}
		fixed[index].Width = unit.Dp(140 * flex)
		fixed[index].Flex = 0
	}
	return fixed
}

func (w *Widget) layoutCardHeader(gtx layout.Context, theme *material.Theme, snapshot Snapshot, columns []Column) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(columns)+2)
	if w.EnableSelection {
		allSelected := len(snapshot.Rows) > 0
		rowIDs := make([]string, 0, len(snapshot.Rows))
		for _, row := range snapshot.Rows {
			rowIDs = append(rowIDs, row.ID)
			allSelected = allSelected && snapshot.Selection[row.ID]
		}
		w.selectAll.Value = allSelected
		if w.selectAll.Update(gtx) {
			w.Controller.SetRowsSelected(rowIDs, w.selectAll.Value)
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedCell(gtx, selectionColumnWidth, material.CheckBox(theme, &w.selectAll, "").Layout)
		}))
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		label := material.Caption(theme, "Sort")
		label.Color = color.NRGBA{R: 91, G: 105, B: 127, A: 255}
		return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, label.Layout)
	}))
	for _, column := range columns {
		if !column.Sortable {
			continue
		}
		column := column
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			click := w.header(column.ID)
			if click.Clicked(gtx) {
				_ = w.Controller.ToggleSort(column.ID, w.AdditiveSort)
			}
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.Caption(theme, column.Header+sortSuffix(snapshot.Sort, column.ID))
				label.Color = color.NRGBA{R: 33, G: 65, B: 130, A: 255}
				label.Alignment = text.Start
				switch column.Align {
				case AlignMiddle:
					label.Alignment = text.Middle
				case AlignEnd:
					label.Alignment = text.End
				}
				label.MaxLines = 1
				return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(2), Right: unit.Dp(2)}.Layout(gtx, label.Layout)
			})
		}))
	}
	return surface(gtx, color.NRGBA{R: 237, G: 242, B: 251, A: 255}, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

func (w *Widget) layoutCardRows(gtx layout.Context, theme *material.Theme, snapshot Snapshot, columns []Column) layout.Dimensions {
	if snapshot.State == Failed {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(material.Body1(theme, "Could not load records.").Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if w.retry.Clicked(gtx) {
						_ = w.Controller.Retry()
					}
					return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, material.Button(theme, &w.retry, "RETRY").Layout)
				}),
			)
		})
	}
	if snapshot.State == Empty {
		return layout.Center.Layout(gtx, material.Body1(theme, "No records match this filter.").Layout)
	}
	if len(snapshot.Rows) == 0 && snapshot.State == Loading {
		return layout.Center.Layout(gtx, material.Body1(theme, "Loading records…").Layout)
	}

	extra := 0
	if len(snapshot.Rows) > 0 || snapshot.HasMore || snapshot.State == Loading {
		extra = 1
	}
	if w.List.Position.First+w.List.Position.Count >= len(snapshot.Rows)-5 && snapshot.HasMore && snapshot.State != Loading {
		_ = w.Controller.LoadNext()
	}
	return material.List(theme, &w.List).Layout(gtx, len(snapshot.Rows)+extra, func(gtx layout.Context, index int) layout.Dimensions {
		if index >= len(snapshot.Rows) {
			message := "End of results"
			if snapshot.HasMore || snapshot.State == Loading {
				message = "Loading more records…"
			}
			return layout.UniformInset(unit.Dp(14)).Layout(gtx, material.Body2(theme, message).Layout)
		}
		return w.layoutCard(gtx, theme, snapshot.Rows[index], columns, snapshot.Selection[snapshot.Rows[index].ID])
	})
}

func (w *Widget) layoutCard(gtx layout.Context, theme *material.Theme, row Row, columns []Column, selected bool) layout.Dimensions {
	titleID, summaryID := w.cardFieldIDs(columns)
	check := w.rowCheck(row.ID)
	check.Value = selected
	if check.Update(gtx) {
		w.Controller.ToggleSelection(row.ID)
	}
	rowClick := w.row(row.ID)
	if rowClick.Clicked(gtx) && w.OnRow != nil {
		w.OnRow(row)
	}
	bg := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	if selected {
		bg = color.NRGBA{R: 225, G: 237, B: 255, A: 255}
	}
	return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return surface(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !w.EnableSelection {
						return layout.Dimensions{}
					}
					return fixedCell(gtx, selectionColumnWidth, material.CheckBox(theme, check, "").Layout)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return rowClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							items := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := material.Body1(theme, row.Cells[titleID])
								label.Color = color.NRGBA{R: 39, G: 92, B: 225, A: 255}
								return label.Layout(gtx)
							})}
							if summaryID != "" {
								items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									label := material.Body2(theme, row.Cells[summaryID])
									label.MaxLines = 3
									return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(6)}.Layout(gtx, label.Layout)
								}))
							}
							for _, column := range columns {
								if column.ID == titleID || column.ID == summaryID {
									continue
								}
								column := column
								items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									label := material.Caption(theme, fmt.Sprintf("%s: %s", column.Header, row.Cells[column.ID]))
									label.Color = color.NRGBA{R: 91, G: 105, B: 127, A: 255}
									label.MaxLines = 1
									return label.Layout(gtx)
								}))
							}
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.H6(theme, "›")
					label.Color = color.NRGBA{R: 39, G: 92, B: 225, A: 255}
					return layout.Inset{Right: unit.Dp(12)}.Layout(gtx, label.Layout)
				}),
			)
		})
	})
}

func (w *Widget) cardFieldIDs(columns []Column) (title, summary string) {
	title = w.CardTitleColumn
	if title == "" {
		title = w.OpenColumn
	}
	if title == "" && len(columns) > 0 {
		title = columns[0].ID
	}
	summary = w.CardSummaryColumn
	if summary == "" {
		for _, column := range columns {
			if column.ID != title {
				summary = column.ID
				break
			}
		}
	}
	return title, summary
}
