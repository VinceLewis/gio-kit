package script

import "encoding/json"

// JSONSchema deterministically generates the version-1 script schema from an
// explicit union description. Reflection is deliberately avoided: ordinary
// struct reflection cannot express the operation and selector discriminators.
func JSONSchema() []byte {
	schema := map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$id":     "https://github.com/VinceLewis/gio-kit/guitest/script/schema-v1.json",
		"title":   "Gio test script, version 1",
		"type":    "object",
		"properties": map[string]any{
			"version":  map[string]any{"const": Version, "type": "integer"},
			"name":     stringSchema(1),
			"fixture":  stringSchema(1),
			"defaults": map[string]any{"$ref": "#/$defs/defaults"},
			"steps":    map[string]any{"type": "array", "maxItems": DefaultMaxSteps, "items": map[string]any{"$ref": "#/$defs/step"}},
		},
		"required":             []string{"version", "name", "steps"},
		"additionalProperties": false,
	}
	schema["$defs"] = schemaDefinitions()
	data, _ := json.MarshalIndent(schema, "", "  ")
	return append(data, '\n')
}

func schemaDefinitions() map[string]any {
	defs := map[string]any{
		"duration": map[string]any{
			"type": "string", "maxLength": DefaultMaxString,
			"pattern": `^(?:0|\+?(?:(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:ns|us|µs|μs|ms|s|m|h))+)$`,
		},
		"defaults": objectSchema(map[string]any{"timeout": ref("duration")}, nil),
		"point":    objectSchema(map[string]any{"x": numberSchema(), "y": numberSchema()}, []string{"x", "y"}),
		"size": objectSchema(map[string]any{
			"width": positiveIntegerSchema(), "height": positiveIntegerSchema(),
		}, []string{"width", "height"}),
		"metric": objectSchema(map[string]any{
			"pxPerDp": positiveNumberSchema(), "pxPerSp": positiveNumberSchema(),
		}, []string{"pxPerDp", "pxPerSp"}),
		"range": objectSchema(map[string]any{
			"start": nonNegativeIntegerSchema(), "end": nonNegativeIntegerSchema(),
		}, []string{"start", "end"}),
		"key": objectSchema(map[string]any{
			"name": stringSchema(1),
			"modifiers": map[string]any{"type": "array", "uniqueItems": true, "items": enumSchema(
				string(ModifierShift), string(ModifierControl), string(ModifierAlt), string(ModifierSuper), string(ModifierCommand)), "maxItems": 5},
		}, []string{"name"}),
	}
	defs["selector"] = selectorSchema()
	defs["stepBase"] = objectProperties(map[string]any{
		"id": stringSchema(1), "op": enumSchema(operationNames()...), "timeout": ref("duration"),
	}, []string{"op"})
	defs["step"] = map[string]any{"oneOf": stepSchemas()}
	return defs
}

func selectorSchema() map[string]any {
	branches := []any{}
	for _, name := range []string{"label", "description", "name", "text"} {
		branches = append(branches, objectSchema(map[string]any{name: stringSchema(0)}, []string{name}))
	}
	branches = append(branches,
		objectSchema(map[string]any{"id": stringSchema(1)}, []string{"id"}),
		objectSchema(map[string]any{"role": enumSchema(
			string(RoleUnknown), string(RoleButton), string(RoleCheckbox), string(RoleEditor), string(RoleRadio), string(RoleSwitch))}, []string{"role"}),
		objectSchema(map[string]any{"enabled": map[string]any{"type": "boolean"}}, []string{"enabled"}),
		objectSchema(map[string]any{"selected": map[string]any{"type": "boolean"}}, []string{"selected"}),
		objectSchema(map[string]any{"all": map[string]any{
			"type": "array", "minItems": 1, "maxItems": DefaultSelectorTerms, "items": ref("selector"),
		}}, []string{"all"}),
		objectSchema(map[string]any{"within": objectSchema(map[string]any{
			"target": ref("selector"), "ancestor": ref("selector"),
		}, []string{"target", "ancestor"})}, []string{"within"}),
		objectSchema(map[string]any{"containing": objectSchema(map[string]any{
			"target": ref("selector"), "descendant": ref("selector"),
		}, []string{"target", "descendant"})}, []string{"containing"}),
		objectSchema(map[string]any{"nth": objectSchema(map[string]any{
			"selector": ref("selector"), "index": nonNegativeIntegerSchema(),
		}, []string{"selector", "index"})}, []string{"nth"}),
	)
	return map[string]any{"oneOf": branches}
}

