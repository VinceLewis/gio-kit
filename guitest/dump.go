package guitest

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"sort"
	"strings"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/semantic"
	"github.com/VinceLewis/gio-kit/diagnostic"
)

const SchemaVersion = 2

var ErrOutputLimit = errors.New("guitest: diagnostic output exceeds byte limit")

// DumpOptions can tighten hard limits but cannot disable redaction. Redact
// supplies additional string redaction after safe defaults. Component restricts
// output to one provider; Request selects a bounded logical item sample.
type DumpOptions struct {
	MaxBytes, MaxNodes, MaxDepth, MaxValues, MaxStringBytes int
	Component                                               string
	Request                                                 diagnostic.Request
	Redact                                                  func(path, value string) string
}

func (o DumpOptions) bounded() DumpOptions {
	bound := func(n, fallback, maximum int) int {
		if n <= 0 {
			return fallback
		}
		if n > maximum {
			return maximum
		}
		return n
	}
	o.MaxBytes = bound(o.MaxBytes, 256<<10, 4<<20)
	o.MaxNodes = bound(o.MaxNodes, 512, 4096)
	o.MaxDepth = bound(o.MaxDepth, 16, 64)
	o.MaxValues = bound(o.MaxValues, 4096, 32768)
	o.MaxStringBytes = bound(o.MaxStringBytes, 2048, 16384)
	o.Request = o.Request.Bounded()
	return o
}

type Bounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

func bounds(r image.Rectangle) Bounds { return Bounds{r.Min.X, r.Min.Y, r.Dx(), r.Dy()} }

type FrameNode struct {
	ID                   string   `json:"id"`
	Parent               string   `json:"parent,omitempty"`
	Role                 string   `json:"role"`
	Label                string   `json:"label"`
	Description          string   `json:"description"`
	Bounds               Bounds   `json:"bounds"`
	ViewportIntersection Bounds   `json:"viewportIntersection"`
	ClipBounds           *Bounds  `json:"clipBounds"`
	Visibility           string   `json:"visibility"`
	ViewportVisibility   string   `json:"viewportVisibility"`
	Coverage             string   `json:"coverage"`
	Enabled              bool     `json:"enabled"`
	Selected             bool     `json:"selected"`
	Interactable         string   `json:"interactable"`
	Actions              []string `json:"actions"`
	// Valid, when known, mirrors a form field's own validation state (see
	// form.Field.Error via Form.DebugSnapshot). It is nil for nodes that are
	// not matched to a component field that reports validity today.
	Valid *bool `json:"valid,omitempty"`
	// ErrorMessage carries a matched component's existing validation error
	// text (currently only form fields, via form.Form.DebugSnapshot's
	// per-field "validation" state). It is empty when no owning component
	// surfaces an error for this node.
	ErrorMessage string `json:"errorMessage,omitempty"`
	// DisabledReason carries a matched component's existing disabled-reason
	// text (shell.Control/shell.Item.DisabledReason and
	// picker.Option.DisabledReason, both already surfaced by their
	// DebugSnapshot implementations). grid and dialog do not yet surface a
	// per-node disabled reason from their DebugSnapshot, so this remains
	// empty for their nodes until those packages add one.
	DisabledReason string `json:"disabledReason,omitempty"`
}

type Dump struct {
	Version     int                 `json:"version"`
	Frame       uint64              `json:"frame"`
	Time        time.Time           `json:"time"`
	Viewport    Bounds              `json:"viewport"`
	Metrics     map[string]float32  `json:"metrics"`
	Locale      map[string]string   `json:"locale"`
	Orientation string              `json:"orientation"`
	Focus       string              `json:"focus"`
	Idle        *bool               `json:"idle"`
	// FocusOrder lists node IDs in traversal order. Gio's input.Router
	// (and the wider gioui.org/io/input package) exposes no explicit
	// tab/focus order query, so this is approximated from paint order —
	// the same order as Nodes — as a documented first-pass approximation.
	// It is NOT a verified platform tab order; only a real device/TalkBack
	// session can prove actual keyboard or accessibility traversal order.
	FocusOrder    []string                        `json:"focusOrder,omitempty"`
	PendingTimers int                             `json:"pendingTimers"`
	Nodes         []FrameNode                     `json:"nodes"`
	Components    map[string]diagnostic.Component `json:"components"`
	Truncated     bool                            `json:"truncated"`
}

