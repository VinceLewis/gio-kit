package form

import (
	"fmt"
	"image"
	"strings"
	"time"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type temporalAdjustment struct {
	label string
	apply func(time.Time) time.Time
}

func (w *Widget) temporalInput(gtx layout.Context, theme *material.Theme, field FieldState) layout.Dimensions {
	editor := w.editor(field)
	focused := gtx.Focused(editor)
	if w.focused[field.Schema.ID] && !focused {
		_ = w.Form.Blur(field.Schema.ID)
	}
	w.focused[field.Schema.ID] = focused
	if editor.Text() != field.Value {
		_ = w.Form.SetValue(field.Schema.ID, editor.Text())
	}
	editor.ReadOnly = field.ReadOnly
	style := material.Editor(theme, editor, temporalHint(field.Schema.Type))
	style.TextSize = unit.Sp(16)
	pick := w.button("temporal.open." + field.Schema.ID)
	if pick.Clicked(gtx) && !field.ReadOnly {
		if w.temporalOpen == field.Schema.ID {
			w.temporalOpen = ""
		} else {
			w.temporalOpen = field.Schema.ID
			w.temporalDraft[field.Schema.ID] = temporalDraft(field.Schema.Type, field.Value, gtx.Now)
		}
	}
	if field.ReadOnly {
		gtx = gtx.Disabled()
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return w.outline(gtx, field, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						dims := style.Layout(gtx)
						w.handleTextPress(gtx, field.Schema.ID, editor, dims)
						return dims
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						icon := w.dateIcon
						label := "Pick date"
						if field.Schema.Type == FieldTime {
							icon, label = w.timeIcon, "Pick time"
						} else if field.Schema.Type == FieldDateTime {
							label = "Pick date and time"
						}
						return temporalPickerButton(gtx, theme, pick, icon, label)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if w.temporalOpen != field.Schema.ID || field.ReadOnly {
				return layout.Dimensions{}
			}
			return w.temporalPanel(gtx, theme, field)
		}),
	)
}

func temporalPickerButton(gtx layout.Context, theme *material.Theme, click *widget.Clickable, icon *widget.Icon, label string) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(48)), gtx.Dp(unit.Dp(48)))
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(24)), gtx.Dp(unit.Dp(24)))
			gtx.Constraints.Max = gtx.Constraints.Min
			return icon.Layout(gtx, theme.Palette.Fg)
		})
	})
}

