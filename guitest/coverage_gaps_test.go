package guitest_test

import (
	"errors"
	"testing"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"github.com/VinceLewis/gio-kit/diagnostic"
	"github.com/VinceLewis/gio-kit/dialog"
	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/picker"
	"github.com/VinceLewis/gio-kit/presentation"
	"github.com/VinceLewis/gio-kit/router"
	"github.com/VinceLewis/gio-kit/shell"
)

// GK-3: every reusable navigation/composition provider applies the caller's
// bounded request without requiring a rendered frame or fetching more data.
func TestReusableComponentSnapshotsAreDirectAndBounded(t *testing.T) {
	table, err := router.NewTable(
		router.Definition{Name: "list", Pattern: "/records"},
		router.Definition{Name: "form", Pattern: "/record/:id"},
	)
	if err != nil {
		t.Fatal(err)
	}
	r, err := router.New(table, router.Route{Name: "list"})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two"} {
		if err := r.Push(router.Route{Name: "form", Params: router.Params{"id": router.String(id)}}); err != nil {
			t.Fatal(err)
		}
	}
	routerState := r.DebugSnapshot(diagnostic.Request{Offset: 1, Limit: 1}).State
	if routerState["stackCount"] != 3 || len(routerState["stack"].([]any)) != 1 {
		t.Fatalf("router snapshot ignored bounds: %#v", routerState)
	}

	shellWidget, err := shell.NewWidget(shell.Model{Navigation: []shell.Item{
		{ID: "one", Label: "One", Enabled: true},
		{ID: "two", Label: "Two", Enabled: true},
		{ID: "three", Label: "Three", Enabled: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	shellState := shellWidget.DebugSnapshot(diagnostic.Request{Offset: 1, Limit: 1}).State
	if shellState["navigationCount"] != 3 || len(shellState["navigation"].([]any)) != 1 {
		t.Fatalf("shell snapshot ignored bounds: %#v", shellState)
	}

	pickerWidget := picker.NewWidget(picker.Model{Title: "Choose", Query: "needle", Options: []picker.Option{
		{ID: "one", Label: "One"}, {ID: "two", Label: "Two"}, {ID: "three", Label: "Three"},
	}})
	pickerState := pickerWidget.DebugSnapshot(diagnostic.Request{Offset: 1, Limit: 1, RecordID: "three"}).State
	options := pickerState["options"].([]any)
	if pickerState["optionCount"] != 3 || pickerState["query"] != "needle" || len(options) != 1 || options[0].(map[string]any)["id"] != "three" {
		t.Fatalf("picker snapshot ignored bounds or record selector: %#v", pickerState)
	}

	confirm := dialog.Confirm{Title: "Delete record", Message: "This cannot be undone", Destructive: true, ConfirmLabel: "DELETE"}
	dialogState := confirm.DebugSnapshot(diagnostic.Request{Offset: 99, Limit: 1}).State
	if dialogState["title"] != "Delete record" || dialogState["destructive"] != true || len(dialogState["actions"].([]string)) != 2 {
		t.Fatalf("dialog snapshot lost declared state: %#v", dialogState)
	}

	presentationWidget := presentation.NewWidget(presentation.Page{Title: "Dashboard", Sections: []presentation.Section{
		{ID: "one", Heading: "One"}, {ID: "two", Heading: "Two"}, {ID: "three", Heading: "Three"},
	}})
	presentationState := presentationWidget.DebugSnapshot(diagnostic.Request{Offset: 1, Limit: 1}).State
	sections := presentationState["sections"].([]any)
	if presentationState["sectionCount"] != 3 || len(sections) != 1 || sections[0].(map[string]any)["id"] != "two" {
		t.Fatalf("presentation snapshot ignored bounds: %#v", presentationState)
	}
}

// GK-3: assertions use Gio's actual semantic tree and material input handlers,
// including the named accessibility groups around composed controls.
func TestReusableMaterialControlsExposeRoleNameStateAndGrouping(t *testing.T) {
	th := theme()

	t.Run("picker", func(t *testing.T) {
		w := picker.NewWidget(picker.Model{Title: "Choose record", Options: []picker.Option{
			{ID: "ready", Label: "Ready", Secondary: "Current"},
			{ID: "blocked", Label: "Blocked", Secondary: "Archived", Disabled: true, DisabledReason: "Unavailable"},
		}})
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) })
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		if _, err = d.Find(guitest.All(guitest.Role(semantic.Editor), guitest.Name("Search records"), guitest.Enabled(true))); err != nil {
			t.Fatal(err)
		}
		if _, err = d.Find(guitest.All(guitest.Role(semantic.Button), guitest.Name("Blocked"), guitest.Description("Archived. Unavailable"), guitest.Enabled(false))); err != nil {
			t.Fatal(err)
		}
		if err = d.Tap(guitest.All(guitest.Role(semantic.Button), guitest.Name("Blocked"))); !errors.Is(err, guitest.ErrNotInteractable) {
			t.Fatalf("disabled picker option accepted input: %v", err)
		}
	})

	t.Run("shell", func(t *testing.T) {
		w, err := shell.NewWidget(shell.Model{Title: "Application", Navigation: []shell.Item{
			{ID: "home", Label: "Home", Group: "Primary", Selected: true, Enabled: true},
			{ID: "admin", Label: "Admin", Group: "Primary", Enabled: false, DisabledReason: "Requires access"},
		}})
		if err != nil {
			t.Fatal(err)
		}
		w.Mode = shell.ModeWide
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
			return w.Layout(gtx, th, func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
		}, guitest.Size(1000, 700))
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		if _, err = d.Find(guitest.All(guitest.Role(semantic.Button), guitest.Name("Home"), guitest.Selected(true), guitest.Enabled(true))); err != nil {
			t.Fatalf("%v nodes=%+v", err, d.Nodes())
		}
		disabled := guitest.All(guitest.Role(semantic.Button), guitest.Name("Admin"), guitest.Enabled(false))
		if _, err = d.Find(disabled); err != nil {
			t.Fatalf("disabled shell destination lost accessible semantics: %v", err)
		}
		if err = d.Tap(disabled); !errors.Is(err, guitest.ErrNotInteractable) {
			t.Fatalf("disabled shell destination accepted input: %v", err)
		}
	})

	t.Run("dialog", func(t *testing.T) {
		w := dialog.Confirm{Title: "Confirm deletion", Message: "This cannot be undone"}
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) })
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		group := guitest.All(guitest.Name("Confirm deletion"), guitest.Description("This cannot be undone"))
		for _, label := range []string{"CANCEL", "CONFIRM"} {
			button := guitest.Containing(guitest.All(guitest.Role(semantic.Button), guitest.Enabled(true)), guitest.Label(label))
			if _, err = d.Find(guitest.Within(button, group)); err != nil {
				t.Fatalf("%s is not grouped under the dialog: %v nodes=%+v", label, err, d.Nodes())
			}
		}
	})

	t.Run("presentation", func(t *testing.T) {
		w := presentation.NewWidget(presentation.Page{Sections: []presentation.Section{{Lists: []presentation.List{{Rows: []presentation.Row{
			{ID: "one", AccessibleLabel: "First record", Actions: []presentation.Action{{ID: "edit", Label: "EDIT", Enabled: true}}},
			{ID: "two", AccessibleLabel: "Second record", Actions: []presentation.Action{{ID: "edit", Label: "EDIT", Enabled: false, DisabledReason: "Read only"}}},
		}}}}}})
		d, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) })
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		button := func(enabled bool) guitest.Selector {
			return guitest.Containing(guitest.All(guitest.Role(semantic.Button), guitest.Enabled(enabled)), guitest.Label("EDIT"))
		}
		if _, err = d.Find(guitest.Within(button(true), guitest.Name("First record"))); err != nil {
			t.Fatalf("%v nodes=%+v", err, d.Nodes())
		}
		if _, err = d.Find(guitest.Within(button(false), guitest.Name("Second record"))); err != nil {
			t.Fatal(err)
		}
	})
}
