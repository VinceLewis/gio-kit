// Package script defines and executes bounded, versioned JSON scenarios over
// Gio's in-process guitest driver.
package script

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

const (
	Version              = 1
	DefaultMaxInput      = 1 << 20
	DefaultMaxSteps      = 500
	DefaultSelectorDepth = 16
	DefaultSelectorTerms = 64
	DefaultMaxString     = 2 << 10
	DefaultStepTimeout   = 2 * time.Second
	MaximumStepTimeout   = 5 * time.Second
	DefaultJSONDepth     = 16
	DefaultJSONValues    = 4096
)

// ErrorCode is a stable, machine-readable failure category.
type ErrorCode string

const (
	CodeInvalidScript   ErrorCode = "invalid_script"
	CodeTimeout         ErrorCode = "timeout"
	CodeCancelled       ErrorCode = "cancelled"
	CodeNotFound        ErrorCode = "not_found"
	CodeAmbiguous       ErrorCode = "ambiguous"
	CodeNotInteractable ErrorCode = "not_interactable"
	CodeAssertionFailed ErrorCode = "assertion_failed"
	CodeProbeError      ErrorCode = "probe_error"
	CodeFrameLimit      ErrorCode = "frame_limit"
	CodeClosed          ErrorCode = "closed"
	CodeArtifactError   ErrorCode = "artifact_error"
	CodeInternal        ErrorCode = "internal"
)

var ErrInvalidScript = errors.New("guitest/script: invalid script")

// InvalidScriptError describes a pre-execution protocol or validation error.
// Its text contains no script values, which keeps it safe for bounded traces.
type InvalidScriptError struct {
	Path   string
	Reason string
}

func (e *InvalidScriptError) Error() string {
	if e == nil {
		return ErrInvalidScript.Error()
	}
	if e.Path == "" {
		return fmt.Sprintf("%s: %s", ErrInvalidScript, e.Reason)
	}
	return fmt.Sprintf("%s at %s: %s", ErrInvalidScript, e.Path, e.Reason)
}

func (*InvalidScriptError) Unwrap() error   { return ErrInvalidScript }
func (*InvalidScriptError) Code() ErrorCode { return CodeInvalidScript }

// Duration is a JSON string parsed with time.ParseDuration.
type Duration time.Duration

func (d Duration) Duration() time.Duration { return time.Duration(d) }
func (d Duration) String() string          { return time.Duration(d).String() }

func (d Duration) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

func (d *Duration) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.New("script.Duration: nil receiver")
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return errors.New("duration must be a string")
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return errors.New("duration is invalid")
	}
	*d = Duration(parsed)
	return nil
}

type Script struct {
	Version  int       `json:"version"`
	Name     string    `json:"name"`
	Fixture  string    `json:"fixture,omitempty"`
	Defaults *Defaults `json:"defaults,omitempty"`
	Steps    []Step    `json:"steps"`
}

type Defaults struct {
	Timeout *Duration `json:"timeout,omitempty"`
}

// Step is the closed union of all version-1 operations. Validate rejects every
// field that is not meaningful for the selected Op.
type Step struct {
	ID      string    `json:"id,omitempty"`
	Op      Operation `json:"op"`
	Timeout *Duration `json:"timeout,omitempty"`

	Selector *Selector `json:"selector,omitempty"`
	Duration *Duration `json:"duration,omitempty"`
	Position *Point    `json:"position,omitempty"`
	Delta    *Point    `json:"delta,omitempty"`
	Text     *string   `json:"text,omitempty"`
	Key      *Key      `json:"key,omitempty"`
	Range    *Range    `json:"range,omitempty"`
	Size     *Size     `json:"size,omitempty"`
	Metric   *Metric   `json:"metric,omitempty"`

	Expect Expectation `json:"expect,omitempty"`
	Count  *int        `json:"count,omitempty"`

	Component string          `json:"component,omitempty"`
	Pointer   *string         `json:"pointer,omitempty"`
	Equals    json.RawMessage `json:"equals,omitempty"`
	NotEquals json.RawMessage `json:"notEquals,omitempty"`
	Exists    *bool           `json:"exists,omitempty"`

	Probe string          `json:"probe,omitempty"`
	Args  json.RawMessage `json:"args,omitempty"`

	Name       string         `json:"name,omitempty"`
	Components []string       `json:"components,omitempty"`
	Screenshot ScreenshotMode `json:"screenshot,omitempty"`
}

type Operation string

