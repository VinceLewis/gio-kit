package script

import "encoding/json"

// TraceJSONSchema deterministically generates the closed version-1 trace
// schema. Trace validation additionally enforces sequential indices and the
// exact union of per-step artifact metadata.
func TraceJSONSchema() []byte {
	artifact := map[string]any{
		"oneOf": []any{
			objectSchema(map[string]any{
				"name": map[string]any{"type": "string", "pattern": `^step-[0-4][0-9]{2}-(?:failure\.json|capture\.json)$`},
			}, []string{"name"}),
			objectSchema(map[string]any{
				"name":       map[string]any{"type": "string", "pattern": `^step-[0-4][0-9]{2}-capture-unredacted\.png$`},
				"unredacted": map[string]any{"const": true, "type": "boolean"},
			}, []string{"name", "unredacted"}),
		},
	}
	status := enumSchema(string(StatusPassed), string(StatusFailed))
	errorCode := enumSchema(errorCodeNames()...)
	stepProperties := map[string]any{
		"stepIndex":     map[string]any{"type": "integer", "minimum": 0, "maximum": DefaultMaxSteps - 1},
		"stepId":        stringSchema(0),
		"operation":     enumSchema(operationNames()...),
		"attempts":      positiveIntegerSchema(),
		"frameBefore":   map[string]any{"type": "integer", "minimum": 0},
		"frameAfter":    map[string]any{"type": "integer", "minimum": 0},
		"virtualBefore": map[string]any{"$ref": "#/$defs/duration"},
		"virtualAfter":  map[string]any{"$ref": "#/$defs/duration"},
		"artifacts": map[string]any{
			"type": "array", "maxItems": 3, "uniqueItems": true,
			"items": map[string]any{"$ref": "#/$defs/artifact"},
		},
	}
	passedProperties := cloneSchemaMap(stepProperties)
	passedProperties["status"] = map[string]any{"const": StatusPassed, "type": "string"}
	failedProperties := cloneSchemaMap(stepProperties)
	failedProperties["status"] = map[string]any{"const": StatusFailed, "type": "string"}
	failedProperties["code"] = errorCode
	required := []string{"stepIndex", "operation", "status", "attempts", "frameBefore", "frameAfter", "virtualBefore", "virtualAfter"}
	failedRequired := append(append([]string(nil), required...), "code")

	schema := map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$id":     "https://github.com/VinceLewis/gio-kit/guitest/script/trace-schema-v1.json",
		"title":   "Gio test trace, version 1",
		"type":    "object",
		"properties": map[string]any{
			"version": map[string]any{"const": TraceVersion, "type": "integer"},
			"status":  status,
			"steps": map[string]any{
				"type": "array", "maxItems": DefaultMaxSteps,
				"items": map[string]any{"$ref": "#/$defs/step"},
			},
			"artifacts": map[string]any{
				"type": "array", "maxItems": MaximumArtifactFiles, "uniqueItems": true,
				"items": map[string]any{"$ref": "#/$defs/artifact"},
			},
		},
		"required":             []string{"version", "status", "steps"},
		"additionalProperties": false,
		"$defs": map[string]any{
			"duration": map[string]any{
				"type": "string", "maxLength": DefaultMaxString,
				"pattern": `^(?:0|\+?(?:(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:ns|us|µs|μs|ms|s|m|h))+)$`,
			},
			"artifact": artifact,
			"step": map[string]any{
				"oneOf": []any{
					objectSchema(passedProperties, required),
					objectSchema(failedProperties, failedRequired),
				},
			},
		},
	}
	data, _ := json.MarshalIndent(schema, "", "  ")
	return append(data, '\n')
}

func errorCodeNames() []string {
	return []string{
		string(CodeInvalidScript), string(CodeTimeout), string(CodeCancelled),
		string(CodeNotFound), string(CodeAmbiguous), string(CodeNotInteractable),
		string(CodeAssertionFailed), string(CodeProbeError), string(CodeFrameLimit),
		string(CodeClosed), string(CodeArtifactError), string(CodeInternal),
	}
}

func cloneSchemaMap(source map[string]any) map[string]any {
	clone := make(map[string]any, len(source)+2)
	for key, value := range source {
		clone[key] = value
	}
	return clone
}
