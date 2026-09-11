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

	menu                                  widget.Clickable
	menuIcon                              *widget.Icon
	closeIcon                             *widget.Icon
	drawerOpen                            bool
	navigation                            map[string]*widget.Clickable
	controls                              map[string]*widget.Clickable
	toggles                               map[string]*widget.Bool
	navigationFocus, controlFocus         map[string]bool
	drawerList                            widget.List
	barControls                           []barControl
	overflowControls                      []Control
	navigationRowHeight, utilityRowHeight int
	groupHeadingHeight, helpHeight        int
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
	closeIcon, err := widget.NewIcon(icons.NavigationClose)
	if err != nil {
		return nil, err
	}
	return &Widget{
		Model: model, Breakpoint: 720, menuIcon: menuIcon, closeIcon: closeIcon,
		navigation: make(map[string]*widget.Clickable), controls: make(map[string]*widget.Clickable), toggles: make(map[string]*widget.Bool),
		navigationFocus: make(map[string]bool), controlFocus: make(map[string]bool),
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
				icon, label := w.menuIcon, w.Model.OpenNavigationLabel
				if w.drawerOpen {
					icon, label = w.closeIcon, w.Model.CloseNavigationLabel
				}
				if label == "" {
					label = w.Model.DrawerTitle
				}
				button := material.IconButton(theme, &w.menu, icon, label)
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
			return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return (accessibility.Group{Description: w.Model.Title}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return boundedText(gtx, unit.Dp(32), label.Layout)
				})
			})
		}))
		for _, item := range w.barControls {
			item := item
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = item.width, item.width
				return w.control(gtx, theme, item.control, true, false)
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
		if control.Kind == ControlToggle || control.CompactLabel != "" || control.Icon == nil {
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
	reasons := make(map[string]int)
	for _, control := range controls {
		if reason := visibleDisabledReason(control, w.Model.UnavailableSummary); !control.Enabled && reason != "" {
			reasons[reason]++
		}
	}
	repeatedReasons := 0
	if w.Model.UnavailableSummary != "" {
		for _, count := range reasons {
			if count > 1 {
				repeatedReasons++
			}
		}
	}
	sharedUnavailable := repeatedReasons == 1
	utilityHeader := len(controls) > 0
	count := 1 + len(w.Model.Navigation) + len(controls)
	if utilityHeader {
		count++
	}
	if sharedUnavailable {
		count++
	}
	return surface(gtx, theme.Palette.Bg, func(gtx layout.Context) layout.Dimensions {
		list := material.List(theme, &w.drawerList)
		list.Track.MinorPadding = unit.Dp(3)
		list.Indicator.MinorWidth = unit.Dp(3)
		list.Indicator.CornerRadius = unit.Dp(1.5)
		list.Indicator.Color = alpha(theme.Palette.Fg, 96)
		list.Indicator.HoverColor = alpha(theme.Palette.Fg, 160)
		return list.Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
			// List children receive an unbounded vertical maximum. Clear the
			// viewport minimum so every row keeps its own intrinsic height.
			gtx.Constraints.Min.Y = 0
			if index == 0 {
				if w.Model.DrawerTitle == "" {
					return layout.Dimensions{}
				}
				label := material.Body1(theme, w.Model.DrawerTitle)
				label.Font.Weight, label.MaxLines, label.Truncator = font.Bold, 2, "…"
				return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(8), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return boundedText(gtx, unit.Dp(56), label.Layout)
				})
			}
			index--
			if index < len(w.Model.Navigation) {
				return w.navigationItem(gtx, theme, w.Model.Navigation[index], index)
			}
			index -= len(w.Model.Navigation)
			if utilityHeader {
				if index == 0 {
					return w.utilityHeader(gtx, theme, len(w.Model.Navigation) > 0)
				}
				index--
			}
			if sharedUnavailable {
				if index == 0 {
					var reason string
					for candidate, count := range reasons {
						if count > 1 {
							reason = candidate
							break
						}
					}
					dims := w.supportingText(gtx, theme, reason, unit.Dp(6), unit.Dp(8))
					w.helpHeight = max(w.helpHeight, dims.Size.Y)
					return dims
				}
				index--
			}
			control := controls[index]
			showReason := reasons[visibleDisabledReason(control, w.Model.UnavailableSummary)] < 2 || !sharedUnavailable
			return w.control(gtx, theme, control, false, showReason)
		})
	})
}

