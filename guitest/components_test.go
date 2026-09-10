package guitest_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"github.com/VinceLewis/gio-kit/diagnostic"
	"github.com/VinceLewis/gio-kit/dialog"
	formkit "github.com/VinceLewis/gio-kit/form"
	"github.com/VinceLewis/gio-kit/grid"
	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/picker"
	"github.com/VinceLewis/gio-kit/presentation"
	"github.com/VinceLewis/gio-kit/shell"
)

type sourceFunc func(context.Context, int, int, []grid.SortSpec, map[string]grid.Filter) ([]grid.Row, int, error)

func (f sourceFunc) Fetch(ctx context.Context, offset, limit int, sort []grid.SortSpec, filters map[string]grid.Filter) ([]grid.Row, int, error) {
	return f(ctx, offset, limit, sort, filters)
}

// G1/G2/G3/G4/G7: actual grid input, 10,000 logical rows, bounded snapshots.
func TestGridSelectionSortScrollAndBoundedSnapshot(t *testing.T) {
	var controller *grid.Controller
	var w *grid.Widget
	opened := ""
	th := theme()
	d, err := guitest.NewApp(func(env guitest.Environment) (guitest.Harness, error) {
		var err error
		controller, err = grid.NewController([]grid.Column{{ID: "name", Header: "Name", Sortable: true}}, sourceFunc(func(ctx context.Context, offset, limit int, sort []grid.SortSpec, _ map[string]grid.Filter) ([]grid.Row, int, error) {
			rows := make([]grid.Row, 0, limit)
			for i := offset; i < 10000 && len(rows) < limit; i++ {
				n := i
				if len(sort) > 0 && sort[0].Descending {
					n = 9999 - i
				}
				rows = append(rows, grid.Row{ID: fmt.Sprint(n), Cells: map[string]string{"name": fmt.Sprintf("Record %05d", n)}})
			}
			return rows, 10000, ctx.Err()
		}), 10000, env.Invalidate)
		if err != nil {
			return guitest.Harness{}, err
		}
		w = grid.NewWidget(controller)
		w.EnableSelection = true
		w.ViewMode = grid.ViewTable
		w.OnRow = func(row grid.Row) { opened = row.ID }
		if err = controller.Refresh(); err != nil {
			return guitest.Harness{}, err
		}
		return guitest.Harness{Layout: func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) }, Idle: func() bool { return controller.Snapshot().State != grid.Loading }, Close: func() error { controller.Close(); controller.Wait(); return nil }, Providers: map[string]diagnostic.Provider{"grid": w}}, nil
	}, guitest.Size(800, 500))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err = d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if err = d.Tap(guitest.All(guitest.Role(semantic.CheckBox), guitest.Name("Select Record 00000"))); err != nil {
		t.Fatal(err)
	}
	if !controller.Snapshot().Selection["0"] || opened != "" {
		t.Fatal("selection opened a row or failed")
	}
	if err = d.Tap(guitest.Label("Open Record 00000")); err != nil {
		t.Fatal(err)
	}
	if opened != "0" {
		t.Fatal("row action did not open record")
	}
	if err = d.Scroll(guitest.Label("Open Record 00000"), f32.Pt(0, 900)); err != nil {
		t.Fatal(err)
	}
	if err = d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if w.List.Position.First == 0 {
		t.Fatal("grid did not scroll")
	}
	dump, err := d.Capture(guitest.DumpOptions{Component: "grid", Request: diagnostic.Request{Offset: 9990, Limit: 4}})
	if err != nil {
		t.Fatal(err)
	}
	rows := dump.Components["grid"].State["rows"].([]any)
	if len(rows) != 4 || rows[0].(map[string]any)["virtualized"] != true || len(dump.Nodes) > 160 {
		t.Fatal("unbounded or inaccurate grid diagnostic")
	}
	if err = d.Tap(guitest.Label("Sort by Name")); err != nil {
		t.Fatal(err)
	}
	if err = d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if len(controller.Snapshot().Sort) != 1 {
		t.Fatal("sort did not reach controller")
	}
}

