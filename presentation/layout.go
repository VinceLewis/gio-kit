package presentation

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/accessibility"
)

// flow measures each whole item once, then wraps its recorded operations.
// Measuring with the full available width preserves short labels; replaying
// instead of laying out again avoids consuming a routed event twice.
func flow(gtx layout.Context, gap unit.Dp, children ...layout.Widget) layout.Dimensions {
	var x, y, rowHeight, width int
	spacing := gtx.Dp(gap)
	for _, child := range children {
		childContext := gtx
		childContext.Constraints.Min = image.Point{}
		record := op.Record(gtx.Ops)
		dims := child(childContext)
		call := record.Stop()
		if x > 0 && x+dims.Size.X > gtx.Constraints.Max.X {
			x = 0
			y += rowHeight + spacing
			rowHeight = 0
		}
		offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		width = max(width, x+dims.Size.X)
		rowHeight = max(rowHeight, dims.Size.Y)
		x += dims.Size.X + spacing
	}
	return layout.Dimensions{Size: gtx.Constraints.Constrain(image.Pt(width, y+rowHeight))}
}

func heading(theme *material.Theme, value string) material.LabelStyle {
	label := material.Body1(theme, value)
	label.Font.Weight = font.Bold
	label.MaxLines, label.Truncator = 2, "…"
	return label
}

func boundedLabel(theme *material.Theme, value string, lines int) material.LabelStyle {
	label := material.Body2(theme, value)
	label.MaxLines, label.Truncator = lines, "…"
	return label
}

func blend(background, foreground color.NRGBA, amount uint8) color.NRGBA {
	mix := func(a, b uint8) uint8 {
		return uint8((uint32(a)*uint32(255-amount) + uint32(b)*uint32(amount)) / 255)
	}
	return color.NRGBA{R: mix(background.R, foreground.R), G: mix(background.G, foreground.G), B: mix(background.B, foreground.B), A: 255}
}

func (w *Widget) resolveColor(token string, supplied, fallback color.NRGBA) color.NRGBA {
	if token != "" && w.ResolveColor != nil {
		if resolved := w.ResolveColor(token); resolved.A != 0 {
			return resolved
		}
	}
	if supplied.A != 0 {
		return supplied
	}
	return fallback
}

func (w *Widget) icon(name string) *widget.Icon {
	if w.icons == nil {
		w.icons = map[string]*widget.Icon{}
	}
	if icon, exists := w.icons[name]; exists {
		return icon
	}
	icon := SemanticIcon(name)
	w.icons[name] = icon
	return icon
}

func layoutIcon(gtx layout.Context, icon *widget.Icon, foreground color.NRGBA) layout.Dimensions {
	if icon == nil {
		return layout.Dimensions{}
	}
	size := min(gtx.Dp(18), gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	return icon.Layout(gtx, foreground)
}

func (w *Widget) iconLabel(gtx layout.Context, theme *material.Theme, name, label string, foreground color.NRGBA) layout.Dimensions {
	// Touch-target minima belong to the containing control. Intrinsic text
	// and icon measurements must stay small enough to align their centres.
	gtx.Constraints.Min = image.Point{}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			icon := w.icon(name)
			if icon == nil {
				return layout.Dimensions{}
			}
			return layout.Inset{Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layoutIcon(gtx, icon, foreground)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			text := boundedLabel(theme, label, 2)
			text.Color = foreground
			return text.Layout(gtx)
		}),
	)
}

func (w *Widget) layoutToggle(gtx layout.Context, theme *material.Theme, control Control) layout.Dimensions {
	check := w.check(control.ID, control.Value == "true")
	if check.Update(gtx) && w.OnEvent != nil {
		w.OnEvent(Event{Kind: EventControl, ID: control.ID, Value: boolText(check.Value)})
	}
	content := func(gtx layout.Context) layout.Dimensions {
		semantic.Switch.Add(gtx.Ops)
		semantic.LabelOp(control.Label).Add(gtx.Ops)
		semantic.DescriptionOp(control.DisabledReason).Add(gtx.Ops)
		gtx.Constraints.Min.Y = min(gtx.Dp(48), gtx.Constraints.Max.Y)
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: 6, Left: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return switchIndicator(gtx, theme, check.Value, gtx.Focused(check))
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return w.iconLabel(gtx, theme, control.Icon, control.Label, theme.Fg)
				}),
			)
		})
	}
	if !gtx.Enabled() {
		// Gio drops semantic validity for a registered input handler without
		// filters. A disabled switch therefore needs a measured semantic region
		// with no input registration, retaining its role and selected state.
		return (accessibility.Group{Disabled: true, Selected: check.Value}).Layout(gtx, content)
	}
	return check.Layout(gtx, content)
}