func (w *Widget) utilityHeader(gtx layout.Context, theme *material.Theme, separated bool) layout.Dimensions {
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !separated {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				height := max(1, gtx.Dp(unit.Dp(1)))
				width := gtx.Constraints.Max.X
				paint.FillShape(gtx.Ops, alpha(theme.Palette.Fg, 48), clip.Rect{Max: image.Pt(width, height)}.Op())
				return layout.Dimensions{Size: image.Pt(width, height)}
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if w.Model.UtilityHeading == "" {
				return layout.Dimensions{}
			}
			label := material.Caption(theme, w.Model.UtilityHeading)
			label.Font.Weight, label.MaxLines, label.Truncator = font.Bold, 2, "…"
			return layout.Inset{Bottom: unit.Dp(2), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return boundedText(gtx, unit.Dp(40), label.Layout)
			})
		}),
	)
	w.groupHeadingHeight = max(w.groupHeadingHeight, dims.Size.Y)
	return dims
}

func (w *Widget) supportingText(gtx layout.Context, theme *material.Theme, value string, top, bottom unit.Dp) layout.Dimensions {
	label := material.Caption(theme, value)
	label.Alignment, label.MaxLines, label.Truncator = text.Start, 4, "…"
	return layout.Inset{Top: top, Bottom: bottom, Left: unit.Dp(48), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return boundedText(gtx, unit.Dp(72), label.Layout)
	})
}

func (w *Widget) navigationItem(gtx layout.Context, theme *material.Theme, item Item, index int) layout.Dimensions {
	if item.Group != "" && (index == 0 || item.Group != w.Model.Navigation[index-1].Group) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Caption(theme, item.Group)
				label.MaxLines, label.Truncator = 2, "…"
				dims := layout.Inset{Top: unit.Dp(12), Left: unit.Dp(16), Right: unit.Dp(16), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return boundedText(gtx, unit.Dp(40), label.Layout)
				})
				w.groupHeadingHeight = max(w.groupHeadingHeight, dims.Size.Y)
				return dims
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.itemButton(gtx, theme, item) }),
		)
	}
	return w.itemButton(gtx, theme, item)
}

func (w *Widget) itemButton(gtx layout.Context, theme *material.Theme, item Item) layout.Dimensions {
	click := w.clickable(w.navigation, item.ID)
	w.navigationFocus[item.ID] = gtx.Focused(click)
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
	dims := surface(gtx, background, func(gtx layout.Context) layout.Dimensions {
		dims := semanticClickable(gtx, click, item.Selected, foreground, func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(item.Label).Add(gtx.Ops)
			semantic.DescriptionOp(item.DisabledReason).Add(gtx.Ops)
			semantic.SelectedOp(item.Selected).Add(gtx.Ops)
			return centeredRow(gtx, layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(16), Right: unit.Dp(16)}, func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, 2)
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return drawerIconColumn(gtx, item.Icon, foreground)
				}))
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(theme, item.Label)
					label.Color, label.MaxLines, label.Truncator = foreground, 2, "…"
					if item.Selected {
						label.Font.Weight = font.Bold
					}
					return boundedText(gtx, unit.Dp(44), label.Layout)
				}))
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
			})
		})
		if item.Selected {
			selectionMark(gtx, dims, foreground, true)
		}
		return dims
	})
	w.navigationRowHeight = max(w.navigationRowHeight, dims.Size.Y)
	return dims
}

func (w *Widget) control(gtx layout.Context, theme *material.Theme, control Control, top, showReason bool) layout.Dimensions {
	if control.Kind == ControlToggle {
		return w.toggleControl(gtx, theme, control, top, showReason)
	}
	click := w.clickable(w.controls, control.ID)
	w.controlFocus[control.ID] = gtx.Focused(click)
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
		dims := semanticClickable(gtx, click, control.Selected, foreground, func(gtx layout.Context) layout.Dimensions {
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
				if !top {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return drawerIconColumn(gtx, control.Icon, foreground)
					}))
				} else if control.Icon != nil {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return iconLayout(gtx, control.Icon, foreground) })
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
					return boundedText(gtx, unit.Dp(44), label.Layout)
				}))
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
			})
		})
		if control.Selected {
			selectionMark(gtx, dims, foreground, !top)
		}
		return dims
	}
	if top || !disabled || control.DisabledReason == "" || !showReason {
		dims := controlRow(gtx)
		if !top {
			w.utilityRowHeight = max(w.utilityRowHeight, dims.Size.Y)
		}
		return dims
	}
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(controlRow),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dims := w.supportingText(gtx, theme, visibleDisabledReason(control, w.Model.UnavailableSummary), 0, unit.Dp(8))
			w.helpHeight = max(w.helpHeight, dims.Size.Y)
			return dims
		}),
	)
	w.utilityRowHeight = max(w.utilityRowHeight, dims.Size.Y)
	return dims
}

