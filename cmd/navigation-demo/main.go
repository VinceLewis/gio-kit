package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/router"
)

var (
	background = color.NRGBA{R: 244, G: 247, B: 251, A: 255}
	card       = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	ink        = color.NRGBA{R: 25, G: 36, B: 56, A: 255}
	muted      = color.NRGBA{R: 91, G: 105, B: 127, A: 255}
	primary    = color.NRGBA{R: 39, G: 92, B: 225, A: 255}
	danger     = color.NRGBA{R: 183, G: 48, B: 62, A: 255}
	success    = color.NRGBA{R: 26, G: 128, B: 78, A: 255}
)

type listScreenState struct {
	list   widget.List
	filter widget.Editor
}

type externalResult struct {
	message string
	err     error
}

type demoUI struct {
	window          *app.Window
	theme           *material.Theme
	table           *router.Table
	router          *router.Router
	statePath       string
	lastVersion     uint64
	listState       *listScreenState
	rows            [100]widget.Clickable
	back            widget.Clickable
	newIncident     widget.Clickable
	deepLink        widget.Clickable
	notification    widget.Clickable
	openGrid        widget.Clickable
	gridDemo        *gridDemo
	formDemos       map[string]*incidentFormDemo
	lookup          *lookupDemo
	restore         widget.Clickable
	edit            widget.Clickable
	clean           widget.Clickable
	modalCancel     widget.Clickable
	overlayBlocker  widget.Clickable
	confirmLeave    widget.Clickable
	confirmStay     widget.Clickable
	dirty           bool
	confirming      bool
	status          string
	statusOK        bool
	lastStatus      string
	statusSince     time.Time
	statusDismissed bool
	statusDismiss   widget.Clickable
	backTag         struct{}
	externalNav     chan externalResult
}

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
			if ui != nil && ui.gridDemo != nil {
				ui.Close()
			}
			return e.Err
		case app.FrameEvent:
			if ui == nil {
				created, err := newDemoUI(window)
				if err != nil {
					return err
				}
				ui = created
			}
			gtx := app.NewContext(&ops, e)
			ui.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func routeTable() (*router.Table, error) {
	return router.NewTable(
		router.Definition{Name: "incident.list", Pattern: "/incidents"},
		router.Definition{Name: "incident.form", Pattern: "/incident/:id"},
		router.Definition{Name: "incident.new", Pattern: "/new-incident"},
		router.Definition{Name: "incident.grid", Pattern: "/incident-grid"},
		router.Definition{Name: "lookup.grid", Pattern: "/lookup/:table/:field"},
	)
}

func newDemoUI(window *app.Window) (*demoUI, error) {
	table, err := routeTable()
	if err != nil {
		return nil, err
	}
	r, err := router.New(table, router.Route{Name: "incident.list", Params: router.Params{"filter": router.String("")}})
	if err != nil {
		return nil, err
	}
	dataDir, err := app.DataDir()
	if err != nil {
		return nil, err
	}
	u := &demoUI{
		window:      window,
		table:       table,
		router:      r,
		statePath:   filepath.Join(dataDir, "navigation-state.json"),
		listState:   &listScreenState{list: widget.List{List: layout.List{Axis: layout.Vertical}}},
		status:      "Ready: complete the five test cards below.",
		statusOK:    true,
		externalNav: make(chan externalResult, 1),
		formDemos:   make(map[string]*incidentFormDemo),
	}
	u.listState.filter.SingleLine = true
	u.listState.filter.SetText("active")
	u.theme = material.NewTheme()
	u.theme.Palette.Bg, u.theme.Palette.Fg = background, ink
	u.theme.Palette.ContrastBg, u.theme.Palette.ContrastFg = primary, card
	u.gridDemo = newGridDemo(window, dataDir)
	_ = u.router.SetCurrentState(u.listState)
	if saved, readErr := os.ReadFile(u.statePath); readErr == nil {
		if restoreErr := u.router.Restore(saved); restoreErr == nil {
			u.status = "Restored saved navigation stack from the previous launch."
		}
	}
	u.installGuard()
	u.lastVersion = u.router.Version()
	return u, nil
}

