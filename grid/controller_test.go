package grid

import (
	"context"
	"errors"
	"fmt"
	"image"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

type memorySource struct {
	mu       sync.Mutex
	rows     []Row
	queries  []queryCall
	failNext bool
	delay    time.Duration
}

type queryCall struct {
	offset, limit int
	sort          []SortSpec
	filters       map[string]Filter
}

func (s *memorySource) Fetch(ctx context.Context, offset, limit int, specs []SortSpec, filters map[string]Filter) ([]Row, int, error) {
	s.mu.Lock()
	s.queries = append(s.queries, queryCall{offset: offset, limit: limit, sort: append([]SortSpec(nil), specs...), filters: cloneFilters(filters)})
	fail := s.failNext
	s.failNext = false
	delay := s.delay
	rows := cloneRows(s.rows)
	s.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		}
	}
	if fail {
		return nil, 0, errors.New("temporary fetch failure")
	}
	for column, filter := range filters {
		filtered := rows[:0]
		for _, row := range rows {
			value := strings.ToLower(row.Cells[column])
			query := strings.ToLower(filter.Value)
			match := false
			switch filter.Operator {
			case Equals:
				match = value == query
			case StartsWith:
				match = strings.HasPrefix(value, query)
			default:
				match = strings.Contains(value, query)
			}
			if match {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	for i := len(specs) - 1; i >= 0; i-- {
		spec := specs[i]
		sort.SliceStable(rows, func(i, j int) bool {
			less := rows[i].Cells[spec.ColumnID] < rows[j].Cells[spec.ColumnID]
			if spec.Descending {
				return !less && rows[i].Cells[spec.ColumnID] != rows[j].Cells[spec.ColumnID]
			}
			return less
		})
	}
	total := len(rows)
	if offset >= total {
		return nil, total, nil
	}
	end := min(total, offset+limit)
	return cloneRows(rows[offset:end]), total, nil
}

func columns() []Column {
	return []Column{
		{ID: "number", Header: "Number", Sortable: true, Filter: FilterText, Flex: 1},
		{ID: "priority", Header: "Priority", Sortable: true, Filter: FilterChoice, Flex: 1},
	}
}

func records(count int) []Row {
	rows := make([]Row, count)
	for i := range rows {
		rows[i] = Row{ID: fmt.Sprintf("id-%05d", i), Cells: map[string]string{
			"number": fmt.Sprintf("INC%05d", i), "priority": fmt.Sprintf("P%d", i%4+1),
		}}
	}
	return rows
}

func await(t *testing.T, controller *Controller, condition func(Snapshot) bool) Snapshot {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := controller.Snapshot()
		if condition(snapshot) {
			return snapshot
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for grid state")
	return Snapshot{}
}

func TestG1TenThousandRowsRemainVirtualized(t *testing.T) {
	source := &memorySource{rows: records(10_000)}
	controller, err := NewController(columns(), source, 50, nil)
	if err != nil {
		t.Fatal(err)
	}
	controller.mu.Lock()
	controller.rows = records(10_000)
	controller.total, controller.state = 10_000, Ready
	controller.mu.Unlock()

	widget := NewWidget(controller)
	theme := material.NewTheme()
	var ops op.Ops
	gtx := layout.Context{
		Ops: &ops, Constraints: layout.Exact(image.Pt(420, 600)),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
	}
	widget.Layout(gtx, theme)
	if laidOut := len(widget.rows); laidOut >= 100 {
		t.Fatalf("laid out %d of 10,000 rows; expected viewport-only layout", laidOut)
	}
}

func TestG3SortRefetchesAndSupportsMultipleKeys(t *testing.T) {
	source := &memorySource{rows: records(100)}
	controller, _ := NewController(columns(), source, 25, nil)
	if err := controller.ToggleSort("priority", false); err != nil {
		t.Fatal(err)
	}
	await(t, controller, func(s Snapshot) bool { return s.State == Ready })
	if err := controller.ToggleSort("number", true); err != nil {
		t.Fatal(err)
	}
	snapshot := await(t, controller, func(s Snapshot) bool { return s.State == Ready && len(s.Sort) == 2 })
	if snapshot.Sort[0].ColumnID != "priority" || snapshot.Sort[1].ColumnID != "number" {
		t.Fatalf("unexpected sort: %+v", snapshot.Sort)
	}
	source.mu.Lock()
	last := source.queries[len(source.queries)-1]
	source.mu.Unlock()
	if len(last.sort) != 2 || last.offset != 0 {
		t.Fatalf("data source did not receive refreshed multi-sort: %+v", last)
	}
}

func TestG4FilterIsSentToDataSource(t *testing.T) {
	source := &memorySource{rows: records(100)}
	controller, _ := NewController(columns(), source, 20, nil)
	if err := controller.SetFilter("number", Filter{Operator: Contains, Value: "00042"}); err != nil {
		t.Fatal(err)
	}
	snapshot := await(t, controller, func(s Snapshot) bool { return s.State == Ready })
	if snapshot.Total != 1 || snapshot.Rows[0].ID != "id-00042" {
		t.Fatalf("server filter result: total=%d rows=%v", snapshot.Total, snapshot.Rows)
	}
	source.mu.Lock()
	filter := source.queries[len(source.queries)-1].filters["number"]
	source.mu.Unlock()
	if filter.Value != "00042" {
		t.Fatal("filter was not sent to DataSource")
	}
}

func TestG5SelectionSurvivesPageBoundaries(t *testing.T) {
	source := &memorySource{rows: records(75)}
	controller, _ := NewController(columns(), source, 25, nil)
	_ = controller.Refresh()

	await(t, controller, func(s Snapshot) bool { return len(s.Rows) == 25 })
	controller.ToggleSelection("id-00003")
	_ = controller.LoadNext()
	await(t, controller, func(s Snapshot) bool { return len(s.Rows) == 50 })
	controller.ToggleSelection("id-00040")
	snapshot := controller.Snapshot()
	if !snapshot.Selection["id-00003"] || !snapshot.Selection["id-00040"] {
		t.Fatalf("selection was lost: %v", snapshot.Selection)
	}
}

func TestG7PagingAndG8ErrorRetryEmptyStates(t *testing.T) {
	source := &memorySource{rows: records(51), failNext: true}
	controller, _ := NewController(columns(), source, 25, nil)
	_ = controller.Refresh()
	await(t, controller, func(s Snapshot) bool { return s.State == Failed })
	_ = controller.Retry()
	await(t, controller, func(s Snapshot) bool { return len(s.Rows) == 25 && s.HasMore })
	_ = controller.LoadNext()
	await(t, controller, func(s Snapshot) bool { return len(s.Rows) == 50 })
	_ = controller.LoadNext()
	snapshot := await(t, controller, func(s Snapshot) bool { return len(s.Rows) == 51 })
	if snapshot.HasMore {
		t.Fatal("grid still reports more rows after last page")
	}

	emptySource := &memorySource{}
	empty, _ := NewController(columns(), emptySource, 25, nil)
	_ = empty.Refresh()
	await(t, empty, func(s Snapshot) bool { return s.State == Empty })
}

type staleSource struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
}

func (s *staleSource) Fetch(ctx context.Context, offset, limit int, sort []SortSpec, filters map[string]Filter) ([]Row, int, error) {
	s.mu.Lock()
	s.calls++
	call := s.calls
	s.mu.Unlock()
	if call == 1 {
		close(s.started)
		time.Sleep(80 * time.Millisecond) // Deliberately ignores cancellation.
		return []Row{{ID: "stale", Cells: map[string]string{"number": "STALE"}}}, 1, nil
	}
	return []Row{{ID: "fresh", Cells: map[string]string{"number": "FRESH"}}}, 1, nil
}

func TestOutOfOrderCompletionCannotReplaceNewQuery(t *testing.T) {
	source := &staleSource{started: make(chan struct{})}
	controller, _ := NewController(columns(), source, 25, nil)
	_ = controller.Refresh()
	select {
	case <-source.started:
	case <-time.After(time.Second):
		t.Fatal("first request did not start")
	}
	_ = controller.Refresh()
	snapshot := await(t, controller, func(s Snapshot) bool { return s.State == Ready })
	if snapshot.Rows[0].ID != "fresh" {
		t.Fatalf("first completion won: %v", snapshot.Rows)
	}
	time.Sleep(100 * time.Millisecond)
	if got := controller.Snapshot().Rows[0].ID; got != "fresh" {
		t.Fatalf("stale completion replaced query: %s", got)
	}
}

func TestG9PreferencesRoundTrip(t *testing.T) {
	source := &memorySource{}
	controller, _ := NewController(columns(), source, 25, nil)
	_ = controller.SetColumnVisible("priority", false)
	_ = controller.ResizeColumn("number", 144)
	_ = controller.MoveColumn("priority", 0)
	data, err := controller.SerializePreferences()
	if err != nil {
		t.Fatal(err)
	}
	restored, _ := NewController(columns(), source, 25, nil)
	if err := restored.RestorePreferences(data); err != nil {
		t.Fatal(err)
	}
	columns := restored.Snapshot().Columns
	if columns[0].ID != "priority" || columns[0].Visible || columns[1].Width != 144 {
		t.Fatalf("preferences not restored: %+v", columns)
	}
}

func TestG5SetRowsSelectedForHeaderCheckbox(t *testing.T) {
	controller, err := NewController(columns(), &memorySource{}, 25, nil)
	if err != nil {
		t.Fatal(err)
	}
	controller.SetRowsSelected([]string{"id-1", "id-2"}, true)
	selection := controller.Snapshot().Selection
	if !selection["id-1"] || !selection["id-2"] {
		t.Fatalf("rows were not selected: %v", selection)
	}
	controller.SetRowsSelected([]string{"id-1", "id-2"}, false)
	if selection := controller.Snapshot().Selection; len(selection) != 0 {
		t.Fatalf("rows were not deselected: %v", selection)
	}
}
