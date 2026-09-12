package script_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gioui.org/gesture"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/diagnostic"
	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/guitest/script"
)

func TestRunRoutesV1ActionsThroughGio(t *testing.T) {
	var button widget.Clickable
	var editor widget.Editor
	theme := material.NewTheme()
	clicks := 0
	driver, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		if button.Clicked(gtx) {
			clicks++
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.Button(theme, &button, "Action").Layout),
			layout.Rigid(material.Editor(theme, &editor, "Summary").Layout),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = driver.Close() })
	action := script.Selector{Label: textPointer("Action")}
	editorSelector := script.Selector{Role: rolePointer(script.RoleEditor)}
	steps := []script.Step{
		{Op: script.OpTap, Selector: &action},
		{Op: script.OpDoubleTap, Selector: &action},
		{Op: script.OpLongPress, Selector: &action, Duration: durationPointer(20 * time.Millisecond)},
		{Op: script.OpPress, Selector: &action},
		{Op: script.OpMove, Position: &script.Point{X: 20, Y: 20}},
		{Op: script.OpRelease},
		{Op: script.OpPress, Selector: &action},
		{Op: script.OpCancelPointer},
		{Op: script.OpDrag, Selector: &action, Delta: &script.Point{X: 4, Y: 4}, Duration: durationPointer(40 * time.Millisecond)},
		{Op: script.OpFocus, Selector: &editorSelector},
		{Op: script.OpType, Selector: &editorSelector, Text: textPointer("abc")},
		{Op: script.OpSetSelection, Range: &script.Range{Start: 0, End: 1}},
		{Op: script.OpEdit, Range: &script.Range{Start: 0, End: 1}, Text: textPointer("Z")},
		{Op: script.OpSetComposition, Range: &script.Range{Start: 0, End: 1}},
		{Op: script.OpKey, Key: &script.Key{Name: "Escape", Modifiers: []script.KeyModifier{script.ModifierShift, script.ModifierControl}}},
		{Op: script.OpBack},
		{Op: script.OpClearFocus},
		{Op: script.OpResize, Size: &script.Size{Width: 640, Height: 400}, Metric: &script.Metric{PxPerDp: 1, PxPerSp: 1}},
		{Op: script.OpAdvance, Duration: durationPointer(0)},
		{Op: script.OpSettle},
		{Op: script.OpExpect, Selector: &editorSelector, Expect: script.ExpectPresent},
	}
	result, err := script.Run(context.Background(), driver, validScript(steps...), script.Options{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != script.StatusPassed || result.Steps != len(steps) {
		t.Fatalf("result = %+v", result)
	}
	if clicks < 3 {
		t.Fatalf("routed clicks = %d", clicks)
	}
	if editor.Text() != "Zbc" {
		t.Fatalf("editor text = %q", editor.Text())
	}
	if err = driver.Frame(); err != nil {
		t.Fatalf("runner closed caller driver: %v", err)
	}
}

func TestRunReadinessWaitsThenDispatchesExactlyOnce(t *testing.T) {
	var button widget.Clickable
	theme := material.NewTheme()
	frames, clicks := 0, 0
	driver, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		frames++
		enabled := frames >= 4
		if !enabled {
			gtx.Execute(op.InvalidateCmd{})
		}
		if button.Clicked(gtx) {
			clicks++
		}
		if !enabled {
			gtx = gtx.Disabled()
		}
		return material.Button(theme, &button, "Eventually enabled").Layout(gtx)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = driver.Close() })
	selector := script.Selector{Label: textPointer("Eventually enabled")}
	result, err := script.Run(context.Background(), driver, validScript(script.Step{Op: script.OpTap, Selector: &selector}), script.Options{})
	if err != nil || result.Status != script.StatusPassed {
		t.Fatalf("Run = (%+v, %v)", result, err)
	}
	if clicks != 1 {
		t.Fatalf("action dispatched %d times", clicks)
	}
}

