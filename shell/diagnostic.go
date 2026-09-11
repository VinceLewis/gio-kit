package shell

import "github.com/VinceLewis/gio-kit/diagnostic"

func (w *Widget) DebugSnapshot(request diagnostic.Request) diagnostic.Component {
	r := request.Bounded()
	items := []any{}
	for i := r.Offset; i < len(w.Model.Navigation) && len(items) < r.Limit; i++ {
		item := w.Model.Navigation[i]
		items = append(items, map[string]any{"id": item.ID, "label": item.Label, "selected": item.Selected, "enabled": item.Enabled})
	}
	controls := []any{}
	for i := r.Offset; i < len(w.barControls) && len(controls) < r.Limit; i++ {
		item := w.barControls[i].control
		controls = append(controls, map[string]any{"id": item.ID, "label": item.Label, "selected": item.Selected, "enabled": item.Enabled})
	}
	return diagnostic.Component{Kind: "shell", State: map[string]any{
		"drawerOpen": w.drawerOpen, "navigationCount": len(w.Model.Navigation), "navigation": items,
		"pageTitle": w.Model.PageTitle, "topBar": controls, "overflowCount": len(w.overflowControls),
		"drawerFirst": w.drawerList.Position.First, "drawerOffset": w.drawerList.Position.Offset,
	}}
}
