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
	"github.com/VinceLewis/gio-kit/theme"
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

// flowControls lays whole controls out in equal-width columns on one row
// when every control's own intrinsic width fits the shared column width
// (available/len(children), after the gap), and otherwise wraps between
// complete controls exactly like flow. Every control is measured exactly
// once regardless of which arrangement is chosen, so a routed event a
// control's Layout consumes is never consumed twice.
func flowControls(gtx layout.Context, m theme.Metrics, children ...layout.Widget) layout.Dimensions {
	n := len(children)
	if n == 0 {
		return layout.Dimensions{}
	}
	type recordedChild struct {
		call op.CallOp
		dims layout.Dimensions
	}
	items := make([]recordedChild, n)
	for i, child := range children {
		childContext := gtx
		childContext.Constraints.Min = image.Point{}
		record := op.Record(gtx.Ops)
		dims := child(childContext)
		call := record.Stop()
		items[i] = recordedChild{call: call, dims: dims}
	}
	spacing := gtx.Dp(m.ControlGap)
	available := gtx.Constraints.Max.X
	columnWidth := (available - spacing*(n-1)) / n
	minTouch := gtx.Dp(theme.MinTouchTarget)
	fitsColumns := columnWidth >= minTouch
	height := 0
	for _, item := range items {
		if item.dims.Size.X > columnWidth {
			fitsColumns = false
		}
		height = max(height, item.dims.Size.Y)
	}
	if fitsColumns {
		x := 0
		for _, item := range items {
			offset := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
			item.call.Add(gtx.Ops)
			offset.Pop()
			x += columnWidth + spacing
		}
		return layout.Dimensions{Size: gtx.Constraints.Constrain(image.Pt(available, height))}
	}
	// Doesn't fit as equal columns: wrap between complete controls, matching
	// flow's own greedy left-to-right packing, replaying the same recordings
	// so nothing is measured (and no routed event consumed) twice.
	var x, y, rowHeight, width int
	for _, item := range items {
		if x > 0 && x+item.dims.Size.X > gtx.Constraints.Max.X {
			x = 0
			y += rowHeight + spacing
			rowHeight = 0
		}
		offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		item.call.Add(gtx.Ops)
		offset.Pop()
		width = max(width, x+item.dims.Size.X)
		rowHeight = max(rowHeight, item.dims.Size.Y)
		x += item.dims.Size.X + spacing
	}
	return layout.Dimensions{Size: gtx.Constraints.Constrain(image.Pt(width, y+rowHeight))}
}

func heading(th *material.Theme, m theme.Metrics, value string) material.LabelStyle {
	label := material.Body1(th, value)
	label.Font.Weight = font.Bold
	label.TextSize = m.HeadingSize
	label.MaxLines, label.Truncator = 2, "…"
	return label
}

func boundedLabel(th *material.Theme, m theme.Metrics, value string, lines int) material.LabelStyle {
	label := material.Body2(th, value)
	label.TextSize = m.BodySize
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

func layoutIcon(gtx layout.Context, icon *widget.Icon, size unit.Dp, foreground color.NRGBA) layout.Dimensions {
	if icon == nil {
		return layout.Dimensions{}
	}
	side := min(gtx.Dp(size), gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	gtx.Constraints = layout.Exact(image.Pt(side, side))
	return icon.Layout(gtx, foreground)
}

func (w *Widget) iconLabel(gtx layout.Context, th *material.Theme, m theme.Metrics, name, label string, foreground color.NRGBA) layout.Dimensions {
	// Touch-target minima belong to the containing control. Intrinsic text
	// and icon measurements must stay small enough to align their centres.
	gtx.Constraints.Min = image.Point{}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			icon := w.icon(name)
			if icon == nil {
				return layout.Dimensions{}
			}
			return layout.Inset{Right: m.InlineGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layoutIcon(gtx, icon, m.IconSize, foreground)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			text := boundedLabel(th, m, label, 2)
			text.Color = foreground
			return text.Layout(gtx)
		}),
	)
}