func TestRunAmbiguityFailsBeforeInput(t *testing.T) {
	var buttons [2]widget.Clickable
	theme := material.NewTheme()
	clicks := 0
	driver, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		for index := range buttons {
			if buttons[index].Clicked(gtx) {
				clicks++
			}
		}
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(1, material.Button(theme, &buttons[0], "Duplicate").Layout),
			layout.Flexed(1, material.Button(theme, &buttons[1], "Duplicate").Layout),
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = driver.Close() })
	selector := script.Selector{Label: textPointer("Duplicate")}
	frame := driver.FrameNumber()
	result, err := script.Run(context.Background(), driver, validScript(script.Step{Op: script.OpTap, Selector: &selector}), script.Options{})
	assertStepError(t, result, err, script.CodeAmbiguous, guitest.ErrAmbiguous)
	if clicks != 0 || driver.FrameNumber() != frame {
		t.Fatalf("ambiguous action mutated UI: clicks=%d frames=%d->%d", clicks, frame, driver.FrameNumber())
	}
}

func TestRunSemanticSnapshotAndCanonicalProbeAssertions(t *testing.T) {
	selected := true
	provider := diagnostic.ProviderFunc(func(diagnostic.Request) diagnostic.Component {
		return diagnostic.Component{Kind: "test", State: map[string]any{
			"loading": false, "items": []any{map[string]any{"count": 2}},
		}}
	})
	driver, err := guitest.NewApp(func(guitest.Environment) (guitest.Harness, error) {
		return guitest.Harness{
			Layout: func(gtx layout.Context) layout.Dimensions {
				defer clip.Rect(image.Rect(0, 0, 100, 40)).Push(gtx.Ops).Pop()
				semantic.LabelOp("Selected item").Add(gtx.Ops)
				semantic.SelectedOp(selected).Add(gtx.Ops)
				return layout.Dimensions{Size: image.Pt(100, 40)}
			},
			Providers: map[string]diagnostic.Provider{"state": provider},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = driver.Close() })
	selector := script.Selector{Label: textPointer("Selected item")}
	pointer := "/items/0/count"
	root := ""
	probeCalls := 0
	hugeExponent := json.RawMessage("1e" + strings.Repeat("9", 100))
	steps := []script.Step{
		{Op: script.OpExpect, Selector: &selector, Expect: script.ExpectPresent},
		{Op: script.OpExpect, Selector: &selector, Expect: script.ExpectSelected},
		{Op: script.OpExpect, Selector: &selector, Expect: script.ExpectViewportVisible},
		{Op: script.OpExpectSnapshot, Component: "state", Pointer: &pointer, Equals: json.RawMessage(`2.0`)},
		{Op: script.OpExpectSnapshot, Component: "state", Pointer: &root, NotEquals: json.RawMessage(`null`)},
		{Op: script.OpExpectProbe, Probe: "integrity", Args: json.RawMessage(`{"kind":"test"}`), Equals: json.RawMessage(`{"a":1,"b":2}`)},
		{Op: script.OpExpectProbe, Probe: "huge-number", Args: json.RawMessage(`{}`), Equals: hugeExponent},
	}
	options := script.Options{Probes: map[string]script.ProbeFunc{
		"integrity": func(context.Context, json.RawMessage) (json.RawMessage, error) {
			probeCalls++
			if probeCalls == 1 {
				return json.RawMessage(`{"a":0,"b":2}`), nil
			}
			return json.RawMessage(`{"b":2.0,"a":1.0}`), nil
		},
		"huge-number": func(context.Context, json.RawMessage) (json.RawMessage, error) {
			return append(json.RawMessage(nil), hugeExponent...), nil
		},
	}}
	result, err := script.Run(context.Background(), driver, validScript(steps...), options)
	if err != nil || result.Status != script.StatusPassed {
		t.Fatalf("Run = (%+v, %v)", result, err)
	}
	if probeCalls != 2 {
		t.Fatalf("probe calls = %d, want 2", probeCalls)
	}
}

func TestRunRejectsUnknownProbeBeforeEarlierMutation(t *testing.T) {
	var button widget.Clickable
	theme := material.NewTheme()
	clicks := 0
	driver, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		if button.Clicked(gtx) {
			clicks++
		}
		return material.Button(theme, &button, "Mutate").Layout(gtx)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = driver.Close() })
	selector := script.Selector{Label: textPointer("Mutate")}
	document := validScript(
		script.Step{Op: script.OpTap, Selector: &selector},
		script.Step{Op: script.OpExpectProbe, Probe: "unknown", Equals: json.RawMessage(`true`)},
	)
	_, err = script.Run(context.Background(), driver, document, script.Options{})
	if !errors.Is(err, script.ErrInvalidScript) || clicks != 0 {
		t.Fatalf("preflight = (%v, clicks %d)", err, clicks)
	}
}