func (w *Widget) toggleControl(gtx layout.Context, theme *material.Theme, control Control, top, showReason bool) layout.Dimensions {
	state := w.toggle(control.ID)
	w.controlFocus[control.ID] = gtx.Focused(state)
	state.Value = control.Value
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
	description := control.ValueLabel
	if control.DisabledReason != "" {
		if description != "" {
			description += ". "
		}
		description += control.DisabledReason
	}
	row := func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = max(gtx.Constraints.Min.Y, gtx.Dp(unit.Dp(48)))
		content := func(gtx layout.Context) layout.Dimensions {
			semantic.Switch.Add(gtx.Ops)
			semantic.LabelOp(control.Label).Add(gtx.Ops)
			semantic.DescriptionOp(description).Add(gtx.Ops)
			semantic.SelectedOp(control.Value).Add(gtx.Ops)
			inset := layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(16), Right: unit.Dp(16)}
			if top {
				inset.Left, inset.Right = unit.Dp(8), unit.Dp(8)
			}
			return centeredRow(gtx, inset, func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, 3)
				if !top {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return drawerIconColumn(gtx, control.Icon, foreground)
					}))
				} else if control.Icon != nil {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return iconLayout(gtx, control.Icon, foreground) })
					}))
				}
				children = append(children,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := material.Body2(theme, control.Label)
								label.Color, label.MaxLines, label.Truncator = foreground, 2, "…"
								return boundedText(gtx, unit.Dp(44), label.Layout)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if control.ValueLabel == "" {
									return layout.Dimensions{}
								}
								value := material.Caption(theme, control.ValueLabel)
								value.Color, value.MaxLines, value.Truncator = foreground, 1, "…"
								return boundedText(gtx, unit.Dp(24), value.Layout)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return toggleIndicator(gtx, theme, state)
						})
					}),
				)
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
			})
		}
		if !gtx.Enabled() {
			return (accessibility.Group{Disabled: true, Selected: control.Value}).Layout(gtx, content)
		}
		return state.Layout(gtx, content)
	}
	var dims layout.Dimensions
	if top || !disabled || control.DisabledReason == "" || !showReason {
		dims = row(gtx)
	} else {
		dims = layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(row),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				dims := w.supportingText(gtx, theme, visibleDisabledReason(control, w.Model.UnavailableSummary), 0, unit.Dp(8))
				w.helpHeight = max(w.helpHeight, dims.Size.Y)
				return dims
			}),
		)
	}
	if !top {
		w.utilityRowHeight = max(w.utilityRowHeight, dims.Size.Y)
	}
	return dims
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

func visibleDisabledReason(control Control, fallback string) string {
	if control.Availability != "" {
		return control.Availability
	}
	if control.DisabledReason != "" {
		return control.DisabledReason
	}
	return fallback
}

func semanticClickable(gtx layout.Context, click *widget.Clickable, selected bool, foreground color.NRGBA, content layout.Widget) layout.Dimensions {
	if !gtx.Enabled() {
		// Gio omits semantics on disabled input regions without filters. A
		// measured region with no input registration retains the button's
		// accessible role, name and state while rejecting all interaction.
		return (accessibility.Group{Disabled: true, Selected: selected}).Layout(gtx, content)
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				var opacity uint8
				switch {
				case click.Pressed():
					opacity = 44
				case gtx.Focused(click):
					opacity = 32
				case click.Hovered():
					opacity = 20
				}
				if opacity != 0 {
					paint.FillShape(gtx.Ops, alpha(foreground, opacity), clip.Rect{Max: gtx.Constraints.Min}.Op())
				}
				return layout.Dimensions{Size: gtx.Constraints.Min}
			},
			content,
		)
	})
}

