//go:build guitest || guitestrunner

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"

	"gioui.org/font/gofont"
	"gioui.org/text"
	"github.com/VinceLewis/gio-kit/diagnostic"
	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/guitest/script"
	"github.com/VinceLewis/gio-kit/guitest/scriptcli"
)

const (
	demoScriptFixture     = "navigation-demo.v1"
	demoStartupDelayProbe = "startup.pending"
)

// newDemoDriver is the sole in-process construction seam for both the tagged
// Go tests and the compiled script launcher. It constructs the same demoUI
// root used by the windowed application; only the Gio host is replaced.
func newDemoDriver(dataDir string) (*guitest.Driver, *demoUI, error) {
	var ui *demoUI
	driver, err := guitest.NewApp(func(environment guitest.Environment) (guitest.Harness, error) {
		var err error
		ui, err = newDemoUI(demoEnvironment{
			DataDir:    dataDir,
			Context:    environment.Context,
			Invalidate: environment.Invalidate,
			Sleep:      environment.Clock.Sleep,
		})
		if err != nil {
			return guitest.Harness{}, err
		}
		ui.theme.Shaper = text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
		return guitest.Harness{
			Layout: ui.Layout,
			Idle:   ui.Idle,
			Close:  ui.Close,
			Providers: map[string]diagnostic.Provider{
				"router": ui.router,
				"grid": diagnostic.ProviderFunc(func(request diagnostic.Request) diagnostic.Component {
					if ui.gridDemo.widget == nil {
						return diagnostic.Component{Kind: "grid", State: map[string]any{"loading": true}}
					}
					return ui.gridDemo.widget.DebugSnapshot(request)
				}),
				"form": diagnostic.ProviderFunc(func(request diagnostic.Request) diagnostic.Component {
					id := ui.router.Current().Route.Params["id"].String()
					if form := ui.formDemos[id]; form != nil {
						return form.widget.DebugSnapshot(request)
					}
					return diagnostic.Component{Kind: "form", State: map[string]any{"available": false}}
				}),
			},
		}, nil
	})
	if err != nil {
		return nil, ui, err
	}
	return driver, ui, nil
}

func demoScriptConfig(open scriptcli.Opener) scriptcli.Config {
	return scriptcli.Config{
		Open:     open,
		Fixtures: scriptcli.Names{demoScriptFixture: {}},
		Probes:   scriptcli.Names{demoStartupDelayProbe: {}},
	}
}

func newDemoScriptApplication(dataDir string) (scriptcli.Application, *demoUI, error) {
	driver, ui, err := newDemoDriver(dataDir)
	application := scriptcli.Application{Driver: driver}
	if driver != nil {
		application.Probes = map[string]script.ProbeFunc{
			demoStartupDelayProbe: func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
				if !bytes.Equal(bytes.TrimSpace(args), []byte("{}")) {
					return nil, errors.New("navigation demo: startup probe takes no arguments")
				}
				if driver.Clock().Pending() > 0 {
					return json.RawMessage("true"), nil
				}
				return json.RawMessage("false"), nil
			},
		}
	}
	return application, ui, err
}

func openTemporaryDemo(_ context.Context, fixture string) (scriptcli.Application, error) {
	if fixture != demoScriptFixture {
		return scriptcli.Application{}, errors.New("navigation demo: unsupported fixture")
	}
	dataDir, err := os.MkdirTemp("", "gio-kit-navigation-guitest-")
	if err != nil {
		return scriptcli.Application{}, err
	}
	application, _, openErr := newDemoScriptApplication(dataDir)
	application.Close = func() error { return os.RemoveAll(dataDir) }
	if openErr != nil {
		return application, openErr
	}
	return application, nil
}
