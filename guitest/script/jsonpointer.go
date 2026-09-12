package script

import (
	"errors"
	"strconv"
	"strings"
)

var ErrJSONPointer = errors.New("guitest/script: invalid JSON Pointer")

// ResolveJSONPointer resolves the bounded RFC 6901 pointer against decoded
// JSON. A syntactically valid pointer to a missing member returns found=false.
func ResolveJSONPointer(value any, pointer string) (result any, found bool, err error) {
	if !validJSONPointer(pointer) {
		return nil, false, ErrJSONPointer
	}
	if pointer == "" {
		return value, true, nil
	}
	current := value
	for _, encoded := range strings.Split(pointer[1:], "/") {
		token := strings.ReplaceAll(strings.ReplaceAll(encoded, "~1", "/"), "~0", "~")
		switch container := current.(type) {
		case map[string]any:
			current, found = container[token]
			if !found {
				return nil, false, nil
			}
		case []any:
			if token == "" || token == "-" || len(token) > 1 && token[0] == '0' {
				return nil, false, nil
			}
			index, parseErr := strconv.Atoi(token)
			if parseErr != nil || index < 0 || index >= len(container) {
				return nil, false, nil
			}
			current = container[index]
		default:
			return nil, false, nil
		}
	}
	return current, true, nil
}
