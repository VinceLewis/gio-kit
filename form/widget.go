package form

import (
	"context"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/accessibility"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

// Widget renders every visible field from the schema. Retain it across frames.
type Widget struct {
	Form           *Form
	OnReference    func(FieldSchema)
	OnAttachment   func(FieldSchema)
	AfterFields    func(layout.Context, *material.Theme) layout.Dimensions
	OnSaveAndClose func()
	OnDelete       func()
	DeleteLabel    string
	OnInvalid      func()
	// RequireDirtySubmit disables save actions while an update form is clean.
	// Leave it false for create forms, including pristine default-only creates.
	RequireDirtySubmit bool
	// AdditionalDirty joins caller-owned staged state to save gating.
	AdditionalDirty bool
	Editors         map[string]*widget.Editor
	buttons         map[string]*widget.Clickable
	checks          map[string]*widget.Bool
	focused         map[string]bool
	textPress       map[string]*textPress
	choiceLists     map[string]*widget.List
	textMenu        string
	openChoice      string
	choiceMaxHeight int
	choiceScrollTo  int
	choicePending   bool
	choiceReady     bool
	save            widget.Clickable
	saveClose       widget.Clickable
	cancel          widget.Clickable
	delete          widget.Clickable
	list            widget.List
	pendingClose    bool
	cancelIcon      *widget.Icon
	saveIcon        *widget.Icon
	saveCloseIcon   *widget.Icon
	saveCloseExit   *widget.Icon
	deleteIcon      *widget.Icon
	dateIcon        *widget.Icon
	timeIcon        *widget.Icon
	temporalOpen    string
	temporalDraft   map[string]time.Time
}

func NewWidget(form *Form) *Widget {
	return &Widget{
		Form: form, Editors: make(map[string]*widget.Editor),
		buttons:       make(map[string]*widget.Clickable),
		checks:        make(map[string]*widget.Bool),
		focused:       make(map[string]bool),
		textPress:     make(map[string]*textPress),
		choiceLists:   make(map[string]*widget.List),
		list:          widget.List{List: layout.List{Axis: layout.Vertical}},
		cancelIcon:    staticIcon(icons.NavigationClose),
		saveIcon:      staticIcon(icons.ContentSave),
		saveCloseIcon: staticIcon(icons.ContentSave),
		saveCloseExit: staticIcon(icons.ActionExitToApp),
		deleteIcon:    staticIcon(icons.ActionDelete),
		dateIcon:      staticIcon(icons.ActionDateRange),
		timeIcon:      staticIcon(icons.DeviceAccessTime),
		temporalDraft: make(map[string]time.Time),
	}
}

func staticIcon(data []byte) *widget.Icon {
	icon, _ := widget.NewIcon(data)
	return icon
}

func (w *Widget) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if w == nil || w.Form == nil {
		return material.Body1(theme, "Form is not configured").Layout(gtx)
	}
	snapshot := w.Form.Snapshot()
	w.completePendingClose(snapshot)
	visible := make([]FieldState, 0, len(snapshot.Fields))
	for _, field := range snapshot.Fields {
		if field.Visible {
			visible = append(visible, field)
		}
	}
	rowCount := len(visible)
	if w.AfterFields != nil {
		rowCount++
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			w.choiceMaxHeight = gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(80))
			if w.choicePending {
				w.list.ScrollTo(w.choiceScrollTo)
				w.choicePending = false
				w.choiceReady = true
			}
			return w.list.Layout(gtx, rowCount, func(gtx layout.Context, index int) layout.Dimensions {
				if index == len(visible) {
					return w.AfterFields(gtx, theme)
				}
				return w.layoutField(gtx, theme, visible[index], index)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if snapshot.SubmitError == nil {
				return layout.Dimensions{}
			}
			label := material.Caption(theme, snapshot.SubmitError.Error())
			label.Color = color.NRGBA{R: 183, G: 48, B: 62, A: 255}
			return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, label.Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return w.actions(gtx, theme, snapshot)
		}),
	)
}

