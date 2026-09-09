package form

import (
	"context"
	"image/color"
	"time"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Widget renders every visible field from the schema. Retain it across frames.
type Widget struct {
	Form            *Form
	OnReference     func(FieldSchema)
	OnAttachment    func(FieldSchema)
	OnSaveAndClose  func()
	OnDelete        func()
	DeleteLabel     string
	OnInvalid       func()
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
}

func NewWidget(form *Form) *Widget {
	return &Widget{
		Form: form, Editors: make(map[string]*widget.Editor),
		buttons:     make(map[string]*widget.Clickable),
		checks:      make(map[string]*widget.Bool),
		focused:     make(map[string]bool),
		textPress:   make(map[string]*textPress),
		choiceLists: make(map[string]*widget.List),
		list:        widget.List{List: layout.List{Axis: layout.Vertical}},
	}
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
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			w.choiceMaxHeight = gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(80))
			if w.choicePending {
				w.list.ScrollTo(w.choiceScrollTo)
				w.choicePending = false
				w.choiceReady = true
			}
			return w.list.Layout(gtx, len(visible), func(gtx layout.Context, index int) layout.Dimensions {
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
	if !snapshot.Submitting && snapshot.SubmitError == nil && !snapshot.Dirty && w.pendingClose {
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
				return w.input(gtx, theme, field, index)
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
				return layout.Inset{Top: unit.Dp(3)}.Layout(gtx, errLabel.Layout)
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
			return material.CheckBox(theme, check, "Active").Layout(gtx)
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
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, material.Body1(theme, name).Layout),
					layout.Rigid(material.Caption(theme, action).Layout),
				)
			})
		})
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
	if w.delete.Clicked(gtx) && !snapshot.Submitting && w.OnDelete != nil {
		w.OnDelete()
	}
	if w.cancel.Clicked(gtx) {
		w.Form.Cancel()
	}
	if w.save.Clicked(gtx) && !snapshot.Submitting {
		if snapshot.Valid {
			w.pendingClose = false
			_ = w.Form.Submit(context.Background())
		} else if w.OnInvalid != nil {
			w.OnInvalid()
		}
	}
	if w.saveClose.Clicked(gtx) && !snapshot.Submitting {
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
					layout.Flexed(1, w.actionButton(gtx, theme, &w.cancel, "CANCEL", snapshot.Submitting, false)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(5)}.Layout(gtx) }),
					layout.Flexed(1, w.actionButton(gtx, theme, &w.save, "SAVE", !snapshot.Valid || snapshot.Submitting, false)),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx, layout.Flexed(1, w.actionButton(gtx, theme, &w.saveClose, "SAVE & CLOSE", !snapshot.Valid || snapshot.Submitting, true)))
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return w.actionStatus(gtx, theme, snapshot)
				})
			}),
		)
	}
	children := []layout.FlexChild{
		layout.Flexed(1, w.actionButton(gtx, theme, &w.cancel, "CANCEL", snapshot.Submitting, false)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Spacer{Width: unit.Dp(5)}.Layout(gtx)
		}),
		layout.Flexed(1, w.actionButton(gtx, theme, &w.save, "SAVE", !snapshot.Valid || snapshot.Submitting, false)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Spacer{Width: unit.Dp(5)}.Layout(gtx)
		}),
		layout.Flexed(1.35, w.actionButton(gtx, theme, &w.saveClose, "SAVE & CLOSE", !snapshot.Valid || snapshot.Submitting, true)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return w.dirtyStatus(gtx, theme, snapshot) }),
	}
	if w.OnDelete != nil {
		label := w.DeleteLabel
		if label == "" {
			label = "DELETE"
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, material.Button(theme, &w.delete, label).Layout)
		}))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func stackFormActions(gtx layout.Context) bool {
	return gtx.Constraints.Max.X < gtx.Dp(unit.Dp(520))
}

func (w *Widget) actionStatus(gtx layout.Context, theme *material.Theme, snapshot Snapshot) layout.Dimensions {
	children := []layout.FlexChild{layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return w.dirtyStatus(gtx, theme, snapshot)
	})}
	if w.OnDelete != nil {
		label := w.DeleteLabel
		if label == "" {
			label = "DELETE"
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, material.Button(theme, &w.delete, label).Layout)
		}))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

func (w *Widget) dirtyStatus(gtx layout.Context, theme *material.Theme, snapshot Snapshot) layout.Dimensions {
	mark := "CLEAN"
	if snapshot.Dirty {
		mark = "DIRTY"
	}
	label := material.Caption(theme, mark)
	label.Alignment = text.Middle
	return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, label.Layout)
}

func (w *Widget) actionButton(gtx layout.Context, theme *material.Theme, click *widget.Clickable, label string, disabled, primary bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		button := material.Button(theme, click, label)
		button.TextSize = unit.Sp(11)
		if !primary {
			button.Background = color.NRGBA{R: 56, G: 76, B: 112, A: 255}
		}
		if disabled {
			button.Background = color.NRGBA{R: 150, G: 157, B: 168, A: 255}
		}
		return button.Layout(gtx)
	}
}
