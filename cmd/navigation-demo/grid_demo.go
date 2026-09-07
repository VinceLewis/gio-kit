package main

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/grid"
	gridsqlite "github.com/VinceLewis/gio-kit/grid/sqlite"
	"github.com/VinceLewis/gio-kit/router"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed incidents.sql
var incidentSQL string

type gridSetup struct {
	db         *sql.DB
	controller *grid.Controller
	err        error
}

type flakySource struct {
	source   grid.DataSource
	failNext atomic.Bool
	delay    time.Duration
}

func (s *flakySource) Fetch(ctx context.Context, offset, limit int, sort []grid.SortSpec, filters map[string]grid.Filter) ([]grid.Row, int, error) {
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		}
	}
	if s.failNext.CompareAndSwap(true, false) {
		return nil, 0, errors.New("simulated SQLite query failure")
	}
	return s.source.Fetch(ctx, offset, limit, sort, filters)
}

type gridDemo struct {
	ready       chan gridSetup
	db          *sql.DB
	controller  *grid.Controller
	widget      *grid.Widget
	flaky       *flakySource
	setupErr    error
	filter      widget.Editor
	applyFilter widget.Clickable
	clearFilter widget.Clickable
	empty       widget.Clickable
	fail        widget.Clickable
	addSort     widget.Clickable
	additive    bool
}

func newGridDemo(window *app.Window, dataDir string) *gridDemo {
	demo := &gridDemo{ready: make(chan gridSetup, 1)}
	demo.filter.SingleLine = true
	go func() {
		db, err := sql.Open("sqlite3", filepath.Join(dataDir, "gio-kit-demo.db")+"?_busy_timeout=5000&_foreign_keys=on")
		if err == nil {
			db.SetMaxOpenConns(1)
			_, err = db.Exec(incidentSQL)
			if err != nil {
				if migrationErr := migrateIncidentTable(db); migrationErr != nil {
					err = errors.Join(err, migrationErr)
				} else {
					_, err = db.Exec(incidentSQL)
				}
			} else {
				err = migrateIncidentTable(db)
			}
		}
		if err != nil {
			if db != nil {
				_ = db.Close()
			}
			demo.ready <- gridSetup{err: err}
			window.Invalidate()
			return
		}
		source, err := gridsqlite.New(db, gridsqlite.Config{
			Table: "incident", IDColumn: "sys_id", Columns: map[string]string{
				"number": "number", "description": "short_description", "priority": "priority", "state": "state", "opened": "opened_at", "category": "category", "configuration_item": "configuration_item", "details": "details", "active": "active",
			},
		})
		if err != nil {
			_ = db.Close()
			demo.ready <- gridSetup{err: err}
			window.Invalidate()
			return
		}
		flaky := &flakySource{source: source, delay: 350 * time.Millisecond}
		controller, err := grid.NewController([]grid.Column{
			{ID: "number", Header: "Number", Width: 105, Sortable: true, Visible: true, Filter: grid.FilterText},
			{ID: "description", Header: "Description", Flex: 2, Sortable: true, Visible: true, Filter: grid.FilterText},
			{ID: "priority", Header: "P", Width: 44, Align: grid.AlignMiddle, Sortable: true, Visible: true, Filter: grid.FilterChoice},
			{ID: "state", Header: "State", Flex: 1, Sortable: true, Visible: true, Filter: grid.FilterChoice},
		}, flaky, 50, window.Invalidate)
		if err == nil {
			err = controller.Refresh()
		}
		demo.flaky = flaky
		demo.ready <- gridSetup{db: db, controller: controller, err: err}
		window.Invalidate()
	}()
	return demo
}

func (d *gridDemo) poll(ui *demoUI) {
	if d.controller != nil || d.setupErr != nil {
		return
	}
	select {
	case result := <-d.ready:
		d.db, d.controller, d.setupErr = result.db, result.controller, result.err
		if d.controller != nil {
			d.widget = grid.NewWidget(d.controller)
			d.widget.OpenColumn = "number"
			d.widget.OnRow = func(row grid.Row) { d.openRow(ui, row) }
		}
	default:
	}
}