func (w *Widget) completePendingClose(snapshot Snapshot) {
	if !snapshot.Submitting && snapshot.SubmitError == nil && !snapshot.Dirty && !w.AdditionalDirty && w.pendingClose {
		w.pendingClose = false
		if w.OnSaveAndClose != nil {
			w.OnSaveAndClose()
		}
	}
}

func (w *Widget) layoutField(gtx layout.Context, theme *material.Theme, field FieldState, index int) layout.Dimensions {
	label := field.Schema.Label
	if field.Mandatory {
		label += " *"
	}
	return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				title := material.Caption(theme, label)
				title.Color = color.NRGBA{R: 70, G: 84, B: 105, A: 255}
				return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, title.Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				var help []string
				if field.Mandatory {
					help = append(help, "Required")
				}
				if field.ReadOnly {
					help = append(help, "Read only")
				}
				if field.Error != "" {
					help = append(help, field.Error)
				}
				if field.Schema.UnavailableReason != "" {
					help = append(help, field.Schema.UnavailableReason)
				}
				return (accessibility.Group{Label: field.Schema.Label, Description: strings.Join(help, ". "), Disabled: field.ReadOnly}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return w.input(gtx, theme, field, index)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !isTextField(field.Schema.Type) || w.textMenu != field.Schema.ID || field.ReadOnly {
					return layout.Dimensions{}
				}
				return w.textToolbar(gtx, theme, field.Schema.ID)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if field.Error == "" {
					return layout.Dimensions{}
				}
				errLabel := material.Caption(theme, field.Error)
				errLabel.Color = color.NRGBA{R: 183, G: 48, B: 62, A: 255}
				return (accessibility.Group{Label: field.Schema.Label + " error"}).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(3)}.Layout(gtx, errLabel.Layout)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if field.Schema.UnavailableReason == "" || field.Schema.Type == FieldAttachment && w.OnAttachment != nil {
					return layout.Dimensions{}
				}
				return layout.Inset{Top: unit.Dp(3)}.Layout(gtx, material.Caption(theme, field.Schema.UnavailableReason).Layout)
			}),
		)
	})
}

