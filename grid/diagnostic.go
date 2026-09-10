package grid

import "github.com/VinceLewis/gio-kit/diagnostic"

// DebugSnapshot copies only the requested loaded rows, never the whole cache.
func (c *Controller) DebugSnapshot(request diagnostic.Request) diagnostic.Component {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := request.Bounded()
	if r.RecordID != "" {
		r.Offset = len(c.rows)
		for i, row := range c.rows {
			if row.ID == r.RecordID {
				r.Offset = i
				break
			}
		}
		r.Limit = 1
	}
	rows := make([]any, 0)
	for i := r.Offset; i < len(c.rows) && len(rows) < r.Limit; i++ {
		row := c.rows[i]
		cells := make(map[string]any, len(row.Cells))
		for id, value := range row.Cells {
			cells[id] = value
		}
		rows = append(rows, map[string]any{"index": i, "id": row.ID, "cells": cells, "selected": c.selection[row.ID]})
	}
	message := ""
	if c.err != nil {
		message = c.err.Error()
	}
	states := [...]string{"idle", "loading", "ready", "empty", "failed"}
	return diagnostic.Component{Kind: "grid", State: map[string]any{
		"totalRows": c.total, "loadedRange": []int{0, len(c.rows)}, "rows": rows,
		"state": states[c.state], "loading": c.state == Loading, "error": message,
		"hasMore": c.total < 0 || len(c.rows) < c.total, "pageSize": c.pageSize,
		"selectedCount": len(c.selection), "sort": append([]SortSpec(nil), c.sort...), "filters": cloneFilters(c.filters),
	}}
}

// DebugSnapshot must be called on the widget's frame goroutine.
func (w *Widget) DebugSnapshot(request diagnostic.Request) diagnostic.Component {
	result := w.Controller.DebugSnapshot(request)
	first, end := w.List.Position.First, w.List.Position.First+w.List.Position.Count
	result.State["renderedRange"] = []int{first, end}
	result.State["scroll"] = map[string]any{"first": first, "offset": w.List.Position.Offset, "horizontalFirst": w.Horizontal.Position.First, "horizontalOffset": w.Horizontal.Position.Offset}
	for _, value := range result.State["rows"].([]any) {
		row := value.(map[string]any)
		i := row["index"].(int)
		row["virtualized"] = i < first || i >= end
	}
	return result
}
