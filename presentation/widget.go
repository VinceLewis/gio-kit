package presentation

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/accessibility"
)

// Widget retains interaction and scroll state across immediate-mode frames.
type Widget struct {
	Page       Page
	OnEvent    func(Event)
	list       widget.List
	clicks     map[string]*widget.Clickable
	checks     map[string]*widget.Bool
	selections map[string]string
}

func NewWidget(page Page) *Widget {
	return &Widget{Page: page, list: widget.List{List: layout.List{Axis: layout.Vertical}}, clicks: map[string]*widget.Clickable{}, checks: map[string]*widget.Bool{}, selections: map[string]string{}}
}

func (w *Widget) SetPage(page Page) { w.Page = page }

func (w *Widget) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if w == nil {
		return layout.Dimensions{}
	}
	if w.Page.Error != "" {
		return layout.Center.Layout(gtx, material.Body1(theme, w.Page.Error).Layout)
	}
	if w.Page.Loading && len(w.Page.Sections) == 0 {
		return layout.Center.Layout(gtx, material.Body1(theme, "Loading…").Layout)
	}
	count := len(w.Page.Sections)
	if len(w.Page.Legends) != 0 {
		count++
	}
	return material.List(theme, &w.list).Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
		if index == len(w.Page.Sections) {
			return w.layoutLegends(gtx, theme)
		}
		return w.layoutSection(gtx, theme, w.Page.Sections[index])
	})
}

func (w *Widget) layoutSection(gtx layout.Context, theme *material.Theme, section Section) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{}
		if section.Heading != "" {
			children = append(children, layout.Rigid(material.H6(theme, section.Heading).Layout))
		}
		if section.Comment != "" {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, material.Body2(theme, section.Comment).Layout)
			}))
		}
		if len(section.Controls) != 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutControls(gtx, theme, section.Controls) }))
		}
		if len(section.Actions) != 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return w.layoutActions(gtx, theme, section.Actions, "", "")
			}))
		}
		for _, list := range section.Lists {
			list := list
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutList(gtx, theme, list) }))
		}
		for _, calendar := range section.Calendars {
			calendar := calendar
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutCalendar(gtx, theme, calendar) }))
		}
		for _, matrix := range section.Matrices {
			matrix := matrix
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutMatrix(gtx, theme, matrix) }))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (w *Widget) layoutControls(gtx layout.Context, theme *material.Theme, controls []Control) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(controls))
	for _, control := range controls {
		control := control
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: unit.Dp(12), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if !control.Enabled {
					gtx = gtx.Disabled()
				}
				switch control.Kind {
				case "toggle":
					check := w.check(control.ID, control.Value == "true")
					if check.Update(gtx) && w.OnEvent != nil {
						w.OnEvent(Event{Kind: EventControl, ID: control.ID, Value: boolText(check.Value)})
					}
					return material.CheckBox(theme, check, control.Label).Layout(gtx)
				case "select", "contextSelector":
					return w.layoutSelect(gtx, theme, control)
				default:
					return material.Body1(theme, control.Label).Layout(gtx)
				}
			})
		}))
	}
	return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx, children...)
}

