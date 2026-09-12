package script

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf8"
)

// Validate checks a decoded or programmatically constructed script against the
// same closed version-1 contract used by Decode.
func Validate(script Script) error {
	if script.Version != Version {
		return invalid("version", "unsupported version")
	}
	if err := validString("name", script.Name, true); err != nil {
		return err
	}
	if script.Fixture != "" {
		if err := validString("fixture", script.Fixture, true); err != nil {
			return err
		}
	}
	if script.Steps == nil {
		return invalid("steps", "field is required")
	}
	if len(script.Steps) > DefaultMaxSteps {
		return invalid("steps", "step limit exceeded")
	}
	if script.Defaults != nil && script.Defaults.Timeout != nil {
		if err := validTimeout("defaults.timeout", *script.Defaults.Timeout); err != nil {
			return err
		}
	}
	ids := make(map[string]struct{}, len(script.Steps))
	for index := range script.Steps {
		path := "steps[" + quoteIndex(index) + "]"
		step := &script.Steps[index]
		if step.ID != "" {
			if err := validString(path+".id", step.ID, true); err != nil {
				return err
			}
			if _, exists := ids[step.ID]; exists {
				return invalid(path+".id", "step ID is duplicated")
			}
			ids[step.ID] = struct{}{}
		}
		if step.Timeout != nil {
			if err := validTimeout(path+".timeout", *step.Timeout); err != nil {
				return err
			}
		}
		if err := validateStep(path, step); err != nil {
			return err
		}
	}
	return nil
}

func validTimeout(path string, duration Duration) error {
	if duration.Duration() <= 0 || duration.Duration() > MaximumStepTimeout {
		return invalid(path, "timeout must be positive and at most five seconds")
	}
	return nil
}

