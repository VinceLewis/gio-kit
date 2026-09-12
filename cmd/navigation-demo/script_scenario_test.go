//go:build guitest

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/guitest/script"
	"github.com/VinceLewis/gio-kit/guitest/scriptcli"
)

// GK-7/N1/N2/N4/N6/G5: the checked-in v1 script drives the production demo
// root, real form and grid widgets, virtual startup delay and SQLite storage.
func TestScriptScenario_ProductionRootAndJoinedTeardown(t *testing.T) {
	dataDir := t.TempDir()
	artifactDir := t.TempDir()
	var (
		driver      *guitest.Driver
		ui          *demoUI
		openCount   int
		consumerEnd bool
	)
	config := demoScriptConfig(func(_ context.Context, fixture string) (scriptcli.Application, error) {
		openCount++
		if fixture != demoScriptFixture {
			t.Fatalf("fixture = %q", fixture)
		}
		application, createdUI, err := newDemoScriptApplication(dataDir)
		driver, ui = application.Driver, createdUI
		application.Close = func() error {
			consumerEnd = true
			return nil
		}
		return application, err
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := scriptcli.Run(ctx, []string{
		"run",
		"-script", filepath.Join("testdata", "guitest", "navigation-demo-v1.json"),
		"-artifacts", artifactDir,
	}, config)
	if err != nil {
		t.Fatalf("scenario failed: %v (result=%+v)", err, result)
	}
	if result.Status != script.StatusPassed || result.Steps != 24 || result.Failure != nil {
		t.Fatalf("scenario result = %+v", result)
	}
	if openCount != 1 || !consumerEnd {
		t.Fatalf("lifecycle: opens=%d consumerClose=%t", openCount, consumerEnd)
	}
	assertDemoJoined(t, 0, driver, ui)
	for _, path := range []string{
		filepath.Join(dataDir, "gio-kit-demo.db"),
		filepath.Join(dataDir, "navigation-state.json"),
		filepath.Join(artifactDir, "result.json"),
		filepath.Join(artifactDir, "trace.json"),
		filepath.Join(artifactDir, "step-022-capture.json"),
	} {
		if info, statErr := os.Stat(path); statErr != nil || info.Size() == 0 {
			t.Fatalf("expected non-empty %s: info=%v err=%v", path, info, statErr)
		}
	}
}
