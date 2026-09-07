// Package grid provides asynchronous, pageable data-grid state and a Gio
// renderer. Controller contains the reusable behavior; Widget is the optional
// Gio presentation layer.
package grid

import (
	"context"
	"time"

	"gioui.org/unit"
)

type Alignment uint8

const (
	AlignStart Alignment = iota
	AlignMiddle
	AlignEnd
)

type FilterKind uint8

const (
	FilterNone FilterKind = iota
	FilterText
	FilterNumber
	FilterDateRange
	FilterChoice
)

type FilterOperator string

const (
	Contains   FilterOperator = "contains"
	Equals     FilterOperator = "equals"
	StartsWith FilterOperator = "starts-with"
	Between    FilterOperator = "between"
)

// Filter is passed unchanged to DataSource; filtering is never performed on a
// partially loaded page by Controller.
type Filter struct {
	Operator FilterOperator `json:"operator"`
	Value    string         `json:"value"`
	To       string         `json:"to,omitempty"`
}

type Column struct {
	ID       string
	Header   string
	Width    unit.Dp
	Flex     float32
	Align    Alignment
	Sortable bool
	Visible  bool
	Filter   FilterKind
}

type SortSpec struct {
	ColumnID   string `json:"columnId"`
	Descending bool   `json:"descending"`
}

type Row struct {
	ID    string
	Cells map[string]string
}

type DataSource interface {
	Fetch(ctx context.Context, offset, limit int, sort []SortSpec, filters map[string]Filter) (rows []Row, total int, err error)
}

type LoadState uint8

const (
	Idle LoadState = iota
	Loading
	Ready
	Empty
	Failed
)

type Snapshot struct {
	Columns   []Column
	Rows      []Row
	Total     int
	Sort      []SortSpec
	Filters   map[string]Filter
	Selection map[string]bool
	State     LoadState
	Err       error
	HasMore   bool
	UpdatedAt time.Time
}