// Capture reads the last frame without advancing time or settling the app.
// Unknown clip, coverage and focus facts are explicit. IDs without bindings
// are deterministic tree indices, not identities stable across different trees.
func (d *Driver) Capture(options DumpOptions) (Dump, error) {
	if d.closed.Load() {
		return Dump{}, ErrClosed
	}
	o := options.bounded()
	s := sanitizer{options: o}
	result := Dump{Version: SchemaVersion, Frame: d.frame, Time: d.clock.Now(),
		Viewport:    bounds(image.Rectangle{Max: d.config.size}),
		Metrics:     map[string]float32{"pxPerDp": d.config.metric.PxPerDp, "pxPerSp": d.config.metric.PxPerSp},
		Locale:      map[string]string{"language": string(d.config.locale.Language), "direction": fmt.Sprint(d.config.locale.Direction)},
		Orientation: "portrait", Focus: "unknown", PendingTimers: d.clock.Pending(),
		Nodes: []FrameNode{}, Components: map[string]diagnostic.Component{}}
	if d.config.size.X > d.config.size.Y {
		result.Orientation = "landscape"
	}
	if d.app.Idle != nil {
		idle := d.app.Idle()
		result.Idle = &idle
	}
	names := make([]string, 0, len(d.app.Providers))
	for name := range d.app.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	if o.Component != "" {
		if _, ok := d.app.Providers[o.Component]; !ok {
			return Dump{}, fmt.Errorf("%w: component", ErrNotFound)
		}
	}
	sensitiveLabels := make(map[string]bool)
	// hints are matched against a node's own pre-redaction label, the same
	// best-effort join `sensitiveLabels` above already relies on: nothing in
	// today's Provider interface (diagnostic.Provider) or in the semantic
	// tree (input.SemanticDesc) links a node back to the provider name that
	// rendered it, so gio-kit cannot yet scope these hints to "this node
	// belongs to component X" more precisely than "this label was reported
	// by a component of kind K". A future provider API could close this gap;
	// until then, treat these as approximate, not authoritative.
	roleHints, errorHints, disabledReasonHints := map[string]string{}, map[string]string{}, map[string]string{}
	validHints := map[string]bool{}
	kindsPresent := map[string]bool{}
	gridCardMode := false
	addString := func(m map[string]string, key, value string) {
		if key != "" && value != "" {
			m[key] = value
		}
	}
	for _, name := range names {
		provider := d.app.Providers[name]
		if provider == nil {
			continue
		}
		// Query every provider's sensitivity metadata even when output is scoped.
		component := provider.DebugSnapshot(o.Request)
		for _, label := range component.SensitiveLabels {
			if label != "" {
				sensitiveLabels[label] = true
			}
		}
		kindsPresent[component.Kind] = true
		// Best-effort role/reason/validity hints, derived only from state
		// each component already exposes through its existing DebugSnapshot
		// (no new provider API is introduced here). gioui.org/io/semantic
		// has no distinct ClassOp for column headers, cards, pickers, tabs,
		// dialogs or drawers (semantic.ClassOp is generic: button, checkbox,
		// editor, radio, switch), so those roles cannot come from
		// n.Desc.Class alone; they are inferred below from each component's
		// own reported state instead.
		switch component.Kind {
		case "form":
			// form.Form.DebugSnapshot already reports each field's label,
			// its validation error (nil when valid) and whether the field
			// itself is private; skip private fields so an error message
			// or validity bit can never leak text a redacted node hides.
			if fields, ok := component.State["fields"].([]any); ok {
				for _, entry := range fields {
					field, ok := entry.(map[string]any)
					if !ok {
						continue
					}
					label, _ := field["label"].(string)
					if label == "" || sensitiveLabels[label] {
						continue
					}
					valid := true
					if err, ok := field["validation"].(error); ok && err != nil {
						addString(errorHints, label, err.Error())
						valid = false
					}
					validHints[label] = valid
				}
			}
		case "shell":
			// shell.Widget.DebugSnapshot already reports each control's
			// label and DisabledReason; drawerControls specifically
			// identifies drawer items, which is the only sub-role gio-kit's
			// shell component makes distinguishable today. Shell has no
			// widget backing a "tab" role yet (its wide-mode navigation
			// rail reuses the same drawer-item rendering), so "tab" cannot
			// be populated from this component.
			if controls, ok := component.State["topBar"].([]any); ok {
				for _, entry := range controls {
					if control, ok := entry.(map[string]any); ok {
						label, _ := control["label"].(string)
						reason, _ := control["disabledReason"].(string)
						if label != "" && !sensitiveLabels[label] {
							addString(disabledReasonHints, label, reason)
						}
					}
				}
			}
			if controls, ok := component.State["drawerControls"].([]any); ok {
				for _, entry := range controls {
					if control, ok := entry.(map[string]any); ok {
						label, _ := control["label"].(string)
						reason, _ := control["disabledReason"].(string)
						if label != "" && !sensitiveLabels[label] {
							roleHints[label] = "drawer"
							addString(disabledReasonHints, label, reason)
						}
					}
				}
			}
		case "picker":
			// picker.Widget.DebugSnapshot already reports each option's
			// label and DisabledReason. Every option is tagged role
			// "picker"; gio-kit has no separate node for the picker as a
			// whole (its search field keeps role "editor").
			if options, ok := component.State["options"].([]any); ok {
				for _, entry := range options {
					if option, ok := entry.(map[string]any); ok {
						label, _ := option["label"].(string)
						reason, _ := option["disabledReason"].(string)
						if label != "" && !sensitiveLabels[label] {
							roleHints[label] = "picker"
							addString(disabledReasonHints, label, reason)
						}
					}
				}
			}
		case "dialog":
			// dialog.Confirm.DebugSnapshot reports "title", which is also
			// the accessible label dialog.Confirm's accessibility.Group
			// applies to its whole content (see accessibility.Group.Layout,
			// which sets no semantic.ClassOp, so roleName would otherwise
			// leave it "unknown").
			if title, ok := component.State["title"].(string); ok && title != "" {
				roleHints[title] = "dialog"
			}
		case "grid":
			// grid.Widget.DebugSnapshot reports resolvedViewMode ("cards" or
			// "table"), the actual presentation Layout chose last frame, so
			// a card row can be told apart from a table row without
			// guessing from label text.
			if mode, ok := component.State["resolvedViewMode"].(string); ok && mode == "cards" {
				gridCardMode = true
			}
		}
		if o.Component != "" && name != o.Component {
			continue
		}
		clean := s.value("components."+name, component.State, 0)
		state, ok := clean.(map[string]any)
		if !ok {
			state = map[string]any{"unavailable": "truncated"}
		}
		result.Components[s.text("componentName", name)] = diagnostic.Component{Kind: s.text("componentKind", component.Kind), State: state}
	}
	ids := make([]string, len(d.nodes))
	boundCounts := map[string]int{}
	for _, n := range d.nodes {
		if len(n.ids) > 0 {
			boundCounts[n.ids[0]]++
		}
	}
	for i, n := range d.nodes {
		ids[i] = fmt.Sprintf("node/%d", i)
		if len(n.ids) > 0 {
			ids[i] = "test/" + n.ids[0]
			if boundCounts[n.ids[0]] > 1 {
				ids[i] += fmt.Sprintf("/node/%d", i)
			}
		}
	}
	for _, n := range d.nodes {
		depth, sensitive := 0, false
		for p := n.Index; p >= 0; p = d.nodes[p].Parent {
			depth++
			label := d.nodes[p].Desc.Label
			sensitive = sensitive || sensitiveLabels[label] || sensitiveText(label)
		}
		if depth > o.MaxDepth || len(result.Nodes) >= o.MaxNodes {
			s.truncated = true
			continue
		}
		intersection := n.Desc.Bounds.Intersect(image.Rectangle{Max: d.config.size})
		node := FrameNode{ID: ids[n.Index], Role: roleName(n.Desc.Class), Label: n.Desc.Label,
			Description: n.Desc.Description, Bounds: bounds(n.Desc.Bounds), ViewportIntersection: bounds(intersection),
			Visibility: "unknown", ViewportVisibility: "visible", Coverage: "unknown", Enabled: Enabled(true)(n),
			Selected: n.Desc.Selected, Interactable: "unknown", Actions: []string{}}
		if n.Parent >= 0 {
			node.Parent = ids[n.Parent]
		}
		if intersection.Empty() {
			node.Visibility, node.ViewportVisibility = "clipped", "clipped"
		} else if intersection != n.Desc.Bounds {
			node.ViewportVisibility = "partially_visible"
		}
		if n.Desc.Gestures&input.ClickGesture != 0 {
			node.Actions = append(node.Actions, "tap", "long-press")
		}
		if n.Desc.Gestures&input.ScrollGesture != 0 {
			node.Actions = append(node.Actions, "scroll")
		}
		if n.Desc.Class == semantic.Editor {
			node.Actions = append(node.Actions, "focus", "type")
		}
		if !node.Enabled || intersection.Empty() || len(node.Actions) == 0 {
			node.Interactable = "no"
		}
		if !sensitive {
			// Grid renders both column-sort headers/menus and row/card
			// entries as plain semantic.Button nodes with a distinctive
			// label prefix (see grid/widget.go, grid/card_layout.go,
			// grid/row_layout.go). "Sort by " labels a table column header
			// in table mode but a card-mode sort *menu trigger* in card
			// mode (not a column header), and "Open " labels a row-open
			// target in both modes; gridCardMode (from the grid
			// component's resolvedViewMode, see above) disambiguates both.
			if kindsPresent["grid"] {
				switch {
				case !gridCardMode && strings.HasPrefix(n.Desc.Label, "Sort by "):
					node.Role = "columnHeader"
				case gridCardMode && strings.HasPrefix(n.Desc.Label, "Open "):
					node.Role = "card"
				}
			}
			if hint, ok := roleHints[n.Desc.Label]; ok {
				node.Role = hint
			}
			if valid, ok := validHints[n.Desc.Label]; ok {
				v := valid
				node.Valid = &v
			}
			node.ErrorMessage = errorHints[n.Desc.Label]
			node.DisabledReason = disabledReasonHints[n.Desc.Label]
		}
		if sensitive {
			node.Label, node.Description = diagnostic.Redacted, diagnostic.Redacted
		}
		node.Label = s.text("nodes."+node.ID+".label", node.Label)
		node.Description = s.text("nodes."+node.ID+".description", node.Description)
		node.ErrorMessage = s.text("nodes."+node.ID+".errorMessage", node.ErrorMessage)
		node.DisabledReason = s.text("nodes."+node.ID+".disabledReason", node.DisabledReason)
		result.Nodes = append(result.Nodes, node)
	}
	result.Truncated = s.truncated
	// FocusOrder is approximated from paint order (see the field's doc
	// comment): Nodes above are already appended in d.nodes iteration order.
	result.FocusOrder = make([]string, len(result.Nodes))
	for i, node := range result.Nodes {
		result.FocusOrder[i] = node.ID
	}
	computeCoverage(result.Nodes)
	// Apply the byte cap to Capture as well as DumpJSON. No partial dump escapes.
	data, err := json.Marshal(result)
	if err != nil {
		return Dump{}, err
	}
	if len(data) > o.MaxBytes {
		return Dump{}, ErrOutputLimit
	}
	return result, nil
}

