package script

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// TraceVersion identifies the stable trace artifact format.
const TraceVersion = 1

// ErrInvalidTrace is returned when a trace violates the v1 trace contract.
var ErrInvalidTrace = errors.New("script: invalid trace")

// Artifact identifies a deterministic artifact without exposing user data in
// its name. Unredacted is true for image captures, whose pixels cannot be
// generically redacted by the runner.
type Artifact struct {
	Name       string `json:"name"`
	Unredacted bool   `json:"unredacted,omitempty"`
}

// TraceStep is the bounded, privacy-safe record for one executed step.
type TraceStep struct {
	StepIndex     int        `json:"stepIndex"`
	StepID        string     `json:"stepId,omitempty"`
	Operation     Operation  `json:"operation"`
	Status        Status     `json:"status"`
	Attempts      int        `json:"attempts"`
	FrameBefore   uint64     `json:"frameBefore"`
	FrameAfter    uint64     `json:"frameAfter"`
	VirtualBefore Duration   `json:"virtualBefore"`
	VirtualAfter  Duration   `json:"virtualAfter"`
	Code          ErrorCode  `json:"code,omitempty"`
	Artifacts     []Artifact `json:"artifacts,omitempty"`
}

// Trace is the stable, bounded v1 execution trace.
type Trace struct {
	Version   int         `json:"version"`
	Status    Status      `json:"status"`
	Steps     []TraceStep `json:"steps"`
	Artifacts []Artifact  `json:"artifacts,omitempty"`
}

// TraceBuilder incrementally builds a trace. It copies every supplied step so
// callers may safely reuse their slices after Add returns.
type TraceBuilder struct {
	mu       sync.Mutex
	steps    []TraceStep
	finished bool
}

// NewTraceBuilder returns an empty v1 trace builder.
func NewTraceBuilder() *TraceBuilder { return &TraceBuilder{} }

