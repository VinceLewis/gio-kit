package script

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

// Decode reads, strictly decodes, and validates one version-1 script. No
// caller-provided behavior is invoked until this function has succeeded.
func Decode(reader io.Reader) (Script, error) {
	if reader == nil {
		return Script{}, invalid("document", "reader is nil")
	}
	data, err := io.ReadAll(io.LimitReader(reader, DefaultMaxInput+1))
	if err != nil {
		return Script{}, invalid("document", "input could not be read")
	}
	if len(data) > DefaultMaxInput {
		return Script{}, invalid("document", "input exceeds byte limit")
	}
	if !utf8.Valid(data) {
		return Script{}, invalid("document", "input is not valid UTF-8")
	}
	if err := rejectDuplicateFields(data); err != nil {
		return Script{}, invalid("document", "JSON contains duplicate fields")
	}

	var script Script
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&script); err != nil {
		return Script{}, invalid("document", "JSON does not match the script schema")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Script{}, invalid("document", "trailing JSON is not permitted")
	}
	if err := Validate(script); err != nil {
		return Script{}, err
	}
	return script, nil
}

// DecodeBytes is a convenience wrapper around Decode.
func DecodeBytes(data []byte) (Script, error) { return Decode(bytes.NewReader(data)) }

func invalid(path, reason string) error {
	return &InvalidScriptError{Path: path, Reason: reason}
}

// rejectDuplicateFields applies to the full document, including selector and
// probe-value objects. encoding/json otherwise silently accepts the last copy.
func rejectDuplicateFields(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing token")
		}
		return err
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			nameToken, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := nameToken.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if _, exists := seen[name]; exists {
				return fmt.Errorf("duplicate field")
			}
			seen[name] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("invalid object")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("invalid array")
		}
	default:
		return errors.New("unexpected delimiter")
	}
	return nil
}
