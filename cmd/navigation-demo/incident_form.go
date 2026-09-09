package main

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	formkit "github.com/VinceLewis/gio-kit/form"
	"github.com/VinceLewis/gio-kit/grid"
	gridsqlite "github.com/VinceLewis/gio-kit/grid/sqlite"
	"github.com/VinceLewis/gio-kit/router"
)

type incidentFormDemo struct {
	id     string
	form   *formkit.Form
	widget *formkit.Widget
	saved  chan struct{}
}

type lookupDemo struct {
	target *incidentFormDemo
	field  string
	ctrl   *grid.Controller
	widget *grid.Widget
	filter widget.Editor
	apply  widget.Clickable
}

func incidentSchema() []formkit.FieldSchema {
	return []formkit.FieldSchema{
		{ID: "number", Label: "Number", Type: formkit.FieldText, ReadOnly: true},
		{ID: "short_description", Label: "Short description", Type: formkit.FieldText, Mandatory: true, Validators: []formkit.Validator{formkit.MaxLength(160)}},
		{ID: "details", Label: "Details", Type: formkit.FieldTextArea, Validators: []formkit.Validator{formkit.MaxLength(1000)}},
		{ID: "priority", Label: "Priority (1–4)", Type: formkit.FieldNumber, Mandatory: true, Validators: []formkit.Validator{formkit.NumberRange(1, 4)}},
		{ID: "active", Label: "Active", Type: formkit.FieldBoolean, DefaultValue: "true"},
		{ID: "opened_at", Label: "Opened at", Type: formkit.FieldDateTime, Mandatory: true},
		{ID: "response_time", Label: "Response time", Type: formkit.FieldTime},
		{ID: "evidence", Label: "Evidence", Type: formkit.FieldAttachment},
		{ID: "state", Label: "State", Type: formkit.FieldChoice, Choices: []formkit.Choice{
			{Value: "New", Label: "New"}, {Value: "In Progress", Label: "In Progress"},
			{Value: "On Hold", Label: "On Hold"}, {Value: "Resolved", Label: "Resolved"},
		}},
		{ID: "category", Label: "Category", Type: formkit.FieldChoice, Choices: []formkit.Choice{
			{Value: "software", Label: "Software"}, {Value: "hardware", Label: "Hardware"},
		}},
		{ID: "configuration_item", Label: "Configuration item", Type: formkit.FieldReference, RefTable: "incident"},
	}
}

func incidentRules() []formkit.UIRule {
	return []formkit.UIRule{
		{
			Conditions: []formkit.Condition{{FieldID: "category", Equals: "hardware"}},
			Effects: []formkit.Effect{{
				FieldID: "configuration_item", SetVisible: true, Visible: true,
				SetMandatory: true, Mandatory: true,
			}},
		},
		{
			Conditions: []formkit.Condition{{FieldID: "category", Equals: "hardware", Not: true}},
			Effects:    []formkit.Effect{{FieldID: "configuration_item", SetVisible: true, Visible: false}},
		},
	}
}

func (u *demoUI) dataFormScreen(gtx layout.Context, route router.Route) layout.Dimensions {
	u.gridDemo.poll(u)
	if u.gridDemo.setupErr != nil {
		return u.infoCard(gtx, "SQLite setup failed", u.gridDemo.setupErr.Error())
	}
	if u.gridDemo.db == nil {
		return layout.Center.Layout(gtx, material.Body1(u.theme, "Opening incident database…").Layout)
	}
	id := route.Params["id"].String()
	demo := u.formDemos[id]
	if demo == nil {
		var err error
		demo, err = u.newIncidentForm(route)
		if err != nil {
			return u.infoCard(gtx, "Form setup failed", err.Error())
		}
		u.formDemos[id] = demo
		_ = u.router.SetCurrentState(demo)
	}
	select {
	case <-demo.saved:
		_ = u.gridDemo.controller.Refresh()
		u.status, u.statusOK = "Saved to SQLite; the record remains open.", true
	default:
	}
	snapshot := demo.form.Snapshot()
	u.dirty = snapshot.Dirty
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			subtitle := "Schema-rendered form • values save to the incident table"
			if snapshot.SubmitError != nil {
				subtitle = "Save rejected: " + snapshot.SubmitError.Error()
			}
			return u.heading(gtx, route.Params["number"].String(), subtitle)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return u.panel(gtx, func(gtx layout.Context) layout.Dimensions {
				return demo.widget.Layout(gtx, u.theme)
			})
		}),
	)
}