func TestRunExplicitAdvanceObservesLoadingWithoutAutoSettle(t *testing.T) {
	var button widget.Clickable
	var mu sync.Mutex
	loading, done := false, false
	completed := make(chan struct{}, 1)
	var worker sync.WaitGroup
	theme := material.NewTheme()
	driver, err := guitest.NewApp(func(env guitest.Environment) (guitest.Harness, error) {
		return guitest.Harness{
			Layout: func(gtx layout.Context) layout.Dimensions {
				select {
				case <-completed:
					mu.Lock()
					loading, done = false, true
					mu.Unlock()
				default:
				}
				if button.Clicked(gtx) {
					mu.Lock()
					loading = true
					mu.Unlock()
					worker.Add(1)
					go func() {
						defer worker.Done()
						if env.Clock.Sleep(env.Context, time.Second) == nil {
							completed <- struct{}{}
							env.Invalidate()
						}
					}()
				}
				mu.Lock()
				isLoading, isDone := loading, done
				mu.Unlock()
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.Button(theme, &button, "Start").Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := "Idle"
						if isLoading {
							label = "Loading"
						}
						if isDone {
							label = "Done"
						}
						defer clip.Rect(image.Rect(0, 0, 100, 20)).Push(gtx.Ops).Pop()
						semantic.LabelOp(label).Add(gtx.Ops)
						return layout.Dimensions{Size: image.Pt(100, 20)}
					}),
				)
			},
			Idle:  func() bool { mu.Lock(); defer mu.Unlock(); return !loading },
			Close: func() error { worker.Wait(); return nil },
			Providers: map[string]diagnostic.Provider{"runtime": diagnostic.ProviderFunc(func(diagnostic.Request) diagnostic.Component {
				return diagnostic.Component{Kind: "runtime", State: map[string]any{"pending": env.Clock.Pending()}}
			})},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = driver.Close() })
	start := script.Selector{Label: textPointer("Start")}
	loadingSelector := script.Selector{Label: textPointer("Loading")}
	doneSelector := script.Selector{Label: textPointer("Done")}
	pending := "/pending"
	steps := []script.Step{
		{Op: script.OpTap, Selector: &start},
		{Op: script.OpExpect, Selector: &loadingSelector, Expect: script.ExpectPresent},
		{Op: script.OpExpectSnapshot, Component: "runtime", Pointer: &pending, Equals: json.RawMessage(`1`)},
		{Op: script.OpAdvance, Duration: durationPointer(time.Second)},
		{Op: script.OpSettle},
		{Op: script.OpExpect, Selector: &doneSelector, Expect: script.ExpectPresent},
	}
	if _, err = script.Run(context.Background(), driver, validScript(steps...), script.Options{}); err != nil {
		t.Fatal(err)
	}
}

