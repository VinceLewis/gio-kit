package presentation

import (
	"image"
	"image/color"
	"strings"
	"unicode"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/accessibility"
	"github.com/VinceLewis/gio-kit/theme"
)

// Widget retains interaction and scroll state across immediate-mode frames.
type Widget struct {
	Page    Page
	OnEvent func(Event)
	// ResolveColor resolves application-owned theme tokens on each frame.
	// A zero colour leaves the supplied colour or theme fallback in effect.
	ResolveColor func(string) color.NRGBA
	// Metrics supplies catalog-backed profiles. The zero value uses the
	// built-in theme profiles, preserving existing callers' geometry.
	Metrics theme.Set
	// Density is the least specific declared density, typically from a
	// consumer's declared theme. Page/section/component values override it.
	Density string

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

// metrics resolves the declared-density precedence shared by every layout
// function in this package: component > section > page > Widget.Density >
// Set.Default. Callers pass their declarations most-specific-first, e.g.
// w.metrics(list.Density, section.Density, w.Page.Density, w.Density).
func (w *Widget) metrics(names ...string) theme.Metrics {
	return w.Metrics.Resolve(names...)
}

func (w *Widget) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if w == nil {
		return layout.Dimensions{}
	}
	if w.Page.Error != "" {
		return layout.Center.Layout(gtx, material.Body1(th, w.Page.Error).Layout)
	}
	if w.Page.Loading && len(w.Page.Sections) == 0 {
		return layout.Center.Layout(gtx, material.Body1(th, "Loading…").Layout)
	}
	count := len(w.Page.Sections)
	if len(w.Page.Legends) != 0 {
		count++
	}
	return material.List(th, &w.list).Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
		if index == len(w.Page.Sections) {
			return w.layoutLegends(gtx, th)
		}
		return w.layoutSection(gtx, th, w.Page.Sections[index])
	})
}

func (w *Widget) layoutSection(gtx layout.Context, th *material.Theme, section Section) layout.Dimensions {
	pm := w.metrics(w.Page.Density, w.Density)
	sm := w.metrics(section.Density, w.Page.Density, w.Density)
	return layout.Inset{Bottom: pm.PageGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{}
		if section.Heading != "" || len(section.Actions) != 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return w.layoutHeading(gtx, th, sm, section.Heading, section.Actions)
			}))
		}
		if section.Comment != "" {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: sm.SectionGap}.Layout(gtx, material.Body2(th, section.Comment).Layout)
			}))
		}
		if len(section.Controls) != 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutControls(gtx, th, sm, section.Controls) }))
		}
		for _, list := range section.Lists {
			list := list
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutList(gtx, th, list, section.Density) }))
		}
		for _, calendar := range section.Calendars {
			calendar := calendar
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return w.layoutCalendar(gtx, th, calendar, section.Density)
			}))
		}
		for _, matrix := range section.Matrices {
			matrix := matrix
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutMatrix(gtx, th, matrix, section.Density) }))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (w *Widget) layoutControls(gtx layout.Context, th *material.Theme, m theme.Metrics, controls []Control) layout.Dimensions {
	children := make([]layout.Widget, 0, len(controls))
	for _, control := range controls {
		control := control
		children = append(children, func(gtx layout.Context) layout.Dimensions {
			if !control.Enabled {
				gtx = gtx.Disabled()
			}
			switch control.Kind {
			case "toggle":
				return w.layoutToggle(gtx, th, m, control)
			case "select", "contextSelector":
				return w.layoutSelect(gtx, th, m, control)
			default:
				return material.Body1(th, control.Label).Layout(gtx)
			}
		})
	}
	return flowControls(gtx, m, children...)
}

func (w *Widget) layoutSelect(gtx layout.Context, th *material.Theme, m theme.Metrics, control Control) layout.Dimensions {
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
				return w.actionButton(gtx, th, m, button, Action{Label: label, Enabled: control.Enabled, DisabledReason: control.DisabledReason, Placement: "inline"})
			})
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(boundedLabel(th, m, control.Label, 2).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return flow(gtx, m.ControlGap, children...) }),
	)
}

