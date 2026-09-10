package guitest

import (
	"errors"
	"fmt"
	"image"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/semantic"
)

var (
	ErrNotFound        = errors.New("guitest: selector did not match")
	ErrAmbiguous       = errors.New("guitest: selector is ambiguous")
	ErrNotInteractable = errors.New("guitest: target is not interactable")
)

// Node is a copy of public Gio semantics. Index and Parent refer to this frame
// only; Parent is -1 for the root. Bounds do not prove exact clipping/occlusion.
// The native semantic ID is intentionally not exposed as a persistent test ID.
type Node struct {
	Index  int
	Parent int
	Desc   input.SemanticDesc
	id     input.SemanticID
	tree   *frameTree
	ids    []string
}

type frameTree struct{ nodes []Node }

type Selector func(Node) bool

// Label matches a node's literal Gio label. A button's label may be a child of
// its input handler; Tap resolves that child to the nearest actionable ancestor.
func Label(value string) Selector { return func(n Node) bool { return n.Desc.Label == value } }
func Description(value string) Selector {
	return func(n Node) bool { return n.Desc.Description == value }
}
func Role(class semantic.ClassOp) Selector { return func(n Node) bool { return n.Desc.Class == class } }

// Name matches a literal label, or the nearest named ancestor of an unnamed
// node. Combine with Role for composed controls, such as a named editor group.
func Name(value string) Selector {
	return func(n Node) bool {
		for {
			if n.Desc.Label != "" {
				return n.Desc.Label == value
			}
			if n.Parent < 0 || n.tree == nil || n.Parent >= len(n.tree.nodes) {
				return false
			}
			n = n.tree.nodes[n.Parent]
		}
	}
}

func Enabled(value bool) Selector {
	return func(n Node) bool {
		for {
			if n.Desc.Disabled {
				return !value
			}
			if n.Parent < 0 || n.tree == nil || n.Parent >= len(n.tree.nodes) {
				return value
			}
			n = n.tree.nodes[n.Parent]
		}
	}
}
func Selected(value bool) Selector { return func(n Node) bool { return n.Desc.Selected == value } }
func Text(value string) Selector {
	return func(n Node) bool {
		return strings.Contains(n.Desc.Label, value) || strings.Contains(n.Desc.Description, value)
	}
}

// Within matches descendants, excluding the ancestor itself. The ancestry is
// always evaluated against the current frame, including after virtualization.
func Within(selector, ancestor Selector) Selector {
	return func(n Node) bool {
		if n.tree == nil || selector == nil || ancestor == nil || !selector(n) {
			return false
		}
		for p := n.Parent; p >= 0 && p < len(n.tree.nodes); p = n.tree.nodes[p].Parent {
			if ancestor(n.tree.nodes[p]) {
				return true
			}
		}
		return false
	}
}

// Containing matches nodes with a matching descendant, excluding themselves.
func Containing(selector, descendant Selector) Selector {
	return func(n Node) bool {
		if n.tree == nil || selector == nil || descendant == nil || !selector(n) {
			return false
		}
		for _, child := range n.tree.nodes {
			if !descendant(child) {
				continue
			}
			for p := child.Parent; p >= 0; p = n.tree.nodes[p].Parent {
				if p == n.Index {
					return true
				}
			}
		}
		return false
	}
}

// Nth explicitly selects a zero-based occurrence in the current semantic tree.
// Prefer names or scoped selectors when order can change.
func Nth(selector Selector, occurrence int) Selector {
	return func(n Node) bool {
		if n.tree == nil || selector == nil || occurrence < 0 {
			return false
		}
		count := 0
		for _, candidate := range n.tree.nodes {
			if selector(candidate) {
				if count == occurrence {
					return n.Index == candidate.Index
				}
				count++
			}
		}
		return false
	}
}

// ID matches a test-only binding supplied with BindID. IDs never enter Gio's
// accessibility text or depend on native semantic IDs.
func ID(id string) Selector {
	return func(n Node) bool {
		for _, bound := range n.ids {
			if bound == id {
				return true
			}
		}
		return false
	}
}
func All(selectors ...Selector) Selector {
	return func(n Node) bool {
		for _, selector := range selectors {
			if selector == nil || !selector(n) {
				return false
			}
		}
		return true
	}
}

func (d *Driver) captureNodes() {
	semantics := d.router.AppendSemantics(nil)
	indices := make(map[input.SemanticID]int, len(semantics))
	d.nodes = make([]Node, len(semantics))
	for i, n := range semantics {
		indices[n.ID] = i
	}
	for i, n := range semantics {
		parent := -1
		if n.ParentID != 0 {
			parent = indices[n.ParentID]
		}
		d.nodes[i] = Node{Index: i, Parent: parent, Desc: n.Desc, id: n.ID}
	}
	tree := &frameTree{nodes: d.nodes}
	for i := range d.nodes {
		d.nodes[i].tree = tree
	}
	// Evaluate bindings before assigning any IDs: bindings cannot depend on
	// other bindings, and their declaration order does not affect resolution.
	ids := make([][]string, len(d.nodes))
	for _, binding := range d.config.bindings {
		for i, node := range d.nodes {
			if binding.selector(node) {
				ids[i] = append(ids[i], binding.id)
			}
		}
	}
	for i := range d.nodes {
		d.nodes[i].ids = ids[i]
	}
}