// The full labelled widget.Bool owns pointer and keyboard input. Its visual
// indicator uses position as well as theme colour to convey selection.
func switchIndicator(gtx layout.Context, theme *material.Theme, selected, focused bool) layout.Dimensions {
	size, thumbBounds := switchGeometry(gtx, selected)
	track := blend(theme.Bg, theme.Fg, 100)
	thumb := theme.Fg
	if selected && gtx.Enabled() {
		track, thumb = theme.ContrastBg, theme.ContrastFg
	}
	if focused {
		paint.FillShape(gtx.Ops, theme.Fg, clip.UniformRRect(image.Rectangle{Min: image.Pt(-gtx.Dp(2), -gtx.Dp(2)), Max: size.Add(image.Pt(gtx.Dp(2), gtx.Dp(2)))}, size.Y/2).Op(gtx.Ops))
	}
	paint.FillShape(gtx.Ops, track, clip.UniformRRect(image.Rectangle{Max: size}, size.Y/2).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, thumb, clip.Ellipse(thumbBounds).Op(gtx.Ops))
	return layout.Dimensions{Size: size}
}

func switchGeometry(gtx layout.Context, selected bool) (image.Point, image.Rectangle) {
	gtx.Constraints.Min = image.Point{}
	size := gtx.Constraints.Constrain(image.Pt(gtx.Dp(36), gtx.Dp(22)))
	margin := min(gtx.Dp(3), min(size.X, size.Y)/2)
	diameter := max(0, min(size.X, size.Y)-2*margin)
	x := margin
	if selected {
		x = size.X - margin - diameter
	}
	y := (size.Y - diameter) / 2
	return size, image.Rect(x, y, x+diameter, y+diameter)
}

func (w *Widget) actionButton(gtx layout.Context, theme *material.Theme, button *widget.Clickable, action Action) layout.Dimensions {
	content := func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(action.Label).Add(gtx.Ops)
		semantic.DescriptionOp(action.DisabledReason).Add(gtx.Ops)
		semantic.EnabledOp(gtx.Enabled()).Add(gtx.Ops)
		gtx.Constraints.Min.Y = min(gtx.Dp(48), gtx.Constraints.Max.Y)
		background, foreground := theme.ContrastBg, theme.ContrastFg
		if !gtx.Enabled() || action.Placement == "secondary" || action.Placement == "row" || action.Placement == "inline" {
			background, foreground = blend(theme.Bg, theme.Fg, 22), theme.Fg
		}
		if button.Hovered() || gtx.Focused(button) {
			background = blend(background, foreground, 36)
		}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 6, Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return surface(gtx, background, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.Y = min(gtx.Dp(24), gtx.Constraints.Max.Y)
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return w.iconLabel(gtx, theme, action.Icon, action.Label, foreground)
						})
					})
				})
			})
		})
	}
	if !gtx.Enabled() {
		return (accessibility.Group{Label: action.Label, Disabled: true}).Layout(gtx, content)
	}
	return button.Layout(gtx, content)
}

func (w *Widget) layoutStatus(gtx layout.Context, theme *material.Theme, label, icon string, accent color.NRGBA) layout.Dimensions {
	return layout.Inset{Top: 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					size := min(gtx.Dp(6), gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
					paint.FillShape(gtx.Ops, accent, clip.Ellipse(image.Rect(0, 0, size, size)).Op(gtx.Ops))
					return layout.Dimensions{Size: image.Pt(size, size)}
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return w.iconLabel(gtx, theme, icon, label, theme.Fg)
			}),
		)
	})
}
