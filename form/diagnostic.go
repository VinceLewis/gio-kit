package form

import "github.com/VinceLewis/gio-kit/diagnostic"

func (f *Form) DebugSnapshot(request diagnostic.Request) diagnostic.Component {
	s := f.Snapshot()
	r := request.Bounded()
	fields := make([]any, 0)
	result := diagnostic.Component{Kind: "form"}
	hasPrivate := false
	for i, field := range s.Fields {
		private := field.Schema.Sensitive || field.Schema.Type == FieldAttachment || diagnostic.SensitiveName(field.Schema.ID) || diagnostic.SensitiveName(field.Schema.Label)
		if private {
			hasPrivate = true
			result.SensitiveLabels = append(result.SensitiveLabels, field.Schema.Label, field.Schema.Label+" error")
		}
		if r.RecordID != "" && r.RecordID != field.Schema.ID || r.RecordID == "" && (i < r.Offset || len(fields) >= r.Limit) {
			continue
		}
		var value any = field.Value
		var validation any = field.Error
		if private {
			value = diagnostic.Sensitive{Value: value}
			validation = diagnostic.Sensitive{Value: validation}
		}
		fields = append(fields, map[string]any{"id": field.Schema.ID, "label": field.Schema.Label, "value": value,
			"visible": field.Visible, "readOnly": field.ReadOnly, "mandatory": field.Mandatory,
			"dirty": field.Dirty, "validation": validation})
	}
	var message any = ""
	if s.SubmitError != nil {
		message = s.SubmitError.Error()
		if hasPrivate {
			result.SensitiveLabels = append(result.SensitiveLabels, s.SubmitError.Error())
			message = diagnostic.Sensitive{Value: message}
		}
	}
	result.State = map[string]any{"fieldCount": len(s.Fields), "fields": fields, "dirty": s.Dirty, "valid": s.Valid, "submitting": s.Submitting, "error": message}
	return result
}

func (w *Widget) DebugSnapshot(request diagnostic.Request) diagnostic.Component {
	result := w.Form.DebugSnapshot(request)
	result.State["scroll"] = map[string]any{"first": w.list.Position.First, "offset": w.list.Position.Offset}
	result.State["choiceOpen"] = w.openChoice
	result.State["textMenu"] = w.textMenu
	// This map is maintained using gtx.Focused during the most recent layout.
	for _, field := range w.Form.Snapshot().Fields {
		if w.focused[field.Schema.ID] {
			result.State["focusedField"] = field.Schema.ID
			break
		}
	}
	return result
}
