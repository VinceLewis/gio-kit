package guitest

import (
	"fmt"
	"strings"
)

// AccessibilityViolation describes one rule a captured node broke against
// the default baseline check (gio-json-test-upgrade-plan.md item 3).
type AccessibilityViolation struct {
	NodeID string
	Rule   string
	Detail string
}

func (v AccessibilityViolation) String() string {
	return fmt.Sprintf("%s: %s (%s)", v.NodeID, v.Rule, v.Detail)
}

// ErrAccessibilityViolations is returned by Capture/DumpJSON when the
// default accessibility check finds a violation and DumpOptions did not opt
// out. It is deliberately a hard failure, not a warning: per
// gio-json-test-upgrade-plan.md's consensus, the instruction this enables is
// "do not suppress it," not "remember to call it."
type ErrAccessibilityViolations struct {
	Violations []AccessibilityViolation
}

func (e *ErrAccessibilityViolations) Error() string {
	lines := make([]string, len(e.Violations))
	for i, v := range e.Violations {
		lines[i] = v.String()
	}
	return "guitest: accessibility violations:\n" + strings.Join(lines, "\n")
}

// minimumTouchTargetDp is the Material/WCAG-aligned minimum interactable
// dimension. It is enforced only when DumpOptions.MinTouchTargetDp is set
// explicitly (opt-in): gio-kit's current material.Button/IconButton default
// sizing is smaller than this in several existing widgets (see the filed
// gio-kit issue tracking that remediation), so making it a hard default
// today would fail nearly every existing test for a pre-existing production
// sizing gap this change does not fix. The accessible-name rule below has no
// such blocker and is a hard default.
const minimumTouchTargetDp = 48

// CheckAccessibility runs the baseline checks against already-captured
// nodes: every interactable node must have a non-empty accessible name
// somewhere in its own ancestor or descendant chain. gio-kit's widgets name
// a composite control from either direction: material.Button wraps a
// labeled text leaf in unlabeled outer Clickable/click-target ancestors, so
// the name must be found on a descendant; accessibility.Group instead names
// an outer scope around an unlabeled input leaf (e.g. grid's selection
// checkbox), so the name must be found on an ancestor. Gio's own widget.List
// scrollbar track/thumb register a ClickGesture with no name in either
// direction and cannot yet be distinguished structurally from a genuine
// app-level defect; this is why DumpOptions.CheckAccessibility defaults to
// off instead of the plan's target opt-out default (see
// https://github.com/VinceLewis/gio-kit/issues/2). A real accessibility
// tree resolves the name the same either way, so requiring every individual
// node to carry its own label would be a false positive, not a real gap.
// When minTouchTargetDp > 0, every interactable node must also be at least
// that large in both dimensions. pxPerDp <= 0 is treated as 1 (already the
// Driver's own fallback for an invalid metric).
func CheckAccessibility(nodes []FrameNode, pxPerDp float32, minTouchTargetDp float32) []AccessibilityViolation {
	if pxPerDp <= 0 {
		pxPerDp = 1
	}
	parentOf := make(map[string]string, len(nodes))
	selfNamed := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		parentOf[n.ID] = n.Parent
		selfNamed[n.ID] = n.Label != "" || n.Description != ""
	}
	hasName := make(map[string]bool, len(nodes))
	// Upward: a named node also names every ancestor (covers material.Button's
	// labeled leaf inside unlabeled click-target ancestors).
	for _, n := range nodes {
		if !selfNamed[n.ID] {
			continue
		}
		for id := n.ID; id != "" && !hasName[id]; id = parentOf[id] {
			hasName[id] = true
		}
	}
	// Downward: a named node also names every descendant (covers
	// accessibility.Group naming an outer scope around an unlabeled input
	// leaf). Assumes nodes are in pre-order (a parent precedes its
	// children), which matches the paint-order traversal Capture builds them
	// in.
	inherited := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		inherited[n.ID] = selfNamed[n.ID] || inherited[n.Parent]
		if inherited[n.ID] {
			hasName[n.ID] = true
		}
	}
	var violations []AccessibilityViolation
	for _, n := range nodes {
		if n.Interactable == "no" || len(n.Actions) == 0 {
			continue
		}
		if !hasName[n.ID] {
			violations = append(violations, AccessibilityViolation{NodeID: n.ID, Rule: "accessible-name", Detail: "interactable node has no label or description in its own subtree"})
		}
		if minTouchTargetDp <= 0 {
			continue
		}
		widthDp, heightDp := float32(n.Bounds.Width)/pxPerDp, float32(n.Bounds.Height)/pxPerDp
		if widthDp < minTouchTargetDp || heightDp < minTouchTargetDp {
			violations = append(violations, AccessibilityViolation{NodeID: n.ID, Rule: "touch-target-size",
				Detail: fmt.Sprintf("%.0fx%.0fdp, want >= %.0fdp", widthDp, heightDp, minTouchTargetDp)})
		}
	}
	return violations
}