func (u *demoUI) installGuard() {
	u.router.SetGuard(func(transition router.Transition) bool {
		if u.dirty && transition.From.Name == "incident.form" && transition.Operation != router.PushModalOperation {
			u.confirming = true
			u.status, u.statusOK = "Guard blocked navigation until you confirm.", false
			u.window.Invalidate()
			return false
		}
		return true
	})
}

func (u *demoUI) layout(gtx layout.Context) layout.Dimensions {
	paint.Fill(gtx.Ops, background)
	select {
	case result := <-u.externalNav:
		if result.err != nil {
			u.fail(result.err)
		} else {
			u.status, u.statusOK = result.message, true
		}
	default:
	}
	if len(u.router.Stack()) > 1 || len(u.router.Modals()) > 0 || u.confirming {
		event.Op(gtx.Ops, &u.backTag)
		for {
			e, ok := gtx.Event(key.Filter{Name: key.NameBack})
			if !ok {
				break
			}
			if e, ok := e.(key.Event); ok && e.State == key.Press {
				u.navigateBack()
			}
		}
	}

	if u.status != u.lastStatus {
		u.lastStatus = u.status
		u.statusSince = gtx.Now
		u.statusDismissed = false
	}
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(u.appBar),
		layout.Rigid(u.statusBar),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			inset := unit.Dp(16)
			if gtx.Constraints.Max.Y < gtx.Dp(unit.Dp(520)) {
				inset = 8
			}
			return layout.UniformInset(inset).Layout(gtx, u.content)
		}),
	)
	if len(u.router.Modals()) > 0 {
		if u.router.Current().Route.Name == "lookup.grid" {
			u.lookupModal(gtx)
		} else {
			u.newIncidentModal(gtx)
		}
	}
	if u.confirming {
		u.confirmDialog(gtx)
	}
	if u.router.Version() != u.lastVersion {
		u.lastVersion = u.router.Version()
		if data, err := u.router.Serialize(); err == nil {
			if err = os.WriteFile(u.statePath, data, 0o600); err != nil {
				u.status, u.statusOK = "Could not persist route state: "+err.Error(), false
			}
		}
		gtx.Execute(op.InvalidateCmd{})
	}
	return dims
}

func (u *demoUI) appBar(gtx layout.Context) layout.Dimensions {
	compact := gtx.Constraints.Max.Y < gtx.Dp(unit.Dp(600))
	verticalInset := unit.Dp(8)
	buttonSize := unit.Dp(48)
	if compact {
		verticalInset = 2
		buttonSize = 40
	}
	return u.surface(gtx, 0, card, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: verticalInset, Bottom: verticalInset, Left: unit.Dp(8), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if len(u.router.Stack()) <= 1 && len(u.router.Modals()) == 0 {
						return layout.Dimensions{Size: image.Pt(gtx.Dp(buttonSize), gtx.Dp(buttonSize))}
					}
					if u.back.Clicked(gtx) {
						u.navigateBack()
					}
					button := material.Button(u.theme, &u.back, "‹")
					button.Background = color.NRGBA{}
					button.Color = primary
					button.TextSize = unit.Sp(24)
					button.Inset = layout.UniformInset(unit.Dp(5))
					return button.Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					var label material.LabelStyle
					if compact {
						label = material.Body1(u.theme, "Gio Kit CRUD Lab")
					} else {
						label = material.H6(u.theme, "Gio Kit CRUD Lab")
					}
					label.Alignment = text.Middle
					label.MaxLines = 1
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Caption(u.theme, fmt.Sprintf("%d", len(u.router.Stack())))
					label.Color = muted
					return label.Layout(gtx)
				}),
			)
		})
	})
}