func TestRunCancellationFrameLimitAndHeldPointerRecovery(t *testing.T) {
	t.Run("cancelled", func(t *testing.T) {
		driver, err := guitest.New(func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = driver.Close() })
		missing := script.Selector{Label: textPointer("Missing")}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result, err := script.Run(ctx, driver, validScript(script.Step{Op: script.OpExpect, Selector: &missing, Expect: script.ExpectPresent}), script.Options{})
		assertStepError(t, result, err, script.CodeCancelled, context.Canceled)
	})

	t.Run("frame limit", func(t *testing.T) {
		driver, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
			gtx.Execute(op.InvalidateCmd{})
			return layout.Dimensions{}
		}, guitest.MaxFrames(4))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = driver.Close() })
		missing := script.Selector{Label: textPointer("Missing")}
		result, err := script.Run(context.Background(), driver, validScript(script.Step{Op: script.OpExpect, Selector: &missing, Expect: script.ExpectPresent}), script.Options{})
		assertStepError(t, result, err, script.CodeFrameLimit, guitest.ErrFrameLimit)
	})

	t.Run("held pointer", func(t *testing.T) {
		var click gesture.Click
		driver, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
			for {
				if _, ok := click.Update(gtx.Source); !ok {
					break
				}
			}
			defer clip.Rect(image.Rect(0, 0, 100, 60)).Push(gtx.Ops).Pop()
			semantic.LabelOp("Touch").Add(gtx.Ops)
			click.Add(gtx.Ops)
			return layout.Dimensions{Size: image.Pt(100, 60)}
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = driver.Close() })
		touch := script.Selector{Label: textPointer("Touch")}
		missing := script.Selector{Label: textPointer("Missing")}
		timeout := durationPointer(time.Millisecond)
		document := validScript(
			script.Step{Op: script.OpPress, Selector: &touch},
			script.Step{Op: script.OpExpect, Selector: &missing, Expect: script.ExpectPresent, Timeout: timeout},
			script.Step{Op: script.OpCancelPointer},
		)
		result, err := script.Run(context.Background(), driver, document, script.Options{})
		assertStepError(t, result, err, script.CodeAssertionFailed, script.ErrAssertion)
		if err = driver.Press(guitest.Label("Touch")); err != nil {
			t.Fatalf("pointer was not cancelled after failure: %v", err)
		}
		_ = driver.CancelPointer()
	})
}