func (w *Widget) layoutToggle(gtx layout.Context, th *material.Theme, m theme.Metrics, control Control) layout.Dimensions {
	check := w.check(control.ID, control.Value == "true")
	if check.Update(gtx) && w.OnEvent != nil {
		w.OnEvent(Event{Kind: EventControl, ID: control.ID, Value: boolText(check.Value)})
	}
	content := func(gtx layout.Context) layout.Dimensions {
		semantic.Switch.Add(gtx.Ops)
		semantic.LabelOp(control.Label).Add(gtx.Ops)
		semantic.DescriptionOp(control.DisabledReason).Add(gtx.Ops)
		gtx.Constraints.Min.Y = min(gtx.Dp(theme.MinTouchTarget), gtx.Constraints.Max.Y)
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: m.InlineGap, Left: m.InlineGap / 3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return switchIndicator(gtx, th, check.Value, gtx.Focused(check))
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return w.iconLabel(gtx, th, m, control.Icon, control.Label, th.Fg)
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
// indicator uses position as well as theme colour to convey selection. This
// geometry is an interactive control and is never scaled by density.
func switchIndicator(gtx layout.Context, th *material.Theme, selected, focused bool) layout.Dimensions {
	size, thumbBounds := switchGeometry(gtx, selected)
	track := blend(th.Bg, th.Fg, 100)
	thumb := th.Fg
	if selected && gtx.Enabled() {
		track, thumb = th.ContrastBg, th.ContrastFg
	}
	if focused {
		paint.FillShape(gtx.Ops, th.Fg, clip.UniformRRect(image.Rectangle{Min: image.Pt(-gtx.Dp(2), -gtx.Dp(2)), Max: size.Add(image.Pt(gtx.Dp(2), gtx.Dp(2)))}, size.Y/2).Op(gtx.Ops))
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

func (w *Widget) actionButton(gtx layout.Context, th *material.Theme, m theme.Metrics, button *widget.Clickable, action Action) layout.Dimensions {
	content := func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(action.Label).Add(gtx.Ops)
		semantic.DescriptionOp(action.DisabledReason).Add(gtx.Ops)
		semantic.EnabledOp(gtx.Enabled()).Add(gtx.Ops)
		gtx.Constraints.Min.Y = min(gtx.Dp(theme.MinTouchTarget), gtx.Constraints.Max.Y)
		background, foreground := th.ContrastBg, th.ContrastFg
		if !gtx.Enabled() || action.Placement == "secondary" || action.Placement == "row" || action.Placement == "inline" {
			background, foreground = blend(th.Bg, th.Fg, 22), th.Fg
		}
		if button.Hovered() || gtx.Focused(button) {
			background = blend(background, foreground, 36)
		}
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: m.ButtonPaddingY, Bottom: m.ButtonPaddingY}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return surface(gtx, background, m.SurfaceRadius, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: m.ButtonPaddingY, Bottom: m.ButtonPaddingY, Left: m.ButtonPaddingX, Right: m.ButtonPaddingX}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.Y = min(gtx.Dp(theme.MinTouchTarget/2), gtx.Constraints.Max.Y)
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return w.iconLabel(gtx, th, m, action.Icon, action.Label, foreground)
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

func (w *Widget) layoutStatus(gtx layout.Context, th *material.Theme, m theme.Metrics, label, icon string, accent color.NRGBA) layout.Dimensions {
	return layout.Inset{Top: m.InlineGap / 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: m.InlineGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					size := min(gtx.Dp(m.InlineGap), gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
					paint.FillShape(gtx.Ops, accent, clip.Ellipse(image.Rect(0, 0, size, size)).Op(gtx.Ops))
					return layout.Dimensions{Size: image.Pt(size, size)}
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return w.iconLabel(gtx, th, m, icon, label, th.Fg)
			}),
		)
	})
}

func surface(gtx layout.Context, background color.NRGBA, radius unit.Dp, content layout.Widget) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dims := content(gtx)
	call := recording.Stop()
	area := clip.UniformRRect(image.Rectangle{Max: dims.Size}, gtx.Dp(radius)).Push(gtx.Ops)
	paint.Fill(gtx.Ops, background)
	call.Add(gtx.Ops)
	area.Pop()
	return dims
}

// feedDivider paints a hairline rule beneath the content instead of a tinted
// card surface, so a compactFeed row carries noticeably less grey mass than
// the default card rendering.
func feedDivider(gtx layout.Context, line color.NRGBA, content layout.Widget) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dims := content(gtx)
	call := recording.Stop()
	call.Add(gtx.Ops)
	height := gtx.Dp(1)
	if height > dims.Size.Y {
		height = dims.Size.Y
	}
	if height > 0 && dims.Size.X > 0 {
		paint.FillShape(gtx.Ops, line, clip.Rect(image.Rect(0, dims.Size.Y-height, dims.Size.X, dims.Size.Y)).Op())
	}
	return dims
}
