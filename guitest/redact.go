package guitest

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/VinceLewis/gio-kit/diagnostic"
)

type sanitizer struct {
	options   DumpOptions
	values    int
	truncated bool
}

func sensitiveText(value string) bool {
	return diagnostic.SensitiveName(value)
}

func (s *sanitizer) text(path, value string) string {
	if value == diagnostic.Redacted || sensitiveText(path) || sensitiveText(value) {
		return diagnostic.Redacted
	}
	if s.options.Redact != nil {
		value = s.options.Redact(path, value)
	}
	if len(value) > s.options.MaxStringBytes {
		s.truncated = true
		value = value[:s.options.MaxStringBytes]
		for !utf8.ValidString(value) && len(value) > 0 {
			value = value[:len(value)-1]
		}
		value += "…"
	}
	return value
}

// value walks JSON-like data without invoking arbitrary MarshalJSON methods.
// Cycles and large collections stop at depth/value limits. Map order is stable.
func (s *sanitizer) value(path string, value any, depth int) any {
	if _, ok := value.(diagnostic.Sensitive); ok {
		return diagnostic.Redacted
	}
	if sensitiveText(path) {
		return diagnostic.Redacted
	}
	s.values++
	if depth >= s.options.MaxDepth || s.values > s.options.MaxValues {
		s.truncated = true
		return "[truncated]"
	}
	if value == nil {
		return nil
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return nil
		}
		return s.value(path, v.Elem().Interface(), depth+1)
	case reflect.String:
		return s.text(path, v.String())
	case reflect.Bool:
		return v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	case reflect.Float32, reflect.Float64:
		f := v.Float()
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return "unknown"
		}
		return f
	case reflect.Slice, reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return diagnostic.Redacted
		}
		result := []any{}
		for i := 0; i < v.Len(); i++ {
			if s.values >= s.options.MaxValues {
				s.truncated = true
				break
			}
			result = append(result, s.value(fmt.Sprintf("%s.%d", path, i), v.Index(i).Interface(), depth+1))
		}
		return result
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return "unavailable"
		}
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
		result := map[string]any{}
		for _, key := range keys {
			if s.values >= s.options.MaxValues {
				s.truncated = true
				break
			}
			name := key.String()
			// Preserve ordinary field names, redact values under sensitive keys.
			result[s.key(name)] = s.value(path+"."+name, v.MapIndex(key).Interface(), depth+1)
		}
		return result
	case reflect.Struct:
		result := map[string]any{}
		for i := 0; i < v.NumField(); i++ {
			field := v.Type().Field(i)
			if field.PkgPath != "" {
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			if s.values >= s.options.MaxValues {
				s.truncated = true
				break
			}
			result[s.key(name)] = s.value(path+"."+name, v.Field(i).Interface(), depth+1)
		}
		return result
	default:
		return "unavailable"
	}
}

func (s *sanitizer) key(value string) string {
	if len(value) > s.options.MaxStringBytes {
		s.truncated = true
		return s.text("key", value)
	}
	return value
}
