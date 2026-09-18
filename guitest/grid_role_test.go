package guitest_test

import (
	"context"
	"fmt"
	"testing"

	"gioui.org/layout"
	"github.com/VinceLewis/gio-kit/diagnostic"
	"github.com/VinceLewis/gio-kit/grid"
	"github.com/VinceLewis/gio-kit/guitest"
)

func gridFixture(t *testing.T, mode grid.ViewMode) (*guitest.Driver, *grid.Widget) {
	t.Helper()
	th := theme()
	var w *grid.Widget
	d, err := guitest.NewApp(func(env guitest.Environment) (guitest.Harness, error) {
		controller, err := grid.NewController([]grid.Column{{ID: "name", Header: "Name", Sortable: true}}, sourceFunc(func(ctx context.Context, offset, limit int, sort []grid.SortSpec, _ map[string]grid.Filter) ([]grid.Row, int, error) {
			rows := make([]grid.Row, 0, limit)
			for i := offset; i < 5 && len(rows) < limit; i++ {
				rows = append(rows, grid.Row{ID: fmt.Sprint(i), Cells: map[string]string{"name": fmt.Sprintf("Record %02d", i)}})
			}
			return rows, 5, ctx.Err()
		}), 5, env.Invalidate)
		if err != nil {
			return guitest.Harness{}, err
		}
		w = grid.NewWidget(controller)
		w.ViewMode = mode
		if err = controller.Refresh(); err != nil {
			return guitest.Harness{}, err
		}
		return guitest.Harness{Layout: func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) },
			Idle:  func() bool { return controller.Snapshot().State != grid.Loading },
			Close: func() error { controller.Close(); controller.Wait(); return nil },
			Providers: map[string]diagnostic.Provider{"grid": w},
		}, nil
	}, guitest.Size(800, 500))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	return d, w
}

// TestGridRoleReflectsResolvedViewMode covers the gap flagged against
// gio-json-test-upgrade-plan.md's item 2/F-003 draft: role assignment for
// grid rows must come from the component's actual resolvedViewMode
// (grid.Widget.DebugSnapshot), not a label-prefix guess that cannot tell a
// table row from a card.
func TestGridRoleReflectsResolvedViewMode(t *testing.T) {
	t.Run("table mode has column headers and no cards", func(t *testing.T) {
		d, _ := gridFixture(t, grid.ViewTable)
		dump, err := d.Capture(guitest.DumpOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(nodesWithTestRole(dump, "columnHeader")) == 0 {
			t.Fatal("table mode: expected at least one columnHeader node")
		}
		if len(nodesWithTestRole(dump, "card")) != 0 {
			t.Fatal("table mode: expected no card nodes")
		}
	})
	t.Run("card mode has cards and no column headers", func(t *testing.T) {
		d, _ := gridFixture(t, grid.ViewCards)
		dump, err := d.Capture(guitest.DumpOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(nodesWithTestRole(dump, "card")) == 0 {
			t.Fatal("card mode: expected at least one card node")
		}
		if len(nodesWithTestRole(dump, "columnHeader")) != 0 {
			t.Fatal("card mode: expected no columnHeader nodes (sort trigger is not a column header)")
		}
	})
}

func nodesWithTestRole(dump guitest.Dump, role string) []guitest.FrameNode {
	var out []guitest.FrameNode
	for _, n := range dump.Nodes {
		if n.Role == role {
			out = append(out, n)
		}
	}
	return out
}