func (u *demoUI) statusBar(gtx layout.Context) layout.Dimensions {
	if u.status == "" || u.statusDismissed {
		return layout.Dimensions{}
	}
	if u.statusDismiss.Clicked(gtx) {
		u.statusDismissed = true
		return layout.Dimensions{}
	}
	if u.statusOK && !u.statusSince.IsZero() {
		expires := u.statusSince.Add(4 * time.Second)
		if !gtx.Now.Before(expires) {
			u.statusDismissed = true
			return layout.Dimensions{}
		}
		gtx.Execute(op.InvalidateCmd{At: expires})
	}
	bg := success
	if !u.statusOK {
		bg = danger
	}
	return u.surface(gtx, 0, bg, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5), Left: unit.Dp(10), Right: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					label := material.Caption(u.theme, u.status)
					label.Color = card
					label.MaxLines = 1
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					button := material.Button(u.theme, &u.statusDismiss, "×")
					button.Background = color.NRGBA{}
					button.Color = card
					button.Inset = layout.UniformInset(unit.Dp(3))
					return button.Layout(gtx)
				}),
			)
		})
	})
}

func (u *demoUI) content(gtx layout.Context) layout.Dimensions {
	current := u.router.MainCurrent().Route
	switch current.Name {
	case "incident.list":
		return u.listScreen(gtx)
	case "incident.grid":
		return u.gridDemo.Layout(gtx, u)
	case "incident.form":
		return u.dataFormScreen(gtx, current)
	default:
		return material.Body1(u.theme, "Unknown route: "+current.Name).Layout(gtx)
	}
}

func (u *demoUI) listScreen(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.heading(gtx, "Incident list", "Scroll, filter, open a record, then return. This screen owns its widget state.")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			editor := material.Editor(u.theme, &u.listState.filter, "Filter incidents")
			editor.TextSize = unit.Sp(16)
			return u.panel(gtx, editor.Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if u.newIncident.Clicked(gtx) {
							if err := u.router.PushModal(router.Route{Name: "incident.new"}); err != nil {
								u.fail(err)
							} else {
								u.status, u.statusOK = "Modal opened above the retained list.", true
							}
						}
						return material.Button(u.theme, &u.newIncident, "NEW MODAL").Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(8)}.Layout(gtx) }),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if u.deepLink.Clicked(gtx) {
							route, err := u.router.ResolveURL("gio-kit://app/incident/sys_id_123?openedFrom=deep-link")
							if err == nil {
								err = u.router.Push(route)
							}
							if err != nil {
								u.fail(err)
							} else {
								u.status, u.statusOK = "Deep link resolved cold: sys_id_123.", true
							}
						}
						return material.Button(u.theme, &u.deepLink, "DEEP LINK").Layout(gtx)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if u.notification.Clicked(gtx) {
				u.status, u.statusOK = "External event queued…", true
				go func() {
					time.Sleep(700 * time.Millisecond)
					route, err := u.router.ResolveURL("gio-kit://notification/incident/INC0042?openedFrom=notification")
					if err == nil {
						err = u.router.Push(route)
					}
					u.externalNav <- externalResult{message: "Background notification pushed INC0042.", err: err}
					u.window.Invalidate()
				}()
			}
			if u.openGrid.Clicked(gtx) {
				if err := u.router.Push(router.Route{Name: "incident.grid"}); err != nil {
					u.fail(err)
				}
			}
			externalButton := material.Button(u.theme, &u.notification, "EXTERNAL EVENT")
			externalButton.Background = ink
			return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, externalButton.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(8)}.Layout(gtx) }),
					layout.Flexed(1, material.Button(u.theme, &u.openGrid, "OPEN SQLITE GRID").Layout),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			caption := material.Caption(u.theme, fmt.Sprintf("Visible list state • filter=%q • first row=%d", u.listState.filter.Text(), u.listState.list.Position.First+1))
			caption.Color = muted
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, caption.Layout)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			needle := strings.ToLower(strings.TrimSpace(u.listState.filter.Text()))
			indices := make([]int, 0, len(u.rows))
			for i := range u.rows {
				text := fmt.Sprintf("INC%04d active network incident %d", i+1, i+1)
				if needle == "" || strings.Contains(strings.ToLower(text), needle) {
					indices = append(indices, i)
				}
			}
			return u.listState.list.Layout(gtx, len(indices), func(gtx layout.Context, row int) layout.Dimensions {
				i := indices[row]
				if u.rows[i].Clicked(gtx) {
					id := fmt.Sprintf("INC%04d", i+1)
					err := u.router.Push(router.Route{Name: "incident.form", Params: router.Params{
						"id": router.String(id), "row": router.Int(int64(i + 1)), "openedFrom": router.String("list"),
					}})
					if err != nil {
						u.fail(err)
					} else {
						u.status, u.statusOK = "Record pushed; use Back to verify retained state.", true
					}
				}
				return u.rows[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return u.row(gtx, fmt.Sprintf("INC%04d", i+1), fmt.Sprintf("Active network incident %d", i+1))
				})
			})
		}),
	)
}

