package script_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/VinceLewis/gio-kit/guitest/script"
)

func TestJSONSchemaCheckedIn(t *testing.T) {
	generated := script.JSONSchema()
	checked, err := os.ReadFile("schema-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(checked, generated) {
		t.Fatal("schema-v1.json is stale; regenerate it from JSONSchema")
	}
}

func TestJSONSchemaRepresentsClosedUnions(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(script.JSONSchema(), &schema); err != nil {
		t.Fatal(err)
	}
	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		t.Fatal("schema has no $defs")
	}
	selector := defs["selector"].(map[string]any)["oneOf"].([]any)
	if len(selector) != 12 {
		t.Fatalf("selector union branches = %d, want 12", len(selector))
	}
	steps := defs["step"].(map[string]any)["oneOf"].([]any)
	if len(steps) != 24 {
		t.Fatalf("step union branches = %d, want 24", len(steps))
	}
	for index, branch := range steps {
		object := branch.(map[string]any)
		if closed, ok := object["unevaluatedProperties"].(bool); !ok || closed {
			t.Fatalf("step branch %d is not closed", index)
		}
	}
}