func drawerIconColumn(gtx layout.Context, icon *widget.Icon, foreground color.NRGBA) layout.Dimensions {
	width := min(gtx.Dp(unit.Dp(32)), gtx.Constraints.Max.X)
	if icon == nil {
		return layout.Dimensions{Size: image.Pt(width, 0)}
	}
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
	gtx.Constraints.Min.Y = 0
	return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return iconLayout(gtx, icon, foreground)
	})
}

func boundedText(gtx layout.Context, maxHeight unit.Dp, textWidget layout.Widget) layout.Dimensions {
	maxWidth := gtx.Constraints.Max.X
	if gtx.Constraints.Min.X > 0 {
		// A Flexed child communicates its assigned width through Min.X while
		// retaining the parent's larger Max.X. Preserve that assigned width as
		// the text maximum before BoundedText clears inherited minimums.
		maxWidth = min(maxWidth, gtx.Constraints.Min.X)
	}
	maximum := image.Pt(min(maxWidth, gtx.Dp(unit.Dp(1024))), min(gtx.Constraints.Max.Y, gtx.Dp(maxHeight)))
	return accessibility.BoundedText(gtx, maximum, textWidget)
}

func toggleIndicator(gtx layout.Context, theme *material.Theme, state *widget.Bool) layout.Dimensions {
	width, height := gtx.Dp(unit.Dp(36)), gtx.Dp(unit.Dp(28))
	trackHeight, thumb := gtx.Dp(unit.Dp(16)), gtx.Dp(unit.Dp(20))
	width, height = min(width, gtx.Constraints.Max.X), min(height, gtx.Constraints.Max.Y)
	trackHeight, thumb = min(trackHeight, height), min(thumb, height)
	track := image.Rect(0, (height-trackHeight)/2, width, (height+trackHeight)/2)
	trackColor := alpha(theme.Palette.Fg, 96)
	thumbColor := theme.Palette.Bg
	if state.Value {
		trackColor = alpha(theme.Palette.ContrastBg, 144)
		thumbColor = theme.Palette.ContrastBg
	}
	if !gtx.Enabled() {
		trackColor = alpha(trackColor, 128)
		thumbColor = alpha(theme.Palette.Fg, 128)
	}
	paint.FillShape(gtx.Ops, trackColor, clip.UniformRRect(track, trackHeight/2).Op(gtx.Ops))
	x := thumb / 2
	if state.Value {
		x = width - thumb/2
	}
	center := image.Pt(x, height/2)
	if state.Pressed() || state.Hovered() || gtx.Focused(state) {
		opacity := uint8(24)
		if state.Pressed() {
			opacity = 44
		} else if gtx.Focused(state) {
			opacity = 32
		}
		haloRadius := min(height/2, gtx.Dp(unit.Dp(14)))
		halo := image.Rect(center.X-haloRadius, center.Y-haloRadius, center.X+haloRadius, center.Y+haloRadius)
		paint.FillShape(gtx.Ops, alpha(theme.Palette.ContrastBg, opacity), clip.Ellipse(halo).Op(gtx.Ops))
	}
	radius := thumb / 2
	thumbRect := image.Rect(center.X-radius, center.Y-radius, center.X+radius, center.Y+radius)
	paint.FillShape(gtx.Ops, thumbColor, clip.Ellipse(thumbRect).Op(gtx.Ops))
	return layout.Dimensions{Size: image.Pt(width, height)}
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
			if !control.Enabled {
				continue
			}
			activated := false
			if control.Kind == ControlToggle {
				toggle := w.toggle(control.ID)
				toggle.Value = control.Value
				activated = toggle.Update(gtx)
			} else {
				activated = w.clickable(w.controls, control.ID).Clicked(gtx)
			}
			if activated {
				if control.Dismissal == DismissDrawer {
					w.drawerOpen = false
				}
				if w.OnControl != nil {
					w.OnControl(control.ID)
				}
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

func (w *Widget) toggle(id string) *widget.Bool {
	if w.toggles[id] == nil {
		w.toggles[id] = new(widget.Bool)
	}
	return w.toggles[id]
}

func alpha(value color.NRGBA, opacity uint8) color.NRGBA {
	value.A = uint8(uint16(value.A) * uint16(opacity) / 255)
	return value
}

func surface(gtx layout.Context, fill color.NRGBA, content layout.Widget) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dimensions := content(gtx)
	call := recording.Stop()
	paint.FillShape(gtx.Ops, fill, clip.Rect(image.Rectangle{Max: dimensions.Size}).Op())
	call.Add(gtx.Ops)
	return dimensions
}