func validateStep(path string, step *Step) error {
	allowed := map[string]bool{}
	require := func(names ...string) {
		for _, name := range names {
			allowed[name] = true
		}
	}
	switch step.Op {
	case OpTap, OpDoubleTap, OpPress, OpFocus:
		require("selector")
	case OpLongPress:
		require("selector", "duration")
	case OpMove:
		require("position")
	case OpRelease, OpCancelPointer, OpBack, OpClearFocus, OpSettle:
	case OpDrag:
		require("selector", "delta", "duration")
	case OpScroll:
		require("selector", "delta")
	case OpType:
		require("selector", "text")
	case OpKey:
		require("key")
	case OpSetSelection, OpSetComposition:
		require("range")
	case OpEdit:
		require("range", "text")
	case OpResize:
		require("size", "metric")
	case OpAdvance:
		require("duration")
	case OpExpect:
		require("selector", "expect")
		if step.Expect == ExpectCount {
			require("count")
		}
	case OpExpectSnapshot:
		require("component", "pointer")
		allowed["equals"] = true
		allowed["notEquals"] = true
		allowed["exists"] = true
	case OpExpectProbe:
		require("probe", "args", "equals")
	case OpCapture:
		require("name", "components", "screenshot")
	default:
		return invalid(path+".op", "operation is not recognized")
	}

	present := stepFields(step)
	for name := range present {
		if !allowed[name] {
			return invalid(path+"."+name, "field is incompatible with operation")
		}
	}
	for name := range allowed {
		if !present[name] && requiredStepField(step.Op, name) {
			return invalid(path+"."+name, "field is required for operation")
		}
	}

	if step.Selector != nil {
		terms := 0
		if err := validateSelector(path+".selector", step.Selector, 1, &terms); err != nil {
			return err
		}
	}
	for name, point := range map[string]*Point{"position": step.Position, "delta": step.Delta} {
		if point != nil && (!finite(point.X) || !finite(point.Y)) {
			return invalid(path+"."+name, "coordinates must be finite")
		}
	}
	if step.Duration != nil {
		duration := step.Duration.Duration()
		if duration < 0 || (step.Op == OpLongPress || step.Op == OpDrag) && duration == 0 {
			return invalid(path+".duration", "duration is outside the permitted range")
		}
	}
	if step.Text != nil {
		if err := validString(path+".text", *step.Text, false); err != nil {
			return err
		}
	}
	if step.Key != nil {
		if err := validateKey(path+".key", step.Key); err != nil {
			return err
		}
	}
	if step.Range != nil && (step.Range.Start < 0 || step.Range.End < 0) {
		return invalid(path+".range", "range positions must be non-negative")
	}
	if step.Size != nil && (step.Size.Width <= 0 || step.Size.Height <= 0) {
		return invalid(path+".size", "dimensions must be positive")
	}
	if step.Metric != nil && (!finite(step.Metric.PxPerDp) || !finite(step.Metric.PxPerSp) || step.Metric.PxPerDp <= 0 || step.Metric.PxPerSp <= 0) {
		return invalid(path+".metric", "metric values must be finite and positive")
	}

	switch step.Op {
	case OpExpect:
		if !validExpectation(step.Expect) {
			return invalid(path+".expect", "expectation is not recognized")
		}
		if step.Expect == ExpectCount && (step.Count == nil || *step.Count < 0) {
			return invalid(path+".count", "count must be non-negative")
		}
	case OpExpectSnapshot:
		if err := validString(path+".component", step.Component, true); err != nil {
			return err
		}
		if step.Pointer == nil || !validJSONPointer(*step.Pointer) {
			return invalid(path+".pointer", "JSON Pointer is invalid or too deep")
		}
		if err := validateComparison(path, step); err != nil {
			return err
		}
	case OpExpectProbe:
		if err := validString(path+".probe", step.Probe, true); err != nil {
			return err
		}
		if !rawPresent(step.Equals) {
			return invalid(path+".equals", "field is required for operation")
		}
		if rawPresent(step.NotEquals) || step.Exists != nil {
			return invalid(path, "probe assertion accepts equals only")
		}
		if err := validateRaw(path+".args", step.Args, true); err != nil {
			return err
		}
		if err := validateRaw(path+".equals", step.Equals, false); err != nil {
			return err
		}
	case OpCapture:
		if err := validString(path+".name", step.Name, true); err != nil {
			return err
		}
		if len(step.Components) > DefaultSelectorTerms {
			return invalid(path+".components", "component limit exceeded")
		}
		seen := map[string]bool{}
		for i, component := range step.Components {
			itemPath := fmt.Sprintf("%s.components[%d]", path, i)
			if err := validString(itemPath, component, true); err != nil {
				return err
			}
			if seen[component] {
				return invalid(itemPath, "component is duplicated")
			}
			seen[component] = true
		}
		if step.Screenshot != "" && step.Screenshot != ScreenshotNever && step.Screenshot != ScreenshotOptional && step.Screenshot != ScreenshotRequired {
			return invalid(path+".screenshot", "screenshot mode is not recognized")
		}
	}
	return nil
}

func requiredStepField(op Operation, name string) bool {
	if op == OpCapture && (name == "components" || name == "screenshot") {
		return false
	}
	if op == OpExpectProbe && name == "args" {
		return false
	}
	if op == OpExpectSnapshot && (name == "equals" || name == "notEquals" || name == "exists") {
		return false
	}
	return true
}