func (w *Widget) temporalPanel(gtx layout.Context, theme *material.Theme, field FieldState) layout.Dimensions {
	draft := w.temporalDraft[field.Schema.ID]
	adjustments := temporalAdjustments(field.Schema.Type)
	children := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		label := material.H6(theme, temporalDisplay(field.Schema.Type, draft))
		label.Alignment = text.Middle
		return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(8)}.Layout(gtx, label.Layout)
	})}
	for index := 0; index < len(adjustments); index += 2 {
		end := min(index+2, len(adjustments))
		row := append([]temporalAdjustment(nil), adjustments[index:end]...)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			var buttons []layout.FlexChild
			for _, adjustment := range row {
				adjustment := adjustment
				buttons = append(buttons, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					click := w.button("temporal.adjust." + field.Schema.ID + "." + adjustment.label)
					if click.Clicked(gtx) {
						w.temporalDraft[field.Schema.ID] = adjustment.apply(w.temporalDraft[field.Schema.ID])
					}
					button := material.Button(theme, click, adjustment.label)
					button.CornerRadius = unit.Dp(10)
					button.TextSize = unit.Sp(11)
					return layout.Inset{Left: unit.Dp(3), Right: unit.Dp(3), Bottom: unit.Dp(6)}.Layout(gtx, button.Layout)
				}))
			}
			return layout.Flex{}.Layout(gtx, buttons...)
		}))
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		apply := w.button("temporal.apply." + field.Schema.ID)
		cancel := w.button("temporal.cancel." + field.Schema.ID)
		if apply.Clicked(gtx) {
			value := temporalValue(field.Schema.Type, w.temporalDraft[field.Schema.ID], field.Value)
			_ = w.Form.SetValue(field.Schema.ID, value)
			w.editor(field).SetText(value)
			w.temporalOpen = ""
		}
		if cancel.Clicked(gtx) {
			w.temporalOpen = ""
		}
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				button := material.Button(theme, cancel, "CANCEL PICKER")
				button.CornerRadius = unit.Dp(10)
				return layout.Inset{Right: unit.Dp(3)}.Layout(gtx, button.Layout)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				button := material.Button(theme, apply, "USE VALUE")
				button.CornerRadius = unit.Dp(10)
				return layout.Inset{Left: unit.Dp(3)}.Layout(gtx, button.Layout)
			}),
		)
	}))
	return layout.Inset{Top: unit.Dp(4), Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func temporalHint(fieldType FieldType) string {
	switch fieldType {
	case FieldDate:
		return "YYYY-MM-DD"
	case FieldDateTime:
		return "YYYY-MM-DDThh:mm:ss+hh:mm"
	default:
		return "hh:mm or hh:mm:ss"
	}
}

func temporalDraft(fieldType FieldType, value string, fallback time.Time) time.Time {
	if fallback.IsZero() {
		fallback = time.Now()
	}
	switch fieldType {
	case FieldDate:
		if parsed, err := time.Parse("2006-01-02", value); err == nil {
			return parsed
		}
	case FieldDateTime:
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed
		}
	case FieldTime:
		for _, format := range []string{"15:04:05.999999999", "15:04:05", "15:04"} {
			if parsed, err := time.Parse(format, value); err == nil {
				return time.Date(fallback.Year(), fallback.Month(), fallback.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), fallback.Location())
			}
		}
	}
	return fallback
}

func temporalAdjustments(fieldType FieldType) []temporalAdjustment {
	date := []temporalAdjustment{
		{label: "− YEAR", apply: func(value time.Time) time.Time { return value.AddDate(-1, 0, 0) }},
		{label: "+ YEAR", apply: func(value time.Time) time.Time { return value.AddDate(1, 0, 0) }},
		{label: "− MONTH", apply: func(value time.Time) time.Time { return value.AddDate(0, -1, 0) }},
		{label: "+ MONTH", apply: func(value time.Time) time.Time { return value.AddDate(0, 1, 0) }},
		{label: "− DAY", apply: func(value time.Time) time.Time { return value.AddDate(0, 0, -1) }},
		{label: "+ DAY", apply: func(value time.Time) time.Time { return value.AddDate(0, 0, 1) }},
	}
	clock := []temporalAdjustment{
		{label: "− HOUR", apply: func(value time.Time) time.Time { return value.Add(-time.Hour) }},
		{label: "+ HOUR", apply: func(value time.Time) time.Time { return value.Add(time.Hour) }},
		{label: "− MIN", apply: func(value time.Time) time.Time { return value.Add(-time.Minute) }},
		{label: "+ MIN", apply: func(value time.Time) time.Time { return value.Add(time.Minute) }},
	}
	if fieldType == FieldDate {
		return date
	}
	if fieldType == FieldTime {
		return clock
	}
	return append(date, clock...)
}

func temporalDisplay(fieldType FieldType, value time.Time) string {
	if fieldType == FieldDate {
		return value.Format("Mon 2 Jan 2006")
	}
	if fieldType == FieldTime {
		return value.Format("15:04")
	}
	return value.Format("Mon 2 Jan 2006, 15:04 MST")
}

func temporalValue(fieldType FieldType, value time.Time, original string) string {
	switch fieldType {
	case FieldDate:
		return value.Format("2006-01-02")
	case FieldDateTime:
		return value.Format(time.RFC3339)
	default:
		if strings.Count(original, ":") >= 2 {
			return fmt.Sprintf("%02d:%02d:%02d", value.Hour(), value.Minute(), value.Second())
		}
		return fmt.Sprintf("%02d:%02d", value.Hour(), value.Minute())
	}
}