const (
	OpTap            Operation = "tap"
	OpDoubleTap      Operation = "doubleTap"
	OpLongPress      Operation = "longPress"
	OpPress          Operation = "press"
	OpMove           Operation = "move"
	OpRelease        Operation = "release"
	OpCancelPointer  Operation = "cancelPointer"
	OpDrag           Operation = "drag"
	OpScroll         Operation = "scroll"
	OpFocus          Operation = "focus"
	OpType           Operation = "type"
	OpKey            Operation = "key"
	OpBack           Operation = "back"
	OpClearFocus     Operation = "clearFocus"
	OpSetSelection   Operation = "setSelection"
	OpSetComposition Operation = "setComposition"
	OpEdit           Operation = "edit"
	OpResize         Operation = "resize"
	OpAdvance        Operation = "advance"
	OpSettle         Operation = "settle"
	OpExpect         Operation = "expect"
	OpExpectSnapshot Operation = "expectSnapshot"
	OpExpectProbe    Operation = "expectProbe"
	OpCapture        Operation = "capture"
)

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Metric struct {
	PxPerDp float64 `json:"pxPerDp"`
	PxPerSp float64 `json:"pxPerSp"`
}

type Range struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type Key struct {
	Name      string        `json:"name"`
	Modifiers []KeyModifier `json:"modifiers,omitempty"`
}

type KeyModifier string

const (
	ModifierShift   KeyModifier = "shift"
	ModifierControl KeyModifier = "control"
	ModifierAlt     KeyModifier = "alt"
	ModifierSuper   KeyModifier = "super"
	ModifierCommand KeyModifier = "command"
)

// Selector is a recursive tagged union. Exactly one field must be populated.
// Pointer leaves distinguish an omitted operator from a valid false or empty
// literal.
type Selector struct {
	Label       *string             `json:"label,omitempty"`
	Description *string             `json:"description,omitempty"`
	Name        *string             `json:"name,omitempty"`
	Role        *Role               `json:"role,omitempty"`
	Text        *string             `json:"text,omitempty"`
	ID          *string             `json:"id,omitempty"`
	Enabled     *bool               `json:"enabled,omitempty"`
	Selected    *bool               `json:"selected,omitempty"`
	All         []Selector          `json:"all,omitempty"`
	Within      *WithinSelector     `json:"within,omitempty"`
	Containing  *ContainingSelector `json:"containing,omitempty"`
	Nth         *NthSelector        `json:"nth,omitempty"`
}

type WithinSelector struct {
	Target   Selector `json:"target"`
	Ancestor Selector `json:"ancestor"`
}

type ContainingSelector struct {
	Target     Selector `json:"target"`
	Descendant Selector `json:"descendant"`
}

type NthSelector struct {
	Selector Selector `json:"selector"`
	Index    int      `json:"index"`
}

type Role string

const (
	RoleUnknown  Role = "unknown"
	RoleButton   Role = "button"
	RoleCheckbox Role = "checkbox"
	RoleEditor   Role = "editor"
	RoleRadio    Role = "radio"
	RoleSwitch   Role = "switch"
)

type Expectation string

const (
	ExpectPresent                  Expectation = "present"
	ExpectAbsent                   Expectation = "absent"
	ExpectEnabled                  Expectation = "enabled"
	ExpectDisabled                 Expectation = "disabled"
	ExpectSelected                 Expectation = "selected"
	ExpectUnselected               Expectation = "unselected"
	ExpectCount                    Expectation = "count"
	ExpectViewportVisible          Expectation = "viewportVisible"
	ExpectViewportPartiallyVisible Expectation = "viewportPartiallyVisible"
	ExpectViewportClipped          Expectation = "viewportClipped"
)

type ScreenshotMode string

const (
	ScreenshotNever    ScreenshotMode = "never"
	ScreenshotOptional ScreenshotMode = "optional"
	ScreenshotRequired ScreenshotMode = "required"
)

// Result is the bounded final result written to result.json. Diagnostic detail
// belongs in separate redacted artifacts, not this structure.
type Result struct {
	Version   int      `json:"version"`
	Name      string   `json:"name"`
	Status    Status   `json:"status"`
	Steps     int      `json:"steps"`
	Failure   *Failure `json:"failure,omitempty"`
	Artifacts []string `json:"artifacts,omitempty"`
}

type Status string

const (
	StatusPassed Status = "passed"
	StatusFailed Status = "failed"
)

type Failure struct {
	StepIndex int       `json:"stepIndex"`
	StepID    string    `json:"stepId,omitempty"`
	Operation Operation `json:"operation"`
	Code      ErrorCode `json:"code"`
	Frame     uint64    `json:"frame"`
}

func rawPresent(value json.RawMessage) bool { return len(bytes.TrimSpace(value)) != 0 }

func quoteIndex(index int) string { return strconv.Itoa(index) }