func (w *Widget) layoutSelect(gtx layout.Context, theme *material.Theme, control Control) layout.Dimensions {
	current := control.Value
	if value := w.selections[control.ID]; value != "" {
		current = value
	}
	var children []layout.FlexChild
	children = append(children, layout.Rigid(material.Caption(theme, control.Label).Layout))
	for _, option := range control.Options {
		option := option
		key := "control\x00" + control.ID + "\x00" + option.Value
		button := w.click(key)
		if button.Clicked(gtx) {
			w.selections[control.ID] = option.Value
			if w.OnEvent != nil {
				w.OnEvent(Event{Kind: EventControl, ID: control.ID, Value: option.Value})
			}
		}
		label := option.Label
		if option.Value == current {
			label = "✓ " + label
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return (accessibility.Group{Label: control.Label + ": " + option.Label, Selected: option.Value == current}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.SelectedOp(option.Value == current).Add(gtx.Ops)
				return material.Button(theme, button, label).Layout(gtx)
			})
		}))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func (w *Widget) layoutList(gtx layout.Context, theme *material.Theme, list List) layout.Dimensions {
	children := []layout.FlexChild{}
	if list.Heading != "" {
		children = append(children, layout.Rigid(material.Subtitle1(theme, list.Heading).Layout))
	}
	if len(list.Actions) != 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutActions(gtx, theme, list.Actions, "", "") }))
	}
	if len(list.Rows) == 0 {
		message := list.EmptyText
		if message == "" {
			message = "No items"
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(12)).Layout(gtx, material.Body2(theme, message).Layout)
		}))
	}
	for _, row := range list.Rows {
		row := row
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutRow(gtx, theme, list.ID, row) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (w *Widget) layoutRow(gtx layout.Context, theme *material.Theme, owner string, row Row) layout.Dimensions {
	label := row.AccessibleLabel
	if label == "" {
		var pieces []string
		for _, fragment := range row.Fragments {
			pieces = append(pieces, fragment.Text, fragment.AccessibleLabel)
		}
		label = strings.TrimSpace(strings.Join(pieces, " "))
	}
	return (accessibility.Group{Label: label}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return surface(gtx, statusColor(row.Status, theme.Bg), func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutFragments(gtx, theme, row.Fragments) }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return w.layoutActions(gtx, theme, row.Actions, row.ID, "")
						}),
					)
				})
			})
		})
	})
}

func (w *Widget) layoutFragments(gtx layout.Context, theme *material.Theme, fragments []Fragment) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(fragments))
	for _, fragment := range fragments {
		fragment := fragment
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			value := fragment.Text
			if fragment.Kind == "icon" && fragment.Icon != "" {
				value = "[" + fragment.Icon + "]"
			}
			style := material.Body2(theme, value)
			if fragment.Style == "bold" {
				style.Font.Weight = 700
			}
			if fragment.Style == "muted" || fragment.Style == "caption" {
				style.TextSize = unit.Sp(12)
			}
			style.MaxLines = 3
			return style.Layout(gtx)
		}))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func (w *Widget) layoutActions(gtx layout.Context, theme *material.Theme, actions []Action, rowID, date string) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(actions))
	for _, action := range actions {
		action := action
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			key := "action\x00" + action.ID + "\x00" + rowID + "\x00" + date
			button := w.click(key)
			if button.Clicked(gtx) && action.Enabled && w.OnEvent != nil {
				w.OnEvent(Event{Kind: EventAction, ID: action.ID, RowID: rowID, Date: date})
			}
			if !action.Enabled {
				gtx = gtx.Disabled()
			}
			return layout.Inset{Right: unit.Dp(6), Top: unit.Dp(4)}.Layout(gtx, material.Button(theme, button, action.Label).Layout)
		}))
	}
	return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx, children...)
}

