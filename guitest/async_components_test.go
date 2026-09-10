package guitest_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	formkit "github.com/VinceLewis/gio-kit/form"
	"github.com/VinceLewis/gio-kit/grid"
	"github.com/VinceLewis/gio-kit/guitest"
)

func TestFormValidationSubmissionErrorAndRetry(t *testing.T) {
	var f *formkit.Form
	var attempts atomic.Int32
	th := theme()
	d, err := guitest.NewApp(func(e guitest.Environment) (guitest.Harness, error) {
		var err error
		f, err = formkit.New([]formkit.FieldSchema{{ID: "summary", Label: "Summary", Mandatory: true}, {ID: "notes", Label: "Notes"}}, nil, e.Invalidate)
		if err != nil {
			return guitest.Harness{}, err
		}
		f.SetSubmitter(func(ctx context.Context, _ map[string]string) error {
			if attempts.Add(1) == 1 {
				return errors.New("Temporary service failure")
			}
			return ctx.Err()
		})
		w := formkit.NewWidget(f)
		return guitest.Harness{Layout: func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) }, Idle: func() bool { return !f.Pending() }, Close: func() error { f.Close(); f.Wait(); return nil }}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err = d.Tap(guitest.Label("SAVE")); !errors.Is(err, guitest.ErrNotInteractable) {
		t.Fatalf("invalid save: %v", err)
	}
	if attempts.Load() != 0 {
		t.Fatal("invalid form submitted")
	}
	if err = d.Type(guitest.All(guitest.Role(semantic.Editor), guitest.Name("Summary")), "valid input"); err != nil {
		t.Fatal(err)
	}
	if err = d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if err = d.Key("A", key.ModCtrl); err != nil {
		t.Fatal(err)
	}
	if err = d.Key(key.NameDeleteBackward, 0); err != nil {
		t.Fatal(err)
	}
	if err = d.Type(guitest.All(guitest.Role(semantic.Editor), guitest.Name("Notes")), "note"); err != nil {
		t.Fatal(err)
	}
	if f.Snapshot().Fields[0].Error == "" {
		t.Fatal("blur did not expose required validation")
	}
	if err = d.Type(guitest.All(guitest.Role(semantic.Editor), guitest.Name("Summary")), "valid input"); err != nil {
		t.Fatal(err)
	}
	if err = d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if err = d.Tap(guitest.Label("SAVE")); err != nil {
		t.Fatal(err)
	}
	if err = d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Find(guitest.Label("Temporary service failure")); err != nil {
		t.Fatal(err)
	}
	if !f.IsDirty() {
		t.Fatal("failure discarded edits")
	}
	if err = d.Tap(guitest.Label("SAVE")); err != nil {
		t.Fatal(err)
	}
	if err = d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 2 || f.IsDirty() || f.Snapshot().SubmitError != nil {
		t.Fatal("retry did not commit")
	}
}

func TestGridRoutedSortRejectsOutOfOrderCompletion(t *testing.T) {
	var controller *grid.Controller
	var attempts atomic.Int32
	var oldStarted, oldCanceled atomic.Bool
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	th := theme()
	d, err := guitest.NewApp(func(e guitest.Environment) (guitest.Harness, error) {
		var err error
		controller, err = grid.NewController([]grid.Column{{ID: "name", Header: "Name", Sortable: true}}, sourceFunc(func(ctx context.Context, _ int, _ int, _ []grid.SortSpec, _ map[string]grid.Filter) ([]grid.Row, int, error) {
			name := "Initial"
			switch attempts.Add(1) {
			case 2:
				oldStarted.Store(true)
				e.Invalidate()
				<-release
				oldCanceled.Store(ctx.Err() != nil)
				name = "Old"
			case 3:
				name = "New"
			}
			// Deliberately return data even when canceled to test stale protection.
			return []grid.Row{{ID: name, Cells: map[string]string{"name": name}}}, 1, nil
		}), 20, e.Invalidate)
		if err != nil {
			return guitest.Harness{}, err
		}
		w := grid.NewWidget(controller)
		w.ViewMode = grid.ViewTable
		if err = controller.Refresh(); err != nil {
			return guitest.Harness{}, err
		}
		return guitest.Harness{Layout: func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) }, Idle: func() bool { return !controller.Pending() }, Close: func() error { unblock(); controller.Close(); controller.Wait(); return nil }}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	defer unblock()
	if err = d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if err = d.Tap(guitest.Label("Sort by Name")); err != nil {
		t.Fatal(err)
	}
	if err = d.WaitFor(testContext(t), oldStarted.Load); err != nil {
		t.Fatal(err)
	}
	if err = d.Tap(guitest.Label("Sort by Name")); err != nil {
		t.Fatal(err)
	}
	if err = d.WaitFor(testContext(t), func() bool { s := controller.Snapshot(); return len(s.Rows) == 1 && s.Rows[0].ID == "New" }); err != nil {
		t.Fatal(err)
	}
	unblock()
	if err = d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Find(guitest.Label("Open New")); err != nil {
		t.Fatal(err)
	}
	if !oldCanceled.Load() {
		t.Fatal("superseded request was not canceled")
	}
	if _, err = d.Find(guitest.Label("Open Old")); !errors.Is(err, guitest.ErrNotFound) {
		t.Fatal("stale result reached UI")
	}
}

// G5/G6: error/retry follows real widget input with a deterministic fake source.
func TestGridFetchFailureRetryAndEmpty(t *testing.T) {
	var controller *grid.Controller
	var attempts atomic.Int32
	th := theme()
	d, err := guitest.NewApp(func(e guitest.Environment) (guitest.Harness, error) {
		var err error
		controller, err = grid.NewController([]grid.Column{{ID: "name", Header: "Name"}}, sourceFunc(func(context.Context, int, int, []grid.SortSpec, map[string]grid.Filter) ([]grid.Row, int, error) {
			if attempts.Add(1) == 1 {
				return nil, 0, errors.New("Test fetch failure")
			}
			return nil, 0, nil
		}), 50, e.Invalidate)
		if err != nil {
			return guitest.Harness{}, err
		}
		w := grid.NewWidget(controller)
		if err = controller.Refresh(); err != nil {
			return guitest.Harness{}, err
		}
		return guitest.Harness{Layout: func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) }, Idle: func() bool { return !controller.Pending() }, Close: func() error { controller.Close(); controller.Wait(); return nil }}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err = d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if controller.Snapshot().State != grid.Failed {
		t.Fatal("failure not shown")
	}
	if err = d.Tap(guitest.Label("RETRY")); err != nil {
		t.Fatal(err)
	}
	if err = d.Settle(testContext(t)); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Find(guitest.Label("No records yet.")); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 2 {
		t.Fatal("retry did not fetch")
	}
}