func (u *demoUI) newIncidentForm(route router.Route) (*incidentFormDemo, error) {
	id := route.Params["id"].String()
	controller, err := formkit.New(incidentSchema(), incidentRules(), u.env.Invalidate)
	if err != nil {
		return nil, err
	}
	values := map[string]string{
		"number": route.Params["number"].String(), "short_description": route.Params["description"].String(),
		"details": route.Params["details"].String(), "priority": route.Params["priority"].String(),
		"active": route.Params["active"].String(), "opened_at": route.Params["opened"].String(),
		"state": route.Params["state"].String(), "category": route.Params["category"].String(),
		"configuration_item": route.Params["configuration_item"].String(),
	}
	if values["number"] == "" {
		values["number"] = id
	}
	if values["short_description"] == "" {
		values["short_description"] = "Restored incident"
	}
	if values["priority"] == "" {
		values["priority"] = "2"
	}
	if values["active"] == "" {
		values["active"] = "true"
	}
	if values["opened_at"] == "" {
		values["opened_at"] = "2026-01-01 00:00:00"
	}
	if values["state"] == "" {
		values["state"] = "New"
	}
	if values["category"] == "" {
		values["category"] = "software"
	}
	if err := controller.Load(values); err != nil {
		return nil, err
	}
	demo := &incidentFormDemo{id: id, form: controller, saved: make(chan struct{}, 1)}
	demo.widget = formkit.NewWidget(controller)
	demo.widget.OnSaveAndClose = func() {
		removed, popErr := u.router.Pop()
		if popErr != nil {
			u.fail(popErr)
			return
		}
		u.cleanupForm(removed.Route)
		u.status, u.statusOK = "Saved to SQLite and closed the record.", true
	}
	demo.widget.OnReference = func(field formkit.FieldSchema) {
		u.openLookup(demo, field.ID)
	}
	demo.widget.OnAttachment = func(field formkit.FieldSchema) {
		_ = demo.form.SetValue(field.ID, "example-document.pdf")
		u.status, u.statusOK = "Attachment picker callback invoked.", true
		u.env.Invalidate()
	}
	demo.widget.OnInvalid = func() {
		u.status, u.statusOK = "Fix errors before saving.", false
		u.env.Invalidate()
	}
	controller.SetSubmitter(func(ctx context.Context, values map[string]string) error {
		if !u.work.begin() {
			return context.Canceled
		}
		defer u.work.end()
		ctx, cancel := context.WithCancel(ctx)
		stop := context.AfterFunc(u.env.Context, cancel)
		defer stop()
		defer cancel()
		if err := u.env.Sleep(ctx, 600*time.Millisecond); err != nil {
			return err
		}
		if err := u.env.Context.Err(); err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(values["short_description"]), "server-error") {
			return formkit.FieldErrors{"short_description": "SQLite service rejected this test value"}
		}
		active := 0
		if values["active"] == "true" {
			active = 1
		}
		query := "UPDATE incident SET short_description=?, details=?, priority=?, active=?, opened_at=?, state=?, category=?, configuration_item=? WHERE sys_id=?"
		_, err := u.gridDemo.db.ExecContext(ctx, query,
			values["short_description"], values["details"], values["priority"], active,
			values["opened_at"], values["state"], values["category"],
			values["configuration_item"], id)
		return err
	})
	controller.OnSuccess(func(map[string]string) {
		select {
		case demo.saved <- struct{}{}:
		default:
		}
		u.env.Invalidate()
	})
	controller.OnCancel(u.navigateBack)
	return demo, nil
}

