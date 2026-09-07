package grid

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gioui.org/unit"
	"sync"
	"time"
)

var ErrClosed = errors.New("grid: controller is closed")

type preferences struct {
	Version int      `json:"version"`
	Columns []Column `json:"columns"`
}

// Controller owns query, paging, selection, and personalization state. It is
// safe for frame-loop and background use; Fetch always runs asynchronously.
type Controller struct {
	mu         sync.RWMutex
	source     DataSource
	columns    []Column
	pageSize   int
	rows       []Row
	total      int
	sort       []SortSpec
	filters    map[string]Filter
	selection  map[string]bool
	state      LoadState
	err        error
	updatedAt  time.Time
	generation uint64
	cancel     context.CancelFunc
	notify     func()
	closed     bool
}

func NewController(columns []Column, source DataSource, pageSize int, notify func()) (*Controller, error) {
	if source == nil {
		return nil, errors.New("grid: data source is nil")
	}
	if pageSize <= 0 {
		return nil, errors.New("grid: page size must be positive")
	}
	seen := make(map[string]bool, len(columns))
	copyColumns := append([]Column(nil), columns...)
	for i := range copyColumns {
		column := &copyColumns[i]
		if column.ID == "" || seen[column.ID] {
			return nil, fmt.Errorf("grid: empty or duplicate column id %q", column.ID)
		}
		seen[column.ID] = true
		if column.Header == "" {
			column.Header = column.ID
		}
		// A zero-value column is visible by default. Call SetColumnVisible to hide it.
		if !column.Visible {
			column.Visible = true
		}
		if column.Width <= 0 && column.Flex <= 0 {
			column.Flex = 1
		}
	}
	return &Controller{
		source: source, columns: copyColumns, pageSize: pageSize, total: -1,
		filters: make(map[string]Filter), selection: make(map[string]bool),
		state: Idle, notify: notify,
	}, nil
}

// Refresh cancels any older request, clears loaded pages, and fetches page one.
func (c *Controller) Refresh() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrClosed
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.generation++
	c.rows, c.total, c.err = nil, -1, nil
	c.startFetchLocked(0)
	notify := c.notify
	c.mu.Unlock()
	if notify != nil {
		notify()
	}
	return nil
}

// LoadNext requests exactly one next page. Calls while loading or at the end
// are harmless, making it safe to invoke near the end of every Gio frame.
func (c *Controller) LoadNext() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrClosed
	}
	if c.state == Loading || (c.total >= 0 && len(c.rows) >= c.total) {
		return nil
	}
	c.startFetchLocked(len(c.rows))
	return nil
}

func (c *Controller) Retry() error { return c.LoadNext() }

func (c *Controller) SetFilter(columnID string, filter Filter) error {
	c.mu.Lock()
	if !c.hasColumnLocked(columnID) {
		c.mu.Unlock()
		return fmt.Errorf("grid: unknown column %q", columnID)
	}
	if filter.Value == "" && filter.To == "" {
		delete(c.filters, columnID)
	} else {
		c.filters[columnID] = filter
	}
	c.mu.Unlock()
	return c.Refresh()
}

func (c *Controller) SetFilters(filters map[string]Filter) error {
	c.mu.Lock()
	for columnID := range filters {
		if !c.hasColumnLocked(columnID) {
			c.mu.Unlock()
			return fmt.Errorf("grid: unknown column %q", columnID)
		}
	}
	c.filters = cloneFilters(filters)
	c.mu.Unlock()
	return c.Refresh()
}

// ToggleSort makes columnID the primary sort unless additive is true. Repeated
// activation toggles ascending/descending; additive activation appends a key.
func (c *Controller) ToggleSort(columnID string, additive bool) error {
	c.mu.Lock()
	column, ok := c.columnLocked(columnID)
	if !ok || !column.Sortable {
		c.mu.Unlock()
		return fmt.Errorf("grid: column %q is not sortable", columnID)
	}
	index := -1
	for i, spec := range c.sort {
		if spec.ColumnID == columnID {
			index = i
			break
		}
	}
	if !additive {
		descending := index == 0 && !c.sort[0].Descending
		c.sort = []SortSpec{{ColumnID: columnID, Descending: descending}}
	} else if index >= 0 {
		c.sort[index].Descending = !c.sort[index].Descending
	} else {
		c.sort = append(c.sort, SortSpec{ColumnID: columnID})
	}
	c.mu.Unlock()
	return c.Refresh()
}

func (c *Controller) ToggleSelection(rowID string) {
	c.mu.Lock()
	if c.selection[rowID] {
		delete(c.selection, rowID)
	} else if rowID != "" {
		c.selection[rowID] = true
	}
	notify := c.notify
	c.mu.Unlock()
	if notify != nil {
		notify()
	}
}

