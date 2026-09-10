//go:build guitest

package main

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/VinceLewis/gio-kit/guitest"
)

// GK-3/N5: recreation repeatedly consumes the bytes written by the production
// root. Each Close joins the database, form, grid, persistence and navigation
// workers before the next root opens the same directory.
func TestHarness_RepeatedFreshRootRestorationAndJoinedClose(t *testing.T) {
	dir := t.TempDir()
	for generation := 0; generation < 4; generation++ {
		d, ui := testDemo(t, dir)
		readyDemo(t, d, ui)
		if generation == 0 {
			tap(t, d, "DEEP LINK")
			if err := d.Settle(deadline(t)); err != nil {
				t.Fatal(err)
			}
		}
		route := ui.router.Current().Route
		if route.Name != "incident.form" || route.Params["id"].String() != "sys_id_123" {
			t.Fatalf("generation %d restored route = %v", generation, route)
		}
		if _, err := os.ReadFile(ui.statePath); err != nil {
			t.Fatalf("generation %d did not persist route: %v", generation, err)
		}
		if err := d.Close(); err != nil {
			t.Fatalf("generation %d close: %v", generation, err)
		}
		assertDemoJoined(t, generation, d, ui)
	}
}

// GK-3/N8: a completion queued behind a virtual delay cannot outlive Close,
// mutate persisted navigation, or leak into a newly constructed root.
func TestHarness_RepeatedCloseCancelsQueuedNavigationCompletion(t *testing.T) {
	dir := t.TempDir()
	seed, seededUI := testDemo(t, dir)
	readyDemo(t, seed, seededUI)
	tap(t, seed, "DEEP LINK")
	if err := seed.Back(); err != nil {
		t.Fatal(err)
	}
	if err := seed.Settle(deadline(t)); err != nil {
		t.Fatal(err)
	}
	if seededUI.router.Current().Route.Name != "incident.list" {
		t.Fatal("seed root did not return to list")
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	baseline, err := os.ReadFile(seededUI.statePath)
	if err != nil {
		t.Fatal(err)
	}

	for generation := 0; generation < 4; generation++ {
		d, ui := testDemo(t, dir)
		readyDemo(t, d, ui)
		if ui.router.Current().Route.Name != "incident.list" {
			t.Fatalf("generation %d began on %v", generation, ui.router.Current().Route)
		}
		tap(t, d, "EXTERNAL EVENT")
		if err := d.WaitFor(deadline(t), func() bool { return d.Clock().Pending() == 1 && ui.externalBusy }); err != nil {
			t.Fatal(err)
		}
		if err := d.Close(); err != nil {
			t.Fatalf("generation %d close: %v", generation, err)
		}
		assertDemoJoined(t, generation, d, ui)
		persisted, err := os.ReadFile(ui.statePath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(persisted, baseline) {
			t.Fatalf("generation %d canceled completion changed persisted navigation", generation)
		}
	}
}

// GK-3/N8: rapid foreground navigation and delayed external navigation are
// both applied on the frame goroutine, in their observable completion order.
func TestHarness_RapidNavigationWithQueuedExternalCompletion(t *testing.T) {
	d, ui := testDemo(t, t.TempDir())
	readyDemo(t, d, ui)
	tap(t, d, "EXTERNAL EVENT")
	if err := d.WaitFor(deadline(t), func() bool { return d.Clock().Pending() == 1 }); err != nil {
		t.Fatal(err)
	}
	tap(t, d, "DEEP LINK")
	if route := ui.router.Current().Route; route.Params["id"].String() != "sys_id_123" {
		t.Fatalf("foreground navigation = %v", route)
	}
	if err := d.Advance(700 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := d.Settle(deadline(t)); err != nil {
		t.Fatal(err)
	}
	if route := ui.router.Current().Route; route.Name != "incident.form" || route.Params["id"].String() != "INC0042" {
		t.Fatalf("queued navigation completion = %v", route)
	}
	if len(ui.router.Stack()) != 3 || ui.externalBusy || ui.work.pending() {
		t.Fatalf("rapid navigation did not drain cleanly: stack=%v busy=%t pending=%t", ui.router.Stack(), ui.externalBusy, ui.work.pending())
	}
}

func assertDemoJoined(t *testing.T, generation int, d *guitest.Driver, ui *demoUI) {
	t.Helper()
	if d.Clock().Pending() != 0 || ui.work.pending() || ui.persistence.pending.Load() != 0 || len(ui.persistence.results) != 0 {
		t.Fatalf("generation %d left queued work after Close", generation)
	}
	select {
	case <-ui.gridDemo.done:
	default:
		t.Fatalf("generation %d database worker outlived Close", generation)
	}
	if ui.gridDemo.db != nil {
		if err := ui.gridDemo.db.Ping(); err == nil {
			t.Fatalf("generation %d database remained open", generation)
		}
	}
}
