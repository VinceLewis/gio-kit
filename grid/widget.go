package grid

import (
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Widget renders a Controller using Gio's virtualized layout.List. Keep one
// Widget instance for the screen lifetime so scroll and click state survive.
type Widget struct {
	Controller   *Controller
	List         widget.List
	AdditiveSort bool
	// EnableSelection displays checkbox controls for bulk workflows.
	EnableSelection bool
	// OpenColumn makes that column's cell the row-open target.
	OpenColumn  string
	OnRow       func(Row)
	OnRowAction func(Row)

	headers   map[string]*widget.Clickable
	rows      map[string]*widget.Clickable
	selects   map[string]*widget.Clickable
	rowChecks map[string]*widget.Bool
	rowOpens  map[string]time.Time
	selectAll widget.Bool
	actions   map[string]*widget.Clickable
	retry     widget.Clickable
	clearSel  widget.Clickable
}

func NewWidget(controller *Controller) *Widget {
	return &Widget{
		Controller:      controller,
		EnableSelection: true,
		List:            widget.List{List: layout.List{Axis: layout.Vertical}},
		headers:         make(map[string]*widget.Clickable),
		rows:            make(map[string]*widget.Clickable),
		selects:         make(map[string]*widget.Clickable),
		rowChecks:       make(map[string]*widget.Bool),
		rowOpens:        make(map[string]time.Time),
		actions:         make(map[string]*widget.Clickable),
	}
}

func (w *Widget) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if w == nil || w.Controller == nil {
		return material.Body1(theme, "Grid controller is not configured").Layout(gtx)
	}
	snapshot := w.Controller.Snapshot()
	visible := visibleColumns(snapshot.Columns)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutHeader(gtx, theme, snapshot, visible) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !w.EnableSelection || len(snapshot.Selection) == 0 {
				return layout.Dimensions{}
			}
			if w.clearSel.Clicked(gtx) {
				w.Controller.ClearSelection()
			}
			button := material.Button(theme, &w.clearSel, fmt.Sprintf("CLEAR %d SELECTED", len(snapshot.Selection)))
			button.Background = color.NRGBA{R: 56, G: 76, B: 112, A: 255}
			return layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, button.Layout)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return w.layoutRows(gtx, theme, snapshot, visible)
		}),
	)
}

func (w *Widget) layoutHeader(gtx layout.Context, theme *material.Theme, snapshot Snapshot, columns []Column) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(columns)+1)
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
			return fixedCell(gtx, unit.Dp(80), material.CheckBox(theme, &w.selectAll, "Select").Layout)
		}))
	}
	for _, column := range columns {
		column := column
		child := func(gtx layout.Context) layout.Dimensions {
			click := w.header(column.ID)
			if click.Clicked(gtx) && column.Sortable {
				_ = w.Controller.ToggleSort(column.ID, w.AdditiveSort)
			}
			label := column.Header + sortSuffix(snapshot.Sort, column.ID)
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				textStyle := material.Caption(theme, label)
				textStyle.Color = color.NRGBA{R: 33, G: 65, B: 130, A: 255}
				textStyle.Alignment = text.Middle
				return layout.UniformInset(unit.Dp(8)).Layout(gtx, textStyle.Layout)
			})
		}
		children = append(children, columnChild(column, child))
	}
	return surface(gtx, color.NRGBA{R: 226, G: 234, B: 249, A: 255}, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

func (w *Widget) layoutRows(gtx layout.Context, theme *material.Theme, snapshot Snapshot, columns []Column) layout.Dimensions {
	if snapshot.State == Failed {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(material.Body1(theme, "Fetch failed: "+snapshot.Err.Error()).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if w.retry.Clicked(gtx) {
						_ = w.Controller.Retry()
					}
					return layout.Inset{Top: unit.Dp(10)}.Layout(gtx, material.Button(theme, &w.retry, "RETRY").Layout)
				}),
			)
		})
	}
	if snapshot.State == Empty {
		return layout.Center.Layout(gtx, material.Body1(theme, "No rows match this query.").Layout)
	}
	if len(snapshot.Rows) == 0 && snapshot.State == Loading {
		return layout.Center.Layout(gtx, material.Body1(theme, "Loading first page…").Layout)
	}

	extra := 0
	if snapshot.HasMore || snapshot.State == Loading {
		extra = 1
	}
	count := len(snapshot.Rows) + extra
	if w.List.Position.First+w.List.Position.Count >= len(snapshot.Rows)-5 && snapshot.HasMore && snapshot.State != Loading {
		_ = w.Controller.LoadNext()
	}
	return w.List.Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
		if index >= len(snapshot.Rows) {
			return layout.UniformInset(unit.Dp(14)).Layout(gtx, material.Body2(theme, "Loading next page…").Layout)
		}
		return w.layoutRow(gtx, theme, snapshot.Rows[index], columns, snapshot.Selection[snapshot.Rows[index].ID], index)
	})
}

