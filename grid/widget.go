package grid

import (
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

// Widget renders a Controller using Gio's virtualized layout.List. Keep one
// Widget instance for the screen lifetime so scroll and click state survive.
type Widget struct {
	Controller *Controller
	List       widget.List
	Horizontal widget.List

	AdditiveSort bool
	// ViewMode defaults to ViewAuto: cards on narrow screens, table otherwise.
	ViewMode ViewMode
	// CardBreakpoint defaults to 600dp.
	CardBreakpoint unit.Dp
	// CardTitleColumn and CardSummaryColumn customize the prominent card fields.
	// By default they are OpenColumn/first visible and second visible.
	CardTitleColumn   string
	CardSummaryColumn string
	// EnableSelection displays checkbox controls for bulk workflows.
	EnableSelection bool
	// OpenColumn makes that column's cell the row-open target.
	OpenColumn  string
	OnRow       func(Row)
	OnRowAction func(Row)

	headers          map[string]*widget.Clickable
	rows             map[string]*widget.Clickable
	selects          map[string]*widget.Clickable
	rowChecks        map[string]*widget.Bool
	rowOpens         map[string]time.Time
	measuredOpen     map[string]bool
	measuredOpenID   string
	minimumOpenWidth unit.Dp
	selectAll        widget.Bool
	actions          map[string]*widget.Clickable
	retry            widget.Clickable
	clearSel         widget.Clickable
	cardSort         widget.Clickable
	cardSortIcon     *widget.Icon
	cardSortOpen     bool
}

func NewWidget(controller *Controller) *Widget {
	return &Widget{
		Controller:      controller,
		EnableSelection: true,
		List:            widget.List{List: layout.List{Axis: layout.Vertical}},
		Horizontal:      widget.List{List: layout.List{Axis: layout.Horizontal}},
		CardBreakpoint:  600,
		headers:         make(map[string]*widget.Clickable),
		rows:            make(map[string]*widget.Clickable),
		selects:         make(map[string]*widget.Clickable),
		rowChecks:       make(map[string]*widget.Bool),
		rowOpens:        make(map[string]time.Time),
		measuredOpen:    make(map[string]bool),
		actions:         make(map[string]*widget.Clickable),
		cardSortIcon:    gridIcon(icons.ActionSwapVert),
	}
}

func gridIcon(data []byte) *widget.Icon {
	icon, _ := widget.NewIcon(data)
	return icon
}

func (w *Widget) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if w == nil || w.Controller == nil {
		return material.Body1(theme, "Grid controller is not configured").Layout(gtx)
	}
	snapshot := w.Controller.Snapshot()
	visible := visibleColumns(snapshot.Columns)
	if w.ResolvedViewMode(gtx) == ViewCards {
		return w.layoutCards(gtx, theme, snapshot, visible)
	}
	return w.layoutTable(gtx, theme, snapshot, visible)
}

// ResolvedViewMode reports the actual presentation for the current viewport.
func (w *Widget) ResolvedViewMode(gtx layout.Context) ViewMode {
	pxPerDp := gtx.Metric.PxPerDp
	if pxPerDp <= 0 {
		pxPerDp = 1
	}
	width := unit.Dp(float32(gtx.Constraints.Max.X) / pxPerDp)
	return ResolveViewMode(w.ViewMode, width, w.CardBreakpoint)
}

func (w *Widget) layoutTable(gtx layout.Context, theme *material.Theme, snapshot Snapshot, visible []Column) layout.Dimensions {
	visible = w.expandOpenColumn(gtx, theme, snapshot, visible)
	minimum := tableMinimumWidth(visible, w.EnableSelection)
	if gtx.Constraints.Max.X >= gtx.Dp(minimum) {
		return w.layoutTableContent(gtx, theme, snapshot, visible)
	}
	return material.List(theme, &w.Horizontal).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		width := gtx.Dp(minimum)
		gtx.Constraints.Min.X = width
		gtx.Constraints.Max.X = width
		return w.layoutTableContent(gtx, theme, snapshot, fixedTableColumns(visible))
	})
}

