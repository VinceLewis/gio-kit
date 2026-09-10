package picker

import "github.com/VinceLewis/gio-kit/diagnostic"

func (w *Widget) DebugSnapshot(request diagnostic.Request) diagnostic.Component {
	r := request.Bounded()
	items := []any{}
	for i := r.Offset; i < len(w.Options) && len(items) < r.Limit; i++ {
		option := w.Options[i]
		if r.RecordID != "" && r.RecordID != option.ID {
			continue
		}
		items = append(items, map[string]any{"id": option.ID, "label": option.Label, "disabled": option.Disabled, "disabledReason": option.DisabledReason})
	}
	return diagnostic.Component{Kind: "picker", State: map[string]any{"title": w.Title, "query": w.search.Text(), "loading": w.Loading, "error": w.Error, "optionCount": len(w.Options), "options": items, "first": w.list.Position.First, "offset": w.list.Position.Offset}}
}
