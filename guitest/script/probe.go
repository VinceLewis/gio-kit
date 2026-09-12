package script

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strings"
)

const (
	DefaultMaxProbeBytes      = 64 << 10
	DefaultMaxJSONNumberBytes = 128
)

// ProbeFunc is a consumer-registered, read-only integrity probe. Args and the
// returned JSON are copied, decoded, and bounded by the runner. The callback
// must honor ctx and must not mutate application state.
type ProbeFunc func(ctx context.Context, args json.RawMessage) (json.RawMessage, error)

func (r *runner) expectProbe(ctx context.Context, index int, attempts *int) error {
	step := &r.script.Steps[index]
	probe := r.options.Probes[step.Probe]
	args := append(json.RawMessage(nil), step.Args...)
	if !rawPresent(args) {
		args = json.RawMessage(`{}`)
	}
	var terminal error
	check := func() bool {
		(*attempts)++
		if err := ctx.Err(); err != nil {
			terminal = err
			return true
		}
		result, err := probe(ctx, append(json.RawMessage(nil), args...))
		if err != nil {
			terminal = fmt.Errorf("%w: callback: %w", ErrProbe, err)
			return true
		}
		if err := ctx.Err(); err != nil {
			terminal = err
			return true
		}
		actual, err := decodeBoundedJSON(result, false)
		if err != nil {
			terminal = fmt.Errorf("%w: invalid result", ErrProbe)
			return true
		}
		return equalJSON(actual, r.prepared[index].expected)
	}
	if check() {
		return terminal
	}
	if err := r.driver.WaitFor(ctx, check); err != nil {
		return r.assertionWaitError(err)
	}
	return terminal
}

func decodeBoundedJSON(raw json.RawMessage, object bool) (any, error) {
	if len(raw) > DefaultMaxProbeBytes {
		return nil, errors.New("JSON exceeds probe byte limit")
	}
	if err := validateRaw("JSON", raw, object); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("JSON has trailing data")
	}
	if err := boundedNumbers(value); err != nil {
		return nil, err
	}
	return value, nil
}

func boundedNumbers(value any) error {
	switch value := value.(type) {
	case json.Number:
		if len(value.String()) > DefaultMaxJSONNumberBytes {
			return errors.New("JSON number exceeds byte limit")
		}
	case []any:
		for _, child := range value {
			if err := boundedNumbers(child); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, child := range value {
			if err := boundedNumbers(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func equalJSON(left, right any) bool {
	if leftNumber, ok := jsonNumber(left); ok {
		rightNumber, ok := jsonNumber(right)
		return ok && leftNumber.equal(rightNumber)
	}
	switch left := left.(type) {
	case nil:
		return right == nil
	case bool:
		right, ok := right.(bool)
		return ok && left == right
	case string:
		right, ok := right.(string)
		return ok && left == right
	case []any:
		right, ok := right.([]any)
		if !ok || len(left) != len(right) {
			return false
		}
		for index := range left {
			if !equalJSON(left[index], right[index]) {
				return false
			}
		}
		return true
	case map[string]any:
		right, ok := right.(map[string]any)
		if !ok || len(left) != len(right) {
			return false
		}
		for key, value := range left {
			other, ok := right[key]
			if !ok || !equalJSON(value, other) {
				return false
			}
		}
		return true
	default:
		// Component snapshots can contain native Go numeric and composite types.
		// Normalize those through bounded JSON before comparing.
		data, err := json.Marshal(left)
		if err != nil || len(data) > DefaultMaxProbeBytes {
			return false
		}
		normalized, err := decodeBoundedJSON(data, false)
		if err != nil || reflect.TypeOf(normalized) == reflect.TypeOf(left) {
			return reflect.DeepEqual(left, right)
		}
		return equalJSON(normalized, right)
	}
}

type canonicalNumber struct {
	negative bool
	digits   string
	scale    *big.Int
}

func (n canonicalNumber) equal(other canonicalNumber) bool {
	if n.digits == "0" || other.digits == "0" {
		return n.digits == other.digits
	}
	return n.negative == other.negative && n.digits == other.digits && n.scale.Cmp(other.scale) == 0
}

func jsonNumber(value any) (canonicalNumber, bool) {
	var text string
	switch value := value.(type) {
	case json.Number:
		text = value.String()
	case int:
		text = fmt.Sprint(value)
	case int8:
		text = fmt.Sprint(value)
	case int16:
		text = fmt.Sprint(value)
	case int32:
		text = fmt.Sprint(value)
	case int64:
		text = fmt.Sprint(value)
	case uint:
		text = fmt.Sprint(value)
	case uint8:
		text = fmt.Sprint(value)
	case uint16:
		text = fmt.Sprint(value)
	case uint32:
		text = fmt.Sprint(value)
	case uint64:
		text = fmt.Sprint(value)
	case float32:
		text = fmt.Sprint(value)
	case float64:
		text = fmt.Sprint(value)
	default:
		return canonicalNumber{}, false
	}
	if len(text) > DefaultMaxJSONNumberBytes {
		return canonicalNumber{}, false
	}
	return parseCanonicalNumber(text)
}

// parseCanonicalNumber compares JSON decimals exactly without materializing
// powers of ten. In particular, a bounded token with a huge exponent cannot
// force an equally huge big.Int allocation.
func parseCanonicalNumber(text string) (canonicalNumber, bool) {
	number := canonicalNumber{}
	if strings.HasPrefix(text, "-") {
		number.negative = true
		text = text[1:]
	}
	mantissa, exponentText := text, "0"
	if index := strings.IndexAny(text, "eE"); index >= 0 {
		mantissa, exponentText = text[:index], text[index+1:]
	}
	exponent, ok := new(big.Int).SetString(exponentText, 10)
	if !ok {
		return canonicalNumber{}, false
	}
	integer, fraction := mantissa, ""
	if index := strings.IndexByte(mantissa, '.'); index >= 0 {
		integer, fraction = mantissa[:index], mantissa[index+1:]
	}
	digits := integer + fraction
	if len(digits) == 0 {
		return canonicalNumber{}, false
	}
	for index := range digits {
		if digits[index] < '0' || digits[index] > '9' {
			return canonicalNumber{}, false
		}
	}
	first := strings.IndexFunc(digits, func(r rune) bool { return r != '0' })
	if first < 0 {
		number.digits = "0"
		return number, true
	}
	digits = digits[first:]
	last := len(digits)
	for last > 0 && digits[last-1] == '0' {
		last--
	}
	trailing := len(digits) - last
	number.digits = digits[:last]
	number.scale = new(big.Int).Set(exponent)
	number.scale.Sub(number.scale, big.NewInt(int64(len(fraction))))
	number.scale.Add(number.scale, big.NewInt(int64(trailing)))
	return number, true
}
