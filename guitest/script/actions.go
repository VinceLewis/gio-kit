package script

import (
	"context"
	"errors"
	"fmt"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/unit"
	"github.com/VinceLewis/gio-kit/guitest"
)

func (r *runner) execute(ctx context.Context, index int, attempts *int) ([]Artifact, error) {
	step := &r.script.Steps[index]
	selector := r.prepared[index].selector
	if err := ctx.Err(); err != nil {
		*attempts = 1
		return nil, err
	}
	switch step.Op {
	case OpExpect:
		return nil, r.expectSemantic(ctx, step, selector, attempts)
	case OpExpectSnapshot:
		return nil, r.expectSnapshot(ctx, index, attempts)
	case OpExpectProbe:
		return nil, r.expectProbe(ctx, index, attempts)
	case OpCapture:
		*attempts = 1
		capture, err := r.captureJSON(step.Components)
		if err != nil {
			return nil, fmt.Errorf("%w: capture JSON: %w", ErrArtifact, err)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		mode := step.Screenshot
		if mode == "" {
			mode = ScreenshotNever
		}
		artifacts, err := r.options.Artifacts.StageCapture(ctx, index, capture, mode)
		if err != nil {
			return nil, fmt.Errorf("%w: stage capture: %w", ErrArtifact, err)
		}
		return artifacts, nil
	case OpSettle:
		*attempts = 1
		return nil, r.driver.Settle(ctx)
	case OpAdvance:
		*attempts = 1
		return nil, r.driver.Advance(step.Duration.Duration())
	}

	capability, needsReadiness := actionCapability(step.Op)
	if needsReadiness {
		if err := r.waitReadiness(ctx, selector, capability, attempts); err != nil {
			return nil, err
		}
	} else {
		*attempts = 1
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Exactly one high-level dispatch occurs after the read-only wait. Composite
	// driver operations such as DoubleTap and Drag retain their documented Gio
	// event sequences but are never retried by the runner.
	var err error
	switch step.Op {
	case OpTap:
		err = r.driver.Tap(selector)
	case OpDoubleTap:
		err = r.driver.DoubleTap(selector)
	case OpLongPress:
		err = r.driver.LongPress(selector, step.Duration.Duration())
	case OpPress:
		err = r.driver.Press(selector)
		if err == nil {
			r.held = true
		}
	case OpMove:
		err = r.driver.Move(f32.Pt(float32(step.Position.X), float32(step.Position.Y)))
	case OpRelease:
		err = r.driver.Release()
		if err == nil {
			r.held = false
		}
	case OpCancelPointer:
		err = r.driver.CancelPointer()
		if err == nil {
			r.held = false
		}
	case OpDrag:
		err = r.driver.Drag(selector, f32.Pt(float32(step.Delta.X), float32(step.Delta.Y)), step.Duration.Duration())
	case OpScroll:
		err = r.driver.Scroll(selector, f32.Pt(float32(step.Delta.X), float32(step.Delta.Y)))
	case OpFocus:
		err = r.driver.Focus(selector)
	case OpType:
		err = r.driver.Type(selector, *step.Text)
	case OpKey:
		err = r.driver.Key(key.Name(step.Key.Name), keyModifiers(step.Key.Modifiers))
	case OpBack:
		err = r.driver.Back()
	case OpClearFocus:
		err = r.driver.ClearFocus()
	case OpSetSelection:
		err = r.driver.SetSelection(key.Range{Start: step.Range.Start, End: step.Range.End})
	case OpSetComposition:
		err = r.driver.SetComposition(key.Range{Start: step.Range.Start, End: step.Range.End})
	case OpEdit:
		err = r.driver.Edit(key.Range{Start: step.Range.Start, End: step.Range.End}, *step.Text)
	case OpResize:
		err = r.driver.Resize(step.Size.Width, step.Size.Height, unit.Metric{
			PxPerDp: float32(step.Metric.PxPerDp), PxPerSp: float32(step.Metric.PxPerSp),
		})
	default:
		err = fmt.Errorf("%w: unsupported operation", ErrInternal)
	}
	return nil, err
}

func actionCapability(operation Operation) (guitest.TargetCapability, bool) {
	switch operation {
	case OpTap, OpDoubleTap, OpLongPress, OpPress, OpDrag:
		return guitest.TargetTap, true
	case OpFocus, OpType:
		return guitest.TargetEdit, true
	case OpScroll:
		return guitest.TargetScroll, true
	default:
		return 0, false
	}
}

func (r *runner) waitReadiness(ctx context.Context, selector guitest.Selector, capability guitest.TargetCapability, attempts *int) error {
	var last error
	check := func() bool {
		(*attempts)++
		last = r.driver.CheckTarget(selector, capability)
		if last == nil {
			return true
		}
		return terminalReadiness(last)
	}
	if check() {
		return last
	}
	err := r.driver.WaitFor(ctx, check)
	if err != nil {
		if last != nil {
			return errors.Join(err, last)
		}
		return err
	}
	return last
}

func terminalReadiness(err error) bool {
	return errors.Is(err, guitest.ErrAmbiguous) ||
		errors.Is(err, guitest.ErrTargetCapabilityMismatch) ||
		errors.Is(err, guitest.ErrTargetOffViewport) ||
		errors.Is(err, guitest.ErrClosed)
}

func keyModifiers(values []KeyModifier) key.Modifiers {
	var modifiers key.Modifiers
	for _, value := range values {
		switch value {
		case ModifierShift:
			modifiers |= key.ModShift
		case ModifierControl:
			modifiers |= key.ModCtrl
		case ModifierAlt:
			modifiers |= key.ModAlt
		case ModifierSuper:
			modifiers |= key.ModSuper
		case ModifierCommand:
			modifiers |= key.ModCommand
		}
	}
	return modifiers
}

func (r *runner) captureJSON(components []string) ([]byte, error) {
	options := r.options.DumpOptions
	if len(components) == 1 {
		options.Component = components[0]
	} else {
		options.Component = ""
	}
	dump, err := r.driver.Capture(options)
	if err != nil {
		return nil, err
	}
	if len(components) > 0 {
		keep := make(map[string]bool, len(components))
		for _, component := range components {
			if _, ok := dump.Components[component]; !ok {
				return nil, fmt.Errorf("%w: component", guitest.ErrNotFound)
			}
			keep[component] = true
		}
		for component := range dump.Components {
			if !keep[component] {
				delete(dump.Components, component)
			}
		}
	}
	return marshalCapture(dump)
}
