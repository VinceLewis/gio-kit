// Package dialog provides reusable modal interaction models for Gio apps.
package dialog

import (
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type Confirm struct {
	Title, Message, ConfirmLabel, CancelLabel string
	Destructive                               bool
	OnConfirm, OnCancel                       func()
	confirm, cancel                           widget.Clickable
}

func (d *Confirm) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	semantic.LabelOp(d.Title + ". " + d.Message).Add(gtx.Ops)
	if d.confirm.Clicked(gtx) && d.OnConfirm != nil {
		d.OnConfirm()
	}
	if d.cancel.Clicked(gtx) && d.OnCancel != nil {
		d.OnCancel()
	}
	confirm, cancel := d.ConfirmLabel, d.CancelLabel
	if confirm == "" {
		confirm = "CONFIRM"
	}
	if cancel == "" {
		cancel = "CANCEL"
	}
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if gtx.Constraints.Max.X > gtx.Dp(unit.Dp(420)) {
			gtx.Constraints.Max.X = gtx.Dp(unit.Dp(420))
		}
		return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(material.H6(theme, d.Title).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(18)}.Layout(gtx, material.Body1(theme, d.Message).Layout)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, material.Button(theme, &d.cancel, cancel).Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(8)}.Layout(gtx) }),
						layout.Flexed(1, material.Button(theme, &d.confirm, confirm).Layout),
					)
				}),
			)
		})
	})
}