func (w *Widget) layoutHeading(gtx layout.Context, th *material.Theme, m theme.Metrics, title string, actions []Action) layout.Dimensions {
	var children []layout.Widget
	if title != "" {
		children = append(children, func(gtx layout.Context) layout.Dimensions {
			if len(actions) != 0 {
				gtx.Constraints.Min.Y = min(gtx.Dp(theme.MinTouchTarget), gtx.Constraints.Max.Y)
			}
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return boundedHeading(gtx, heading(th, m, title).Layout)
			})
		})
	}
	for _, action := range actions {
		action := action
		children = append(children, func(gtx layout.Context) layout.Dimensions {
			return w.layoutActions(gtx, th, m, []Action{action}, "", "")
		})
	}
	return flow(gtx, m.ControlGap, children...)
}

// boundedHeading caps a heading's own measurement to a generous but finite
// height before painting it. A heading renders as an item inside this
// package's outer material.List (or as a Rigid child of one), which measures
// its main axis as effectively unbounded so the item can report its own
// natural height; without an explicit cap, gioui.org/widget.Label's own
// semantic clip area inherits that huge constraint as its accessible bounds,
// even though the painted text stays small. accessibility.BoundedText both
// supplies the finite cap accessibility tooling can rely on and clears the
// inherited height minimum, so a short heading still keeps its intrinsic
// height.
func boundedHeading(gtx layout.Context, content layout.Widget) layout.Dimensions {
	maxWidth := min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(2048)))
	maxHeight := min(gtx.Constraints.Max.Y, gtx.Dp(unit.Dp(400)))
	return accessibility.BoundedText(gtx, image.Pt(maxWidth, maxHeight), content)
}

func (w *Widget) layoutList(gtx layout.Context, th *material.Theme, list List, sectionDensity string) layout.Dimensions {
	m := w.metrics(list.Density, sectionDensity, w.Page.Density, w.Density)
	children := []layout.FlexChild{}
	if list.Heading != "" || len(list.Actions) != 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return w.layoutHeading(gtx, th, m, list.Heading, list.Actions)
		}))
	}
	if len(list.Rows) == 0 {
		message := list.EmptyText
		if message == "" {
			message = "No items"
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: m.SectionGap, Bottom: m.SectionGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return boundedHeading(gtx, boundedLabel(th, m, message, 3).Layout)
			})
		}))
	}
	rowMetrics := w.metrics(list.RowDensity, list.Density, sectionDensity, w.Page.Density, w.Density)
	for _, row := range list.Rows {
		row := row
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return w.layoutRow(gtx, th, rowMetrics, list.ID, list.Style, list.RowLayout, row)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// layoutRow dispatches to the card rendering (ListStyleDefault, table, cards)
// or the feed rendering (feed, compactFeed) selected by style. The two paths
// are provably different: the feed path drops its tinted card surface for a
// hairline divider, folds the status into the ordered content run, and
// filters empty/orphan-separator fragments before rendering.
func (w *Widget) layoutRow(gtx layout.Context, th *material.Theme, m theme.Metrics, owner, style, rowLayout string, row Row) layout.Dimensions {
	feed := style == ListStyleFeed || style == ListStyleCompactFeed
	label := rowAccessibleLabel(row, feed)
	return (accessibility.Group{Label: label}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if feed {
			return w.layoutFeedRow(gtx, th, m, rowLayout, row)
		}
		return w.layoutCardRow(gtx, th, m, row)
	})
}

// rowAccessibleLabel computes a row's whole-row accessible summary: its
// fragments (filtered the same way the feed style renders them, when feed is
// true) followed by its status, so semantics contain every available summary
// value and the status even when a caller never sets AccessibleLabel.
func rowAccessibleLabel(row Row, feed bool) string {
	if row.AccessibleLabel != "" {
		return row.AccessibleLabel
	}
	fragments := row.Fragments
	if feed {
		fragments = filterFeedFragments(fragments)
	}
	var pieces []string
	for _, fragment := range fragments {
		pieces = append(pieces, fragment.Text, fragment.AccessibleLabel)
	}
	if status := statusText(row); status != "" {
		pieces = append(pieces, status)
	}
	return strings.TrimSpace(strings.Join(pieces, " "))
}

