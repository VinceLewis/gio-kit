// Package gridsqlite adapts database/sql SQLite tables to grid.DataSource.
package gridsqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/VinceLewis/gio-kit/grid"
)

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Config struct {
	Table    string
	IDColumn string
	// Columns maps public grid column IDs to trusted SQLite column names.
	Columns map[string]string
}

type Source struct {
	db      *sql.DB
	table   string
	id      string
	columns map[string]string
	order   []string
}

func New(db *sql.DB, config Config) (*Source, error) {
	if db == nil {
		return nil, errors.New("grid/sqlite: database is nil")
	}
	if !identifier.MatchString(config.Table) || !identifier.MatchString(config.IDColumn) {
		return nil, errors.New("grid/sqlite: unsafe table or id identifier")
	}
	if len(config.Columns) == 0 {
		return nil, errors.New("grid/sqlite: at least one column is required")
	}
	columns := make(map[string]string, len(config.Columns))
	order := make([]string, 0, len(config.Columns))
	for publicID, sqlName := range config.Columns {
		if !identifier.MatchString(publicID) || !identifier.MatchString(sqlName) {
			return nil, fmt.Errorf("grid/sqlite: unsafe column mapping %q", publicID)
		}
		columns[publicID] = sqlName
		order = append(order, publicID)
	}
	sort.Strings(order)
	return &Source{db: db, table: config.Table, id: config.IDColumn, columns: columns, order: order}, nil
}

func (s *Source) Fetch(ctx context.Context, offset, limit int, specs []grid.SortSpec, filters map[string]grid.Filter) ([]grid.Row, int, error) {
	if offset < 0 || limit <= 0 {
		return nil, 0, errors.New("grid/sqlite: invalid page")
	}
	where, args, err := s.where(filters)
	if err != nil {
		return nil, 0, err
	}
	countQuery := "SELECT COUNT(*) FROM " + quote(s.table) + where
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	orderBy, err := s.orderBy(specs)
	if err != nil {
		return nil, 0, err
	}
	selects := []string{"CAST(" + quote(s.id) + " AS TEXT)"}
	for _, publicID := range s.order {
		selects = append(selects, "CAST("+quote(s.columns[publicID])+" AS TEXT)")
	}
	query := "SELECT " + strings.Join(selects, ", ") + " FROM " + quote(s.table) + where + orderBy + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any(nil), args...), limit, offset)
	result, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer result.Close()
	rows := make([]grid.Row, 0, max(0, min(limit, total-offset)))
	for result.Next() {
		values := make([]sql.NullString, len(s.order)+1)
		destinations := make([]any, len(values))
		for i := range values {
			destinations[i] = &values[i]
		}
		if err := result.Scan(destinations...); err != nil {
			return nil, 0, err
		}
		row := grid.Row{ID: values[0].String, Cells: make(map[string]string, len(s.order))}
		for i, publicID := range s.order {
			row.Cells[publicID] = values[i+1].String
		}
		rows = append(rows, row)
	}
	return rows, total, result.Err()
}

func (s *Source) where(filters map[string]grid.Filter) (string, []any, error) {
	keys := make([]string, 0, len(filters))
	for key := range filters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	clauses := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys)*2)
	for _, publicID := range keys {
		column, ok := s.columns[publicID]
		if !ok {
			return "", nil, fmt.Errorf("grid/sqlite: unknown filter column %q", publicID)
		}
		filter := filters[publicID]
		name := quote(column)
		switch filter.Operator {
		case grid.Equals:
			clauses = append(clauses, name+" = ?")
			args = append(args, filter.Value)
		case grid.StartsWith:
			clauses = append(clauses, "LOWER(CAST("+name+" AS TEXT)) LIKE LOWER(?) ESCAPE '\\'")
			args = append(args, escapeLike(filter.Value)+"%")
		case grid.Between:
			if filter.To == "" {
				return "", nil, fmt.Errorf("grid/sqlite: between filter %q has no upper value", publicID)
			}
			clauses = append(clauses, name+" BETWEEN ? AND ?")
			args = append(args, filter.Value, filter.To)
		case grid.Contains, "":
			clauses = append(clauses, "LOWER(CAST("+name+" AS TEXT)) LIKE LOWER(?) ESCAPE '\\'")
			args = append(args, "%"+escapeLike(filter.Value)+"%")
		default:
			return "", nil, fmt.Errorf("grid/sqlite: unsupported filter operator %q", filter.Operator)
		}
	}
	if len(clauses) == 0 {
		return "", args, nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}

func (s *Source) orderBy(specs []grid.SortSpec) (string, error) {
	if len(specs) == 0 {
		return " ORDER BY " + quote(s.id) + " ASC", nil
	}
	parts := make([]string, len(specs))
	for i, spec := range specs {
		column, ok := s.columns[spec.ColumnID]
		if !ok {
			return "", fmt.Errorf("grid/sqlite: unknown sort column %q", spec.ColumnID)
		}
		direction := " ASC"
		if spec.Descending {
			direction = " DESC"
		}
		parts[i] = quote(column) + direction
	}
	return " ORDER BY " + strings.Join(parts, ", ") + ", " + quote(s.id) + " ASC", nil
}

func quote(value string) string { return `"` + value + `"` }

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
