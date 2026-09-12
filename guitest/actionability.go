package guitest

import (
	"errors"
	"fmt"
	"image"

	"gioui.org/io/input"
	"gioui.org/io/semantic"
)

// TargetCapability identifies the input behavior CheckTarget must be able to
// route to a semantic target.
type TargetCapability uint8

const (
	TargetTap TargetCapability = iota + 1
	TargetEdit
	TargetScroll
)

var (
	// ErrTargetDisabled means the target or an ancestor used for input is
	// explicitly disabled.
	ErrTargetDisabled = errors.New("guitest: target is disabled")
	// ErrTargetOffViewport means the actionable semantic bounds have no
	// intersection with the viewport. It does not make a stronger claim about
	// ancestor clipping, coverage, or occlusion.
	ErrTargetOffViewport = errors.New("guitest: target is off viewport")
	// ErrTargetCapabilityMismatch means neither the selected semantic node nor
	// its ancestors expose the requested input capability.
	ErrTargetCapabilityMismatch = errors.New("guitest: target capability mismatch")
)

// CheckTarget checks whether selector currently resolves to exactly one target
// capable of receiving the requested kind of input. It reads the last frame
// only: it does not lay out a frame, change focus, queue input, or mutate widget
// state.
//
// Readiness is delegated to the same target resolution used by Driver actions.
// ErrNotFound, ErrAmbiguous, and ErrClosed are returned unchanged. Disabled,
// off-viewport, and capability-mismatch results also wrap ErrNotInteractable.
// Other failures to prove a usable hit point remain ErrNotInteractable because
// Gio's public semantics do not establish occlusion or effective clipping.
func (d *Driver) CheckTarget(selector Selector, capability TargetCapability) error {
	editorOnly, scrollOnly, ok := targetCapabilityFlags(capability)
	if !ok {
		return fmt.Errorf("guitest: invalid target capability %d", capability)
	}
	var matches []Node
	checkedSelector := selector
	if selector != nil {
		checkedSelector = func(n Node) bool {
			if !selector(n) {
				return false
			}
			matches = append(matches, n)
			return true
		}
	}
	_, err := d.targetKind(checkedSelector, editorOnly, scrollOnly)
	if err == nil || !errors.Is(err, ErrNotInteractable) {
		return err
	}
	if len(matches) == 1 {
		if reason := d.targetFailureReason(matches[0], editorOnly, scrollOnly); reason != nil {
			return fmt.Errorf("%w: %w", ErrNotInteractable, reason)
		}
	}
	return err
}

func targetCapabilityFlags(capability TargetCapability) (editorOnly, scrollOnly, ok bool) {
	switch capability {
	case TargetTap:
		return false, false, true
	case TargetEdit:
		return true, false, true
	case TargetScroll:
		return false, true, true
	default:
		return false, false, false
	}
}

// targetFailureReason classifies only facts available directly from public
// semantics. targetKind remains authoritative for whether a usable point can be
// resolved.
func (d *Driver) targetFailureReason(n Node, editorOnly, scrollOnly bool) error {
	for {
		if n.Desc.Disabled {
			return ErrTargetDisabled
		}
		if targetSupports(n, editorOnly, scrollOnly) {
			break
		}
		if n.Parent < 0 {
			return ErrTargetCapabilityMismatch
		}
		n = d.nodes[n.Parent]
	}

	viewportIntersection := n.Desc.Bounds.Intersect(image.Rectangle{Max: d.config.size})
	region := viewportIntersection
	for parent := n.Parent; parent >= 0; parent = d.nodes[parent].Parent {
		if d.nodes[parent].Desc.Disabled {
			return ErrTargetDisabled
		}
		region = region.Intersect(d.nodes[parent].Desc.Bounds)
	}
	if region.Empty() && viewportIntersection.Empty() {
		return ErrTargetOffViewport
	}
	return nil
}

func targetSupports(n Node, editorOnly, scrollOnly bool) bool {
	if scrollOnly {
		return n.Desc.Gestures&input.ScrollGesture != 0
	}
	return n.Desc.Class == semantic.Editor || !editorOnly && n.Desc.Gestures&input.ClickGesture != 0
}
