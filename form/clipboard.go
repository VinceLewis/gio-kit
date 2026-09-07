package form

import (
	"io"
	"strings"
	"unicode"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func selectWord(editor *widget.Editor) {
	runes := []rune(editor.Text())
	if len(runes) == 0 {
		return
	}
	start, _ := editor.Selection()
	if start >= len(runes) {
		start = len(runes) - 1
	}
	isWord := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' }
	if !isWord(runes[start]) && start > 0 && isWord(runes[start-1]) {
		start--
	}
	if !isWord(runes[start]) {
		return
	}
	end := start + 1
	for start > 0 && isWord(runes[start-1]) {
		start--
	}
	for end < len(runes) && isWord(runes[end]) {
		end++
	}
	editor.SetCaret(start, end)
}

func isTextField(kind FieldType) bool {
	switch kind {
	case FieldText, FieldTextArea, FieldNumber, FieldDate, FieldDateTime:
		return true
	default:
		return false
	}
}

func (w *Widget) textToolbar(gtx layout.Context, theme *material.Theme, fieldID string) layout.Dimensions {
	editor := w.Editors[fieldID]
	if editor == nil {
		return layout.Dimensions{}
	}
	selectAll := w.button("text.select." + fieldID)
	cut := w.button("text.cut." + fieldID)
	copyButton := w.button("text.copy." + fieldID)
	paste := w.button("text.paste." + fieldID)
	if selectAll.Clicked(gtx) {
		editor.SetCaret(0, editor.Len())
	}
	if copyButton.Clicked(gtx) {
		writeClipboard(gtx, editor.SelectedText())
		w.textMenu = ""
	}
	if cut.Clicked(gtx) {
		if editor.SelectionLen() != 0 {
			writeClipboard(gtx, editor.SelectedText())
			editor.Delete(1)
		}
		w.textMenu = ""
	}
	if paste.Clicked(gtx) {
		gtx.Execute(clipboard.ReadCmd{Tag: editor})
		w.textMenu = ""
	}
	return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(textAction(theme, selectAll, "SELECT ALL")),
			layout.Rigid(textAction(theme, cut, "CUT")),
			layout.Rigid(textAction(theme, copyButton, "COPY")),
			layout.Rigid(textAction(theme, paste, "PASTE")),
		)
	})
}

func textAction(theme *material.Theme, click *widget.Clickable, label string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		button := material.Button(theme, click, label)
		button.TextSize = unit.Sp(10)
		return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, button.Layout)
	}
}

func writeClipboard(gtx layout.Context, value string) {
	if value == "" {
		return
	}
	gtx.Execute(clipboard.WriteCmd{
		Type: "application/text",
		Data: io.NopCloser(strings.NewReader(value)),
	})
}