func (u *demoUI) openLookup(target *incidentFormDemo, field string) {
	if u.lookup != nil {
		u.lookup.Close()
	}
	source, err := gridsqlite.New(u.gridDemo.db, gridsqlite.Config{
		Table: "incident", IDColumn: "sys_id",
		Columns: map[string]string{"number": "number", "description": "short_description", "state": "state"},
	})
	if err != nil {
		u.fail(err)
		return
	}
	controller, err := grid.NewController([]grid.Column{
		{ID: "number", Header: "Number", Width: 105, Sortable: true, Visible: true, Filter: grid.FilterText},
		{ID: "description", Header: "Description", Flex: 2, Visible: true, Filter: grid.FilterText},
	}, source, 30, u.env.Invalidate)
	if err != nil {
		u.fail(err)
		return
	}
	lookup := &lookupDemo{target: target, field: field, ctrl: controller}
	lookup.filter.SingleLine = true
	lookup.widget = grid.NewWidget(controller)
	lookup.widget.OpenColumn = "number"
	lookup.widget.EnableSelection = false
	lookup.widget.OnRow = func(row grid.Row) {
		_ = target.form.SetReference(field, row.ID, row.Cells["number"])
		_, _ = u.router.PopModal()
		u.status, u.statusOK = "Reference selected from the SQLite incident grid.", true
		lookup.Close()
		u.lookup = nil
	}
	u.lookup = lookup
	if err := controller.Refresh(); err != nil {
		u.fail(err)
		return
	}
	err = u.router.PushModal(router.Route{Name: "lookup.grid", Params: router.Params{
		"table": router.String("incident"), "field": router.String(field),
	}})
	if err != nil {
		u.fail(err)
	}
}

func (u *demoUI) lookupModal(gtx layout.Context) layout.Dimensions {
	if u.lookup == nil {
		return u.overlay(gtx, material.Body1(u.theme, "Lookup is unavailable").Layout)
	}
	return u.overlay(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(unit.Dp(620)))
		gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.H5(u.theme, "Select incident reference").Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, material.Editor(u.theme, &u.lookup.filter, "Description contains…").Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if u.lookup.apply.Clicked(gtx) {
							_ = u.lookup.ctrl.SetFilter("description", grid.Filter{Operator: grid.Contains, Value: u.lookup.filter.Text()})
						}
						return material.Button(u.theme, &u.lookup.apply, "FILTER").Layout(gtx)
					}),
				)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return u.lookup.widget.Layout(gtx, u.theme)
			}),
		)
	})
}

func migrateIncidentTable(db *sql.DB) error {
	columns := []struct{ name, definition string }{
		{"category", "TEXT NOT NULL DEFAULT 'software'"},
		{"configuration_item", "TEXT NOT NULL DEFAULT ''"},
		{"details", "TEXT NOT NULL DEFAULT ''"},
		{"active", "INTEGER NOT NULL DEFAULT 1"},
	}
	existing := make(map[string]bool)
	rows, err := db.Query("PRAGMA table_info(incident)")
	if err != nil {
		return err
	}
	for rows.Next() {
		var index, notNull, primaryKey int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&index, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return err
		}
		existing[name] = true
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, column := range columns {
		if existing[column.name] {
			continue
		}
		if _, err := db.Exec("ALTER TABLE incident ADD COLUMN " + column.name + " " + column.definition); err != nil {
			return err
		}
	}
	return nil
}

func (l *lookupDemo) Close() {
	if l != nil && l.ctrl != nil {
		l.ctrl.Close()
	}
}

func (u *demoUI) cleanupForm(route router.Route) {
	if route.Name != "incident.form" {
		return
	}
	id := route.Params["id"].String()
	if demo := u.formDemos[id]; demo != nil {
		demo.form.Close()
		delete(u.formDemos, id)
	}
	u.dirty = false
}

func (u *demoUI) Close() error {
	if u.closed {
		return nil
	}
	u.closed = true
	u.cancel()
	if u.lookup != nil {
		u.lookup.Close()
	}
	for _, demo := range u.formDemos {
		demo.form.Close()
	}
	u.work.stop()
	if u.gridDemo != nil {
		u.gridDemo.Close()
	}
	return u.persistence.close()
}
