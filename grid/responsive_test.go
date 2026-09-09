package grid

import (
	"errors"
	"testing"

	"gioui.org/unit"
)

func TestG2ResponsiveViewMode(t *testing.T) {
	for _, test := range []struct {
		name       string
		mode       ViewMode
		width      unit.Dp
		breakpoint unit.Dp
		want       ViewMode
	}{
		{name: "narrow auto uses cards", mode: ViewAuto, width: 599, breakpoint: 600, want: ViewCards},
		{name: "wide auto uses table", mode: ViewAuto, width: 600, breakpoint: 600, want: ViewTable},
		{name: "default breakpoint", mode: ViewAuto, width: 500, want: ViewCards},
		{name: "table override", mode: ViewTable, width: 320, breakpoint: 600, want: ViewTable},
		{name: "card override", mode: ViewCards, width: 900, breakpoint: 600, want: ViewCards},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ResolveViewMode(test.mode, test.width, test.breakpoint); got != test.want {
				t.Fatalf("ResolveViewMode() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestGridStateMessagesDistinguishEmptyAndFilteredResults(t *testing.T) {
	if got := emptyMessage(Snapshot{}); got != "No records yet." {
		t.Fatalf("unfiltered empty message = %q", got)
	}
	if got := emptyMessage(Snapshot{Filters: map[string]Filter{"name": {Operator: Contains, Value: "x"}}}); got != "No records match the current filters." {
		t.Fatalf("filtered empty message = %q", got)
	}
	if got := failureMessage(Snapshot{State: Failed, Err: errors.New("policy denied")}); got != "Could not load records. policy denied" {
		t.Fatalf("failed message = %q", got)
	}
}

func TestG2TableMinimumWidth(t *testing.T) {
	columns := []Column{{Width: 100}, {Flex: 2}, {Flex: 1}}
	if got, want := tableMinimumWidth(columns, true), unit.Dp(576); got != want {
		t.Fatalf("tableMinimumWidth() = %v, want %v", got, want)
	}
}

func TestG2OpenColumnMinimumWidth(t *testing.T) {
	columns := []Column{
		{ID: "number", Width: 100},
		{ID: "description", Flex: 2},
	}
	adjusted := setColumnMinimumWidth(columns, "number", 148)
	if got := adjusted[0].Width; got != 148 {
		t.Fatalf("open column width = %v, want 148dp", got)
	}
	if columns[0].Width != 100 {
		t.Fatal("source column configuration was mutated")
	}
	unchanged := setColumnMinimumWidth(columns, "number", 80)
	if got := unchanged[0].Width; got != 100 {
		t.Fatalf("open column shrank to %v", got)
	}
}