func stepSchemas() []any {
	selector := ref("selector")
	steps := []any{}
	add := func(op Operation, properties map[string]any, required ...string) {
		properties["op"] = map[string]any{"const": op}
		required = append([]string{"op"}, required...)
		steps = append(steps, stepBranch(properties, required))
	}
	for _, op := range []Operation{OpTap, OpDoubleTap, OpPress, OpFocus} {
		add(op, map[string]any{"selector": selector}, "selector")
	}
	add(OpLongPress, map[string]any{"selector": selector, "duration": ref("duration")}, "selector", "duration")
	add(OpMove, map[string]any{"position": ref("point")}, "position")
	for _, op := range []Operation{OpRelease, OpCancelPointer, OpBack, OpClearFocus, OpSettle} {
		add(op, map[string]any{})
	}
	add(OpDrag, map[string]any{"selector": selector, "delta": ref("point"), "duration": ref("duration")}, "selector", "delta", "duration")
	add(OpScroll, map[string]any{"selector": selector, "delta": ref("point")}, "selector", "delta")
	add(OpType, map[string]any{"selector": selector, "text": stringSchema(0)}, "selector", "text")
	add(OpKey, map[string]any{"key": ref("key")}, "key")
	for _, op := range []Operation{OpSetSelection, OpSetComposition} {
		add(op, map[string]any{"range": ref("range")}, "range")
	}
	add(OpEdit, map[string]any{"range": ref("range"), "text": stringSchema(0)}, "range", "text")
	add(OpResize, map[string]any{"size": ref("size"), "metric": ref("metric")}, "size", "metric")
	add(OpAdvance, map[string]any{"duration": ref("duration")}, "duration")

	expectBase := map[string]any{
		"allOf": []any{ref("stepBase"), objectProperties(map[string]any{
			"op": map[string]any{"const": OpExpect}, "selector": selector,
		}, []string{"op", "selector"})},
		"oneOf": []any{
			objectProperties(map[string]any{"expect": map[string]any{"const": ExpectCount}, "count": nonNegativeIntegerSchema()}, []string{"expect", "count"}),
			objectProperties(map[string]any{"expect": enumSchema(
				string(ExpectPresent), string(ExpectAbsent), string(ExpectEnabled), string(ExpectDisabled),
				string(ExpectSelected), string(ExpectUnselected), string(ExpectViewportVisible),
				string(ExpectViewportPartiallyVisible), string(ExpectViewportClipped)),
			}, []string{"expect"}),
		},
		"unevaluatedProperties": false,
	}
	steps = append(steps, expectBase)

	comparison := []any{
		objectProperties(map[string]any{"equals": map[string]any{}}, []string{"equals"}),
		objectProperties(map[string]any{"notEquals": map[string]any{}}, []string{"notEquals"}),
		objectProperties(map[string]any{"exists": map[string]any{"type": "boolean"}}, []string{"exists"}),
	}
	steps = append(steps, map[string]any{
		"allOf": []any{ref("stepBase"), objectProperties(map[string]any{
			"op": map[string]any{"const": OpExpectSnapshot}, "component": stringSchema(1), "pointer": map[string]any{
				"type": "string", "maxLength": DefaultMaxString, "pattern": `^(?:|(?:/(?:[^~/]|~[01])*)*)$`,
			},
		}, []string{"op", "component", "pointer"})},
		"oneOf": comparison, "unevaluatedProperties": false,
	})
	add(OpExpectProbe, map[string]any{
		"probe": stringSchema(1), "args": map[string]any{"type": "object"}, "equals": map[string]any{},
	}, "probe", "equals")
	add(OpCapture, map[string]any{
		"name":       stringSchema(1),
		"components": map[string]any{"type": "array", "uniqueItems": true, "maxItems": DefaultSelectorTerms, "items": stringSchema(1)},
		"screenshot": enumSchema(string(ScreenshotNever), string(ScreenshotOptional), string(ScreenshotRequired)),
	}, "name")
	return steps
}

func stepBranch(properties map[string]any, required []string) map[string]any {
	return map[string]any{
		"allOf":                 []any{ref("stepBase"), objectProperties(properties, required)},
		"unevaluatedProperties": false,
	}
}

func operationNames() []string {
	operations := []Operation{OpTap, OpDoubleTap, OpLongPress, OpPress, OpMove, OpRelease, OpCancelPointer,
		OpDrag, OpScroll, OpFocus, OpType, OpKey, OpBack, OpClearFocus, OpSetSelection, OpSetComposition,
		OpEdit, OpResize, OpAdvance, OpSettle, OpExpect, OpExpectSnapshot, OpExpectProbe, OpCapture}
	values := make([]string, len(operations))
	for index, operation := range operations {
		values[index] = string(operation)
	}
	return values
}

func ref(name string) map[string]any { return map[string]any{"$ref": "#/$defs/" + name} }

func objectSchema(properties map[string]any, required []string) map[string]any {
	schema := objectProperties(properties, required)
	schema["additionalProperties"] = false
	return schema
}

func objectProperties(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringSchema(minLength int) map[string]any {
	schema := map[string]any{"type": "string", "maxLength": DefaultMaxString}
	if minLength > 0 {
		schema["minLength"] = minLength
	}
	return schema
}

func enumSchema(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}
func numberSchema() map[string]any { return map[string]any{"type": "number"} }
func positiveNumberSchema() map[string]any {
	return map[string]any{"type": "number", "exclusiveMinimum": 0}
}
func nonNegativeIntegerSchema() map[string]any {
	return map[string]any{"type": "integer", "minimum": 0}
}
func positiveIntegerSchema() map[string]any { return map[string]any{"type": "integer", "minimum": 1} }