func TestRunCancellationBoundariesPreventFurtherMutation(t *testing.T) {
	t.Run("already cancelled action and advance", func(t *testing.T) {
		var button widget.Clickable
		clicks := 0
		theme := material.NewTheme()
		driver, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
			if button.Clicked(gtx) {
				clicks++
			}
			return material.Button(theme, &button, "Do not tap").Layout(gtx)
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = driver.Close() })
		beforeFrame, beforeTime := driver.FrameNumber(), driver.Clock().Now()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		selector := script.Selector{Label: textPointer("Do not tap")}
		result, err := script.Run(ctx, driver, validScript(
			script.Step{Op: script.OpTap, Selector: &selector},
			script.Step{Op: script.OpAdvance, Duration: durationPointer(time.Second)},
		), script.Options{})
		assertStepError(t, result, err, script.CodeCancelled, context.Canceled)
		if clicks != 0 || driver.FrameNumber() != beforeFrame || !driver.Clock().Now().Equal(beforeTime) {
			t.Fatalf("cancelled run mutated state: clicks=%d frame=%d->%d time=%v->%v", clicks, beforeFrame, driver.FrameNumber(), beforeTime, driver.Clock().Now())
		}
	})

	t.Run("cancellation observed after action", func(t *testing.T) {
		var button widget.Clickable
		var cancel context.CancelFunc
		clicks := 0
		theme := material.NewTheme()
		driver, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
			if button.Clicked(gtx) {
				clicks++
				if cancel != nil {
					cancel()
				}
			}
			return material.Button(theme, &button, "Cancel during tap").Layout(gtx)
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = driver.Close() })
		ctx, stop := context.WithCancel(context.Background())
		cancel = stop
		selector := script.Selector{Label: textPointer("Cancel during tap")}
		beforeTime := driver.Clock().Now()
		result, err := script.Run(ctx, driver, validScript(
			script.Step{ID: "tap", Op: script.OpTap, Selector: &selector},
			script.Step{ID: "must-not-advance", Op: script.OpAdvance, Duration: durationPointer(time.Second)},
		), script.Options{})
		assertStepError(t, result, err, script.CodeCancelled, context.Canceled)
		if clicks != 1 || result.Failure.StepIndex != 0 || driver.Clock().Now().Sub(beforeTime) >= time.Second {
			t.Fatalf("post-action cancellation: clicks=%d result=%+v elapsed=%v", clicks, result, driver.Clock().Now().Sub(beforeTime))
		}
	})

	t.Run("cancellation after readiness prevents dispatch", func(t *testing.T) {
		var button widget.Clickable
		var cancel context.CancelFunc
		armed, clicks := false, 0
		theme := material.NewTheme()
		driver, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
			if armed && cancel != nil {
				cancel()
			}
			if button.Clicked(gtx) {
				clicks++
			}
			if !armed {
				gtx.Execute(op.InvalidateCmd{})
				gtx = gtx.Disabled()
			}
			return material.Button(theme, &button, "Ready while cancelling").Layout(gtx)
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = driver.Close() })
		ctx, stop := context.WithCancel(context.Background())
		cancel, armed = stop, true
		selector := script.Selector{Label: textPointer("Ready while cancelling")}
		result, err := script.Run(ctx, driver, validScript(script.Step{Op: script.OpTap, Selector: &selector}), script.Options{})
		assertStepError(t, result, err, script.CodeCancelled, context.Canceled)
		if clicks != 0 {
			t.Fatalf("action dispatched after readiness cancelled context: clicks=%d", clicks)
		}
	})

	t.Run("cancelled capture cannot publish success", func(t *testing.T) {
		directory := t.TempDir()
		writer, err := script.NewArtifactWriter(directory, script.ArtifactOptions{})
		if err != nil {
			t.Fatal(err)
		}
		driver, err := guitest.New(func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = driver.Close() })
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result, err := script.Run(ctx, driver, validScript(script.Step{Op: script.OpCapture, Name: "cancelled"}), script.Options{Artifacts: writer})
		assertStepError(t, result, err, script.CodeCancelled, context.Canceled)
		if _, statErr := os.Stat(filepath.Join(directory, "step-000-capture.json")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("cancelled success capture exists: %v", statErr)
		}
		data, readErr := os.ReadFile(filepath.Join(directory, "result.json"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		var published script.Result
		if json.Unmarshal(data, &published) != nil || published.Status != script.StatusFailed || published.Failure == nil || published.Failure.Code != script.CodeCancelled {
			t.Fatalf("published cancelled result = %s", data)
		}
	})

	t.Run("screenshot cancellation remains cancellation", func(t *testing.T) {
		writer, err := script.NewArtifactWriter(t.TempDir(), script.ArtifactOptions{Screenshot: func(context.Context, int) ([]byte, error) {
			return nil, context.Canceled
		}})
		if err != nil {
			t.Fatal(err)
		}
		driver, err := guitest.New(func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = driver.Close() })
		result, err := script.Run(context.Background(), driver, validScript(script.Step{
			Op: script.OpCapture, Name: "screenshot", Screenshot: script.ScreenshotRequired,
		}), script.Options{Artifacts: writer})
		assertStepError(t, result, err, script.CodeCancelled, context.Canceled)
		if !errors.Is(err, script.ErrArtifact) {
			t.Fatalf("screenshot cancellation lost artifact category: %v", err)
		}
	})
}

