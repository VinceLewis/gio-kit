package shell

import (
	"image"
	"image/color"

	"gioui.org/font"
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
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type Widget struct {
	Model      Model
	Mode       Mode
	Breakpoint unit.Dp
	OnNavigate func(string)
	OnControl  func(string)
	// BarBackground and BarForeground optionally separate the app bar from
	// the theme's primary action colors. An alpha of zero preserves the
	// corresponding ContrastBg or ContrastFg color from the supplied theme.
	BarBackground, BarForeground color.NRGBA

	menu             widget.Clickable
	menuIcon         *widget.Icon
	drawerOpen       bool
	navigation       map[string]*widget.Clickable
	controls         map[string]*widget.Clickable
	drawerList       widget.List
	barControls      []barControl
	overflowControls []Control
}

type barControl struct {
	control Control
	width   int
}

func NewWidget(model Model) (*Widget, error) {
	if err := model.Validate(); err != nil {
		return nil, err
	}
	menuIcon, err := widget.NewIcon(icons.NavigationMenu)
	if err != nil {
		return nil, err
	}
	return &Widget{
		Model: model, Breakpoint: 720, menuIcon: menuIcon,
		navigation: make(map[string]*widget.Clickable), controls: make(map[string]*widget.Clickable),
		drawerList: widget.List{List: layout.List{Axis: layout.Vertical}},
	}, nil
}

func (w *Widget) SetModel(model Model) error {
	if err := model.Validate(); err != nil {
		return err
	}
	w.Model = model
	return nil
}

func (w *Widget) DrawerOpen() bool { return w.drawerOpen }
func (w *Widget) CloseDrawer()     { w.drawerOpen = false }

func (w *Widget) ResolvedMode(gtx layout.Context) Mode {
	pxPerDp := gtx.Metric.PxPerDp
	if pxPerDp <= 0 {
		pxPerDp = 1
	}
	return ResolveMode(w.Mode, unit.Dp(float32(gtx.Constraints.Max.X)/pxPerDp), w.Breakpoint)
}

func (w *Widget) Layout(gtx layout.Context, theme *material.Theme, content layout.Widget) layout.Dimensions {
	if w == nil || theme == nil || content == nil {
		return layout.Dimensions{}
	}
	w.handleClicks(gtx)
	wide := w.ResolvedMode(gtx) == ModeWide
	drawerWidth := min(gtx.Dp(unit.Dp(240)), gtx.Constraints.Max.X/2)
	barContext := gtx
	if wide {
		barContext.Constraints.Max.X -= drawerWidth
	}
	w.allocateBar(barContext, !wide)
	return surface(gtx, theme.Palette.Bg, func(gtx layout.Context) layout.Dimensions {
		if wide {
			w.drawerOpen = false
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = drawerWidth, drawerWidth
					gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
					return w.drawer(gtx, theme)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return w.main(gtx, theme, false, content) }),
			)
		}
		return w.main(gtx, theme, true, func(gtx layout.Context) layout.Dimensions {
			if w.drawerOpen {
				return w.drawer(gtx, theme)
			}
			return content(gtx)
		})
	})
}

func (w *Widget) main(gtx layout.Context, theme *material.Theme, compact bool, content layout.Widget) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.topBar(gtx, theme, compact) }),
		layout.Flexed(1, content),
	)
}

func (w *Widget) topBar(gtx layout.Context, theme *material.Theme, compact bool) layout.Dimensions {
	barTheme := w.barTheme(theme)
	theme = &barTheme
	return surface(gtx, theme.Palette.ContrastBg, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(w.barControls)+2)
		if compact {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				button := material.IconButton(theme, &w.menu, w.menuIcon, "Navigation")
				button.Size, button.Inset = unit.Dp(24), layout.UniformInset(unit.Dp(12))
				return button.Layout(gtx)
			}))
		}
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			title := w.Model.PageTitle
			if title == "" {
				title = w.Model.Title
			}
			label := material.Body1(theme, title)
			label.Font.Weight = font.Bold
			label.Color = theme.Palette.ContrastFg
			label.MaxLines, label.Truncator = 1, "…"
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return (accessibility.Group{Description: w.Model.Title}).Layout(gtx, label.Layout)
			})
		}))
		for _, item := range w.barControls {
			item := item
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = item.width, item.width
				return w.control(gtx, theme, item.control, true)
			}))
		}
		return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}

func (w *Widget) barTheme(theme *material.Theme) material.Theme {
	bar := *theme
	if w.BarBackground.A != 0 {
		bar.Palette.ContrastBg = w.BarBackground
	}
	if w.BarForeground.A != 0 {
		bar.Palette.ContrastFg = w.BarForeground
	}
	return bar
}