func (w *Widget) input(gtx layout.Context, theme *material.Theme, field FieldState, index int) layout.Dimensions {
	if field.ReadOnly && !isTextField(field.Schema.Type) {
		gtx = gtx.Disabled()
	}
	switch field.Schema.Type {
	case FieldBoolean:
		check := w.check(field)
		if check.Value != (field.Value == "true") {
			if field.ReadOnly {
				check.Value = field.Value == "true"
			} else {
				_ = w.Form.SetValue(field.Schema.ID, boolString(check.Value))
			}
		}
		if field.ReadOnly {
			gtx = gtx.Disabled()
		}
		return w.outline(gtx, field, func(gtx layout.Context) layout.Dimensions {
			return material.CheckBox(theme, check, field.Schema.Label).Layout(gtx)
		})
	case FieldChoice:
		button := w.button(field.Schema.ID)
		if button.Clicked(gtx) && !field.ReadOnly {
			if w.openChoice == field.Schema.ID {
				w.openChoice = ""
				w.choiceReady = false
			} else {
				w.openChoice = field.Schema.ID
				w.choiceScrollTo = index
				w.choicePending = true
				w.choiceReady = false
				gtx.Execute(op.InvalidateCmd{})
			}
		}
		name := field.Value
		for _, choice := range field.Schema.Choices {
			if choice.Value == field.Value {
				name = choice.Label
			}
		}
		if name == "" {
			name = "Tap to choose"
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return w.outline(gtx, field, func(gtx layout.Context) layout.Dimensions {
					return button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						semantic.Button.Add(gtx.Ops)
						semantic.LabelOp(field.Schema.Label).Add(gtx.Ops)
						semantic.DescriptionOp(name).Add(gtx.Ops)
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, material.Body1(theme, name).Layout),
							layout.Rigid(material.Body1(theme, "v").Layout),
						)
					})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if w.openChoice != field.Schema.ID || !w.choiceReady {
					return layout.Dimensions{}
				}
				return w.choiceMenu(gtx, theme, field)
			}),
		)
	case FieldReference:
		if w.OnReference == nil {
			gtx = gtx.Disabled()
		}
		button := w.button(field.Schema.ID)
		if button.Clicked(gtx) && !field.ReadOnly && w.OnReference != nil {
			w.OnReference(field.Schema)
		}
		name := field.Reference.Display
		if name == "" {
			name = field.Value
		}
		if name == "" {
			name = "No record selected"
		}
		return w.outline(gtx, field, func(gtx layout.Context) layout.Dimensions {
			return button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.Button.Add(gtx.Ops)
				semantic.LabelOp(field.Schema.Label).Add(gtx.Ops)
				semantic.DescriptionOp(name).Add(gtx.Ops)
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, material.Body1(theme, name).Layout),
					layout.Rigid(material.Caption(theme, "LOOK UP").Layout),
				)
			})
		})
	case FieldAttachment:
		button := w.button(field.Schema.ID)
		selectable := !field.ReadOnly && w.OnAttachment != nil
		if button.Clicked(gtx) && selectable {
			w.OnAttachment(field.Schema)
		}
		if !selectable {
			gtx = gtx.Disabled()
		}
		name := field.Value
		if name == "" {
			name = "No attachment selected"
		}
		action := "CHOOSE"
		if w.OnAttachment == nil {
			action = "UNAVAILABLE"
		}
		return w.outline(gtx, field, func(gtx layout.Context) layout.Dimensions {
			return button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.Button.Add(gtx.Ops)
				semantic.LabelOp(field.Schema.Label).Add(gtx.Ops)
				semantic.DescriptionOp(name).Add(gtx.Ops)
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, material.Body1(theme, name).Layout),
					layout.Rigid(material.Caption(theme, action).Layout),
				)
			})
		})
	case FieldDate, FieldDateTime, FieldTime:
		return w.temporalInput(gtx, theme, field)
	default:
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
		style := material.Editor(theme, editor, "")
		style.TextSize = unit.Sp(16)
		return w.outline(gtx, field, func(gtx layout.Context) layout.Dimensions {
			dims := style.Layout(gtx)
			w.handleTextPress(gtx, field.Schema.ID, editor, dims)
			return dims
		})
	}
}

func (w *Widget) outline(gtx layout.Context, field FieldState, content layout.Widget) layout.Dimensions {
	borderColor := color.NRGBA{R: 116, G: 128, B: 146, A: 255}
	if field.ReadOnly {
		borderColor = color.NRGBA{R: 188, G: 195, B: 205, A: 255}
	}
	if field.Error != "" {
		borderColor = color.NRGBA{R: 183, G: 48, B: 62, A: 255}
	}
	return widget.Border{Color: borderColor, CornerRadius: unit.Dp(4), Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(48))
		return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, content)
	})
}

func (w *Widget) editor(field FieldState) *widget.Editor {
	editor := w.Editors[field.Schema.ID]
	if editor == nil {
		editor = new(widget.Editor)
		editor.SingleLine = field.Schema.Type != FieldTextArea
		editor.SetText(field.Value)
		w.Editors[field.Schema.ID] = editor
	}
	return editor
}

func (w *Widget) button(id string) *widget.Clickable {
	if w.buttons[id] == nil {
		w.buttons[id] = new(widget.Clickable)
	}
	return w.buttons[id]
}