func statusText(row Row) string {
	if row.StatusAccessibleLabel != "" {
		return row.StatusAccessibleLabel
	}
	return row.StatusLabel
}

// layoutCardRow keeps the pre-existing tinted-surface row rendering used by
// ListStyleDefault, table, and cards, byte-for-byte.
func (w *Widget) layoutCardRow(gtx layout.Context, th *material.Theme, m theme.Metrics, row Row) layout.Dimensions {
	return layout.Inset{Bottom: m.ListGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return surface(gtx, blend(th.Bg, th.Fg, 12), m.SurfaceRadius, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: m.RowPaddingY, Bottom: m.RowPaddingY, Left: m.RowPaddingX, Right: m.RowPaddingX}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.layoutCardFragments(gtx, th, m, row.Fragments) }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if row.StatusLabel == "" {
							return layout.Dimensions{}
						}
						return (accessibility.Group{Description: row.StatusAccessibleLabel}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return w.layoutStatus(gtx, th, m, row.StatusLabel, row.StatusIcon, w.resolveColor(row.ColorToken, row.Color, th.ContrastBg))
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return w.layoutActions(gtx, th, m, row.Actions, row.ID, "")
					}),
				)
			})
		})
	})
}

// layoutFeedRow renders a feed/compactFeed row: a hairline divider instead of
// a tinted card, no fixed row height, and the status folded into the ordered
// content run instead of a separate line.
func (w *Widget) layoutFeedRow(gtx layout.Context, th *material.Theme, m theme.Metrics, rowLayout string, row Row) layout.Dimensions {
	fragments := filterFeedFragments(row.Fragments)
	pieces := w.feedPieces(th, m, fragments)
	if row.StatusLabel != "" {
		accent := w.resolveColor(row.ColorToken, row.Color, th.ContrastBg)
		pieces = append(pieces, w.feedStatusPiece(th, m, row.StatusLabel, row.StatusIcon, row.StatusAccessibleLabel, accent))
	}
	line := blend(th.Bg, th.Fg, 18)
	return layout.Inset{Bottom: m.ListGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return feedDivider(gtx, line, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: m.RowPaddingY, Bottom: m.RowPaddingY, Left: m.RowPaddingX, Right: m.RowPaddingX}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				var children []layout.FlexChild
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if rowLayout == RowLayoutStack {
						stacked := make([]layout.FlexChild, len(pieces))
						for i, piece := range pieces {
							stacked[i] = layout.Rigid(piece)
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, stacked...)
					}
					// The inline default: one ordered rich-text run that
					// wraps only at the available width. A style change
					// between fragments never forces a break.
					return flow(gtx, m.InlineGap, pieces...)
				}))
				if len(row.Actions) != 0 {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return w.layoutActions(gtx, th, m, row.Actions, row.ID, "")
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		})
	})
}

// filterFeedFragments drops a fragment whose visible text is empty or
// whitespace-only, then collapses a separator fragment that is now adjacent
// to another separator or that sits at the start or end of the run. A
// separator is detected structurally: a Kind "text" fragment whose text has
// no letters or digits.
func filterFeedFragments(fragments []Fragment) []Fragment {
	kept := make([]Fragment, 0, len(fragments))
	for _, fragment := range fragments {
		if fragment.Kind != "icon" && strings.TrimSpace(fragment.Text) == "" {
			continue
		}
		kept = append(kept, fragment)
	}
	result := make([]Fragment, 0, len(kept))
	for _, fragment := range kept {
		if isSeparatorFragment(fragment) && (len(result) == 0 || isSeparatorFragment(result[len(result)-1])) {
			continue
		}
		result = append(result, fragment)
	}
	for len(result) > 0 && isSeparatorFragment(result[len(result)-1]) {
		result = result[:len(result)-1]
	}
	return result
}