func stepFields(step *Step) map[string]bool {
	fields := map[string]bool{}
	if step.Selector != nil {
		fields["selector"] = true
	}
	if step.Duration != nil {
		fields["duration"] = true
	}
	if step.Position != nil {
		fields["position"] = true
	}
	if step.Delta != nil {
		fields["delta"] = true
	}
	if step.Text != nil {
		fields["text"] = true
	}
	if step.Key != nil {
		fields["key"] = true
	}
	if step.Range != nil {
		fields["range"] = true
	}
	if step.Size != nil {
		fields["size"] = true
	}
	if step.Metric != nil {
		fields["metric"] = true
	}
	if step.Expect != "" {
		fields["expect"] = true
	}
	if step.Count != nil {
		fields["count"] = true
	}
	if step.Component != "" {
		fields["component"] = true
	}
	if step.Pointer != nil {
		fields["pointer"] = true
	}
	if rawPresent(step.Equals) {
		fields["equals"] = true
	}
	if rawPresent(step.NotEquals) {
		fields["notEquals"] = true
	}
	if step.Exists != nil {
		fields["exists"] = true
	}
	if step.Probe != "" {
		fields["probe"] = true
	}
	if rawPresent(step.Args) {
		fields["args"] = true
	}
	if step.Name != "" {
		fields["name"] = true
	}
	if step.Components != nil {
		fields["components"] = true
	}
	if step.Screenshot != "" {
		fields["screenshot"] = true
	}
	return fields
}

func validateComparison(path string, step *Step) error {
	count := 0
	if rawPresent(step.Equals) {
		count++
	}
	if rawPresent(step.NotEquals) {
		count++
	}
	if step.Exists != nil {
		count++
	}
	if count != 1 {
		return invalid(path, "exactly one comparison is required")
	}
	if rawPresent(step.Equals) {
		return validateRaw(path+".equals", step.Equals, false)
	}
	if rawPresent(step.NotEquals) {
		return validateRaw(path+".notEquals", step.NotEquals, false)
	}
	return nil
}

func validateKey(path string, key *Key) error {
	if err := validString(path+".name", key.Name, true); err != nil {
		return err
	}
	seen := map[KeyModifier]bool{}
	for i, modifier := range key.Modifiers {
		switch modifier {
		case ModifierShift, ModifierControl, ModifierAlt, ModifierSuper, ModifierCommand:
		default:
			return invalid(fmt.Sprintf("%s.modifiers[%d]", path, i), "modifier is not recognized")
		}
		if seen[modifier] {
			return invalid(path+".modifiers", "modifier is duplicated")
		}
		seen[modifier] = true
	}
	return nil
}

func validateSelector(path string, selector *Selector, depth int, terms *int) error {
	if selector == nil {
		return invalid(path, "selector is missing")
	}
	if depth > DefaultSelectorDepth {
		return invalid(path, "selector depth limit exceeded")
	}
	(*terms)++
	if *terms > DefaultSelectorTerms {
		return invalid(path, "selector term limit exceeded")
	}
	operators := 0
	for _, present := range []bool{selector.Label != nil, selector.Description != nil, selector.Name != nil, selector.Role != nil,
		selector.Text != nil, selector.ID != nil, selector.Enabled != nil, selector.Selected != nil, selector.All != nil,
		selector.Within != nil, selector.Containing != nil, selector.Nth != nil} {
		if present {
			operators++
		}
	}
	if operators != 1 {
		return invalid(path, "selector must contain exactly one operator")
	}
	for name, value := range map[string]*string{"label": selector.Label, "description": selector.Description, "name": selector.Name, "text": selector.Text, "id": selector.ID} {
		if value != nil {
			if err := validString(path+"."+name, *value, name == "id"); err != nil {
				return err
			}
		}
	}
	if selector.Role != nil {
		switch *selector.Role {
		case RoleUnknown, RoleButton, RoleCheckbox, RoleEditor, RoleRadio, RoleSwitch:
		default:
			return invalid(path+".role", "role is not recognized")
		}
	}
	if selector.All != nil {
		if len(selector.All) == 0 {
			return invalid(path+".all", "all requires at least one selector")
		}
		for index := range selector.All {
			if err := validateSelector(fmt.Sprintf("%s.all[%d]", path, index), &selector.All[index], depth+1, terms); err != nil {
				return err
			}
		}
	}
	if selector.Within != nil {
		if err := validateSelector(path+".within.target", &selector.Within.Target, depth+1, terms); err != nil {
			return err
		}
		if err := validateSelector(path+".within.ancestor", &selector.Within.Ancestor, depth+1, terms); err != nil {
			return err
		}
	}
	if selector.Containing != nil {
		if err := validateSelector(path+".containing.target", &selector.Containing.Target, depth+1, terms); err != nil {
			return err
		}
		if err := validateSelector(path+".containing.descendant", &selector.Containing.Descendant, depth+1, terms); err != nil {
			return err
		}
	}
	if selector.Nth != nil {
		if selector.Nth.Index < 0 {
			return invalid(path+".nth.index", "index must be non-negative")
		}
		if err := validateSelector(path+".nth.selector", &selector.Nth.Selector, depth+1, terms); err != nil {
			return err
		}
	}
	return nil
}

