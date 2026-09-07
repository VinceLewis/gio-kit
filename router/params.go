package router

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Kind identifies the serialized type of a parameter.
type Kind string

const (
	StringKind Kind = "string"
	IntKind    Kind = "int"
	BoolKind   Kind = "bool"
	FloatKind  Kind = "float"
)

// Value is a small, JSON-safe typed route parameter.
type Value struct {
	Kind Kind   `json:"kind"`
	Raw  string `json:"value"`
}

func String(value string) Value { return Value{Kind: StringKind, Raw: value} }
func Int(value int64) Value     { return Value{Kind: IntKind, Raw: strconv.FormatInt(value, 10)} }
func Bool(value bool) Value     { return Value{Kind: BoolKind, Raw: strconv.FormatBool(value)} }
func Float(value float64) Value {
	return Value{Kind: FloatKind, Raw: strconv.FormatFloat(value, 'g', -1, 64)}
}

func (v Value) String() string { return v.Raw }

func (v Value) Int64() (int64, error) {
	if v.Kind != IntKind {
		return 0, fmt.Errorf("router: parameter is %q, not int", v.Kind)
	}
	return strconv.ParseInt(v.Raw, 10, 64)
}

func (v Value) Bool() (bool, error) {
	if v.Kind != BoolKind {
		return false, fmt.Errorf("router: parameter is %q, not bool", v.Kind)
	}
	return strconv.ParseBool(v.Raw)
}

func (v Value) Float64() (float64, error) {
	if v.Kind != FloatKind {
		return 0, fmt.Errorf("router: parameter is %q, not float", v.Kind)
	}
	return strconv.ParseFloat(v.Raw, 64)
}

func (v Value) valid() bool {
	switch v.Kind {
	case StringKind:
		return true
	case IntKind:
		_, err := v.Int64()
		return err == nil
	case BoolKind:
		_, err := v.Bool()
		return err == nil
	case FloatKind:
		_, err := v.Float64()
		return err == nil
	default:
		return false
	}
}

// Params is copied at every Router boundary so callers cannot mutate history.
type Params map[string]Value

func (p Params) clone() Params {
	if p == nil {
		return nil
	}
	copy := make(Params, len(p))
	for key, value := range p {
		copy[key] = value
	}
	return copy
}

func (p Params) MarshalJSON() ([]byte, error) {
	type plain Params
	for key, value := range p {
		if key == "" || !value.valid() {
			return nil, fmt.Errorf("router: invalid parameter %q", key)
		}
	}
	return json.Marshal(plain(p))
}
