//go:build guitest

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/io/semantic"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/VinceLewis/gio-kit/grid"
	"github.com/VinceLewis/gio-kit/guitest"
)

func deadline(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func testDemo(t *testing.T, dataDir string) (*guitest.Driver, *demoUI) {
	t.Helper()
	var ui *demoUI
	d, err := guitest.NewApp(func(e guitest.Environment) (guitest.Harness, error) {
		var err error
		ui, err = newDemoUI(demoEnvironment{DataDir: dataDir, Context: e.Context, Invalidate: e.Invalidate, Sleep: e.Clock.Sleep})
		if err != nil {
			return guitest.Harness{}, err
		}
		ui.theme.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
		return guitest.Harness{Layout: ui.Layout, Idle: ui.Idle, Close: ui.Close}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	})
	return d, ui
}

func readyDemo(t *testing.T, d *guitest.Driver, ui *demoUI) {
	t.Helper()
	if err := d.WaitFor(deadline(t), func() bool { return ui.gridDemo.controller != nil || ui.gridDemo.setupErr != nil }); err != nil {
		t.Fatal(err)
	}
	if ui.gridDemo.setupErr != nil {
		t.Fatal(ui.gridDemo.setupErr)
	}
	if err := d.WaitFor(deadline(t), func() bool { return d.Clock().Pending() != 0 }); err != nil {
		t.Fatal(err)
	}
	if err := d.Advance(350 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := d.Settle(deadline(t)); err != nil {
		t.Fatal(err)
	}
	if state := ui.gridDemo.controller.Snapshot(); state.State != grid.Ready || state.Total != 10000 {
		t.Fatalf("startup grid = state %v, total %d", state.State, state.Total)
	}
}

func tap(t *testing.T, d *guitest.Driver, label string) {
	t.Helper()
	if err := d.Tap(guitest.Label(label)); err != nil {
		t.Fatalf("tap %q: %v", label, err)
	}
}

// N1/N2/N3/N4/N6/N7: actions go through the same root and widgets as the APK.
func TestHarness_NavigationModalGuardAndResize(t *testing.T) {
	d, ui := testDemo(t, t.TempDir())
	readyDemo(t, d, ui)
	list := ui.listState
	filter := list.filter.Text()
	tap(t, d, "NEW MODAL")
	if len(ui.router.Modals()) != 1 {
		t.Fatal("modal did not open")
	}
	tap(t, d, "CANCEL MODAL")
	if len(ui.router.Modals()) != 0 || ui.listState != list || list.filter.Text() != filter {
		t.Fatal("modal did not preserve the list")
	}
	tap(t, d, "DEEP LINK")
	if route := ui.router.Current().Route; route.Name != "incident.form" || route.Params["id"].String() != "sys_id_123" {
		t.Fatalf("deep link = %v", route)
	}
	// Until the semantics milestone associates labels with editors, use the
	// second laid-out editor (short description, after read-only Number).
	editIndex, count := -1, 0
	for _, node := range d.Nodes() {
		if node.Desc.Class == semantic.Editor {
			count++
			if count == 2 {
				editIndex = node.Index
				break
			}
		}
	}
	if editIndex < 0 {
		t.Fatal("short-description editor was not laid out")
	}
	if err := d.Type(func(node guitest.Node) bool { return node.Index == editIndex }, " modified"); err != nil {
		t.Fatalf("%v; nodes=%+v", err, d.Nodes())
	}
	if !ui.formDemos["sys_id_123"].form.IsDirty() {
		t.Fatalf("edit did not dirty form: %+v", ui.formDemos["sys_id_123"].form.Snapshot())
	}
	if err := d.Back(); err != nil {
		t.Fatal(err)
	}
	if !ui.confirming || !ui.dirty {
		t.Fatalf("back did not invoke guard: confirming=%t dirty=%t route=%s", ui.confirming, ui.dirty, ui.router.Current().Route.Name)
	}
	tap(t, d, "STAY")
	if ui.confirming || !ui.dirty {
		t.Fatal("stay did not retain dirty form")
	}
	if err := d.Back(); err != nil {
		t.Fatal(err)
	}
	tap(t, d, "DISCARD")
	if ui.router.Current().Route.Name != "incident.list" || ui.listState != list || list.filter.Text() != filter {
		t.Fatal("back lost list state")
	}
	for _, viewport := range [][2]int{{820, 420}, {320, 720}, {420, 820}} {
		if err := d.Resize(viewport[0], viewport[1], unit.Metric{PxPerDp: 1, PxPerSp: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Settle(deadline(t)); err != nil {
		t.Fatal(err)
	}
}

// N5: reconstruction consumes the persisted bytes in a fresh root and storage
// connection, rather than calling Restore on the existing router only.
func TestHarness_N5FreshApplicationRestoration(t *testing.T) {
	dir := t.TempDir()
	d, ui := testDemo(t, dir)
	readyDemo(t, d, ui)
	tap(t, d, "DEEP LINK")
	if err := d.Settle(deadline(t)); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	restored, fresh := testDemo(t, dir)
	if route := fresh.router.Current().Route; route.Name != "incident.form" || route.Params["id"].String() != "sys_id_123" {
		t.Fatalf("restored route = %v", route)
	}
	if err := restored.Back(); err != nil {
		t.Fatal(err)
	}
	if fresh.router.Current().Route.Name != "incident.list" {
		t.Fatal("restored back failed")
	}
}

// N8: deterministic delayed external navigation, applied on the layout thread.
func TestHarness_N8ExternalCompletionAndDispose(t *testing.T) {
	d, ui := testDemo(t, t.TempDir())
	readyDemo(t, d, ui)
	tap(t, d, "EXTERNAL EVENT")
	if !ui.externalBusy || ui.router.Current().Route.Name != "incident.list" {
		t.Fatal("external action skipped its pending state")
	}
	if err := d.WaitFor(deadline(t), func() bool { return d.Clock().Pending() == 1 }); err != nil {
		t.Fatal(err)
	}
	if err := d.Advance(700 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := d.Settle(deadline(t)); err != nil {
		t.Fatal(err)
	}
	if ui.router.Current().Route.Params["id"].String() != "INC0042" {
		t.Fatal("external result did not navigate")
	}
	if err := d.Back(); err != nil {
		t.Fatal(err)
	}
	tap(t, d, "EXTERNAL EVENT")
	if err := d.WaitFor(deadline(t), func() bool { return d.Clock().Pending() == 1 }); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if d.Clock().Pending() != 0 || ui.work.pending() {
		t.Fatal("disposed root retained external work")
	}
}

func TestHarnessCloseDuringDatabaseStartup(t *testing.T) {
	d, ui := testDemo(t, t.TempDir())
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ui.gridDemo.done:
	default:
		t.Fatal("startup worker outlived Close")
	}
	if ui.gridDemo.db != nil {
		if err := ui.gridDemo.db.Ping(); err == nil {
			t.Fatal("startup connection was leaked")
		}
	}
}

func TestStateWriterLatestRequestAndError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	w := newStateWriter(path, func() {})
	for i := 0; i < 100; i++ {
		w.submit(uint64(i), []byte("intermediate"))
	}
	w.submit(100, []byte("final"))
	if err := w.close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "final" || w.pending.Load() != 0 {
		t.Fatalf("last write = %q, %v", data, err)
	}
	w = newStateWriter(filepath.Join(dir, "missing", "state.json"), func() {})
	w.submit(1, []byte("state"))
	if err := w.close(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("write error lost: %v", err)
	}
}
