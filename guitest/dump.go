package guitest

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"sort"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/semantic"
	"github.com/VinceLewis/gio-kit/diagnostic"
)

const SchemaVersion = 1

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
}

type Dump struct {
	Version       int                             `json:"version"`
	Frame         uint64                          `json:"frame"`
	Time          time.Time                       `json:"time"`
	Viewport      Bounds                          `json:"viewport"`
	Metrics       map[string]float32              `json:"metrics"`
	Locale        map[string]string               `json:"locale"`
	Orientation   string                          `json:"orientation"`
	Focus         string                          `json:"focus"`
	Idle          *bool                           `json:"idle"`
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
		if sensitive {
			node.Label, node.Description = diagnostic.Redacted, diagnostic.Redacted
		}
		node.Label = s.text("nodes."+node.ID+".label", node.Label)
		node.Description = s.text("nodes."+node.ID+".description", node.Description)
		result.Nodes = append(result.Nodes, node)
	}
	result.Truncated = s.truncated
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