func (w *Widget) layoutRowNested(gtx layout.Context, theme *material.Theme, row Row, columns []Column, selected bool, index int) layout.Dimensions {
	selection := w.selection(row.ID)
	selectionClicked := selection.Clicked(gtx)
	if selectionClicked {
		w.Controller.ToggleSelection(row.ID)
	}
	action := w.action(row.ID)
	actionClicked := action.Clicked(gtx)
	if actionClicked && w.OnRowAction != nil {
		w.OnRowAction(row)
	}
	rowClick := w.row(row.ID)
	if rowClick.Clicked(gtx) && !selectionClicked && !actionClicked && w.OnRow != nil {
		w.OnRow(row)
	}
	bg := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	if index%2 == 1 {
		bg = color.NRGBA{R: 247, G: 249, B: 253, A: 255}
	}
	if selected {
		bg = color.NRGBA{R: 221, G: 235, B: 255, A: 255}
	}
	return rowClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return surface(gtx, bg, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				mark := "○"
				if selected {
					mark = "●"
				}
				return selection.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := material.Body1(theme, mark)
					label.Color = color.NRGBA{R: 39, G: 92, B: 225, A: 255}
					label.Alignment = text.Middle
					return fixedCell(gtx, unit.Dp(44), label.Layout)
				})
			})}
			for _, column := range columns {
				column := column
				children = append(children, columnChild(column, func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(theme, row.Cells[column.ID])
					label.MaxLines = 2
					switch column.Align {
					case AlignMiddle:
						label.Alignment = text.Middle
					case AlignEnd:
						label.Alignment = text.End
					}
					return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, label.Layout)
				}))
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return action.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					label := material.H6(theme, "›")
					label.Color = color.NRGBA{R: 39, G: 92, B: 225, A: 255}
					label.Alignment = text.Middle
					return fixedCell(gtx, unit.Dp(48), label.Layout)
				})
			}))
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}

func (w *Widget) header(id string) *widget.Clickable    { return clickable(w.headers, id) }
func (w *Widget) row(id string) *widget.Clickable       { return clickable(w.rows, id) }
func (w *Widget) selection(id string) *widget.Clickable { return clickable(w.selects, id) }
func (w *Widget) action(id string) *widget.Clickable    { return clickable(w.actions, id) }

func (w *Widget) rowCheck(id string) *widget.Bool {
	if item := w.rowChecks[id]; item != nil {
		return item
	}
	item := new(widget.Bool)
	w.rowChecks[id] = item
	return item
}

func clickable(items map[string]*widget.Clickable, id string) *widget.Clickable {
	if item := items[id]; item != nil {
		return item
	}
	item := new(widget.Clickable)
	items[id] = item
	return item
}

func visibleColumns(columns []Column) []Column {
	visible := make([]Column, 0, len(columns))
	for _, column := range columns {
		if column.Visible {
			visible = append(visible, column)
		}
	}
	return visible
}

func columnChild(column Column, widget layout.Widget) layout.FlexChild {
	if column.Width > 0 {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions { return fixedCell(gtx, column.Width, widget) })
	}
	weight := column.Flex
	if weight <= 0 {
		weight = 1
	}
	return layout.Flexed(weight, widget)
}

func fixedCell(gtx layout.Context, width unit.Dp, content layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Dp(width)
	gtx.Constraints.Max.X = gtx.Constraints.Min.X
	return content(gtx)
}

func sortSuffix(sort []SortSpec, columnID string) string {
	for i, spec := range sort {
		if spec.ColumnID == columnID {
			direction := "↑"
			if spec.Descending {
				direction = "↓"
			}
			return fmt.Sprintf(" %s%d", direction, i+1)
		}
	}
	return ""
}

func surface(gtx layout.Context, fill color.NRGBA, content layout.Widget) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dimensions := content(gtx)
	call := recording.Stop()
	paint.FillShape(gtx.Ops, fill, clip.Rect{Max: dimensions.Size}.Op())
	paint.FillShape(gtx.Ops, color.NRGBA{R: 218, G: 225, B: 236, A: 255}, clip.Rect{Min: image.Pt(0, max(0, dimensions.Size.Y-1)), Max: dimensions.Size}.Op())
	call.Add(gtx.Ops)
	return dimensions
}
