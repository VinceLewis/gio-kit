package scriptcli_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gioui.org/layout"
	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/guitest/script"
	"github.com/VinceLewis/gio-kit/guitest/scriptcli"
)

func TestRunSuccessPublishesAndClosesInOrder(t *testing.T) {
	var events []string
	application := testApplication(t, &events, nil, nil, nil, nil)
	opened := 0
	config := testConfig(func(ctx context.Context, fixture string) (scriptcli.Application, error) {
		opened++
		if fixture != "basic" {
			t.Fatalf("fixture = %q", fixture)
		}
		return application, nil
	})
	scriptPath := writeScript(t, `{"version":1,"name":"success","fixture":"basic","steps":[]}`)
	artifactDirectory := filepath.Join(t.TempDir(), "artifacts")
	result, err := scriptcli.Run(context.Background(), []string{"run", "-script", scriptPath, "-artifacts", artifactDirectory}, config)
	if err != nil {
		t.Fatal(err)
	}
	if opened != 1 || result.Status != script.StatusPassed {
		t.Fatalf("opened = %d, result = %#v", opened, result)
	}
	if want := []string{"driver", "application"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("close events = %v, want %v", events, want)
	}
	for _, name := range []string{"result.json", "trace.json"} {
		if _, err := os.Stat(filepath.Join(artifactDirectory, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestRunRejectsScriptAndRegistryBeforeMutation(t *testing.T) {
	tests := []struct {
		name   string
		doc    string
		config func(scriptcli.Opener) scriptcli.Config
		want   error
	}{
		{
			name: "invalid script", doc: `{"version":1,"name":"bad","fixture":"basic","steps":[],"unknown":true}`,
			config: testConfig, want: script.ErrInvalidScript,
		},
		{
			name: "unknown fixture", doc: `{"version":1,"name":"bad","fixture":"other","steps":[]}`,
			config: testConfig, want: scriptcli.ErrRegistry,
		},
		{
			name: "unknown probe", doc: `{"version":1,"name":"bad","fixture":"basic","steps":[{"op":"expectProbe","probe":"other","args":{},"equals":true}]}`,
			config: testConfig, want: scriptcli.ErrRegistry,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opened := false
			config := test.config(func(context.Context, string) (scriptcli.Application, error) {
				opened = true
				return scriptcli.Application{}, nil
			})
			_, err := scriptcli.Run(context.Background(), runArgs(t, test.doc), config)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if opened {
				t.Fatal("application opened before validation completed")
			}
		})
	}
}

func TestRunClosesOnPostOpenValidationRuntimeCancellationAndArtifactFailure(t *testing.T) {
	tests := []struct {
		name      string
		doc       string
		configure func(*testing.T, context.CancelFunc, *scriptcli.Config, *scriptcli.Application, string)
		want      error
	}{
		{
			name: "post-open probe validation",
			doc:  `{"version":1,"name":"missing probe","fixture":"basic","steps":[{"op":"expectProbe","probe":"known","args":{},"equals":true}]}`,
			configure: func(_ *testing.T, _ context.CancelFunc, _ *scriptcli.Config, app *scriptcli.Application, _ string) {
				app.Probes = nil
			},
			want: scriptcli.ErrApplication,
		},
		{
			name: "runtime failure",
			doc:  `{"version":1,"name":"runtime","fixture":"basic","steps":[{"op":"expectProbe","probe":"known","args":{},"equals":true}]}`,
			configure: func(_ *testing.T, _ context.CancelFunc, _ *scriptcli.Config, app *scriptcli.Application, _ string) {
				app.Probes = map[string]script.ProbeFunc{"known": func(context.Context, json.RawMessage) (json.RawMessage, error) {
					return nil, errors.New("injected runtime failure")
				}}
			},
			want: script.ErrProbe,
		},
		{
			name: "artifact failure",
			doc:  `{"version":1,"name":"artifact","fixture":"basic","steps":[]}`,
			configure: func(t *testing.T, _ context.CancelFunc, _ *scriptcli.Config, _ *scriptcli.Application, artifacts string) {
				realDirectory := filepath.Join(t.TempDir(), "real")
				if err := os.Mkdir(realDirectory, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(realDirectory, artifacts); err != nil {
					t.Fatal(err)
				}
			},
			want: script.ErrArtifact,
		},
		{
			name: "artifact commit failure",
			doc:  `{"version":1,"name":"artifact limit","fixture":"basic","steps":[]}`,
			configure: func(_ *testing.T, _ context.CancelFunc, config *scriptcli.Config, _ *scriptcli.Application, _ string) {
				config.ArtifactLimits = script.ArtifactLimits{MaxFileBytes: 1, MaxTotalBytes: 1}
			},
			want: script.ErrArtifact,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var events []string
			application := testApplication(t, &events, nil, nil, nil, nil)
			config := testConfig(func(context.Context, string) (scriptcli.Application, error) { return application, nil })
			artifactDirectory := filepath.Join(t.TempDir(), "artifacts")
			test.configure(t, cancel, &config, &application, artifactDirectory)
			config.Open = func(context.Context, string) (scriptcli.Application, error) { return application, nil }
			_, err := scriptcli.Run(ctx, []string{"run", "-script", writeScript(t, test.doc), "-artifacts", artifactDirectory}, config)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if len(events) == 0 || events[len(events)-1] != "application" {
				t.Fatalf("close events = %v", events)
			}
		})
	}
}

func TestRunClosesOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var events []string
	application := testApplication(t, &events, nil, nil, func() bool {
		cancel()
		return false
	}, nil)
	config := testConfig(func(context.Context, string) (scriptcli.Application, error) { return application, nil })
	document := `{"version":1,"name":"cancel","fixture":"basic","steps":[{"op":"settle"}]}`
	_, err := scriptcli.Run(ctx, runArgs(t, document), config)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want cancellation", err)
	}
	if want := []string{"driver", "application"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("close events = %v, want %v", events, want)
	}
}

func TestRunClosesPartialOpenAndJoinsCleanupErrors(t *testing.T) {
	openErr := errors.New("open failed")
	driverCloseErr := errors.New("driver close failed")
	applicationCloseErr := errors.New("application close failed")
	var events []string
	application := testApplication(t, &events, driverCloseErr, applicationCloseErr, nil, nil)
	config := testConfig(func(context.Context, string) (scriptcli.Application, error) {
		return application, openErr
	})
	_, err := scriptcli.Run(context.Background(), runArgs(t, `{"version":1,"name":"open","fixture":"basic","steps":[]}`), config)
	for _, want := range []error{openErr, driverCloseErr, applicationCloseErr} {
		if !errors.Is(err, want) {
			t.Errorf("joined error %v does not contain %v", err, want)
		}
	}
	if want := []string{"driver", "application"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("close events = %v, want %v", events, want)
	}
}

func TestRunStrictFlags(t *testing.T) {
	config := testConfig(func(context.Context, string) (scriptcli.Application, error) {
		t.Fatal("opener called for invalid flags")
		return scriptcli.Application{}, nil
	})
	for _, args := range [][]string{nil, {"help"}, {"run"}, {"run", "-script", "x", "-artifacts", "y", "extra"}} {
		if _, err := scriptcli.Run(context.Background(), args, config); !errors.Is(err, scriptcli.ErrUsage) {
			t.Errorf("args %v error = %v", args, err)
		}
	}
}

func testConfig(open scriptcli.Opener) scriptcli.Config {
	return scriptcli.Config{
		Open:     open,
		Fixtures: scriptcli.Names{"basic": {}},
		Probes:   scriptcli.Names{"known": {}},
	}
}

func testApplication(t *testing.T, events *[]string, driverCloseErr, applicationCloseErr error, idle func() bool, probes map[string]script.ProbeFunc) scriptcli.Application {
	t.Helper()
	driver, err := guitest.NewApp(func(guitest.Environment) (guitest.Harness, error) {
		return guitest.Harness{
			Layout: func(layout.Context) layout.Dimensions { return layout.Dimensions{} },
			Idle:   idle,
			Close: func() error {
				*events = append(*events, "driver")
				return driverCloseErr
			},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return scriptcli.Application{
		Driver: driver, Probes: probes,
		Close: func() error {
			*events = append(*events, "application")
			return applicationCloseErr
		},
	}
}

func writeScript(t *testing.T, document string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "script.json")
	if err := os.WriteFile(path, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runArgs(t *testing.T, document string) []string {
	t.Helper()
	return []string{"run", "-script", writeScript(t, document), "-artifacts", filepath.Join(t.TempDir(), "artifacts")}
}
