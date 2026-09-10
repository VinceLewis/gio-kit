package router

import "github.com/VinceLewis/gio-kit/diagnostic"

func (r *Router) DebugSnapshot(request diagnostic.Request) diagnostic.Component {
	r.mu.Lock()
	defer r.mu.Unlock()
	request = request.Bounded()
	routes := func(entries []Entry) []any {
		result := make([]any, 0)
		for i := request.Offset; i < len(entries) && len(result) < request.Limit; i++ {
			result = append(result, debugRoute(entries[i].Route))
		}
		return result
	}
	return diagnostic.Component{Kind: "router", State: map[string]any{
		"route": debugRoute(r.currentLocked().Route), "stack": routes(r.stack), "modals": routes(r.modals),
		"stackCount": len(r.stack), "modalCount": len(r.modals), "restorationVersion": 1,
		"guardInstalled": r.guard != nil, "activeGuard": "unknown",
	}}
}

func debugRoute(route Route) map[string]any {
	params := make(map[string]any, len(route.Params))
	for id, value := range route.Params {
		params[id] = value.String()
	}
	return map[string]any{"name": route.Name, "params": params}
}