// Nodes returns a frame-local copy. Diagnostic serialization and redaction
// belong to the versioned dump API; raw semantics can contain application data.
func (d *Driver) Nodes() []Node { return append([]Node(nil), d.nodes...) }

func (d *Driver) Find(selector Selector) (Node, error) {
	if d.closed.Load() {
		return Node{}, ErrClosed
	}
	if selector == nil {
		return Node{}, errors.New("guitest: nil selector")
	}
	var found Node
	count := 0
	for _, n := range d.nodes {
		if selector(n) {
			count++
			found = n
		}
	}
	if count == 0 {
		return Node{}, ErrNotFound
	}
	if count > 1 {
		return Node{}, fmt.Errorf("%w: matched %d nodes", ErrAmbiguous, count)
	}
	return found, nil
}

func (d *Driver) target(selector Selector, editorOnly bool) (f32.Point, error) {
	return d.targetKind(selector, editorOnly, false)
}

func (d *Driver) targetKind(selector Selector, editorOnly, scrollOnly bool) (f32.Point, error) {
	n, err := d.Find(selector)
	if err != nil {
		return f32.Point{}, err
	}
	for {
		if n.Desc.Disabled {
			return f32.Point{}, ErrNotInteractable
		}
		if scrollOnly && n.Desc.Gestures&input.ScrollGesture != 0 || !scrollOnly && (n.Desc.Class == semantic.Editor || (!editorOnly && n.Desc.Gestures&input.ClickGesture != 0)) {
			break
		}
		if n.Parent < 0 {
			return f32.Point{}, ErrNotInteractable
		}
		n = d.nodes[n.Parent]
	}
	region := n.Desc.Bounds.Intersect(image.Rectangle{Max: d.config.size})
	for parent := n.Parent; parent >= 0; parent = d.nodes[parent].Parent {
		if d.nodes[parent].Desc.Disabled {
			return f32.Point{}, ErrNotInteractable
		}
		region = region.Intersect(d.nodes[parent].Desc.Bounds)
	}
	if region.Empty() {
		return f32.Point{}, ErrNotInteractable
	}
	// Test a small bounded set of positions through Gio's public hit test.
	// This is a targeting check, not a claim of exact pixel visibility.
	for _, fraction := range [][2]int{{2, 2}, {1, 1}, {3, 1}, {1, 3}, {3, 3}} {
		p := image.Pt(region.Min.X+(region.Dx()-1)*fraction[0]/4, region.Min.Y+(region.Dy()-1)*fraction[1]/4)
		position := f32.Pt(float32(p.X)+0.5, float32(p.Y)+0.5)
		id, ok := d.router.SemanticAt(position)
		if !ok {
			continue
		}
		for _, hit := range d.nodes {
			if hit.id != id {
				continue
			}
			// Gio's material editor replays its hint as a sibling semantic
			// label above the input node. Such paint-only semantics are not
			// input blockers; SemanticAt alone cannot identify the event sink.
			if hit.Desc.Class == semantic.Unknown && hit.Desc.Gestures == 0 {
				return position, nil
			}
			// Composed controls can register a sibling pass-through gesture
			// area, for example the form editor's long-press handler. Gio's
			// semantics do not expose PassOp. Permit the coincident unnamed
			// area, then let real event routing decide which widgets respond.
			if hit.Desc.Class == semantic.Unknown && hit.Parent == n.Parent && hit.Desc.Bounds == n.Desc.Bounds && hit.Desc.Label == "" && hit.Desc.Description == "" {
				return position, nil
			}
			for index := hit.Index; index >= 0; index = d.nodes[index].Parent {
				if index == n.Index {
					return position, nil
				}
			}
		}
	}
	return f32.Point{}, ErrNotInteractable
}

// Tap sends touch press/release events with separate frames. Actions process
// input but do not wait for external work: use Settle or WaitFor explicitly.
func (d *Driver) Tap(selector Selector) error {
	p, err := d.target(selector, false)
	if err != nil {
		return err
	}
	return d.tapAt(p)
}

func (d *Driver) tapAt(p f32.Point) error {
	if err := d.PressAt(p); err != nil {
		return err
	}
	if err := d.clock.advance(50 * time.Millisecond); err != nil {
		return err
	}
	return d.Release()
}

// Type focuses an editor by pointer input and replaces its current selection
// using a Gio EditEvent. Offsets are rune-based, as required by Gio. This tests
// the editor input path; it does not test Android's software keyboard/IME.
func (d *Driver) Type(selector Selector, value string) error {
	p, err := d.target(selector, true)
	if err != nil {
		return err
	}
	if err := d.tapAt(p); err != nil {
		return err
	}
	if err := d.Queue(key.EditEvent{Range: d.router.EditorState().Selection.Range, Text: value}); err != nil {
		return err
	}
	return d.Frame()
}

// Key sends a key press/release through the same filters as real keyboard input.
// Window-provided fallback focus traversal is outside the core driver's scope.
func (d *Driver) Key(name key.Name, modifiers key.Modifiers) error {
	if err := d.Queue(key.Event{Name: name, Modifiers: modifiers, State: key.Press}); err != nil {
		return err
	}
	if err := d.Queue(key.Event{Name: name, Modifiers: modifiers, State: key.Release}); err != nil {
		return err
	}
	return d.Frame()
}

func (d *Driver) Back() error { return d.Key(key.NameBack, 0) }