func (w *Widget) expandOpenColumn(gtx layout.Context, theme *material.Theme, snapshot Snapshot, columns []Column) []Column {
	openID := w.OpenColumn
	if openID == "" && len(columns) > 0 {
		openID = columns[0].ID
	}
	if openID == "" {
		return columns
	}
	if w.measuredOpenID != openID {
		w.measuredOpenID = openID
		w.minimumOpenWidth = 0
		w.measuredOpen = make(map[string]bool)
	}
	measure := func(key, value string, header bool) {
		if w.measuredOpen[key] {
			return
		}
		w.measuredOpen[key] = true
		var label material.LabelStyle
		if header {
			label = material.Caption(theme, value)
		} else {
			label = material.Body2(theme, value)
		}
		label.MaxLines = 1
		recording := op.Record(gtx.Ops)
		measured := gtx
		measured.Constraints.Min = image.Point{}
		measured.Constraints.Max.X = 1 << 20
		dimensions := label.Layout(measured)
		_ = recording.Stop()
		pxPerDp := gtx.Metric.PxPerDp
		if pxPerDp <= 0 {
			pxPerDp = 1
		}
		width := unit.Dp(float32(dimensions.Size.X)/pxPerDp) + 16
		if width > w.minimumOpenWidth {
			w.minimumOpenWidth = width
		}
	}
	for _, column := range columns {
		if column.ID == openID {
			measure("header:"+column.Header, column.Header, true)
			break
		}
	}
	for _, row := range snapshot.Rows {
		value := row.Cells[openID]
		measure(row.ID+"\x00"+value, value, false)
	}
	return setColumnMinimumWidth(columns, openID, w.minimumOpenWidth)
}

func setColumnMinimumWidth(columns []Column, columnID string, minimum unit.Dp) []Column {
	adjusted := append([]Column(nil), columns...)
	for index := range adjusted {
		if adjusted[index].ID == columnID && adjusted[index].Width < minimum {
			adjusted[index].Width = minimum
			adjusted[index].Flex = 0
			break
		}
	}
	return adjusted
}

func (w *Widget) layoutTableContent(gtx layout.Context, theme *material.Theme, snapshot Snapshot, visible []Column) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutHeader(gtx, theme, snapshot, visible) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutClearSelection(gtx, theme, snapshot) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return w.layoutRows(gtx, theme, snapshot, visible)
		}),
	)
}

func (w *Widget) layoutCards(gtx layout.Context, theme *material.Theme, snapshot Snapshot, visible []Column) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(snapshot.Rows) == 0 {
				return layout.Dimensions{}
			}
			return w.layoutCardHeader(gtx, theme, snapshot, visible)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutClearSelection(gtx, theme, snapshot) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return w.layoutCardRows(gtx, theme, snapshot, visible)
		}),
	)
}

func (w *Widget) layoutClearSelection(gtx layout.Context, theme *material.Theme, snapshot Snapshot) layout.Dimensions {
	if !w.EnableSelection || len(snapshot.Selection) == 0 {
		return layout.Dimensions{}
	}
	if w.clearSel.Clicked(gtx) {
		w.Controller.ClearSelection()
	}
	button := material.Button(theme, &w.clearSel, fmt.Sprintf("CLEAR %d SELECTED", len(snapshot.Selection)))
	button.Background = color.NRGBA{R: 56, G: 76, B: 112, A: 255}
	button.TextSize = unit.Sp(12)
	return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, button.Layout)
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
			return selectionControl(gtx, theme, &w.selectAll, "Select loaded rows")
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
			if !column.Sortable {
				gtx = gtx.Disabled()
			}
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.Button.Add(gtx.Ops)
				semantic.LabelOp("Sort by " + column.Header).Add(gtx.Ops)
				semantic.DescriptionOp(sortSuffix(snapshot.Sort, column.ID)).Add(gtx.Ops)
				textStyle := material.Caption(theme, label)
				textStyle.Color = color.NRGBA{R: 33, G: 65, B: 130, A: 255}
				textStyle.Alignment = text.Start
				switch column.Align {
				case AlignMiddle:
					textStyle.Alignment = text.Middle
				case AlignEnd:
					textStyle.Alignment = text.End
				}
				textStyle.MaxLines = 1
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
				layout.Rigid(material.Body1(theme, failureMessage(snapshot)).Layout),
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
		return layout.Center.Layout(gtx, material.Body1(theme, emptyMessage(snapshot)).Layout)
	}
	if len(snapshot.Rows) == 0 && snapshot.State == Loading {
		return layout.Center.Layout(gtx, material.Body1(theme, "Loading first page…").Layout)
	}

	extra := 0
	if len(snapshot.Rows) > 0 || snapshot.HasMore || snapshot.State == Loading {
		extra = 1
	}
	count := len(snapshot.Rows) + extra
	if w.List.Position.First+w.List.Position.Count >= len(snapshot.Rows)-5 && snapshot.HasMore && snapshot.State != Loading {
		_ = w.Controller.LoadNext()
	}
	return material.List(theme, &w.List).Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
		if index >= len(snapshot.Rows) {
			message := "End of results"
			if snapshot.HasMore || snapshot.State == Loading {
				message = "Loading more records…"
			}
			return layout.UniformInset(unit.Dp(14)).Layout(gtx, material.Body2(theme, message).Layout)
		}
		return w.layoutRow(gtx, theme, snapshot.Rows[index], columns, snapshot.Selection[snapshot.Rows[index].ID], index)
	})
}

func failureMessage(snapshot Snapshot) string {
	if snapshot.Err == nil {
		return "Could not load records."
	}
	return "Could not load records. " + snapshot.Err.Error()
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
