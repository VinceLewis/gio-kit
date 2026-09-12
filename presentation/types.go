// Package presentation provides reusable models and Gio rendering for
// composed pages. It deliberately has no knowledge of an application model or
// persistence layer.
package presentation

import (
	"fmt"
	"image/color"

	"github.com/VinceLewis/gio-kit/theme"
)

type Page struct {
	Title    string
	Loading  bool
	Error    string
	Sections []Section
	Legends  []Legend
	// Density is the page's declared density. An empty value inherits the
	// Widget's own declared density, and ultimately the Set default.
	Density string
}

type Section struct {
	ID, Heading, Comment string
	Layout               string
	Controls             []Control
	Actions              []Action
	Lists                []List
	Calendars            []Calendar
	Matrices             []Matrix
	// Density overrides the page density for this section and everything it
	// contains, unless a more specific declaration overrides it again.
	Density string
}

type Control struct {
	ID, Kind, Label, Value, Icon, DisabledReason string
	Enabled                                      bool
	Options                                      []Option
}

type Option struct{ Value, Label string }

type Action struct {
	ID, Label, Icon, Placement, DisabledReason string
	Enabled                                    bool
}

type List struct {
	ID, Heading, Style, EmptyText string
	Rows                          []Row
	Actions                       []Action
	// Density overrides the section density for this list's own heading and
	// spacing.
	Density string
	// RowDensity overrides Density for the list's row template. An empty
	// value inherits Density.
	RowDensity string
	// RowLayout selects how a row's fragments are arranged. See
	// RowLayoutInline and RowLayoutStack; an empty value behaves as
	// RowLayoutInline.
	RowLayout string
}

// List.Style selects how a list's rows render.
//
// feed and compactFeed render the divider-separated feed; table, cards, and
// the empty default render the existing card rows. A caller that never set
// Style keeps its current rendering.
const (
	ListStyleDefault     = ""
	ListStyleTable       = "table"
	ListStyleFeed        = "feed"
	ListStyleCompactFeed = "compactFeed"
	ListStyleCards       = "cards"
)

// RowLayout selects how a row's fragments are arranged within a card or feed
// row. An empty value behaves as RowLayoutInline.
const (
	// RowLayoutInline lays fragments out as one ordered rich-text run that
	// wraps at the available width.
	RowLayoutInline = "inline"
	// RowLayoutStack lays out one fragment per line.
	RowLayoutStack = "stack"
)

type Row struct {
	ID, Status, AccessibleLabel string
	StatusLabel, StatusIcon     string
	StatusAccessibleLabel       string
	ColorToken                  string
	Color                       color.NRGBA
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
	// Density overrides the section density for this calendar.
	Density string
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
	// Density overrides the section density for this matrix.
	Density string
}

type MatrixRow struct {
	ID, Label string
	Cells     []MatrixCell
}

type MatrixCell struct {
	Column, Text, Status, AccessibleLabel string
	ColorToken                            string
	Color                                 color.NRGBA
	Enabled                               bool
}

type Legend struct {
	Title string
	Items []LegendItem
}

type LegendItem struct {
	Status, Label, AccessibleLabel, Icon, ColorToken string
	Color                                            color.NRGBA
}

type Event struct {
	Kind, ID, Value, RowID, Date, Column string
}

const (
	EventControl       = "control"
	EventAction        = "action"
	EventCalendarMonth = "calendarMonth"
	EventMatrixCell    = "matrixCell"
)

// Validate reports the first unknown density or list style in the page. An
// empty value is valid and means "inherit". Validate walks the page, its
// sections, and every list, calendar, and matrix they contain.
func (p Page) Validate() error {
	if err := theme.ValidateDensity(p.Density); err != nil {
		return fmt.Errorf("presentation: page density %q: %w", p.Density, err)
	}
	for _, section := range p.Sections {
		if err := theme.ValidateDensity(section.Density); err != nil {
			return fmt.Errorf("presentation: section %q density %q: %w", section.ID, section.Density, err)
		}
		for _, list := range section.Lists {
			if err := theme.ValidateDensity(list.Density); err != nil {
				return fmt.Errorf("presentation: list %q density %q: %w", list.ID, list.Density, err)
			}
			if err := theme.ValidateDensity(list.RowDensity); err != nil {
				return fmt.Errorf("presentation: list %q row density %q: %w", list.ID, list.RowDensity, err)
			}
			if err := ValidateListStyle(list.Style); err != nil {
				return fmt.Errorf("presentation: list %q: %w", list.ID, err)
			}
			if err := validateRowLayout(list.RowLayout); err != nil {
				return fmt.Errorf("presentation: list %q: %w", list.ID, err)
			}
		}
		for _, calendar := range section.Calendars {
			if err := theme.ValidateDensity(calendar.Density); err != nil {
				return fmt.Errorf("presentation: calendar %q density %q: %w", calendar.ID, calendar.Density, err)
			}
		}
		for _, matrix := range section.Matrices {
			if err := theme.ValidateDensity(matrix.Density); err != nil {
				return fmt.Errorf("presentation: matrix %q density %q: %w", matrix.ID, matrix.Density, err)
			}
		}
	}
	return nil
}

// ValidateListStyle reports whether style is a known List.Style value. An
// empty style is valid and selects the default card rendering.
func ValidateListStyle(style string) error {
	switch style {
	case ListStyleDefault, ListStyleTable, ListStyleFeed, ListStyleCompactFeed, ListStyleCards:
		return nil
	}
	return fmt.Errorf("presentation: unknown list style %q", style)
}

// validateRowLayout reports whether layout is a known List.RowLayout value.
// An empty value is valid and behaves as RowLayoutInline.
func validateRowLayout(layout string) error {
	switch layout {
	case "", RowLayoutInline, RowLayoutStack:
		return nil
	}
	return fmt.Errorf("presentation: unknown row layout %q", layout)
}