// computeCoverage marks each node "covered" when a later-painted node (Gio
// paints in document order, so later index means painted on top) that is
// neither its ancestor nor its descendant fully contains its
// ViewportIntersection, "partially_covered" for a non-empty but partial
// overlap, and leaves "unknown" nodes (empty ViewportIntersection, already
// clipped) untouched. It mutates nodes in place.
func computeCoverage(nodes []FrameNode) {
	parent := make(map[string]string, len(nodes))
	for _, n := range nodes {
		parent[n.ID] = n.Parent
	}
	related := func(a, b string) bool {
		for p := parent[a]; p != ""; p = parent[p] {
			if p == b {
				return true
			}
		}
		for p := parent[b]; p != ""; p = parent[p] {
			if p == a {
				return true
			}
		}
		return false
	}
	rect := func(n FrameNode) image.Rectangle {
		return image.Rect(n.ViewportIntersection.X, n.ViewportIntersection.Y,
			n.ViewportIntersection.X+n.ViewportIntersection.Width,
			n.ViewportIntersection.Y+n.ViewportIntersection.Height)
	}
	for i := range nodes {
		r := rect(nodes[i])
		if r.Empty() {
			continue
		}
		nodes[i].Coverage = "uncovered"
		for j := i + 1; j < len(nodes); j++ {
			if related(nodes[i].ID, nodes[j].ID) {
				continue
			}
			other := rect(nodes[j])
			if other.Empty() {
				continue
			}
			overlap := r.Intersect(other)
			if overlap.Empty() {
				continue
			}
			if overlap == r {
				nodes[i].Coverage = "covered"
				break
			}
			nodes[i].Coverage = "partially_covered"
		}
	}
}

func roleName(class semantic.ClassOp) string {
	switch class {
	case semantic.Button:
		return "button"
	case semantic.CheckBox:
		return "checkbox"
	case semantic.Editor:
		return "editor"
	case semantic.RadioButton:
		return "radio"
	case semantic.Switch:
		return "switch"
	default:
		return "unknown"
	}
}

// DumpJSON writes one complete bounded JSON document. Capture errors leave
// the writer untouched; writer failures and short writes are returned.
func (d *Driver) DumpJSON(writer io.Writer, options DumpOptions) error {
	if writer == nil {
		return errors.New("guitest: nil dump writer")
	}
	dump, err := d.Capture(options)
	if err != nil {
		return err
	}
	data, err := json.Marshal(dump)
	if err != nil {
		return err
	}
	n, err := writer.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return err
}
