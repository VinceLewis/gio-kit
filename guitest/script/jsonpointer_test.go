package script_test

import (
	"errors"
	"testing"

	"github.com/VinceLewis/gio-kit/guitest/script"
)

func TestResolveJSONPointer(t *testing.T) {
	value := map[string]any{
		"a/b": map[string]any{"~key": []any{"zero", map[string]any{"ok": true}}},
		"":    "empty",
	}
	tests := []struct {
		pointer string
		want    any
		found   bool
	}{
		{"", value, true},
		{"/a~1b/~0key/1/ok", true, true},
		{"/", "empty", true},
		{"/a~1b/~0key/2", nil, false},
		{"/a~1b/~0key/01", nil, false},
		{"/missing", nil, false},
	}
	for _, test := range tests {
		got, found, err := script.ResolveJSONPointer(value, test.pointer)
		if err != nil || found != test.found {
			t.Fatalf("pointer %q = (%v, %t, %v)", test.pointer, got, found, err)
		}
		if found && test.pointer != "" && got != test.want {
			t.Fatalf("pointer %q value = %v, want %v", test.pointer, got, test.want)
		}
	}
	if _, _, err := script.ResolveJSONPointer(value, "missing-slash"); !errors.Is(err, script.ErrJSONPointer) {
		t.Fatalf("invalid pointer error = %v", err)
	}
}