// Add appends the next sequential step.
func (b *TraceBuilder) Add(step TraceStep) error {
	if b == nil {
		return traceError("builder", "is nil")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.finished {
		return traceError("builder", "is already finished")
	}
	if len(b.steps) >= DefaultMaxSteps {
		return traceError("steps", "exceeds %d", DefaultMaxSteps)
	}
	if step.StepIndex != len(b.steps) {
		return traceError("stepIndex", "got %d, want %d", step.StepIndex, len(b.steps))
	}
	if err := validateTraceStep(step); err != nil {
		return err
	}
	b.steps = append(b.steps, cloneTraceStep(step))
	return nil
}

// Finish completes the trace and returns an independent copy.
func (b *TraceBuilder) Finish(status Status) (Trace, error) {
	if b == nil {
		return Trace{}, traceError("builder", "is nil")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.finished {
		return Trace{}, traceError("builder", "is already finished")
	}
	b.finished = true
	trace := Trace{
		Version: TraceVersion,
		Status:  status,
		Steps:   cloneTraceSteps(b.steps),
	}
	trace.Artifacts = collectTraceArtifacts(trace.Steps)
	if err := ValidateTrace(trace); err != nil {
		return Trace{}, err
	}
	return trace, nil
}

// ValidateTrace validates the bounded v1 trace contract.
func ValidateTrace(trace Trace) error {
	if trace.Version != TraceVersion {
		return traceError("version", "got %d, want %d", trace.Version, TraceVersion)
	}
	if trace.Status != StatusPassed && trace.Status != StatusFailed {
		return traceError("status", "must be %q or %q", StatusPassed, StatusFailed)
	}
	if trace.Steps == nil {
		return traceError("steps", "must be present")
	}
	if len(trace.Steps) > DefaultMaxSteps {
		return traceError("steps", "exceeds %d", DefaultMaxSteps)
	}
	if len(trace.Artifacts) > MaximumArtifactFiles {
		return traceError("artifacts", "exceeds %d", MaximumArtifactFiles)
	}
	for i, step := range trace.Steps {
		if step.StepIndex != i {
			return traceError(fmt.Sprintf("steps[%d].stepIndex", i), "got %d, want %d", step.StepIndex, i)
		}
		if err := validateTraceStep(step); err != nil {
			return traceError(fmt.Sprintf("steps[%d]", i), "%v", err)
		}
		if trace.Status == StatusPassed && step.Status == StatusFailed {
			return traceError(fmt.Sprintf("steps[%d].status", i), "failed step in passed trace")
		}
		if i > 0 {
			previous := trace.Steps[i-1]
			if previous.Status == StatusFailed {
				return traceError(fmt.Sprintf("steps[%d]", i), "appears after a failed step")
			}
			if step.FrameBefore < previous.FrameAfter {
				return traceError(fmt.Sprintf("steps[%d].frameBefore", i), "precedes the previous frameAfter")
			}
			if step.VirtualBefore.Duration() < previous.VirtualAfter.Duration() {
				return traceError(fmt.Sprintf("steps[%d].virtualBefore", i), "precedes the previous virtualAfter")
			}
		}
	}
	if trace.Status == StatusFailed {
		if len(trace.Steps) == 0 || trace.Steps[len(trace.Steps)-1].Status != StatusFailed {
			return traceError("status", "failed trace must end with a failed step")
		}
	}
	want := collectTraceArtifacts(trace.Steps)
	if !equalArtifacts(trace.Artifacts, want) {
		return traceError("artifacts", "must be the sorted unique union of step artifacts")
	}
	return nil
}

// ReadTrace strictly decodes and validates one bounded trace JSON document.
func ReadTrace(r io.Reader, maxBytes int64) (Trace, error) {
	if r == nil {
		return Trace{}, traceError("input", "reader is nil")
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxInput
	}
	limited := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return Trace{}, fmt.Errorf("%w: read: %v", ErrInvalidTrace, err)
	}
	if int64(len(data)) > maxBytes {
		return Trace{}, traceError("input", "exceeds %d bytes", maxBytes)
	}
	if !utf8.Valid(data) {
		return Trace{}, traceError("input", "is not valid UTF-8")
	}
	if err := rejectDuplicateFields(data); err != nil {
		return Trace{}, traceError("input", "contains duplicate fields")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var trace Trace
	if err := dec.Decode(&trace); err != nil {
		return Trace{}, fmt.Errorf("%w: decode: %v", ErrInvalidTrace, err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Trace{}, traceError("input", "contains a trailing JSON value")
		}
		return Trace{}, fmt.Errorf("%w: trailing input: %v", ErrInvalidTrace, err)
	}
	if err := ValidateTrace(trace); err != nil {
		return Trace{}, err
	}
	return trace, nil
}

func validateTraceStep(step TraceStep) error {
	if step.StepIndex < 0 || step.StepIndex >= DefaultMaxSteps {
		return traceError("stepIndex", "must be between 0 and %d", DefaultMaxSteps-1)
	}
	if err := validString("stepID", step.StepID, false); err != nil {
		return traceError("stepID", "must be valid UTF-8 and at most %d bytes", DefaultMaxString)
	}
	if !validOperation(step.Operation) {
		return traceError("operation", "is unsupported")
	}
	if step.Status != StatusPassed && step.Status != StatusFailed {
		return traceError("status", "must be %q or %q", StatusPassed, StatusFailed)
	}
	if step.Attempts <= 0 {
		return traceError("attempts", "must be positive")
	}
	if step.FrameAfter < step.FrameBefore {
		return traceError("frameAfter", "must not precede frameBefore")
	}
	if step.VirtualBefore.Duration() < 0 || step.VirtualAfter.Duration() < 0 {
		return traceError("virtual time", "must not be negative")
	}
	if step.VirtualAfter.Duration() < step.VirtualBefore.Duration() {
		return traceError("virtualAfter", "must not precede virtualBefore")
	}
	if step.Status == StatusPassed && step.Code != "" {
		return traceError("code", "must be absent for a passed step")
	}
	if step.Status == StatusFailed && !validErrorCode(step.Code) {
		return traceError("code", "must be a stable v1 error code for a failed step")
	}
	seen := make(map[string]struct{}, len(step.Artifacts))
	hasCaptureJSON, hasCapturePNG, hasFailure := false, false, false
	for i, artifact := range step.Artifacts {
		if err := validateArtifact(artifact); err != nil {
			return traceError(fmt.Sprintf("artifacts[%d]", i), "%v", err)
		}
		artifactStep, ok := artifactStepIndex(artifact.Name)
		if !ok {
			return traceError(fmt.Sprintf("artifacts[%d].name", i), "is not a step artifact")
		}
		if artifactStep != step.StepIndex {
			return traceError(fmt.Sprintf("artifacts[%d].name", i), "belongs to step %d", artifactStep)
		}
		if _, duplicate := seen[artifact.Name]; duplicate {
			return traceError(fmt.Sprintf("artifacts[%d].name", i), "is duplicated")
		}
		if i > 0 && step.Artifacts[i-1].Name >= artifact.Name {
			return traceError(fmt.Sprintf("artifacts[%d].name", i), "is not in deterministic order")
		}
		switch {
		case strings.HasSuffix(artifact.Name, "-failure.json"):
			hasFailure = true
		case strings.HasSuffix(artifact.Name, "-capture.json"):
			hasCaptureJSON = true
		case strings.HasSuffix(artifact.Name, "-capture-unredacted.png"):
			hasCapturePNG = true
		}
		seen[artifact.Name] = struct{}{}
	}
	switch {
	case step.Status == StatusFailed:
		if hasCaptureJSON || hasCapturePNG {
			return traceError("artifacts", "failed step may contain only its failure capture")
		}
	case step.Operation == OpCapture:
		if hasFailure || !hasCaptureJSON {
			return traceError("artifacts", "passed capture step requires its JSON capture")
		}
	default:
		if hasFailure || hasCaptureJSON || hasCapturePNG {
			return traceError("artifacts", "passed non-capture step must not contain artifacts")
		}
	}
	return nil
}

func validateArtifact(artifact Artifact) error {
	if !validArtifactName(artifact.Name) {
		return traceError("name", "is not a deterministic v1 artifact name")
	}
	if artifact.Unredacted != strings.HasSuffix(artifact.Name, "-capture-unredacted.png") {
		return traceError("unredacted", "must mark only PNG captures")
	}
	return nil
}

func validArtifactName(name string) bool {
	if name == "result.json" || name == "trace.json" {
		return true
	}
	_, ok := artifactStepIndex(name)
	return ok
}

func artifactStepIndex(name string) (int, bool) {
	if !strings.HasPrefix(name, "step-") || len(name) < len("step-000-x") {
		return 0, false
	}
	rest := name[len("step-"):]
	dash := strings.IndexByte(rest, '-')
	if dash != 3 {
		return 0, false
	}
	digits := rest[:dash]
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return 0, false
		}
	}
	index, err := strconv.Atoi(digits)
	if err != nil || index < 0 || index >= DefaultMaxSteps {
		return 0, false
	}
	suffix := rest[dash+1:]
	switch suffix {
	case "failure.json", "capture.json", "capture-unredacted.png":
		return index, true
	default:
		return 0, false
	}
}