func (d *gridDemo) openRow(ui *demoUI, row grid.Row) {
	err := ui.router.Push(router.Route{Name: "incident.form", Params: router.Params{
		"id": router.String(row.ID), "number": router.String(row.Cells["number"]), "openedFrom": router.String("SQLite grid"),
		"description": router.String(row.Cells["description"]), "priority": router.String(row.Cells["priority"]),
		"state": router.String(row.Cells["state"]), "opened": router.String(row.Cells["opened"]),
		"category": router.String(row.Cells["category"]), "configuration_item": router.String(row.Cells["configuration_item"]),
		"details": router.String(row.Cells["details"]), "active": router.Bool(row.Cells["active"] == "1"),
	}})
	if err != nil {
		ui.fail(err)
	} else {
		ui.status, ui.statusOK = "Grid row pushed through the shared router.", true
	}
}

func (d *gridDemo) Layout(gtx layout.Context, ui *demoUI) layout.Dimensions {
	d.poll(ui)
	if d.setupErr != nil {
		return ui.infoCard(gtx, "SQLite setup failed", d.setupErr.Error())
	}
	if d.controller == nil {
		return layout.Center.Layout(gtx, material.Body1(ui.theme, "Creating the SQLite database and 10,000 incidents…").Layout)
	}
	snapshot := d.controller.Snapshot()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ui.heading(gtx, "SQLite Incident Grid", "10,000 rows • SQL filtering/sorting • 50-row async pages")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					editor := material.Editor(ui.theme, &d.filter, "Description contains…")
					return ui.panel(gtx, editor.Layout)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(6)}.Layout(gtx) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if d.applyFilter.Clicked(gtx) {
						_ = d.controller.SetFilter("description", grid.Filter{Operator: grid.Contains, Value: d.filter.Text()})
						d.widget.List.Position = layout.Position{}
					}
					return material.Button(ui.theme, &d.applyFilter, "FILTER").Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if d.clearFilter.Clicked(gtx) {
						d.filter.SetText("")
						_ = d.controller.SetFilters(nil)
						d.widget.List.Position = layout.Position{}
					}
					button := material.Button(ui.theme, &d.clearFilter, "ALL 10K")
					button.TextSize = 11
					return button.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(5)}.Layout(gtx) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if d.empty.Clicked(gtx) {
						d.filter.SetText("no-such-incident-value")
						_ = d.controller.SetFilter("description", grid.Filter{Operator: grid.Contains, Value: d.filter.Text()})
					}
					button := material.Button(ui.theme, &d.empty, "EMPTY")
					button.TextSize = 11
					return button.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(5)}.Layout(gtx) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if d.fail.Clicked(gtx) {
						d.flaky.failNext.Store(true)
						_ = d.controller.Refresh()
					}
					button := material.Button(ui.theme, &d.fail, "ERROR")
					button.TextSize = 11
					return button.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(5)}.Layout(gtx) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if d.addSort.Clicked(gtx) {
						d.additive = !d.additive
						d.widget.AdditiveSort = d.additive
					}
					label := "1 SORT"
					if d.additive {
						label = "+ SORT"
					}
					button := material.Button(ui.theme, &d.addSort, label)
					button.TextSize = 11
					return button.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			loaded := fmt.Sprintf("Loaded %d / %d • selected %d", len(snapshot.Rows), snapshot.Total, len(snapshot.Selection))
			if snapshot.Total < 0 {
				loaded = fmt.Sprintf("Loading SQL query… • selected %d", len(snapshot.Selection))
			}
			label := material.Caption(ui.theme, loaded)
			label.Color = muted
			return layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, label.Layout)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return d.widget.Layout(gtx, ui.theme) }),
	)
}

func (d *gridDemo) Close() {
	if d.controller != nil {
		d.controller.Close()
	}
	if d.db != nil {
		_ = d.db.Close()
	}
}
