// Package presentation provides reusable models and Gio rendering for
// composed pages. It deliberately has no knowledge of an application model or
// persistence layer.
package presentation

type Page struct {
	Title    string
	Loading  bool
	Error    string
	Sections []Section
	Legends  []Legend
}

type Section struct {
	ID, Heading, Comment string
	Layout               string
	Controls             []Control
	Actions              []Action
	Lists                []List
	Calendars            []Calendar
	Matrices             []Matrix
}

type Control struct {
	ID, Kind, Label, Value, DisabledReason string
	Enabled                                bool
	Options                                []Option
}

type Option struct{ Value, Label string }

type Action struct {
	ID, Label, Icon, DisabledReason string
	Enabled                         bool
}

type List struct {
	ID, Heading, Style, EmptyText string
	Rows                          []Row
	Actions                       []Action
}

type Row struct {
	ID, Status, AccessibleLabel string
	Fragments                   []Fragment
	Actions                     []Action
}

type Fragment struct {
	Kind, Text, Icon, Style, AccessibleLabel string
}

type Calendar struct {
	ID, Heading, Month, EmptyText string
	Days                          []CalendarDay
	Actions                       []Action
}

type CalendarDay struct {
	Date, Label string
	Rows        []Row
}

type Matrix struct {
	ID, Heading, EmptyText, DisabledReason string
	Columns                                []string
	Rows                                   []MatrixRow
	Editable                               bool
}

type MatrixRow struct {
	ID, Label string
	Cells     []MatrixCell
}

type MatrixCell struct {
	Column, Text, Status, AccessibleLabel string
	Enabled                               bool
}

type Legend struct {
	Title string
	Items []LegendItem
}

type LegendItem struct{ Status, Label, AccessibleLabel string }

type Event struct {
	Kind, ID, Value, RowID, Date, Column string
}

const (
	EventControl       = "control"
	EventAction        = "action"
	EventCalendarMonth = "calendarMonth"
	EventMatrixCell    = "matrixCell"
)
