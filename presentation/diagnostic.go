package presentation

import "github.com/VinceLewis/gio-kit/diagnostic"

func (w *Widget) DebugSnapshot(request diagnostic.Request) diagnostic.Component {
	r := request.Bounded()
	sections := []any{}
	for i := r.Offset; i < len(w.Page.Sections) && len(sections) < r.Limit; i++ {
		section := w.Page.Sections[i]
		sections = append(sections, map[string]any{"id": section.ID, "heading": section.Heading, "listCount": len(section.Lists), "calendarCount": len(section.Calendars), "matrixCount": len(section.Matrices)})
	}
	return diagnostic.Component{Kind: "presentation", State: map[string]any{"title": w.Page.Title, "loading": w.Page.Loading, "error": w.Page.Error, "sectionCount": len(w.Page.Sections), "sections": sections, "first": w.list.Position.First, "offset": w.list.Position.Offset}}
}