func TestRunProbeCancellationPreservesCauseAndCode(t *testing.T) {
	newDriver := func(t *testing.T) *guitest.Driver {
		t.Helper()
		driver, err := guitest.New(func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = driver.Close() })
		return driver
	}
	step := script.Step{Op: script.OpExpectProbe, Probe: "cancel", Args: json.RawMessage(`{}`), Equals: json.RawMessage(`true`)}

	t.Run("callback wraps cancellation", func(t *testing.T) {
		result, err := script.Run(context.Background(), newDriver(t), validScript(step), script.Options{Probes: map[string]script.ProbeFunc{
			"cancel": func(context.Context, json.RawMessage) (json.RawMessage, error) {
				return nil, fmt.Errorf("consumer stopped: %w", context.Canceled)
			},
		}})
		assertStepError(t, result, err, script.CodeCancelled, context.Canceled)
	})

	t.Run("context cancelled by callback", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		result, err := script.Run(ctx, newDriver(t), validScript(step), script.Options{Probes: map[string]script.ProbeFunc{
			"cancel": func(context.Context, json.RawMessage) (json.RawMessage, error) {
				cancel()
				return json.RawMessage(`true`), nil
			},
		}})
		assertStepError(t, result, err, script.CodeCancelled, context.Canceled)
	})
}

func TestRunArtifactFinalizationFailureReturnsCoherentFailure(t *testing.T) {
	directory := t.TempDir()
	writer, err := script.NewArtifactWriter(directory, script.ArtifactOptions{Limits: script.ArtifactLimits{MaxFileBytes: 1, MaxTotalBytes: 1}})
	if err != nil {
		t.Fatal(err)
	}
	driver, err := guitest.New(func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = driver.Close() })
	result, err := script.Run(context.Background(), driver, validScript(script.Step{ID: "settled", Op: script.OpSettle}), script.Options{Artifacts: writer})
	assertStepError(t, result, err, script.CodeArtifactError, script.ErrArtifact)
	if result.Status != script.StatusFailed || result.Steps != 1 || result.Failure.StepIndex != 0 || result.Failure.StepID != "settled" {
		t.Fatalf("artifact failure result = %+v", result)
	}
	if len(result.Artifacts) != 0 {
		t.Fatalf("failed publication advertised artifacts: %v", result.Artifacts)
	}
	entries, readErr := os.ReadDir(directory)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("failed publication left files: %v", entries)
	}
}

func TestRunScrollsVirtualizedTenThousandRows(t *testing.T) {
	var list widget.List
	list.Axis = layout.Vertical
	buttons := make(map[int]*widget.Clickable)
	theme := material.NewTheme()
	driver, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		return list.Layout(gtx, 10000, func(gtx layout.Context, index int) layout.Dimensions {
			if buttons[index] == nil {
				buttons[index] = new(widget.Clickable)
			}
			return material.Button(theme, buttons[index], fmt.Sprintf("Row %d", index)).Layout(gtx)
		})
	}, guitest.Size(300, 400))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = driver.Close() })
	rowZero := script.Selector{Label: textPointer("Row 0")}
	steps := []script.Step{
		{Op: script.OpScroll, Selector: &rowZero, Delta: &script.Point{Y: 600}},
		{Op: script.OpSettle},
		{Op: script.OpExpect, Selector: &rowZero, Expect: script.ExpectAbsent},
	}
	if _, err = script.Run(context.Background(), driver, validScript(steps...), script.Options{}); err != nil {
		t.Fatal(err)
	}
	if list.Position.First == 0 || len(driver.Nodes()) > 100 {
		t.Fatalf("virtualized scroll: first=%d nodes=%d", list.Position.First, len(driver.Nodes()))
	}
}

