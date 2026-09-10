package guitest_test

import (
	"testing"

	"gioui.org/layout"
	kitcollection "github.com/VinceLewis/gio-kit/collection"
	"github.com/VinceLewis/gio-kit/guitest"
)

func TestOrderedCollectionControlsEmitDeclaredIntents(t *testing.T) {
	var added, edited, removed bool
	moved := -1
	model := kitcollection.Model{Title: "Children", CanAdd: true, Items: []kitcollection.Item{
		{ID: "one", Label: "First", CanEdit: true, CanRemove: true, CanMoveDown: true},
		{ID: "two", Label: "Second", CanMoveUp: true},
	}}
	widget := kitcollection.NewWidget(model)
	widget.OnAdd = func() { added = true }
	widget.OnEdit = func(kitcollection.Item) { edited = true }
	widget.OnRemove = func(kitcollection.Item) { removed = true }
	widget.OnMove = func(_ kitcollection.Item, target int) { moved = target }
	theme := theme()
	driver, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return widget.Layout(gtx, theme) })
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Tap(guitest.Description("Add to Children")); err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Fatal("add callback did not run")
	}
	for selector, state := range map[string]*bool{"Edit First": &edited, "Remove First": &removed} {
		if err := driver.Tap(guitest.Name(selector)); err != nil {
			t.Fatalf("tap %s: %v", selector, err)
		}
		if !*state {
			t.Fatalf("%s callback did not run", selector)
		}
	}
	if err := driver.Tap(guitest.Name("Move down First")); err != nil {
		t.Fatal(err)
	}
	if moved != 1 {
		t.Fatalf("move target = %d", moved)
	}
	if err := driver.Tap(guitest.Name("Move up First")); err == nil {
		t.Fatal("unavailable move-up action was rendered")
	}
}