func (w *Widget) check(field FieldState) *widget.Bool {
	if w.checks[field.Schema.ID] == nil {
		w.checks[field.Schema.ID] = &widget.Bool{Value: field.Value == "true"}
	}
	return w.checks[field.Schema.ID]
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func (w *Widget) choiceMenu(gtx layout.Context, theme *material.Theme, field FieldState) layout.Dimensions {
	return widget.Border{Color: color.NRGBA{R: 116, G: 128, B: 146, A: 255}, CornerRadius: unit.Dp(4), Width: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		list := w.choiceLists[field.Schema.ID]
		if list == nil {
			list = &widget.List{List: layout.List{Axis: layout.Vertical}}
			w.choiceLists[field.Schema.ID] = list
		}
		const visibleChoices = 5
		maxHeight := gtx.Dp(unit.Dp(48 * visibleChoices))
		if w.choiceMaxHeight > 0 && w.choiceMaxHeight < maxHeight {
			maxHeight = w.choiceMaxHeight
		}
		if gtx.Constraints.Max.Y > maxHeight {
			gtx.Constraints.Max.Y = maxHeight
		}
		gtx.Constraints.Min.Y = 0
		return list.Layout(gtx, len(field.Schema.Choices), func(gtx layout.Context, index int) layout.Dimensions {
			choice := field.Schema.Choices[index]
			click := w.button("choice." + field.Schema.ID + "." + choice.Value)
			if click.Clicked(gtx) {
				_ = w.Form.SetValue(field.Schema.ID, choice.Value)
				w.openChoice = ""
				w.choiceReady = false
			}
			label := choice.Label
			if choice.Value == field.Value {
				label = "✓  " + label
			}
			return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.RadioButton.Add(gtx.Ops)
				semantic.LabelOp(choice.Label).Add(gtx.Ops)
				semantic.SelectedOp(choice.Value == field.Value).Add(gtx.Ops)
				return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, material.Body1(theme, label).Layout)
			})
		})
	})
}

type textPress struct {
	click gesture.Click
	at    time.Time
}

func (w *Widget) handleTextPress(gtx layout.Context, fieldID string, editor *widget.Editor, dims layout.Dimensions) {
	state := w.textPress[fieldID]
	if state == nil {
		state = new(textPress)
		w.textPress[fieldID] = state
	}
	for {
		event, ok := state.click.Update(gtx.Source)
		if !ok {
			break
		}
		switch event.Kind {
		case gesture.KindPress:
			state.at = gtx.Now
		case gesture.KindClick:
			if event.Source == pointer.Touch && !state.at.IsZero() && gtx.Now.Sub(state.at) >= 500*time.Millisecond {
				selectWord(editor)
				w.textMenu = fieldID
			} else if w.textMenu == fieldID {
				w.textMenu = ""
			}
			state.at = time.Time{}
		case gesture.KindCancel:
			state.at = time.Time{}
		}
	}
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
	state.click.Add(gtx.Ops)
}

func (w *Widget) actions(gtx layout.Context, theme *material.Theme, snapshot Snapshot) layout.Dimensions {
	saveDisabled := w.submitDisabled(snapshot)
	if w.delete.Clicked(gtx) && !snapshot.Submitting && w.OnDelete != nil {
		w.OnDelete()
	}
	if w.cancel.Clicked(gtx) {
		w.Form.Cancel()
	}
	if w.save.Clicked(gtx) && !saveDisabled {
		if snapshot.Valid {
			w.pendingClose = false
			_ = w.Form.Submit(context.Background())
		} else if w.OnInvalid != nil {
			w.OnInvalid()
		}
	}
	if w.saveClose.Clicked(gtx) && !saveDisabled {
		if snapshot.Valid {
			w.pendingClose = true
			_ = w.Form.Submit(context.Background())
		} else if w.OnInvalid != nil {
			w.OnInvalid()
		}
	}
	if stackFormActions(gtx) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, w.actionButton(gtx, theme, &w.cancel, w.cancelIcon, nil, "CANCEL", snapshot.Submitting, false, false)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(5)}.Layout(gtx) }),
					layout.Flexed(1, w.actionButton(gtx, theme, &w.save, w.saveIcon, nil, "SAVE", saveDisabled, false, false)),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx, layout.Flexed(1, w.actionButton(gtx, theme, &w.saveClose, w.saveCloseIcon, w.saveCloseExit, "SAVE & CLOSE", saveDisabled, true, false)))
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return w.deleteAction(gtx, theme, snapshot)
				})
			}),
		)
	}
	if compactFormActions(gtx) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(w.actionButton(gtx, theme, &w.cancel, w.cancelIcon, nil, "CANCEL", snapshot.Submitting, false, false)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(5)}.Layout(gtx) }),
					layout.Rigid(w.actionButton(gtx, theme, &w.save, w.saveIcon, nil, "SAVE", saveDisabled, false, false)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(5)}.Layout(gtx) }),
					layout.Rigid(w.actionButton(gtx, theme, &w.saveClose, w.saveCloseIcon, w.saveCloseExit, "SAVE & CLOSE", saveDisabled, true, false)),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return w.deleteAction(gtx, theme, snapshot)
				})
			}),
		)
	}
	children := []layout.FlexChild{
		layout.Rigid(w.actionButton(gtx, theme, &w.cancel, w.cancelIcon, nil, "CANCEL", snapshot.Submitting, false, false)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Spacer{Width: unit.Dp(5)}.Layout(gtx)
		}),
		layout.Rigid(w.actionButton(gtx, theme, &w.save, w.saveIcon, nil, "SAVE", saveDisabled, false, false)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Spacer{Width: unit.Dp(5)}.Layout(gtx)
		}),
		layout.Rigid(w.actionButton(gtx, theme, &w.saveClose, w.saveCloseIcon, w.saveCloseExit, "SAVE & CLOSE", saveDisabled, true, false)),
	}
	if w.OnDelete != nil {
		label := w.DeleteLabel
		if label == "" {
			label = "DELETE"
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, w.actionButton(gtx, theme, &w.delete, w.deleteIcon, nil, label, snapshot.Submitting, false, true))
		}))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func (w *Widget) submitDisabled(snapshot Snapshot) bool {
	return !snapshot.Valid || snapshot.Submitting || w.RequireDirtySubmit && !snapshot.Dirty && !w.AdditionalDirty
}

