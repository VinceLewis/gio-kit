//go:build !guitest && !guitestrunner

package main

import (
	"errors"
	"log"

	"gioui.org/app"
	"gioui.org/op"
)

func main() {
	go func() {
		window := new(app.Window)
		window.Option(app.Title("Gio Kit — Navigation Lab"), app.Size(420, 820))
		if err := run(window); err != nil {
			log.Print(err)
		}
	}()
	app.Main()
}

func run(window *app.Window) error {
	var ui *demoUI
	var ops op.Ops
	for {
		switch e := window.Event().(type) {
		case app.DestroyEvent:
			if ui != nil {
				return errors.Join(e.Err, ui.Close())
			}
			return e.Err
		case app.FrameEvent:
			if ui == nil {
				dataDir, err := app.DataDir()
				if err != nil {
					return err
				}
				created, err := newDemoUI(demoEnvironment{DataDir: dataDir, Invalidate: window.Invalidate})
				if err != nil {
					return err
				}
				ui = created
			}
			gtx := app.NewContext(&ops, e)
			ui.Layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}
