package script

import (
	"context"
	"errors"
	"fmt"
	"image"

	"github.com/VinceLewis/gio-kit/guitest"
)

func (r *runner) expectSemantic(ctx context.Context, step *Step, selector guitest.Selector, attempts *int) error {
	var terminal error
	check := func() bool {
		(*attempts)++
		passed, err := r.semanticAssertion(step, selector)
		terminal = err
		return passed || err != nil
	}
	if check() {
		return terminal
	}
	if err := r.driver.WaitFor(ctx, check); err != nil {
		return r.assertionWaitError(err)
	}
	if terminal != nil {
		return terminal
	}
	return nil
}

func (r *runner) semanticAssertion(step *Step, selector guitest.Selector) (bool, error) {
	matches := matchingNodes(r.driver, selector)
	if step.Expect == ExpectCount {
		return len(matches) == *step.Count, nil
	}
	if step.Expect == ExpectAbsent {
		return len(matches) == 0, nil
	}
	if len(matches) > 1 {
		return false, fmt.Errorf("%w: assertion selector matched %d nodes", guitest.ErrAmbiguous, len(matches))
	}
	if len(matches) == 0 {
		return false, nil
	}
	node := matches[0]
	switch step.Expect {
	case ExpectPresent:
		return true, nil
	case ExpectEnabled:
		return guitest.Enabled(true)(node), nil
	case ExpectDisabled:
		return guitest.Enabled(false)(node), nil
	case ExpectSelected:
		return guitest.Selected(true)(node), nil
	case ExpectUnselected:
		return guitest.Selected(false)(node), nil
	case ExpectViewportVisible, ExpectViewportPartiallyVisible, ExpectViewportClipped:
		dump, err := r.driver.Capture(guitest.DumpOptions{MaxNodes: 1, MaxDepth: 1, MaxValues: 1})
		if err != nil {
			return false, err
		}
		viewport := image.Rect(0, 0, dump.Viewport.Width, dump.Viewport.Height)
		intersection := node.Desc.Bounds.Intersect(viewport)
		switch step.Expect {
		case ExpectViewportVisible:
			return !intersection.Empty() && intersection == node.Desc.Bounds, nil
		case ExpectViewportPartiallyVisible:
			return !intersection.Empty() && intersection != node.Desc.Bounds, nil
		default:
			return intersection.Empty(), nil
		}
	default:
		return false, fmt.Errorf("%w: unsupported semantic assertion", ErrInternal)
	}
}

func (r *runner) expectSnapshot(ctx context.Context, index int, attempts *int) error {
	step := &r.script.Steps[index]
	var terminal error
	check := func() bool {
		(*attempts)++
		dump, err := r.driver.Capture(withComponent(r.options.DumpOptions, step.Component))
		if err != nil {
			if errors.Is(err, guitest.ErrNotFound) {
				return false
			}
			terminal = err
			return true
		}
		component, ok := dump.Components[step.Component]
		if !ok {
			return false
		}
		value, found, err := ResolveJSONPointer(component.State, *step.Pointer)
		if err != nil {
			terminal = err
			return true
		}
		switch {
		case step.Exists != nil:
			return found == *step.Exists
		case rawPresent(step.Equals):
			return found && equalJSON(value, r.prepared[index].expected)
		case rawPresent(step.NotEquals):
			return found && !equalJSON(value, r.prepared[index].expected)
		default:
			terminal = fmt.Errorf("%w: missing snapshot comparison", ErrInternal)
			return true
		}
	}
	if check() {
		return terminal
	}
	if err := r.driver.WaitFor(ctx, check); err != nil {
		return r.assertionWaitError(err)
	}
	return terminal
}

func matchingNodes(driver *guitest.Driver, selector guitest.Selector) []guitest.Node {
	nodes := driver.Nodes()
	matches := make([]guitest.Node, 0, 1)
	for _, node := range nodes {
		if selector(node) {
			matches = append(matches, node)
		}
	}
	return matches
}

func withComponent(options guitest.DumpOptions, component string) guitest.DumpOptions {
	options.Component = component
	return options
}

func (r *runner) assertionWaitError(err error) error {
	if parentErr := r.ctx.Err(); parentErr != nil {
		return parentErr
	}
	if errors.Is(err, guitest.ErrFrameLimit) || errors.Is(err, guitest.ErrClosed) {
		return err
	}
	return errors.Join(ErrAssertion, err)
}
