package main

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"image/color"
	"path/filepath"
	"sync/atomic"
	"time"

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
	flaky      *flakySource
}

type flakySource struct {
	source   grid.DataSource
	failNext atomic.Bool
	delay    time.Duration
	sleep    func(context.Context, time.Duration) error
}

func (s *flakySource) Fetch(ctx context.Context, offset, limit int, sort []grid.SortSpec, filters map[string]grid.Filter) ([]grid.Row, int, error) {
	if s.delay > 0 {
		if err := s.sleep(ctx, s.delay); err != nil {
			return nil, 0, err
		}
	}
	if s.failNext.CompareAndSwap(true, false) {
		return nil, 0, errors.New("simulated SQLite query failure")
	}
	return s.source.Fetch(ctx, offset, limit, sort, filters)
}

type gridDemo struct {
	cancel      context.CancelFunc
	done        chan struct{}
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
	showCards   widget.Clickable
	showTable   widget.Clickable
	additive    bool
}

func newGridDemo(env demoEnvironment) *gridDemo {
	ctx, cancel := context.WithCancel(env.Context)
	demo := &gridDemo{ready: make(chan gridSetup, 1), cancel: cancel, done: make(chan struct{})}
	demo.filter.SingleLine = true
	go func() {
		defer close(demo.done)
		db, err := sql.Open("sqlite3", filepath.Join(env.DataDir, "gio-kit-demo.db")+"?_busy_timeout=5000&_foreign_keys=on")
		if err == nil {
			db.SetMaxOpenConns(1)
			_, err = db.ExecContext(ctx, incidentSQL)
			if ctx.Err() != nil {
				err = ctx.Err()
			} else if err != nil {
				if migrationErr := migrateIncidentTable(db); migrationErr != nil {
					err = errors.Join(err, migrationErr)
				} else {
					_, err = db.ExecContext(ctx, incidentSQL)
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
			env.Invalidate()
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
			env.Invalidate()
			return
		}
		flaky := &flakySource{source: source, delay: 350 * time.Millisecond, sleep: env.Sleep}
		controller, err := grid.NewController([]grid.Column{
			{ID: "number", Header: "Number", Width: 112, Sortable: true, Visible: true, Filter: grid.FilterText},
			{ID: "description", Header: "Description", Flex: 2, Sortable: true, Visible: true, Filter: grid.FilterText},
			{ID: "priority", Header: "Priority", Width: 88, Align: grid.AlignStart, Sortable: true, Visible: true, Filter: grid.FilterChoice},
			{ID: "state", Header: "State", Flex: 1, Sortable: true, Visible: true, Filter: grid.FilterChoice},
		}, flaky, 50, env.Invalidate)
		if err == nil {
			err = controller.Refresh()
		}
		demo.ready <- gridSetup{db: db, controller: controller, err: err, flaky: flaky}
		env.Invalidate()
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
		d.flaky = result.flaky
		if d.controller != nil {
			d.widget = grid.NewWidget(d.controller)
			d.widget.OpenColumn = "number"
			d.widget.CardTitleColumn = "number"
			d.widget.CardSummaryColumn = "description"
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
		return layout.Center.Layout(gtx, material.Body1(ui.theme, "Creating 10,000 SQLite incidents…").Layout)
	}
	snapshot := d.controller.Snapshot()
	view := d.widget.ResolvedViewMode(gtx)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.layoutTitleAndView(gtx, ui, view)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.layoutSearch(gtx, ui) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.layoutPresets(gtx, ui, snapshot) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			loaded := fmt.Sprintf("Showing %d of %d records", len(snapshot.Rows), snapshot.Total)
			if snapshot.Total < 0 {
				loaded = "Running SQLite query…"
			}
			if len(snapshot.Selection) > 0 {
				loaded += fmt.Sprintf(" • %d selected", len(snapshot.Selection))
			}
			label := material.Caption(ui.theme, loaded)
			label.Color = ink
			return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(5)}.Layout(gtx, label.Layout)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return d.widget.Layout(gtx, ui.theme) }),
	)
}

