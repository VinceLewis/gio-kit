package guitest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"time"
)

// JSONSchema generates the version-1 schema from public dump types. The
// checked-in schema is verified against this output by the core tests.
func JSONSchema() []byte {
	schema := schemaFor(reflect.TypeOf(Dump{}))
	schema["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	schema["$id"] = "https://github.com/VinceLewis/gio-kit/guitest/schema-v1.json"
	schema["title"] = "Gio test frame dump, version 1"
	properties := schema["properties"].(map[string]any)
	properties["version"] = map[string]any{"const": SchemaVersion, "type": "integer"}
	data, _ := json.MarshalIndent(schema, "", "  ")
	return append(data, '\n')
}

func schemaFor(t reflect.Type) map[string]any {
	if t == reflect.TypeOf(time.Time{}) {
		return map[string]any{"type": "string", "format": "date-time"}
	}
	switch t.Kind() {
	case reflect.Pointer:
		return map[string]any{"anyOf": []any{schemaFor(t.Elem()), map[string]any{"type": "null"}}}
	case reflect.Struct:
		properties := map[string]any{}
		required := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" {
				continue
			}
			parts := strings.Split(f.Tag.Get("json"), ",")
			name := parts[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			properties[name] = schemaFor(f.Type)
			if len(parts) < 2 || parts[1] != "omitempty" {
				required = append(required, name)
			}
		}
		if t == reflect.TypeOf(FrameNode{}) {
			for name, choices := range map[string][]string{"role": {"unknown", "button", "checkbox", "editor", "radio", "switch"}, "visibility": {"unknown", "visible", "partially_visible", "clipped", "covered", "virtualized"}, "viewportVisibility": {"visible", "partially_visible", "clipped"}, "coverage": {"unknown", "covered", "uncovered"}, "interactable": {"unknown", "yes", "no"}} {
				properties[name] = map[string]any{"type": "string", "enum": choices}
			}
		}
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": schemaFor(t.Elem())}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": schemaFor(t.Elem())}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Interface:
		return map[string]any{}
	default:
		return map[string]any{"type": "integer"}
	}
}

// ReadDump validates version, shape and enumerations against the generated
// schema. It caps input before decoding. It does not re-redact an external dump;
// only read artifacts from trusted producers, and treat their text as data.
func ReadDump(reader io.Reader, maxBytes int) (Dump, error) {
	if reader == nil {
		return Dump{}, errors.New("guitest: nil dump reader")
	}
	maxBytes = (DumpOptions{MaxBytes: maxBytes}).bounded().MaxBytes
	data, err := io.ReadAll(io.LimitReader(reader, int64(maxBytes)+1))
	if err != nil {
		return Dump{}, err
	}
	if len(data) > maxBytes {
		return Dump{}, ErrOutputLimit
	}
	var raw any
	if err = json.Unmarshal(data, &raw); err != nil {
		return Dump{}, err
	}
	var schema map[string]any
	_ = json.Unmarshal(JSONSchema(), &schema)
	if err = validateShape(schema, raw, "dump"); err != nil {
		return Dump{}, err
	}
	var dump Dump
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&dump); err != nil {
		return Dump{}, err
	}
	return dump, nil
}

// validateShape handles the finite schema vocabulary emitted by schemaFor;
// it is not a general-purpose JSON Schema implementation.
func validateShape(schema map[string]any, value any, path string) error {
	bad := func() error { return fmt.Errorf("guitest: invalid dump at %s", path) }
	if choices, ok := schema["anyOf"].([]any); ok {
		for _, choice := range choices {
			if validateShape(choice.(map[string]any), value, path) == nil {
				return nil
			}
		}
		return bad()
	}
	if expected, ok := schema["const"]; ok && !reflect.DeepEqual(expected, value) {
		return bad()
	}
	if choices, ok := schema["enum"].([]any); ok {
		found := false
		for _, choice := range choices {
			found = found || reflect.DeepEqual(choice, value)
		}
		if !found {
			return bad()
		}
	}
	switch schema["type"] {
	case "null":
		if value != nil {
			return bad()
		}
	case "string":
		if _, ok := value.(string); !ok {
			return bad()
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return bad()
		}
	case "number", "integer":
		number, ok := value.(float64)
		if !ok || schema["type"] == "integer" && number != math.Trunc(number) {
			return bad()
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			return bad()
		}
		for i, item := range items {
			if err := validateShape(schema["items"].(map[string]any), item, fmt.Sprintf("%s.%d", path, i)); err != nil {
				return err
			}
		}
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return bad()
		}
		if required, ok := schema["required"].([]any); ok {
			for _, name := range required {
				if _, exists := object[name.(string)]; !exists {
					return bad()
				}
			}
		}
		properties, _ := schema["properties"].(map[string]any)
		for name, item := range object {
			child, exists := properties[name]
			if !exists {
				child = schema["additionalProperties"]
				if child == false {
					return bad()
				}
				if child == nil || child == true {
					continue
				}
			}
			if err := validateShape(child.(map[string]any), item, path+"."+name); err != nil {
				return err
			}
		}
	}
	return nil
}