func validExpectation(expect Expectation) bool {
	switch expect {
	case ExpectPresent, ExpectAbsent, ExpectEnabled, ExpectDisabled, ExpectSelected, ExpectUnselected, ExpectCount,
		ExpectViewportVisible, ExpectViewportPartiallyVisible, ExpectViewportClipped:
		return true
	default:
		return false
	}
}

func validString(path, value string, required bool) error {
	if !utf8.ValidString(value) {
		return invalid(path, "string is not valid UTF-8")
	}
	if required && value == "" {
		return invalid(path, "string must not be empty")
	}
	if len(value) > DefaultMaxString {
		return invalid(path, "string byte limit exceeded")
	}
	return nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func validJSONPointer(pointer string) bool {
	if len(pointer) > DefaultMaxString || !utf8.ValidString(pointer) {
		return false
	}
	if pointer == "" {
		return true
	}
	if !strings.HasPrefix(pointer, "/") {
		return false
	}
	if strings.Count(pointer, "/") > DefaultJSONDepth {
		return false
	}
	for index := 0; index < len(pointer); index++ {
		if pointer[index] == '~' && (index+1 >= len(pointer) || pointer[index+1] != '0' && pointer[index+1] != '1') {
			return false
		}
		if pointer[index] == '~' {
			index++
		}
	}
	return true
}

func validateRaw(path string, raw json.RawMessage, object bool) error {
	if !rawPresent(raw) {
		return invalid(path, "JSON value is required")
	}
	if len(raw) > DefaultMaxInput {
		return invalid(path, "JSON value exceeds byte limit")
	}
	if !utf8.Valid(raw) {
		return invalid(path, "JSON value is not valid UTF-8")
	}
	if err := rejectDuplicateFields(raw); err != nil {
		return invalid(path, "JSON value is invalid or contains duplicate fields")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return invalid(path, "JSON value is invalid")
	}
	if _, err := decoder.Token(); !errorsIsEOF(err) {
		return invalid(path, "JSON value has trailing data")
	}
	if object {
		if _, ok := value.(map[string]any); !ok {
			return invalid(path, "probe arguments must be an object")
		}
	}
	values := 0
	if err := validateJSONValue(path, value, 1, &values); err != nil {
		return err
	}
	return nil
}

func errorsIsEOF(err error) bool { return err == io.EOF }

func validateJSONValue(path string, value any, depth int, values *int) error {
	if depth > DefaultJSONDepth {
		return invalid(path, "JSON depth limit exceeded")
	}
	*values++
	if *values > DefaultJSONValues {
		return invalid(path, "JSON value limit exceeded")
	}
	switch value := value.(type) {
	case string:
		return validString(path, value, false)
	case []any:
		for index, child := range value {
			if err := validateJSONValue(fmt.Sprintf("%s[%d]", path, index), child, depth+1, values); err != nil {
				return err
			}
		}
	case map[string]any:
		for name, child := range value {
			if err := validString(path+".<key>", name, false); err != nil {
				return err
			}
			if err := validateJSONValue(path+".<value>", child, depth+1, values); err != nil {
				return err
			}
		}
	}
	return nil
}