func (u *demoUI) formScreen(gtx layout.Context, route router.Route) layout.Dimensions {
	id := route.Params["id"].String()
	openedFrom := route.Params["openedFrom"].String()
	if openedFrom == "" {
		openedFrom = "restored route"
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.heading(gtx, id, "Parameterized incident.form • opened from "+openedFrom)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			stack := u.router.Stack()
			parts := make([]string, len(stack))
			for i, entry := range stack {
				parts[i] = entry.Route.Name
			}
			return u.infoCard(gtx, "Inspect history / breadcrumbs", strings.Join(parts, "  ›  "))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.infoCard(gtx, "Typed parameters (N1/N3)", fmt.Sprintf("id: %s\nopenedFrom: %s", id, openedFrom))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if u.edit.Clicked(gtx) {
				u.dirty = true
				u.status, u.statusOK = "Unsaved edit enabled; Back must now be guarded.", true
			}
			label := "MAKE UNSAVED EDIT"
			if u.dirty {
				label = "UNSAVED EDIT ACTIVE"
			}
			button := material.Button(u.theme, &u.edit, label)
			if u.dirty {
				button.Background = danger
			}
			return layout.Inset{Top: unit.Dp(12)}.Layout(gtx, button.Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if u.restore.Clicked(gtx) {
				data, err := u.router.Serialize()
				if err == nil {
					fresh, freshErr := router.New(u.table, router.Route{Name: "incident.list"})
					if freshErr != nil {
						err = freshErr
					} else if err = fresh.Restore(data); err == nil {
						u.router = fresh
						u.installGuard()
						u.lastVersion = fresh.Version()
					}
				}
				if err != nil {
					u.fail(err)
				} else {
					u.status, u.statusOK = "New Router restored this record from serialized state.", true
				}
			}
			button := material.Button(u.theme, &u.restore, "SIMULATE PROCESS DEATH + RESTORE")
			button.Background = ink
			return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, button.Layout)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Body2(u.theme, "Use the top Back button or Android system Back. If an edit is active, navigation must pause for confirmation.")
			label.Color = muted
			return layout.Inset{Top: unit.Dp(16)}.Layout(gtx, label.Layout)
		}),
	)
}

func (u *demoUI) navigateBack() {
	removed, err := u.router.Pop()
	switch {
	case err == nil:
		u.cleanupForm(removed.Route)
		u.dirty = false
		u.status, u.statusOK = "Back completed; verify filter and scroll marker were preserved.", true
	case errors.Is(err, router.ErrGuarded):
		// Guard already opened the confirmation dialog.
	case errors.Is(err, router.ErrRoot):
		u.status, u.statusOK = "At root route; Android may close on a second system Back.", false
	default:
		u.fail(err)
	}
}