func stackFormActions(gtx layout.Context) bool {
	return gtx.Constraints.Max.X < gtx.Dp(unit.Dp(360))
}

func compactFormActions(gtx layout.Context) bool {
	return gtx.Constraints.Max.X < gtx.Dp(unit.Dp(520))
}

func (w *Widget) deleteAction(gtx layout.Context, theme *material.Theme, snapshot Snapshot) layout.Dimensions {
	children := []layout.FlexChild{layout.Flexed(1, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })}
	if w.OnDelete != nil {
		label := w.DeleteLabel
		if label == "" {
			label = "DELETE"
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, w.actionButton(gtx, theme, &w.delete, w.deleteIcon, nil, label, snapshot.Submitting, false, true))
		}))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func (w *Widget) actionButton(gtx layout.Context, theme *material.Theme, click *widget.Clickable, icon, secondIcon *widget.Icon, label string, disabled, primary, destructive bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		background := color.NRGBA{R: 56, G: 76, B: 112, A: 255}
		if primary {
			background = theme.Palette.ContrastBg
		}
		if destructive {
			background = color.NRGBA{R: 170, G: 42, B: 52, A: 255}
		}
		if disabled {
			background = color.NRGBA{R: 150, G: 157, B: 168, A: 255}
			gtx = gtx.Disabled()
		}
		button := material.ButtonLayout(theme, click)
		button.Background = background
		button.CornerRadius = unit.Dp(12)
		return button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp(label).Add(gtx.Ops)
			height := min(gtx.Dp(unit.Dp(48)), gtx.Constraints.Max.Y)
			gtx.Constraints.Min.Y = height
			gtx.Constraints.Max.Y = height
			return layout.Inset{Left: unit.Dp(10), Right: unit.Dp(10), Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if icon == nil {
							return layout.Dimensions{}
						}
						gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(20)), gtx.Dp(unit.Dp(20)))
						gtx.Constraints.Max = gtx.Constraints.Min
						return icon.Layout(gtx, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
					}),
				}
				if secondIcon != nil {
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(2)}.Layout(gtx) }),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(20)), gtx.Dp(unit.Dp(20)))
							gtx.Constraints.Max = gtx.Constraints.Min
							return secondIcon.Layout(gtx, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
						}),
					)
				}
				children = append(children,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(7)}.Layout(gtx) }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						labelStyle := material.Label(theme, unit.Sp(11), label)
						labelStyle.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
						labelStyle.Alignment = text.Middle
						return labelStyle.Layout(gtx)
					}),
				)
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
			})
		})
	}
}