func isSeparatorFragment(fragment Fragment) bool {
	if fragment.Kind != "text" {
		return false
	}
	for _, r := range fragment.Text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// feedPieces converts filtered fragments into ordered inline renderers. An
// icon fragment stays paired with the text fragment that follows it so the
// two travel together when the run wraps.
func (w *Widget) feedPieces(th *material.Theme, m theme.Metrics, fragments []Fragment) []layout.Widget {
	var pieces []layout.Widget
	var pendingIcons []Fragment
	for _, fragment := range fragments {
		fragment := fragment
		if fragment.Kind == "icon" {
			pendingIcons = append(pendingIcons, fragment)
			continue
		}
		icons := pendingIcons
		pendingIcons = nil
		pieces = append(pieces, w.feedFragmentPiece(th, m, fragment, icons))
	}
	for _, icon := range pendingIcons {
		icon := icon
		pieces = append(pieces, func(gtx layout.Context) layout.Dimensions { return w.layoutFragmentIcon(gtx, th, m, icon) })
	}
	return pieces
}

func (w *Widget) feedFragmentPiece(th *material.Theme, m theme.Metrics, fragment Fragment, icons []Fragment) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		style := boundedLabel(th, m, fragment.Text, 2)
		switch fragment.Style {
		case "bold":
			style = heading(th, m, fragment.Text)
		case "muted", "caption":
			style.TextSize = m.SecondarySize
		}
		var parts []layout.FlexChild
		for _, icon := range icons {
			icon := icon
			parts = append(parts, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Right: m.InlineGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return w.layoutFragmentIcon(gtx, th, m, icon)
				})
			}))
		}
		parts = append(parts, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return silentLabel(gtx, style) }))
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, parts...)
	}
}

// feedStatusPiece renders the row's status as one more piece in the ordered
// content run, keeping the accent colour and the accessible description
// while dropping the separate status line the card rendering uses.
func (w *Widget) feedStatusPiece(th *material.Theme, m theme.Metrics, label, icon, accessibleLabel string, accent color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return (accessibility.Group{Description: accessibleLabel}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return w.layoutStatus(gtx, th, m, label, icon, accent)
		})
	}
}

// layoutCardFragments keeps the pre-existing card-row fragment rendering:
// declared order, a bold value starting a primary line, subsequent summary
// values wrapping individually. Used only by the card rendering path.
func (w *Widget) layoutCardFragments(gtx layout.Context, th *material.Theme, m theme.Metrics, fragments []Fragment) layout.Dimensions {
	var children []layout.FlexChild
	var line []layout.Widget
	lineText := false
	var pendingIcons []Fragment
	flush := func() {
		if len(line) == 0 {
			return
		}
		items := line
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return flow(gtx, m.InlineGap/2, items...) }))
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
			style := boundedLabel(th, m, value, 2)
			if fragment.Style == "bold" {
				style = heading(th, m, value)
			}
			if fragment.Style == "muted" || fragment.Style == "caption" {
				style.TextSize = m.SecondarySize
			}
			var parts []layout.FlexChild
			for _, icon := range icons {
				icon := icon
				parts = append(parts, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: m.InlineGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return w.layoutFragmentIcon(gtx, th, m, icon)
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
		line = append(line, func(gtx layout.Context) layout.Dimensions { return w.layoutFragmentIcon(gtx, th, m, icon) })
	}
	flush()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (w *Widget) layoutFragmentIcon(gtx layout.Context, th *material.Theme, m theme.Metrics, fragment Fragment) layout.Dimensions {
	icon := w.icon(fragment.Icon)
	if icon == nil {
		return boundedLabel(th, m, fragment.AccessibleLabel, 2).Layout(gtx)
	}
	return (accessibility.Group{Label: fragment.AccessibleLabel}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layoutIcon(gtx, icon, m.IconSize, th.Fg)
	})
}

func (w *Widget) layoutActions(gtx layout.Context, th *material.Theme, m theme.Metrics, actions []Action, rowID, date string) layout.Dimensions {
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
			return w.actionButton(gtx, th, m, button, action)
		})
	}
	return flow(gtx, m.ControlGap, children...)
}

