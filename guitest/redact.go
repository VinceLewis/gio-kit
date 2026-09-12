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
		usedKeys := make(map[string]struct{}, len(keys))
		for _, key := range keys {
			if s.values >= s.options.MaxValues {
				s.truncated = true
				break
			}
			name := key.String()
			cleanName := s.mapKey(path, name)
			uniqueName, ok := s.uniqueKey(cleanName, usedKeys)
			if !ok {
				s.truncated = true
				break
			}
			if sensitiveText(name) {
				result[uniqueName] = diagnostic.Redacted
				continue
			}
			// Descendant paths use only the sanitized key. This prevents a caller's
			// Redact callback from observing or accidentally returning the original.
			result[uniqueName] = s.value(path+"."+uniqueName, v.MapIndex(key).Interface(), depth+1)
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

// mapKey treats runtime map names as data rather than schema fields. Safe
// defaults run before the caller's additive policy, and the callback receives
// a structural path that does not repeat the possibly sensitive key.
func (s *sanitizer) mapKey(path, value string) string {
	if sensitiveMapKey(value) {
		return s.boundedMapKey(diagnostic.Redacted)
	}
	return s.boundedMapKey(s.text(path+".<map-key>", value))
}

func (s *sanitizer) boundedMapKey(value string) string {
	if len(value) <= s.options.MaxStringBytes {
		return value
	}
	s.truncated = true
	if value == diagnostic.Redacted {
		return strings.Repeat("*", s.options.MaxStringBytes)
	}
	value = value[:s.options.MaxStringBytes]
	for !utf8.ValidString(value) && len(value) > 0 {
		value = value[:len(value)-1]
	}
	return value
}

// uniqueKey preserves every map entry when redaction or truncation coalesces
// names. Its ordinal suffix depends only on stable sorted position, never on a
// hash or fragment of the original key.
func (s *sanitizer) uniqueKey(base string, used map[string]struct{}) (string, bool) {
	if _, exists := used[base]; !exists {
		used[base] = struct{}{}
		return base, true
	}
	for ordinal := 2; ordinal <= s.options.MaxValues; ordinal++ {
		suffix := fmt.Sprintf("#%d", ordinal)
		if len(suffix) > s.options.MaxStringBytes {
			return "", false
		}
		prefix := base
		limit := s.options.MaxStringBytes - len(suffix)
		if limit < 0 {
			prefix = ""
		} else if len(prefix) > limit {
			prefix = prefix[:limit]
			for !utf8.ValidString(prefix) && len(prefix) > 0 {
				prefix = prefix[:len(prefix)-1]
			}
			s.truncated = true
		}
		candidate := prefix + suffix
		if _, exists := used[candidate]; exists {
			continue
		}
		used[candidate] = struct{}{}
		return candidate, true
	}
	return "", false
}

func sensitiveMapKey(value string) bool {
	if value == diagnostic.Redacted || sensitiveText(value) {
		return true
	}
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	if looksLikeEmail(trimmed) || looksLikeUUID(trimmed) || looksLikeOpaqueID(trimmed) {
		return true
	}
	for _, prefix := range []string{"record-", "record_", "record/", "record:", "rec-", "rec_", "rec/", "rec:", "sk-", "ghp_", "github_pat_"} {
		if strings.HasPrefix(lower, prefix) && len(lower) > len(prefix) {
			return true
		}
	}
	return false
}

func looksLikeEmail(value string) bool {
	if strings.ContainsAny(value, " \t\r\n") || strings.Count(value, "@") != 1 {
		return false
	}
	parts := strings.SplitN(value, "@", 2)
	return parts[0] != "" && strings.Contains(parts[1], ".") && !strings.HasPrefix(parts[1], ".") && !strings.HasSuffix(parts[1], ".")
}

func looksLikeUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if !((character >= '0') && (character <= '9')) && !((character >= 'a') && (character <= 'f')) && !((character >= 'A') && (character <= 'F')) {
			return false
		}
	}
	return true
}

func looksLikeOpaqueID(value string) bool {
	if len(value) < 20 || strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	letters, digits := false, false
	for _, character := range value {
		switch {
		case character >= '0' && character <= '9':
			digits = true
		case character >= 'a' && character <= 'z', character >= 'A' && character <= 'Z':
			letters = true
		case character == '-', character == '_', character == '.', character == '/', character == ':':
		default:
			return false
		}
	}
	return letters && digits
}
