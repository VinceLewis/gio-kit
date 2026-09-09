package shell

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

type Widget struct {
	Model      Model
	Mode       Mode
	Breakpoint unit.Dp
	OnNavigate func(string)
	OnControl  func(string)

	menu       widget.Clickable
	menuIcon   *widget.Icon
	drawerOpen bool
	navigation map[string]*widget.Clickable
	controls   map[string]*widget.Clickable
	drawerList widget.List
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
	return surface(gtx, theme.Palette.Bg, func(gtx layout.Context) layout.Dimensions {
		if w.ResolvedMode(gtx) == ModeWide {
			w.drawerOpen = false
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(unit.Dp(240)), gtx.Dp(unit.Dp(240))
					return w.drawer(gtx, theme, false)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return w.main(gtx, theme, false, content) }),
			)
		}
		return w.main(gtx, theme, true, func(gtx layout.Context) layout.Dimensions {
			if w.drawerOpen {
				return w.drawer(gtx, theme, true)
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
	return surface(gtx, theme.Palette.ContrastBg, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(w.Model.TopBar)+2)
		if compact {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				button := material.IconButton(theme, &w.menu, w.menuIcon, "Navigation")
				button.Size, button.Inset = unit.Dp(24), layout.UniformInset(unit.Dp(12))
				return button.Layout(gtx)
			}))
		}
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := material.H6(theme, w.Model.Title)
			label.Color = theme.Palette.ContrastFg
			label.MaxLines = 1
			return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(8)}.Layout(gtx, label.Layout)
		}))
		for _, item := range w.Model.TopBar {
			control := item
			if compact && control.Icon == nil {
				continue
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return w.control(gtx, theme, control, true)
			}))
		}
		return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}

func (w *Widget) drawer(gtx layout.Context, theme *material.Theme, compact bool) layout.Dimensions {
	controls := append([]Control(nil), w.Model.Drawer...)
	if compact {
		for _, control := range w.Model.TopBar {
			if control.Icon == nil {
				controls = append(controls, control)
			}
		}
	}
	count := len(w.Model.Navigation) + len(controls) + 1
	return surface(gtx, theme.Palette.Bg, func(gtx layout.Context) layout.Dimensions {
		return material.List(theme, &w.drawerList).Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
			if index == 0 {
				return layout.UniformInset(unit.Dp(16)).Layout(gtx, material.H6(theme, w.Model.DrawerTitle).Layout)
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
	if index == 0 || item.Group != w.Model.Navigation[index-1].Group {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Caption(theme, item.Group)
				return layout.Inset{Top: unit.Dp(12), Left: unit.Dp(16), Bottom: unit.Dp(2)}.Layout(gtx, label.Layout)
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
	return surface(gtx, background, func(gtx layout.Context) layout.Dimensions {
		return material.Clickable(gtx, click, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, 2)
				if item.Icon != nil {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min, gtx.Constraints.Max = image.Pt(gtx.Dp(unit.Dp(24)), gtx.Dp(unit.Dp(24))), image.Pt(gtx.Dp(unit.Dp(24)), gtx.Dp(unit.Dp(24)))
						return layout.Inset{Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return item.Icon.Layout(gtx, foreground) })
					}))
				}
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					label := material.Body1(theme, item.Label)
					label.Color, label.MaxLines = foreground, 1
					return label.Layout(gtx)
				}))
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
			})
		})
	})
}

func (w *Widget) control(gtx layout.Context, theme *material.Theme, control Control, top bool) layout.Dimensions {
	click := w.clickable(w.controls, control.ID)
	disabled := !control.Enabled
	if !control.Enabled {
		gtx = gtx.Disabled()
	}
	if top && control.Icon != nil {
		button := material.IconButton(theme, click, control.Icon, control.Label)
		button.Size, button.Inset = unit.Dp(24), layout.UniformInset(unit.Dp(12))
		return button.Layout(gtx)
	}
	button := material.Button(theme, click, control.Label)
	button.Inset = layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12), Left: unit.Dp(16), Right: unit.Dp(16)}
	if !top {
		button.Background = color.NRGBA{}
		button.Color = theme.Palette.Fg
	}
	if top || !disabled || control.DisabledReason == "" {
		return button.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(button.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			message := material.Caption(theme, control.DisabledReason)
			return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Bottom: unit.Dp(8)}.Layout(gtx, message.Layout)
		}),
	)
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
