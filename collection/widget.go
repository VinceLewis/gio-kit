// Package collection provides reusable controls for editing an ordered child
// collection. It owns presentation and emits intents; callers own persistence.
package collection

import (
	"image"
	"image/color"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type Item struct {
	ID, Label, Secondary   string
	CanEdit, CanRemove     bool
	CanMoveUp, CanMoveDown bool
}

type Model struct {
	Title, EmptyText, Summary, DisabledReason string
	CanAdd                                    bool
	Items                                     []Item
}

type Widget struct {
	Model
	OnAdd    func()
	OnEdit   func(Item)
	OnRemove func(Item)
	OnMove   func(Item, int)

	buttons                                         map[string]*widget.Clickable
	add                                             widget.Clickable
	addIcon, editIcon, removeIcon, upIcon, downIcon *widget.Icon
}

func NewWidget(model Model) *Widget {
	return &Widget{
		Model: model, buttons: map[string]*widget.Clickable{},
		addIcon: icon(icons.ContentAdd), editIcon: icon(icons.EditorModeEdit),
		removeIcon: icon(icons.ActionDelete), upIcon: icon(icons.NavigationArrowUpward),
		downIcon: icon(icons.NavigationArrowDownward),
	}
}

func icon(data []byte) *widget.Icon { value, _ := widget.NewIcon(data); return value }

func (w *Widget) SetModel(model Model) { w.Model = model }

func (w *Widget) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if w.add.Clicked(gtx) && w.CanAdd && w.OnAdd != nil {
		w.OnAdd()
	}
	children := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, material.Subtitle1(theme, w.Title).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !w.CanAdd {
					return layout.Dimensions{}
				}
				button := material.IconButton(theme, &w.add, w.addIcon, "Add to "+w.Title)
				button.Size, button.Inset = unit.Dp(24), layout.UniformInset(unit.Dp(12))
				return button.Layout(gtx)
			}),
		)
	})}
	if len(w.Items) == 0 && w.EmptyText != "" {
		children = append(children, layout.Rigid(material.Body2(theme, w.EmptyText).Layout))
	}
	for index, item := range w.Items {
		index, item := index, item
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return w.layoutItem(gtx, theme, item, index)
		}))
	}
	if w.Summary != "" {
		children = append(children, layout.Rigid(material.Caption(theme, w.Summary).Layout))
	}
	if w.DisabledReason != "" {
		children = append(children, layout.Rigid(material.Caption(theme, w.DisabledReason).Layout))
	}
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (w *Widget) layoutItem(gtx layout.Context, theme *material.Theme, item Item, index int) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return widget.Border{Color: color.NRGBA{R: 190, G: 199, B: 213, A: 255}, CornerRadius: unit.Dp(10), Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(12), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(material.Body1(theme, item.Label).Layout),
							layout.Rigid(material.Caption(theme, item.Secondary).Layout),
						)
					}),
					layout.Rigid(w.itemAction(theme, item, "up", w.upIcon, "Move up", item.CanMoveUp, func() {
						if w.OnMove != nil {
							w.OnMove(item, index-1)
						}
					})),
					layout.Rigid(w.itemAction(theme, item, "down", w.downIcon, "Move down", item.CanMoveDown, func() {
						if w.OnMove != nil {
							w.OnMove(item, index+1)
						}
					})),
					layout.Rigid(w.itemAction(theme, item, "edit", w.editIcon, "Edit", item.CanEdit, func() {
						if w.OnEdit != nil {
							w.OnEdit(item)
						}
					})),
					layout.Rigid(w.itemAction(theme, item, "remove", w.removeIcon, "Remove", item.CanRemove, func() {
						if w.OnRemove != nil {
							w.OnRemove(item)
						}
					})),
				)
			})
		})
	})
}

func (w *Widget) itemAction(theme *material.Theme, item Item, suffix string, icon *widget.Icon, label string, enabled bool, action func()) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if !enabled {
			return layout.Dimensions{}
		}
		click := w.buttons[item.ID+"."+suffix]
		if click == nil {
			click = new(widget.Clickable)
			w.buttons[item.ID+"."+suffix] = click
		}
		if click.Clicked(gtx) {
			action()
		}
		return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(label + " " + item.Label).Add(gtx.Ops)
			gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(44)), gtx.Dp(unit.Dp(44)))
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(20)), gtx.Dp(unit.Dp(20)))
				gtx.Constraints.Max = gtx.Constraints.Min
				return icon.Layout(gtx, theme.Palette.Fg)
			})
		})
	}
}