func validErrorCode(code ErrorCode) bool {
	switch code {
	case CodeInvalidScript, CodeTimeout, CodeCancelled, CodeNotFound,
		CodeAmbiguous, CodeNotInteractable, CodeAssertionFailed, CodeProbeError,
		CodeFrameLimit, CodeClosed, CodeArtifactError, CodeInternal:
		return true
	default:
		return false
	}
}

func validOperation(operation Operation) bool {
	for _, candidate := range operationNames() {
		if string(operation) == candidate {
			return true
		}
	}
	return false
}

func traceError(field, format string, args ...any) error {
	return fmt.Errorf("%w: %s: %s", ErrInvalidTrace, field, fmt.Sprintf(format, args...))
}

func collectTraceArtifacts(steps []TraceStep) []Artifact {
	byName := make(map[string]Artifact)
	for _, step := range steps {
		for _, artifact := range step.Artifacts {
			byName[artifact.Name] = artifact
		}
	}
	artifacts := make([]Artifact, 0, len(byName))
	for _, artifact := range byName {
		artifacts = append(artifacts, artifact)
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Name < artifacts[j].Name })
	return artifacts
}

func equalArtifacts(got, want []Artifact) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func cloneTraceStep(step TraceStep) TraceStep {
	step.Artifacts = append([]Artifact(nil), step.Artifacts...)
	return step
}

func cloneTraceSteps(steps []TraceStep) []TraceStep {
	cloned := make([]TraceStep, len(steps))
	for i, step := range steps {
		cloned[i] = cloneTraceStep(step)
	}
	return cloned
}

func cloneTrace(trace Trace) Trace {
	trace.Steps = cloneTraceSteps(trace.Steps)
	trace.Artifacts = append([]Artifact(nil), trace.Artifacts...)
	return trace
}