// Keep space for the destination title before allocating controls. Overflow
// remains available in the independently scrolling drawer in either mode.
func (w *Widget) allocateBar(gtx layout.Context, compact bool) {
	w.barControls = w.barControls[:0]
	w.overflowControls = w.overflowControls[:0]
	available := max(0, gtx.Constraints.Max.X-gtx.Dp(unit.Dp(8)))
	if compact {
		available = max(0, available-gtx.Dp(unit.Dp(48)))
	}
	reservedTitle := min(gtx.Dp(unit.Dp(144)), available/3)
	remaining := available - reservedTitle
	for _, control := range w.Model.TopBar {
		width := gtx.Dp(unit.Dp(48))
		if control.CompactLabel != "" || control.Icon == nil {
			width = gtx.Dp(unit.Dp(144))
		}
		if (compact && control.Icon == nil && control.CompactLabel == "") || remaining < width {
			w.overflowControls = append(w.overflowControls, control)
			continue
		}
		remaining -= width
		w.barControls = append(w.barControls, barControl{control: control, width: width})
	}
}

func (w *Widget) drawer(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	controls := append([]Control(nil), w.Model.Drawer...)
	controls = append(controls, w.overflowControls...)
	count := len(w.Model.Navigation) + len(controls) + 1
	return surface(gtx, theme.Palette.Bg, func(gtx layout.Context) layout.Dimensions {
		return material.List(theme, &w.drawerList).Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
			// List children receive an unbounded vertical maximum. Clear the
			// viewport minimum so every row keeps its own intrinsic height.
			gtx.Constraints.Min.Y = 0
			if index == 0 {
				if w.Model.DrawerTitle == "" {
					return layout.Dimensions{}
				}
				label := material.Body1(theme, w.Model.DrawerTitle)
				label.Font.Weight, label.MaxLines, label.Truncator = font.Bold, 2, "…"
				return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(8), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, label.Layout)
			}
			index--
			if index < len(w.Model.Navigation) {
				return w.navigationItem(gtx, theme, w.Model.Navigation[index], index)
			}
			return w.control(gtx, theme, controls[index-len(w.Model.Navigation)], false)
		})
	})
}

func (w *Widget) navigationItem(gtx layout.Context, theme *material.Theme, item Item, index int) layout.Dimensions {
	if item.Group != "" && (index == 0 || item.Group != w.Model.Navigation[index-1].Group) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Caption(theme, item.Group)
				label.MaxLines, label.Truncator = 2, "…"
				return layout.Inset{Top: unit.Dp(12), Left: unit.Dp(16), Right: unit.Dp(16), Bottom: unit.Dp(2)}.Layout(gtx, label.Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.itemButton(gtx, theme, item) }),
		)
	}
	return w.itemButton(gtx, theme, item)
}

func (w *Widget) itemButton(gtx layout.Context, theme *material.Theme, item Item) layout.Dimensions {
	click := w.clickable(w.navigation, item.ID)
	if !item.Enabled {
		gtx = gtx.Disabled()
	}
	background := color.NRGBA{}
	foreground := theme.Palette.Fg
	if item.Selected {
		background, foreground = theme.Palette.ContrastBg, theme.Palette.ContrastFg
	}
	if !gtx.Enabled() {
		foreground.A = uint8(uint16(foreground.A) * 2 / 3)
	}
	gtx.Constraints.Min.Y = max(gtx.Constraints.Min.Y, gtx.Dp(unit.Dp(48)))
	return surface(gtx, background, func(gtx layout.Context) layout.Dimensions {
		dims := semanticClickable(gtx, click, item.Selected, func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(item.Label).Add(gtx.Ops)
			semantic.DescriptionOp(item.DisabledReason).Add(gtx.Ops)
			semantic.SelectedOp(item.Selected).Add(gtx.Ops)
			return centeredRow(gtx, layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(16), Right: unit.Dp(16)}, func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, 2)
				if item.Icon != nil {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return iconLayout(gtx, item.Icon, foreground) })
					}))
				}
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(theme, item.Label)
					label.Color, label.MaxLines, label.Truncator = foreground, 2, "…"
					if item.Selected {
						label.Font.Weight = font.Bold
					}
					return label.Layout(gtx)
				}))
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
			})
		})
		if item.Selected {
			selectionMark(gtx, dims, foreground, true)
		}
		return dims
	})
}