func TestFormNamedControlsReadOnlyChoicesAndLongPress(t *testing.T) {
	f, err := formkit.New([]formkit.FieldSchema{
		{ID: "number", Label: "Number", ReadOnly: true, DefaultValue: "001"},
		{ID: "text", Label: "Summary"},
		{ID: "choice", Label: "Priority", Type: formkit.FieldChoice, Choices: []formkit.Choice{{Value: "a", Label: "Normal"}, {Value: "b", Label: "Urgent"}}, DefaultValue: "a"},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := formkit.NewWidget(f)
	th := theme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) })
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err = d.Type(guitest.All(guitest.Role(semantic.Editor), guitest.Name("Number")), "bad"); !errors.Is(err, guitest.ErrNotInteractable) {
		t.Fatal(err)
	}
	text := guitest.All(guitest.Role(semantic.Editor), guitest.Name("Summary"))
	if err = d.Type(text, "routed input"); err != nil {
		t.Fatal(err)
	}
	if err = d.LongPress(text, 600*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Find(guitest.Label("COPY")); err != nil {
		t.Fatal("long press did not expose text toolbar", err)
	}
	if err = d.Tap(guitest.All(guitest.Role(semantic.Button), guitest.Name("Priority"))); err != nil {
		t.Fatal(err)
	}
	if err = d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if err = d.Tap(guitest.All(guitest.Role(semantic.RadioButton), guitest.Name("Urgent"))); err != nil {
		t.Fatal(err)
	}
	if f.Values()["choice"] != "b" || f.Values()["text"] != "routed input" {
		t.Fatal("form input bypassed or failed")
	}
}

func TestPickerShellDialogAndPresentationSemantics(t *testing.T) {
	th := theme()
	t.Run("picker", func(t *testing.T) {
		w := picker.NewWidget(picker.Model{Title: "Choose", Options: []picker.Option{{ID: "1", Label: "Available"}, {ID: "2", Label: "Blocked", Disabled: true, DisabledReason: "Unavailable"}}})
		selected, query := "", ""
		w.OnSelect = func(o picker.Option) { selected = o.ID }
		w.OnSearch = func(q string) { query = q }
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) })
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		if err = d.Type(guitest.All(guitest.Role(semantic.Editor), guitest.Name("Search records")), "abc"); err != nil {
			t.Fatal(err)
		}
		if err = d.Tap(guitest.Label("SEARCH")); err != nil {
			t.Fatal(err)
		}
		if query != "abc" {
			t.Fatal("search input missing")
		}
		if err = d.Tap(guitest.All(guitest.Role(semantic.Button), guitest.Name("Blocked"))); !errors.Is(err, guitest.ErrNotInteractable) {
			t.Fatal(err)
		}
		if err = d.Tap(guitest.All(guitest.Role(semantic.Button), guitest.Name("Available"))); err != nil {
			t.Fatal(err)
		}
		if selected != "1" {
			t.Fatal("selection missing")
		}
	})
	t.Run("shell", func(t *testing.T) {
		w, err := shell.NewWidget(shell.Model{Title: "Application", Navigation: []shell.Item{{ID: "home", Label: "Home", Enabled: true, Selected: true}}})
		if err != nil {
			t.Fatal(err)
		}
		w.Mode = shell.ModeWide
		selected := ""
		w.OnNavigate = func(id string) { selected = id }
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
			return w.Layout(gtx, th, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
		}, guitest.Size(1000, 700))
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		if err = d.Tap(guitest.All(guitest.Role(semantic.Button), guitest.Name("Home"), guitest.Selected(true))); err != nil {
			t.Fatal(err)
		}
		if selected != "home" {
			t.Fatal("navigation missing")
		}
	})
	t.Run("dialog", func(t *testing.T) {
		confirmed := false
		w := dialog.Confirm{Title: "Confirm change", OnConfirm: func() { confirmed = true }}
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) })
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		if err = d.Tap(guitest.Within(guitest.Label("CONFIRM"), guitest.Label("Confirm change"))); err != nil {
			t.Fatal(err)
		}
		if !confirmed {
			t.Fatal("confirmation missing")
		}
	})
	t.Run("presentation", func(t *testing.T) {
		w := presentation.NewWidget(presentation.Page{Sections: []presentation.Section{{Lists: []presentation.List{{ID: "list", Rows: []presentation.Row{{ID: "a", AccessibleLabel: "First record", Actions: []presentation.Action{{ID: "edit", Label: "EDIT", Enabled: true}}}, {ID: "b", AccessibleLabel: "Second record", Actions: []presentation.Action{{ID: "edit", Label: "EDIT", Enabled: true}}}}}}}}})
		var event presentation.Event
		w.OnEvent = func(e presentation.Event) { event = e }
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) })
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		if err = d.Tap(guitest.Within(guitest.Label("EDIT"), guitest.Label("Second record"))); err != nil {
			t.Fatal(err)
		}
		if event.RowID != "b" {
			t.Fatal("scoped action selected wrong row")
		}
	})
}
