package router

import (
	"fmt"
	"net/url"
	"strings"
)

// Definition binds an application route name to a URL path pattern. A segment
// beginning with ':' is decoded into a string parameter of the same name.
type Definition struct {
	Name    string
	Pattern string
}

// Table is an immutable-after-setup collection of named routes.
type Table struct {
	definitions []Definition
	byName      map[string]Definition
}

func NewTable(definitions ...Definition) (*Table, error) {
	t := &Table{byName: make(map[string]Definition, len(definitions))}
	for _, definition := range definitions {
		if err := t.Register(definition); err != nil {
			return nil, err
		}
	}
	return t, nil
}

func (t *Table) Register(definition Definition) error {
	if definition.Name == "" || definition.Pattern == "" || definition.Pattern[0] != '/' {
		return fmt.Errorf("router: route name and absolute pattern are required")
	}
	if _, exists := t.byName[definition.Name]; exists {
		return fmt.Errorf("router: duplicate route name %q", definition.Name)
	}
	for _, existing := range t.definitions {
		if existing.Pattern == definition.Pattern {
			return fmt.Errorf("router: duplicate route pattern %q", definition.Pattern)
		}
	}
	t.byName[definition.Name] = definition
	t.definitions = append(t.definitions, definition)
	return nil
}

func (t *Table) Has(name string) bool {
	_, ok := t.byName[name]
	return ok
}

// ResolveURL resolves hierarchical URLs such as gio://app/incident/123. The
// host is deliberately ignored, allowing each embedding app to choose it.
func (t *Table) ResolveURL(rawURL string) (Route, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" {
		return Route{}, fmt.Errorf("router: invalid deep link %q", rawURL)
	}
	pathParts := splitPath(u.EscapedPath())
	for _, definition := range t.definitions {
		patternParts := splitPath(definition.Pattern)
		if len(pathParts) != len(patternParts) {
			continue
		}
		params := Params{}
		matched := true
		for i, part := range patternParts {
			decoded, decodeErr := url.PathUnescape(pathParts[i])
			if decodeErr != nil {
				matched = false
				break
			}
			if strings.HasPrefix(part, ":") {
				if decoded == "" {
					matched = false
					break
				}
				params[strings.TrimPrefix(part, ":")] = String(decoded)
			} else if part != decoded {
				matched = false
				break
			}
		}
		if matched {
			for key, values := range u.Query() {
				if len(values) > 0 {
					params[key] = String(values[0])
				}
			}
			return Route{Name: definition.Name, Params: params}, nil
		}
	}
	return Route{}, fmt.Errorf("router: no route matches %q", rawURL)
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}