func (w *Widget) control(gtx layout.Context, theme *material.Theme, control Control, top bool) layout.Dimensions {
	click := w.clickable(w.controls, control.ID)
	disabled := !control.Enabled || !gtx.Enabled()
	if !control.Enabled {
		gtx = gtx.Disabled()
	}
	foreground := theme.Palette.Fg
	if top {
		foreground = theme.Palette.ContrastFg
	}
	if disabled {
		foreground.A = uint8(uint16(foreground.A) * 2 / 3)
	}
	controlRow := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = max(gtx.Constraints.Min.Y, gtx.Dp(unit.Dp(48)))
		dims := semanticClickable(gtx, click, control.Selected, func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(control.Label).Add(gtx.Ops)
			semantic.DescriptionOp(control.DisabledReason).Add(gtx.Ops)
			semantic.SelectedOp(control.Selected).Add(gtx.Ops)
			if top && control.Icon != nil && control.CompactLabel == "" {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return iconLayout(gtx, control.Icon, foreground) })
			}
			inset := layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(16), Right: unit.Dp(16)}
			if top {
				inset.Left, inset.Right = unit.Dp(8), unit.Dp(8)
			}
			return centeredRow(gtx, inset, func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, 2)
				if control.Icon != nil {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return iconLayout(gtx, control.Icon, foreground)
						})
					}))
				}
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					visible := control.Label
					if top && control.CompactLabel != "" {
						visible = control.CompactLabel
					}
					label := material.Body2(theme, visible)
					label.Color, label.MaxLines, label.Truncator = foreground, 2, "…"
					if top {
						label.MaxLines = 1
					}
					if control.Selected {
						label.Font.Weight = font.Bold
					}
					return label.Layout(gtx)
				}))
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
			})
		})
		if control.Selected {
			selectionMark(gtx, dims, foreground, !top)
		}
		return dims
	}
	if top || !disabled || control.DisabledReason == "" {
		return controlRow(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(controlRow),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			message := material.Caption(theme, control.DisabledReason)
			message.Alignment = text.Start
			message.MaxLines = 4
			return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Bottom: unit.Dp(8)}.Layout(gtx, message.Layout)
		}),
	)
}

func centeredRow(gtx layout.Context, inset layout.Inset, content layout.Widget) layout.Dimensions {
	// Keep the outer touch target while measuring the icon and label at their
	// intrinsic heights. A label otherwise reports an inherited minimum height
	// although its glyphs still begin at the top, misaligning the visible row.
	return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = 0
		return inset.Layout(gtx, content)
	})
}

func semanticClickable(gtx layout.Context, click *widget.Clickable, selected bool, content layout.Widget) layout.Dimensions {
	if !gtx.Enabled() {
		// Gio omits semantics on disabled input regions without filters. A
		// measured region with no input registration retains the button's
		// accessible role, name and state while rejecting all interaction.
		return (accessibility.Group{Disabled: true, Selected: selected}).Layout(gtx, content)
	}
	return material.Clickable(gtx, click, content)
}

func iconLayout(gtx layout.Context, icon *widget.Icon, foreground color.NRGBA) layout.Dimensions {
	size := min(gtx.Dp(unit.Dp(20)), gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	return icon.Layout(gtx, foreground)
}

func selectionMark(gtx layout.Context, dims layout.Dimensions, foreground color.NRGBA, vertical bool) {
	mark := image.Rect(gtx.Dp(unit.Dp(8)), dims.Size.Y-gtx.Dp(unit.Dp(3)), dims.Size.X-gtx.Dp(unit.Dp(8)), dims.Size.Y)
	if vertical {
		mark = image.Rect(0, gtx.Dp(unit.Dp(8)), gtx.Dp(unit.Dp(3)), dims.Size.Y-gtx.Dp(unit.Dp(8)))
	}
	paint.FillShape(gtx.Ops, foreground, clip.Rect(mark).Op())
}

func (w *Widget) handleClicks(gtx layout.Context) {
	if w.menu.Clicked(gtx) {
		w.drawerOpen = !w.drawerOpen
	}
	for _, item := range w.Model.Navigation {
		if item.Enabled && w.clickable(w.navigation, item.ID).Clicked(gtx) {
			w.drawerOpen = false
			if w.OnNavigate != nil {
				w.OnNavigate(item.ID)
			}
		}
	}
	for _, controls := range [][]Control{w.Model.TopBar, w.Model.Drawer} {
		for _, control := range controls {
			if control.Enabled && w.clickable(w.controls, control.ID).Clicked(gtx) && w.OnControl != nil {
				w.drawerOpen = false
				w.OnControl(control.ID)
			}
		}
	}
}

func (w *Widget) clickable(values map[string]*widget.Clickable, id string) *widget.Clickable {
	if values[id] == nil {
		values[id] = new(widget.Clickable)
	}
	return values[id]
}

func surface(gtx layout.Context, fill color.NRGBA, content layout.Widget) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dimensions := content(gtx)
	call := recording.Stop()
	paint.FillShape(gtx.Ops, fill, clip.Rect(image.Rectangle{Max: dimensions.Size}).Op())
	call.Add(gtx.Ops)
	return dimensions
}
