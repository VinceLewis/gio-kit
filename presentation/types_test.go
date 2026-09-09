package presentation

import "testing"

func TestPageModelsAllReusableComposedRegions(t *testing.T) {
	page := Page{Sections: []Section{{Controls: []Control{{ID: "filter", Kind: "toggle", Enabled: true}}, Lists: []List{{Rows: []Row{{ID: "one"}}}}, Calendars: []Calendar{{Days: []CalendarDay{{Date: "2026-09-09"}}}}, Matrices: []Matrix{{Rows: []MatrixRow{{Cells: []MatrixCell{{Column: "2026-09-09"}}}}}}}}}
	if len(page.Sections[0].Controls) != 1 || len(page.Sections[0].Lists[0].Rows) != 1 || len(page.Sections[0].Calendars[0].Days) != 1 || len(page.Sections[0].Matrices[0].Rows[0].Cells) != 1 {
		t.Fatalf("composed page = %#v", page)
	}
}