func (w *Widget) layoutCalendar(gtx layout.Context, th *material.Theme, calendar Calendar, sectionDensity string) layout.Dimensions {
	m := w.metrics(calendar.Density, sectionDensity, w.Page.Density, w.Density)
	previous, next := w.click("month.prev\x00"+calendar.ID), w.click("month.next\x00"+calendar.ID)
	if previous.Clicked(gtx) && w.OnEvent != nil {
		w.OnEvent(Event{Kind: EventCalendarMonth, ID: calendar.ID, Value: "previous"})
	}
	if next.Clicked(gtx) && w.OnEvent != nil {
		w.OnEvent(Event{Kind: EventCalendarMonth, ID: calendar.ID, Value: "next"})
	}
	var children []layout.FlexChild
	if calendar.Heading != "" {
		children = append(children, layout.Rigid(heading(th, m, calendar.Heading).Layout))
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return flow(gtx, m.ControlGap,
			func(gtx layout.Context) layout.Dimensions {
				return w.actionButton(gtx, th, m, previous, Action{Label: "PREVIOUS", Icon: "chevron-left", Enabled: true, Placement: "inline"})
			},
			func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.Y = min(gtx.Dp(theme.MinTouchTarget), gtx.Constraints.Max.Y)
				label := material.Subtitle1(th, calendar.Month)
				label.Alignment = text.Middle
				label.MaxLines, label.Truncator = 2, "…"
				return layout.W.Layout(gtx, label.Layout)
			},
			func(gtx layout.Context) layout.Dimensions {
				return w.actionButton(gtx, th, m, next, Action{Label: "NEXT", Icon: "chevron-right", Enabled: true, Placement: "inline"})
			},
		)
	}))
	if len(calendar.Days) == 0 {
		children = append(children, layout.Rigid(material.Body2(th, calendar.EmptyText).Layout))
	}
	for _, day := range calendar.Days {
		day := day
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			rows := []layout.FlexChild{layout.Rigid(material.Subtitle2(th, day.Label).Layout)}
			if len(calendar.Actions) != 0 {
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return w.layoutActions(gtx, th, m, calendar.Actions, "", day.Date)
				}))
			}
			for _, row := range day.Rows {
				row := row
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return w.layoutRow(gtx, th, m, calendar.ID, "", "", row)
				}))
			}
			return layout.Inset{Top: m.SectionGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (w *Widget) layoutMatrix(gtx layout.Context, th *material.Theme, matrix Matrix, sectionDensity string) layout.Dimensions {
	m := w.metrics(matrix.Density, sectionDensity, w.Page.Density, w.Density)
	children := []layout.FlexChild{layout.Rigid(heading(th, m, matrix.Heading).Layout)}
	if matrix.DisabledReason != "" {
		children = append(children, layout.Rigid(material.Caption(th, matrix.DisabledReason).Layout))
	}
	if len(matrix.Rows) == 0 {
		children = append(children, layout.Rigid(material.Body2(th, matrix.EmptyText).Layout))
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
							gtx.Constraints.Min.Y = min(gtx.Dp(theme.MinTouchTarget), gtx.Constraints.Max.Y)
							return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return w.layoutStatus(gtx, th, m, cell.Text, "", w.resolveColor(cell.ColorToken, cell.Color, th.ContrastBg))
							})
						})
					})
				})
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(boundedLabel(th, m, row.Label, 2).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return flow(gtx, m.ControlGap, cells...) }),
			)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (w *Widget) layoutLegends(gtx layout.Context, th *material.Theme) layout.Dimensions {
	pm := w.metrics(w.Page.Density, w.Density)
	var children []layout.FlexChild
	for _, legend := range w.Page.Legends {
		legend := legend
		children = append(children, layout.Rigid(material.Subtitle2(th, legend.Title).Layout))
		for _, item := range legend.Items {
			item := item
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return (accessibility.Group{Label: item.AccessibleLabel}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return w.layoutStatus(gtx, th, pm, item.Label, item.Icon, w.resolveColor(item.ColorToken, item.Color, th.ContrastBg))
				})
			}))
		}
	}
	return layout.Inset{Bottom: pm.PageGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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