func (w *Widget) layoutCalendar(gtx layout.Context, theme *material.Theme, calendar Calendar) layout.Dimensions {
	previous, next := w.click("month.prev\x00"+calendar.ID), w.click("month.next\x00"+calendar.ID)
	if previous.Clicked(gtx) && w.OnEvent != nil {
		w.OnEvent(Event{Kind: EventCalendarMonth, ID: calendar.ID, Value: "previous"})
	}
	if next.Clicked(gtx) && w.OnEvent != nil {
		w.OnEvent(Event{Kind: EventCalendarMonth, ID: calendar.ID, Value: "next"})
	}
	children := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(material.Button(theme, previous, "PREVIOUS").Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				label := material.Subtitle1(theme, calendar.Month)
				label.Alignment = text.Middle
				return label.Layout(gtx)
			}),
			layout.Rigid(material.Button(theme, next, "NEXT").Layout),
		)
	})}
	if len(calendar.Days) == 0 {
		children = append(children, layout.Rigid(material.Body2(theme, calendar.EmptyText).Layout))
	}
	for _, day := range calendar.Days {
		day := day
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			rows := []layout.FlexChild{layout.Rigid(material.Subtitle2(theme, day.Label).Layout)}
			if len(calendar.Actions) != 0 {
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return w.layoutActions(gtx, theme, calendar.Actions, "", day.Date)
				}))
			}
			for _, row := range day.Rows {
				row := row
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutRow(gtx, theme, calendar.ID, row) }))
			}
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (w *Widget) layoutMatrix(gtx layout.Context, theme *material.Theme, matrix Matrix) layout.Dimensions {
	children := []layout.FlexChild{layout.Rigid(material.Subtitle1(theme, matrix.Heading).Layout)}
	if matrix.DisabledReason != "" {
		children = append(children, layout.Rigid(material.Caption(theme, matrix.DisabledReason).Layout))
	}
	if len(matrix.Rows) == 0 {
		children = append(children, layout.Rigid(material.Body2(theme, matrix.EmptyText).Layout))
	}
	for _, row := range matrix.Rows {
		row := row
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			cells := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, material.Body2(theme, row.Label).Layout)
			})}
			for _, cell := range row.Cells {
				cell := cell
				cells = append(cells, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					button := w.click("matrix\x00" + matrix.ID + "\x00" + row.ID + "\x00" + cell.Column)
					if button.Clicked(gtx) && matrix.Editable && cell.Enabled && w.OnEvent != nil {
						w.OnEvent(Event{Kind: EventMatrixCell, ID: matrix.ID, RowID: row.ID, Column: cell.Column})
					}
					if !matrix.Editable || !cell.Enabled {
						gtx = gtx.Disabled()
					}
					name := cell.AccessibleLabel
					if name == "" {
						name = row.Label + ": " + cell.Column
					}
					return (accessibility.Group{Label: name, Description: matrix.DisabledReason, Disabled: !matrix.Editable || !cell.Enabled}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, material.Button(theme, button, cell.Text).Layout)
					})
				}))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, cells...)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (w *Widget) layoutLegends(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	var children []layout.FlexChild
	for _, legend := range w.Page.Legends {
		legend := legend
		children = append(children, layout.Rigid(material.Subtitle2(theme, legend.Title).Layout))
		for _, item := range legend.Items {
			item := item
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return (accessibility.Group{Label: item.AccessibleLabel}).Layout(gtx, material.Caption(theme, "● "+item.Label).Layout)
			}))
		}
	}
	return layout.Inset{Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (w *Widget) click(key string) *widget.Clickable {
	if w.clicks[key] == nil {
		w.clicks[key] = new(widget.Clickable)
	}
	return w.clicks[key]
}
func (w *Widget) check(key string, value bool) *widget.Bool {
	if w.checks[key] == nil {
		w.checks[key] = &widget.Bool{Value: value}
	}
	return w.checks[key]
}
func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func surface(gtx layout.Context, background color.NRGBA, content layout.Widget) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dims := content(gtx)
	call := recording.Stop()
	area := clip.UniformRRect(image.Rectangle{Max: dims.Size}, gtx.Dp(unit.Dp(5))).Push(gtx.Ops)
	paint.Fill(gtx.Ops, background)
	call.Add(gtx.Ops)
	area.Pop()
	return dims
}

func statusColor(status string, fallback color.NRGBA) color.NRGBA {
	switch strings.ToLower(status) {
	case "conflict", "unavailable":
		return color.NRGBA{R: 255, G: 232, B: 232, A: 255}
	case "available":
		return color.NRGBA{R: 229, G: 247, B: 233, A: 255}
	case "busyelwhere", "busyelsewhere":
		return color.NRGBA{R: 255, G: 244, B: 214, A: 255}
	case "event", "rehearsal":
		return color.NRGBA{R: 231, G: 239, B: 252, A: 255}
	default:
		return fallback
	}
}