func (d *gridDemo) layoutTitleAndView(gtx layout.Context, ui *demoUI, view grid.ViewMode) layout.Dimensions {
	if d.showCards.Clicked(gtx) {
		d.widget.ViewMode = grid.ViewCards
		view = grid.ViewCards
	}
	if d.showTable.Clicked(gtx) {
		d.widget.ViewMode = grid.ViewTable
		view = grid.ViewTable
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.H6(ui.theme, "SQLite incidents")
					label.MaxLines = 1
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Caption(ui.theme, "10,000 records • SQL-backed • 50 per page")
					label.Color = muted
					label.MaxLines = 1
					return label.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(d.viewButton(ui, &d.showCards, "CARDS", view == grid.ViewCards)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(4)}.Layout(gtx) }),
				layout.Rigid(d.viewButton(ui, &d.showTable, "TABLE", view == grid.ViewTable)),
			)
		}),
	)
}

func (d *gridDemo) layoutSearch(gtx layout.Context, ui *demoUI) layout.Dimensions {
	if d.applyFilter.Clicked(gtx) {
		_ = d.controller.SetFilter("description", grid.Filter{Operator: grid.Contains, Value: d.filter.Text()})
		d.widget.List.Position = layout.Position{}
	}
	return layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return ui.surface(gtx, 8, card, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(10), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						editor := material.Editor(ui.theme, &d.filter, "Search descriptions")
						editor.TextSize = unit.Sp(16)
						return editor.Layout(gtx)
					}),
					layout.Rigid(d.actionButton(ui, &d.applyFilter, "APPLY", true, false)),
				)
			})
		})
	})
}

func (d *gridDemo) layoutPresets(gtx layout.Context, ui *demoUI, snapshot grid.Snapshot) layout.Dimensions {
	if d.clearFilter.Clicked(gtx) {
		d.filter.SetText("")
		_ = d.controller.SetFilters(nil)
		d.widget.List.Position = layout.Position{}
	}
	if d.empty.Clicked(gtx) {
		d.filter.SetText("no-such-incident-value")
		_ = d.controller.SetFilter("description", grid.Filter{Operator: grid.Contains, Value: d.filter.Text()})
		d.widget.List.Position = layout.Position{}
	}
	if d.fail.Clicked(gtx) {
		d.flaky.failNext.Store(true)
		_ = d.controller.Refresh()
	}
	if d.addSort.Clicked(gtx) {
		d.additive = !d.additive
		d.widget.AdditiveSort = d.additive
	}
	_, filtered := snapshot.Filters["description"]
	emptyActive := d.filter.Text() == "no-such-incident-value"
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(1, d.actionButton(ui, &d.clearFilter, "ALL", !filtered, false)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(4)}.Layout(gtx) }),
		layout.Flexed(1, d.actionButton(ui, &d.empty, "NO RESULTS", emptyActive, false)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(4)}.Layout(gtx) }),
		layout.Flexed(1, d.actionButton(ui, &d.fail, "TEST ERROR", false, true)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(4)}.Layout(gtx) }),
		layout.Flexed(1, d.actionButton(ui, &d.addSort, "MULTI-SORT", d.additive, false)),
	)
}

func (d *gridDemo) actionButton(ui *demoUI, click *widget.Clickable, label string, selected, destructive bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		button := material.Button(ui.theme, click, label)
		button.TextSize = unit.Sp(10)
		button.Inset = layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(3), Right: unit.Dp(3)}
		button.Background = color.NRGBA{R: 226, G: 234, B: 249, A: 255}
		button.Color = ink
		if selected {
			button.Background = primary
			button.Color = card
		} else if destructive {
			button.Color = danger
		}
		return button.Layout(gtx)
	}
}

func (d *gridDemo) viewButton(ui *demoUI, click *widget.Clickable, label string, selected bool) layout.Widget {
	return d.actionButton(ui, click, label, selected, false)
}

func (d *gridDemo) Close() {
	d.cancel()
	<-d.done
	// Startup may have finished without the UI ever consuming its result.
	select {
	case result := <-d.ready:
		d.db, d.controller = result.db, result.controller
	default:
	}
	if d.controller != nil {
		d.controller.Close()
		d.controller.Wait()
	}
	if d.db != nil {
		_ = d.db.Close()
	}
}