func TestRunStagesCaptureTraceResultAndFailureArtifacts(t *testing.T) {
	newDriver := func(t *testing.T) *guitest.Driver {
		t.Helper()
		driver, err := guitest.NewApp(func(guitest.Environment) (guitest.Harness, error) {
			return guitest.Harness{
				Layout: func(gtx layout.Context) layout.Dimensions {
					defer clip.Rect(image.Rect(0, 0, 80, 40)).Push(gtx.Ops).Pop()
					semantic.LabelOp("Ready").Add(gtx.Ops)
					return layout.Dimensions{Size: image.Pt(80, 40)}
				},
				Providers: map[string]diagnostic.Provider{"state": diagnostic.ProviderFunc(func(diagnostic.Request) diagnostic.Component {
					return diagnostic.Component{Kind: "state", State: map[string]any{"ready": true}}
				})},
			}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = driver.Close() })
		return driver
	}

	t.Run("success capture", func(t *testing.T) {
		directory := t.TempDir()
		writer, err := script.NewArtifactWriter(directory, script.ArtifactOptions{})
		if err != nil {
			t.Fatal(err)
		}
		document := validScript(script.Step{Op: script.OpCapture, Name: "checkpoint", Components: []string{"state"}, Screenshot: script.ScreenshotNever})
		result, err := script.Run(context.Background(), newDriver(t), document, script.Options{Artifacts: writer})
		if err != nil || result.Status != script.StatusPassed {
			t.Fatalf("Run = (%+v, %v)", result, err)
		}
		for _, name := range []string{"result.json", "trace.json", "step-000-capture.json"} {
			if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
				t.Fatalf("artifact %s: %v", name, err)
			}
		}
		if fmt.Sprint(result.Artifacts) != "[trace.json step-000-capture.json]" {
			t.Fatalf("result artifacts = %v", result.Artifacts)
		}
	})

	t.Run("failure capture", func(t *testing.T) {
		directory := t.TempDir()
		writer, err := script.NewArtifactWriter(directory, script.ArtifactOptions{})
		if err != nil {
			t.Fatal(err)
		}
		missing := script.Selector{Label: textPointer("Missing")}
		document := validScript(script.Step{Op: script.OpExpect, Selector: &missing, Expect: script.ExpectPresent, Timeout: durationPointer(time.Millisecond)})
		result, err := script.Run(context.Background(), newDriver(t), document, script.Options{Artifacts: writer})
		assertStepError(t, result, err, script.CodeAssertionFailed, script.ErrAssertion)
		for _, name := range []string{"result.json", "trace.json", "step-000-failure.json"} {
			if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
				t.Fatalf("artifact %s: %v", name, err)
			}
		}
	})
}

func TestRunBoundsProbeArgumentsAndResults(t *testing.T) {
	driver, err := guitest.New(func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = driver.Close() })
	step := script.Step{Op: script.OpExpectProbe, Probe: "bounded", Args: json.RawMessage(`{}`), Equals: json.RawMessage(`true`)}
	options := script.Options{Probes: map[string]script.ProbeFunc{
		"bounded": func(context.Context, json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`"` + strings.Repeat("x", script.DefaultMaxProbeBytes) + `"`), nil
		},
	}}
	result, err := script.Run(context.Background(), driver, validScript(step), options)
	assertStepError(t, result, err, script.CodeProbeError, script.ErrProbe)

	largeArgs := json.RawMessage(`{"value":"` + strings.Repeat("x", script.DefaultMaxProbeBytes) + `"}`)
	step.Args = largeArgs
	if _, err = script.Run(context.Background(), driver, validScript(step), options); !errors.Is(err, script.ErrInvalidScript) {
		t.Fatalf("large probe args error = %v", err)
	}
}

func assertStepError(t *testing.T, result script.Result, err error, code script.ErrorCode, target error) {
	t.Helper()
	var stepErr *script.StepError
	if !errors.As(err, &stepErr) || !errors.Is(err, target) {
		t.Fatalf("error = %T %v, want StepError wrapping %v", err, err, target)
	}
	if stepErr.Code != code || result.Failure == nil || result.Failure.Code != code {
		t.Fatalf("codes = step %q result %+v, want %q", stepErr.Code, result.Failure, code)
	}
}

func durationPointer(value time.Duration) *script.Duration {
	duration := script.Duration(value)
	return &duration
}
