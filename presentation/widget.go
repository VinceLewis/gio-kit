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
	Page    Page
	OnEvent func(Event)
	// ResolveColor resolves application-owned theme tokens on each frame.
	// A zero colour leaves the supplied colour or theme fallback in effect.
	ResolveColor  func(string) color.NRGBA
	list          widget.List
	clicks        map[string]*widget.Clickable
	checks        map[string]*widget.Bool
	selections    map[string]string
	controlValues map[string]string
	icons         map[string]*widget.Icon
}

func NewWidget(page Page) *Widget {
	w := &Widget{list: widget.List{List: layout.List{Axis: layout.Vertical}}, clicks: map[string]*widget.Clickable{}, checks: map[string]*widget.Bool{}, selections: map[string]string{}, controlValues: map[string]string{}}
	w.SetPage(page)
	return w
}

func (w *Widget) SetPage(page Page) {
	w.Page = page
	// Apply changed loaded values without resetting an optimistic input value
	// when a caller supplies the same snapshot on every frame.
	for _, section := range page.Sections {
		for _, control := range section.Controls {
			if previous, exists := w.controlValues[control.ID]; !exists || previous != control.Value {
				if check := w.checks[control.ID]; check != nil {
					check.Value = control.Value == "true"
				}
				delete(w.selections, control.ID)
			}
			w.controlValues[control.ID] = control.Value
		}
	}
}

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
	return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{}
		if section.Heading != "" || len(section.Actions) != 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return w.layoutHeading(gtx, theme, section.Heading, section.Actions)
			}))
		}
		if section.Comment != "" {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, material.Body2(theme, section.Comment).Layout)
			}))
		}
		if len(section.Controls) != 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutControls(gtx, theme, section.Controls) }))
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
	children := make([]layout.Widget, 0, len(controls))
	for _, control := range controls {
		control := control
		children = append(children, func(gtx layout.Context) layout.Dimensions {
			if !control.Enabled {
				gtx = gtx.Disabled()
			}
			switch control.Kind {
			case "toggle":
				return w.layoutToggle(gtx, theme, control)
			case "select", "contextSelector":
				return w.layoutSelect(gtx, theme, control)
			default:
				return material.Body1(theme, control.Label).Layout(gtx)
			}
		})
	}
	return flow(gtx, 8, children...)
}

func (w *Widget) layoutSelect(gtx layout.Context, theme *material.Theme, control Control) layout.Dimensions {
	current := control.Value
	if value := w.selections[control.ID]; value != "" {
		current = value
	}
	var children []layout.Widget
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
		children = append(children, func(gtx layout.Context) layout.Dimensions {
			return (accessibility.Group{Label: control.Label + ": " + option.Label, Selected: option.Value == current}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.SelectedOp(option.Value == current).Add(gtx.Ops)
				return w.actionButton(gtx, theme, button, Action{Label: label, Enabled: control.Enabled, DisabledReason: control.DisabledReason, Placement: "inline"})
			})
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(boundedLabel(theme, control.Label, 2).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return flow(gtx, 6, children...) }),
	)
}

func (w *Widget) layoutHeading(gtx layout.Context, theme *material.Theme, title string, actions []Action) layout.Dimensions {
	var children []layout.Widget
	if title != "" {
		children = append(children, func(gtx layout.Context) layout.Dimensions {
			if len(actions) != 0 {
				gtx.Constraints.Min.Y = min(gtx.Dp(48), gtx.Constraints.Max.Y)
			}
			return layout.Center.Layout(gtx, heading(theme, title).Layout)
		})
	}
	for _, action := range actions {
		action := action
		children = append(children, func(gtx layout.Context) layout.Dimensions {
			return w.layoutActions(gtx, theme, []Action{action}, "", "")
		})
	}
	return flow(gtx, 8, children...)
}

