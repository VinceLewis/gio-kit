package shell

import "github.com/VinceLewis/gio-kit/diagnostic"

func (w *Widget) DebugSnapshot(request diagnostic.Request) diagnostic.Component {
	r := request.Bounded()
	items := []any{}
	for i := r.Offset; i < len(w.Model.Navigation) && len(items) < r.Limit; i++ {
		item := w.Model.Navigation[i]
		click := w.clickable(w.navigation, item.ID)
		items = append(items, map[string]any{
			"id": item.ID, "label": item.Label, "selected": item.Selected, "enabled": item.Enabled,
			"pressed": click.Pressed(), "hovered": click.Hovered(), "focused": w.navigationFocus[item.ID],
		})
	}
	controls := []any{}
	for i := r.Offset; i < len(w.barControls) && len(controls) < r.Limit; i++ {
		item := w.barControls[i].control
		pressed, hovered := false, false
		if item.Kind == ControlToggle {
			pressed, hovered = w.toggle(item.ID).Pressed(), w.toggle(item.ID).Hovered()
		} else {
			pressed, hovered = w.clickable(w.controls, item.ID).Pressed(), w.clickable(w.controls, item.ID).Hovered()
		}
		controls = append(controls, map[string]any{
			"id": item.ID, "label": item.Label, "selected": item.Selected, "enabled": item.Enabled,
			"kind": item.Kind, "value": item.Value, "valueLabel": item.ValueLabel, "dismissal": item.Dismissal,
			"disabledReason": item.DisabledReason, "pressed": pressed, "hovered": hovered, "focused": w.controlFocus[item.ID],
		})
	}
	drawerControls := []any{}
	for i := r.Offset; i < len(w.Model.Drawer) && len(drawerControls) < r.Limit; i++ {
		item := w.Model.Drawer[i]
		drawerControls = append(drawerControls, map[string]any{
			"id": item.ID, "label": item.Label, "enabled": item.Enabled, "kind": item.Kind,
			"value": item.Value, "valueLabel": item.ValueLabel, "dismissal": item.Dismissal, "disabledReason": item.DisabledReason,
		})
	}
	return diagnostic.Component{Kind: "shell", State: map[string]any{
		"drawerOpen": w.drawerOpen, "navigationCount": len(w.Model.Navigation), "navigation": items,
		"pageTitle": w.Model.PageTitle, "topBar": controls, "overflowCount": len(w.overflowControls),
		"drawerControls": drawerControls,
		"drawerFirst":    w.drawerList.Position.First, "drawerOffset": w.drawerList.Position.Offset,
		"drawerBeforeEnd": w.drawerList.Position.BeforeEnd, "drawerOffsetLast": w.drawerList.Position.OffsetLast,
		"drawerCount": w.drawerList.Position.Count, "drawerLength": w.drawerList.Position.Length,
		"navigationRowHeight": w.navigationRowHeight, "utilityRowHeight": w.utilityRowHeight,
		"groupHeadingHeight": w.groupHeadingHeight, "helpHeight": w.helpHeight, "barHeight": w.barHeight,
	}}
}