func (u *demoUI) newIncidentModal(gtx layout.Context) layout.Dimensions {
	return u.overlay(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.H5(u.theme, "New Incident").Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Body1(u.theme, "This is an independent modal stack. Cancelling must reveal the exact list state underneath.")
				label.Color = muted
				return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(18)}.Layout(gtx, label.Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if u.modalCancel.Clicked(gtx) {
					if _, err := u.router.PopModal(); err != nil {
						u.fail(err)
					} else {
						u.status, u.statusOK = "Modal cancelled; underlying list instance retained.", true
					}
				}
				button := material.Button(u.theme, &u.modalCancel, "CANCEL MODAL")
				button.Background = primary
				return button.Layout(gtx)
			}),
		)
	})
}

func (u *demoUI) confirmDialog(gtx layout.Context) layout.Dimensions {
	return u.overlay(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.H5(u.theme, "Discard unsaved changes?").Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Body1(u.theme, "The navigation guard blocked Back. Choose Stay to prove the route is unchanged, or Discard to perform the confirmed pop.")
				label.Color = muted
				return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(18)}.Layout(gtx, label.Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if u.confirmStay.Clicked(gtx) {
							u.confirming = false
							u.status, u.statusOK = "Stayed: route and unsaved edit are unchanged.", true
						}
						button := material.Button(u.theme, &u.confirmStay, "STAY")
						button.Background = ink
						return button.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layout.Spacer{Width: unit.Dp(8)}.Layout(gtx) }),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if u.confirmLeave.Clicked(gtx) {
							u.confirming, u.dirty = false, false
							if removed, err := u.router.ForcePop(); err != nil {
								u.fail(err)
							} else {
								u.cleanupForm(removed.Route)
								u.status, u.statusOK = "Discard confirmed; guarded Back completed.", true
							}
						}
						button := material.Button(u.theme, &u.confirmLeave, "DISCARD")
						button.Background = danger
						return button.Layout(gtx)
					}),
				)
			}),
		)
	})
}

func (u *demoUI) overlay(gtx layout.Context, content layout.Widget) layout.Dimensions {
	u.overlayBlocker.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		paint.FillShape(gtx.Ops, color.NRGBA{A: 155}, clip.Rect{Max: gtx.Constraints.Max}.Op())
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(370)))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return u.surface(gtx, 18, card, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(22)).Layout(gtx, content)
		})
	})
}

func (u *demoUI) heading(gtx layout.Context, title, subtitle string) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.H4(u.theme, title).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := material.Body2(u.theme, subtitle)
			label.Color = muted
			return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(12)}.Layout(gtx, label.Layout)
		}),
	)
}

func (u *demoUI) panel(gtx layout.Context, content layout.Widget) layout.Dimensions {
	return u.surface(gtx, 10, card, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, content)
	})
}

func (u *demoUI) surface(gtx layout.Context, radius unit.Dp, fill color.NRGBA, content layout.Widget) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dimensions := content(gtx)
	call := recording.Stop()
	paint.FillShape(gtx.Ops, fill, clip.UniformRRect(image.Rectangle{Max: dimensions.Size}, gtx.Dp(radius)).Op(gtx.Ops))
	call.Add(gtx.Ops)
	return dimensions
}

func (u *demoUI) infoCard(gtx layout.Context, title, body string) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return u.panel(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(material.H6(u.theme, title).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(u.theme, body)
					label.Color = muted
					return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, label.Layout)
				}),
			)
		})
	})
}

func (u *demoUI) row(gtx layout.Context, title, subtitle string) layout.Dimensions {
	return u.surface(gtx, 0, card, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(material.Body1(u.theme, title).Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := material.Caption(u.theme, subtitle)
							label.Color = muted
							return label.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.H6(u.theme, "›")
					label.Color = primary
					return label.Layout(gtx)
				}),
			)
		})
	})
}

func (u *demoUI) fail(err error) {
	u.status, u.statusOK = err.Error(), false
}