func (w *Widget) layoutList(gtx layout.Context, theme *material.Theme, list List) layout.Dimensions {
	children := []layout.FlexChild{}
	if list.Heading != "" || len(list.Actions) != 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return w.layoutHeading(gtx, theme, list.Heading, list.Actions)
		}))
	}
	if len(list.Rows) == 0 {
		message := list.EmptyText
		if message == "" {
			message = "No items"
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 8, Bottom: 10}.Layout(gtx, boundedLabel(theme, message, 3).Layout)
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
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return surface(gtx, blend(theme.Bg, theme.Fg, 12), func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutFragments(gtx, theme, row.Fragments) }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if row.StatusLabel == "" {
								return layout.Dimensions{}
							}
							return (accessibility.Group{Description: row.StatusAccessibleLabel}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return w.layoutStatus(gtx, theme, row.StatusLabel, row.StatusIcon, w.resolveColor(row.ColorToken, row.Color, theme.ContrastBg))
							})
						}),
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
	// Preserve declared order. A bold value starts a primary line; subsequent
	// summary values wrap individually instead of competing for a rigid row.
	var children []layout.FlexChild
	var line []layout.Widget
	lineText := false
	var pendingIcons []Fragment
	flush := func() {
		if len(line) == 0 {
			return
		}
		items := line
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return flow(gtx, 3, items...) }))
		line = nil
		lineText = false
	}
	for _, fragment := range fragments {
		fragment := fragment
		if fragment.Kind == "icon" {
			pendingIcons = append(pendingIcons, fragment)
			continue
		}
		icons := pendingIcons
		pendingIcons = nil
		if fragment.Style == "bold" && lineText {
			flush()
		}
		lineText = lineText || fragment.Text != ""
		line = append(line, func(gtx layout.Context) layout.Dimensions {
			value := fragment.Text
			style := boundedLabel(theme, value, 2)
			if fragment.Style == "bold" {
				style = heading(theme, value)
			}
			if fragment.Style == "muted" || fragment.Style == "caption" {
				style.TextSize = unit.Sp(12)
			}
			var parts []layout.FlexChild
			for _, icon := range icons {
				icon := icon
				parts = append(parts, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return w.layoutFragmentIcon(gtx, theme, icon)
					})
				}))
			}
			parts = append(parts, layout.Rigid(style.Layout))
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, parts...)
		})
		if fragment.Style == "bold" {
			flush()
		}
	}
	for _, icon := range pendingIcons {
		icon := icon
		line = append(line, func(gtx layout.Context) layout.Dimensions { return w.layoutFragmentIcon(gtx, theme, icon) })
	}
	flush()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (w *Widget) layoutFragmentIcon(gtx layout.Context, theme *material.Theme, fragment Fragment) layout.Dimensions {
	icon := w.icon(fragment.Icon)
	if icon == nil {
		return boundedLabel(theme, fragment.AccessibleLabel, 2).Layout(gtx)
	}
	return (accessibility.Group{Label: fragment.AccessibleLabel}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layoutIcon(gtx, icon, theme.Fg)
	})
}

func (w *Widget) layoutActions(gtx layout.Context, theme *material.Theme, actions []Action, rowID, date string) layout.Dimensions {
	children := make([]layout.Widget, 0, len(actions))
	for _, action := range actions {
		action := action
		children = append(children, func(gtx layout.Context) layout.Dimensions {
			key := "action\x00" + action.ID + "\x00" + rowID + "\x00" + date
			button := w.click(key)
			if button.Clicked(gtx) && action.Enabled && w.OnEvent != nil {
				w.OnEvent(Event{Kind: EventAction, ID: action.ID, RowID: rowID, Date: date})
			}
			if !action.Enabled {
				gtx = gtx.Disabled()
			}
			return w.actionButton(gtx, theme, button, action)
		})
	}
	return flow(gtx, 6, children...)
}

func (w *Widget) layoutCalendar(gtx layout.Context, theme *material.Theme, calendar Calendar) layout.Dimensions {
	previous, next := w.click("month.prev\x00"+calendar.ID), w.click("month.next\x00"+calendar.ID)
	if previous.Clicked(gtx) && w.OnEvent != nil {
		w.OnEvent(Event{Kind: EventCalendarMonth, ID: calendar.ID, Value: "previous"})
	}
	if next.Clicked(gtx) && w.OnEvent != nil {
		w.OnEvent(Event{Kind: EventCalendarMonth, ID: calendar.ID, Value: "next"})
	}
	var children []layout.FlexChild
	if calendar.Heading != "" {
		children = append(children, layout.Rigid(heading(theme, calendar.Heading).Layout))
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return flow(gtx, 6,
			func(gtx layout.Context) layout.Dimensions {
				return w.actionButton(gtx, theme, previous, Action{Label: "PREVIOUS", Icon: "chevron-left", Enabled: true, Placement: "inline"})
			},
			func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.Y = min(gtx.Dp(48), gtx.Constraints.Max.Y)
				label := material.Subtitle1(theme, calendar.Month)
				label.Alignment = text.Middle
				label.MaxLines, label.Truncator = 2, "…"
				return layout.W.Layout(gtx, label.Layout)
			},
			func(gtx layout.Context) layout.Dimensions {
				return w.actionButton(gtx, theme, next, Action{Label: "NEXT", Icon: "chevron-right", Enabled: true, Placement: "inline"})
			},
		)
	}))
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
	children := []layout.FlexChild{layout.Rigid(heading(theme, matrix.Heading).Layout)}
	if matrix.DisabledReason != "" {
		children = append(children, layout.Rigid(material.Caption(theme, matrix.DisabledReason).Layout))
	}
	if len(matrix.Rows) == 0 {
		children = append(children, layout.Rigid(material.Body2(theme, matrix.EmptyText).Layout))
	}
	for _, row := range matrix.Rows {
		row := row
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			var cells []layout.Widget
			for _, cell := range row.Cells {
				cell := cell
				cells = append(cells, func(gtx layout.Context) layout.Dimensions {
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
						return button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							semantic.Button.Add(gtx.Ops)
							gtx.Constraints.Min.Y = min(gtx.Dp(48), gtx.Constraints.Max.Y)
							return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return w.layoutStatus(gtx, theme, cell.Text, "", w.resolveColor(cell.ColorToken, cell.Color, theme.ContrastBg))
							})
						})
					})
				})
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(boundedLabel(theme, row.Label, 2).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return flow(gtx, 8, cells...) }),
			)
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
				return (accessibility.Group{Label: item.AccessibleLabel}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return w.layoutStatus(gtx, theme, item.Label, item.Icon, w.resolveColor(item.ColorToken, item.Color, theme.ContrastBg))
				})
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
