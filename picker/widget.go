// Package picker provides a reusable searchable record picker.
package picker

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type Option struct {
	ID, Label, Secondary, DisabledReason string
	Disabled                             bool
}

type Model struct {
	Title, Query, EmptyText, Error string
	Loading                        bool
	Options                        []Option
}

type Widget struct {
	Model
	OnSearch func(string)
	OnSelect func(Option)
	OnCancel func()
	search   widget.Editor
	list     widget.List
	buttons  map[string]*widget.Clickable
	submit   widget.Clickable
	cancel   widget.Clickable
}

func NewWidget(model Model) *Widget {
	w := &Widget{Model: model, list: widget.List{List: layout.List{Axis: layout.Vertical}}, buttons: map[string]*widget.Clickable{}}
	w.search.SingleLine = true
	w.search.SetText(model.Query)
	return w
}

func (w *Widget) SetModel(model Model) { w.Model = model }

func (w *Widget) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if w.submit.Clicked(gtx) && w.OnSearch != nil {
		w.OnSearch(w.search.Text())
	}
	if w.cancel.Clicked(gtx) && w.OnCancel != nil {
		w.OnCancel()
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.H6(theme, w.Title).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, material.Editor(theme, &w.search, "Search").Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, material.Button(theme, &w.submit, "SEARCH").Layout)
					}),
				)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if w.Error != "" {
				return material.Body1(theme, w.Error).Layout(gtx)
			}
			if w.Loading {
				return layout.Center.Layout(gtx, material.Body1(theme, "Loading…").Layout)
			}
			if len(w.Options) == 0 {
				text := w.EmptyText
				if text == "" {
					text = "No matching records"
				}
				return layout.Center.Layout(gtx, material.Body1(theme, text).Layout)
			}
			return material.List(theme, &w.list).Layout(gtx, len(w.Options), func(gtx layout.Context, index int) layout.Dimensions {
				option := w.Options[index]
				button := w.buttons[option.ID]
				if button == nil {
					button = new(widget.Clickable)
					w.buttons[option.ID] = button
				}
				if button.Clicked(gtx) && !option.Disabled && w.OnSelect != nil {
					w.OnSelect(option)
				}
				if option.Disabled {
					gtx = gtx.Disabled()
				}
				return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(48))
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(material.Body1(theme, option.Label).Layout), layout.Rigid(material.Caption(theme, option.Secondary).Layout))
					})
				})
			})
		}),
		layout.Rigid(material.Button(theme, &w.cancel, "CANCEL").Layout),
	)
}