func (c *Controller) ClearSelection() {
	c.mu.Lock()
	c.selection = make(map[string]bool)
	notify := c.notify
	c.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// SetRowsSelected selects or deselects a known set of row IDs in one update.
func (c *Controller) SetRowsSelected(rowIDs []string, selected bool) {
	c.mu.Lock()
	for _, rowID := range rowIDs {
		if rowID == "" {
			continue
		}
		if selected {
			c.selection[rowID] = true
		} else {
			delete(c.selection, rowID)
		}
	}
	notify := c.notify
	c.mu.Unlock()
	if notify != nil {
		notify()
	}
}

func (c *Controller) SetColumnVisible(columnID string, visible bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	column, ok := c.columnPointerLocked(columnID)
	if !ok {
		return fmt.Errorf("grid: unknown column %q", columnID)
	}
	column.Visible = visible
	return nil
}

func (c *Controller) ResizeColumn(columnID string, width unit.Dp) error {
	if width <= 0 {
		return errors.New("grid: width must be positive")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	column, ok := c.columnPointerLocked(columnID)
	if !ok {
		return fmt.Errorf("grid: unknown column %q", columnID)
	}
	column.Width, column.Flex = width, 0
	return nil
}

func (c *Controller) MoveColumn(columnID string, target int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if target < 0 || target >= len(c.columns) {
		return errors.New("grid: target column index is out of range")
	}
	from := -1
	for i := range c.columns {
		if c.columns[i].ID == columnID {
			from = i
			break
		}
	}
	if from < 0 {
		return fmt.Errorf("grid: unknown column %q", columnID)
	}
	column := c.columns[from]
	c.columns = append(c.columns[:from], c.columns[from+1:]...)
	c.columns = append(c.columns, Column{})
	copy(c.columns[target+1:], c.columns[target:])
	c.columns[target] = column
	return nil
}

func (c *Controller) SerializePreferences() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return json.Marshal(preferences{Version: 1, Columns: c.columns})
}

func (c *Controller) RestorePreferences(data []byte) error {
	var saved preferences
	if err := json.Unmarshal(data, &saved); err != nil || saved.Version != 1 {
		return errors.New("grid: invalid preferences")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(saved.Columns) != len(c.columns) {
		return errors.New("grid: preferences do not match columns")
	}
	known := make(map[string]bool, len(c.columns))
	for _, column := range c.columns {
		known[column.ID] = true
	}
	for _, column := range saved.Columns {
		if !known[column.ID] {
			return errors.New("grid: preferences contain an unknown column")
		}
	}
	c.columns = append([]Column(nil), saved.Columns...)
	return nil
}

func (c *Controller) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Snapshot{
		Columns: append([]Column(nil), c.columns...), Rows: cloneRows(c.rows), Total: c.total,
		Sort: append([]SortSpec(nil), c.sort...), Filters: cloneFilters(c.filters),
		Selection: cloneSelection(c.selection), State: c.state, Err: c.err,
		HasMore: c.total < 0 || len(c.rows) < c.total, UpdatedAt: c.updatedAt,
	}
}

func (c *Controller) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.generation++
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *Controller) startFetchLocked(offset int) {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel, c.state, c.err = cancel, Loading, nil
	generation := c.generation
	sort := append([]SortSpec(nil), c.sort...)
	filters := cloneFilters(c.filters)
	go c.fetch(ctx, generation, offset, sort, filters)
}

func (c *Controller) fetch(ctx context.Context, generation uint64, offset int, sort []SortSpec, filters map[string]Filter) {
	rows, total, err := c.source.Fetch(ctx, offset, c.pageSize, sort, filters)
	c.mu.Lock()
	if c.closed || generation != c.generation || ctx.Err() != nil {
		c.mu.Unlock()
		return
	}
	c.cancel = nil
	if err != nil {
		c.err, c.state, c.updatedAt = err, Failed, time.Now()
	} else {
		c.rows = append(c.rows, cloneRows(rows)...)
		c.total, c.updatedAt = max(total, len(c.rows)), time.Now()
		if len(c.rows) == 0 && c.total == 0 {
			c.state = Empty
		} else {
			c.state = Ready
		}
	}
	notify := c.notify
	c.mu.Unlock()
	if notify != nil {
		notify()
	}
}

func (c *Controller) hasColumnLocked(id string) bool {
	_, ok := c.columnLocked(id)
	return ok
}

func (c *Controller) columnLocked(id string) (Column, bool) {
	for _, column := range c.columns {
		if column.ID == id {
			return column, true
		}
	}
	return Column{}, false
}

func (c *Controller) columnPointerLocked(id string) (*Column, bool) {
	for i := range c.columns {
		if c.columns[i].ID == id {
			return &c.columns[i], true
		}
	}
	return nil, false
}

func cloneRows(rows []Row) []Row {
	copyRows := make([]Row, len(rows))
	for i, row := range rows {
		cells := make(map[string]string, len(row.Cells))
		for key, value := range row.Cells {
			cells[key] = value
		}
		copyRows[i] = Row{ID: row.ID, Cells: cells}
	}
	return copyRows
}

func cloneFilters(filters map[string]Filter) map[string]Filter {
	copyFilters := make(map[string]Filter, len(filters))
	for key, value := range filters {
		copyFilters[key] = value
	}
	return copyFilters
}

func cloneSelection(selection map[string]bool) map[string]bool {
	copySelection := make(map[string]bool, len(selection))
	for key, value := range selection {
		copySelection[key] = value
	}
	return copySelection
}
