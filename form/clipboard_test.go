package form

import (
	"testing"

	"gioui.org/widget"
)

func TestF2LongPressWordSelectionHelper(t *testing.T) {
	var editor widget.Editor
	editor.SetText("alpha beta")
	editor.SetCaret(8, 8)

	selectWord(&editor)

	if got := editor.SelectedText(); got != "beta" {
		t.Fatalf("selected %q, want beta", got)
	}
}

func TestF2LongPressWordSelectionAtBoundary(t *testing.T) {
	var editor widget.Editor
	editor.SetText("alpha beta")
	editor.SetCaret(editor.Len(), editor.Len())

	selectWord(&editor)

	if got := editor.SelectedText(); got != "beta" {
		t.Fatalf("selected %q, want beta", got)
	}
}
