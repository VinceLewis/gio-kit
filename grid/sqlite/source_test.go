package gridsqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/VinceLewis/gio-kit/grid"
	_ "github.com/mattn/go-sqlite3"
)

func testSource(t *testing.T) *Source {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "grid.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
CREATE TABLE incident (sys_id TEXT PRIMARY KEY, number TEXT, description TEXT, priority INTEGER, opened TEXT);
INSERT INTO incident VALUES
 ('1', 'INC0001', 'Network outage', 2, '2026-01-01'),
 ('2', 'INC0002', 'Network latency', 1, '2026-01-02'),
 ('3', 'INC0003', 'Printer stalled', 3, '2026-01-03');`)
	if err != nil {
		t.Fatal(err)
	}
	source, err := New(db, Config{Table: "incident", IDColumn: "sys_id", Columns: map[string]string{
		"number": "number", "description": "description", "priority": "priority", "opened": "opened",
	}})
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func TestSQLiteFetchFiltersSortsCountsAndPages(t *testing.T) {
	source := testSource(t)
	rows, total, err := source.Fetch(context.Background(), 0, 1,
		[]grid.SortSpec{{ColumnID: "priority"}},
		map[string]grid.Filter{"description": {Operator: grid.Contains, Value: "network"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(rows) != 1 || rows[0].ID != "2" || rows[0].Cells["number"] != "INC0002" {
		t.Fatalf("unexpected SQL page: total=%d rows=%+v", total, rows)
	}
	rows, total, err = source.Fetch(context.Background(), 1, 1,
		[]grid.SortSpec{{ColumnID: "priority"}},
		map[string]grid.Filter{"description": {Operator: grid.StartsWith, Value: "net"}},
	)
	if err != nil || total != 2 || len(rows) != 1 || rows[0].ID != "1" {
		t.Fatalf("unexpected second SQL page: total=%d rows=%+v err=%v", total, rows, err)
	}
}

func TestSQLiteRangeAndLiteralWildcard(t *testing.T) {
	source := testSource(t)
	rows, total, err := source.Fetch(context.Background(), 0, 10, nil,
		map[string]grid.Filter{"opened": {Operator: grid.Between, Value: "2026-01-02", To: "2026-01-03"}},
	)
	if err != nil || total != 2 || len(rows) != 2 {
		t.Fatalf("range result: total=%d rows=%+v err=%v", total, rows, err)
	}
	rows, total, err = source.Fetch(context.Background(), 0, 10, nil,
		map[string]grid.Filter{"description": {Operator: grid.Contains, Value: "%"}},
	)
	if err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("LIKE wildcard was not escaped: total=%d rows=%+v err=%v", total, rows, err)
	}
}

func TestSQLiteRejectsUntrustedIdentifiersAndColumns(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "grid.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := New(db, Config{Table: "incident; DROP TABLE incident", IDColumn: "id", Columns: map[string]string{"x": "x"}}); err == nil {
		t.Fatal("unsafe identifier accepted")
	}
	source := testSource(t)
	if _, _, err := source.Fetch(context.Background(), 0, 10, []grid.SortSpec{{ColumnID: "missing"}}, nil); err == nil {
		t.Fatal("unknown sort column accepted")
	}
}
