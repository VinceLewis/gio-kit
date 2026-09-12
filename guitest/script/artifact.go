package script

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	DefaultMaxArtifactFileBytes  = 4 << 20
	DefaultMaxArtifactTotalBytes = 16 << 20
	MaximumArtifactFiles         = DefaultMaxSteps*2 + 3
	MaximumScreenshotPixels      = 64 << 20
)

var (
	ErrArtifact              = errors.New("guitest/script: artifact error")
	ErrScreenshotUnavailable = errors.New("guitest/script: screenshot unavailable")
)

// ArtifactLimits bounds every individual file and the complete published set.
type ArtifactLimits struct {
	MaxFileBytes  int
	MaxTotalBytes int
}

// ScreenshotFunc returns PNG bytes for the requested step. Implementations
// should return ErrScreenshotUnavailable when no backend is available.
type ScreenshotFunc func(context.Context, int) ([]byte, error)

// ArtifactOptions configures artifact limits and optional image capture.
type ArtifactOptions struct {
	Limits     ArtifactLimits
	Screenshot ScreenshotFunc
}

// ArtifactError is a stable artifact failure. Its message never includes a
// filesystem path, application error, or artifact content.
type ArtifactError struct {
	Operation string
	Err       error
}

var (
	_ error                         = (*ArtifactError)(nil)
	_ interface{ Code() ErrorCode } = (*ArtifactError)(nil)
)

func (e *ArtifactError) Error() string {
	if e == nil || e.Operation == "" {
		return ErrArtifact.Error()
	}
	return ErrArtifact.Error() + " during " + e.Operation
}
func (e *ArtifactError) Unwrap() error { return e.Err }
func (*ArtifactError) Code() ErrorCode { return CodeArtifactError }
func (e *ArtifactError) Is(target error) bool {
	return target == ErrArtifact || errors.Is(e.Err, target)
}

type artifactPayload struct {
	artifact Artifact
	data     []byte
}

// ArtifactWriter stages a complete result set in memory and publishes it as a
// transaction. It is safe for concurrent staging, but Commit and Abort are
// terminal operations.
type ArtifactWriter struct {
	mu         sync.Mutex
	directory  string
	limits     ArtifactLimits
	screenshot ScreenshotFunc
	fs         artifactFS
	payloads   map[string]artifactPayload
	trace      *Trace
	result     *Result
	finished   bool
}

// NewArtifactWriter validates or creates a private, non-symlink output
// directory. An existing directory is accepted only when it contains private,
// regular files with deterministic artifact names. Staging publishes no files;
// Commit documents its publication and failure semantics.
func NewArtifactWriter(directory string, options ArtifactOptions) (*ArtifactWriter, error) {
	return newArtifactWriter(directory, options, osArtifactFS{})
}

func newArtifactWriter(directory string, options ArtifactOptions, fileSystem artifactFS) (*ArtifactWriter, error) {
	if fileSystem == nil {
		return nil, artifactError("initialize", errors.New("nil filesystem"))
	}
	limits, err := normalizeArtifactLimits(options.Limits)
	if err != nil {
		return nil, artifactError("initialize", err)
	}
	if strings.TrimSpace(directory) == "" {
		return nil, artifactError("initialize", errors.New("empty directory"))
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, artifactError("initialize", err)
	}
	if err := ensurePrivateDirectory(fileSystem, absolute); err != nil {
		return nil, artifactError("initialize", err)
	}
	return &ArtifactWriter{
		directory:  absolute,
		limits:     limits,
		screenshot: options.Screenshot,
		fs:         fileSystem,
		payloads:   make(map[string]artifactPayload),
	}, nil
}

// StageFailure stages a redacted failure capture for a failed step.
func (w *ArtifactWriter) StageFailure(stepIndex int, capture []byte) (Artifact, error) {
	name, err := stepArtifactName(stepIndex, "failure.json")
	if err != nil {
		return Artifact{}, err
	}
	canonical, err := canonicalArtifactJSON(capture, w.fileLimit())
	if err != nil {
		return Artifact{}, artifactError("stage failure", err)
	}
	artifact := Artifact{Name: name}
	if err := w.stage([]artifactPayload{{artifact: artifact, data: canonical}}); err != nil {
		return Artifact{}, err
	}
	return artifact, nil
}

// StageCapture stages a numbered redacted JSON capture and, according to mode,
// an explicitly unredacted numbered PNG capture.
func (w *ArtifactWriter) StageCapture(ctx context.Context, stepIndex int, capture []byte, mode ScreenshotMode) ([]Artifact, error) {
	jsonName, err := stepArtifactName(stepIndex, "capture.json")
	if err != nil {
		return nil, err
	}
	canonical, err := canonicalArtifactJSON(capture, w.fileLimit())
	if err != nil {
		return nil, artifactError("stage capture", err)
	}
	jsonArtifact := Artifact{Name: jsonName}
	payloads := []artifactPayload{{artifact: jsonArtifact, data: canonical}}

	switch mode {
	case "", ScreenshotNever:
	case ScreenshotOptional, ScreenshotRequired:
		if ctx == nil {
			return nil, artifactError("screenshot", errors.New("nil context"))
		}
		if err := ctx.Err(); err != nil {
			return nil, artifactError("screenshot", err)
		}
		if w == nil || w.screenshot == nil {
			if mode == ScreenshotRequired {
				return nil, artifactError("screenshot", ErrScreenshotUnavailable)
			}
			break
		}
		image, screenshotErr := w.screenshot(ctx, stepIndex)
		if screenshotErr != nil {
			if mode == ScreenshotOptional && errors.Is(screenshotErr, ErrScreenshotUnavailable) {
				break
			}
			return nil, artifactError("screenshot", screenshotErr)
		}
		if err := ctx.Err(); err != nil {
			return nil, artifactError("screenshot", err)
		}
		if err := validatePNG(image, w.fileLimit()); err != nil {
			return nil, artifactError("screenshot", err)
		}
		pngName, err := stepArtifactName(stepIndex, "capture-unredacted.png")
		if err != nil {
			return nil, err
		}
		payloads = append(payloads, artifactPayload{
			artifact: Artifact{Name: pngName, Unredacted: true},
			data:     append([]byte(nil), image...),
		})
	default:
		return nil, artifactError("stage capture", errors.New("invalid screenshot mode"))
	}
	sort.Slice(payloads, func(i, j int) bool { return payloads[i].artifact.Name < payloads[j].artifact.Name })

	if err := w.stage(payloads); err != nil {
		return nil, err
	}
	artifacts := make([]Artifact, len(payloads))
	for i, payload := range payloads {
		artifacts[i] = payload.artifact
	}
	return artifacts, nil
}

// StageTrace copies and validates the final trace.
func (w *ArtifactWriter) StageTrace(trace Trace) error {
	if err := ValidateTrace(trace); err != nil {
		return artifactError("stage trace", err)
	}
	if w == nil {
		return artifactError("stage trace", errors.New("nil writer"))
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.canStageLocked(); err != nil {
		return err
	}
	copy := cloneTrace(trace)
	w.trace = &copy
	return nil
}

// StageResult copies and validates the final result. Artifact names supplied by
// the caller are intentionally discarded and derived from staged payloads at
// Commit, preventing user-derived artifact names.
func (w *ArtifactWriter) StageResult(result Result) error {
	if err := validateArtifactResult(result); err != nil {
		return artifactError("stage result", err)
	}
	if w == nil {
		return artifactError("stage result", errors.New("nil writer"))
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.canStageLocked(); err != nil {
		return err
	}
	copy := result
	copy.Artifacts = nil
	if result.Failure != nil {
		failure := *result.Failure
		copy.Failure = &failure
	}
	w.result = &copy
	return nil
}

// Artifacts returns copied, sorted metadata for currently staged diagnostic
// artifacts. result.json and trace.json appear only after Commit composition.
func (w *ArtifactWriter) Artifacts() []Artifact {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.artifactsLocked()
}

// Commit publishes one complete artifact generation. Portable filesystems do
// not provide an atomic multi-file replacement, so result.json is treated as
// the commit marker: the previous marker is backed up before any visible file
// changes and the new marker is published last. Returned publication failures
// before that marker restore the previous generation. A cleanup failure after
// marker publication is still returned, but the complete new generation and
// its private backup remnant are left in place for explicit recovery.
//
// Process interruption can still leave marker-less partial files and private
// .guitest-stage/.guitest-backup files. Power-loss durability is filesystem
// dependent: Go has no portable directory-entry sync primitive, so rename
// ordering is not a durable crash transaction and a marker may survive without
// every companion file. A later writer rejects temporary remnants instead of
// guessing which generation was authoritative; callers must discard or
// explicitly recover the owned artifact directory.
func (w *ArtifactWriter) Commit() error {
	if w == nil {
		return artifactError("commit", errors.New("nil writer"))
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.canStageLocked(); err != nil {
		return err
	}
	if w.trace == nil || w.result == nil {
		return artifactError("commit", errors.New("trace and result are required"))
	}
	diagnostics := w.artifactsLocked()
	if !equalArtifacts(w.trace.Artifacts, diagnostics) {
		return artifactError("commit", errors.New("trace artifact metadata mismatch"))
	}
	if err := validateTraceResultConsistency(*w.trace, *w.result); err != nil {
		return artifactError("commit", errors.New("trace and result metadata mismatch"))
	}
	trace := cloneTrace(*w.trace)
	result := *w.result
	result.Artifacts = []string{"trace.json"}
	for _, artifact := range diagnostics {
		result.Artifacts = append(result.Artifacts, artifact.Name)
	}
	traceJSON, err := marshalArtifactJSON(trace)
	if err != nil {
		return artifactError("commit", err)
	}
	resultJSON, err := marshalArtifactJSON(result)
	if err != nil {
		return artifactError("commit", err)
	}
	files := make(map[string][]byte, len(w.payloads)+2)
	for name, payload := range w.payloads {
		files[name] = append([]byte(nil), payload.data...)
	}
	files["trace.json"] = traceJSON
	files["result.json"] = resultJSON
	if err := validatePayloadLimits(files, w.limits); err != nil {
		return artifactError("commit", err)
	}
	if err := w.publishLocked(files); err != nil {
		w.finished = true
		return artifactError("commit", err)
	}
	w.finished = true
	return nil
}

// Abort discards in-memory staging. Committed artifacts are never deleted.
func (w *ArtifactWriter) Abort() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.finished {
		return nil
	}
	w.payloads = nil
	w.trace = nil
	w.result = nil
	w.finished = true
	return nil
}

func (w *ArtifactWriter) stage(payloads []artifactPayload) error {
	if w == nil {
		return artifactError("stage", errors.New("nil writer"))
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.canStageLocked(); err != nil {
		return err
	}
	additional := 0
	for _, payload := range payloads {
		if err := validateArtifact(payload.artifact); err != nil {
			return artifactError("stage", err)
		}
		if _, exists := w.payloads[payload.artifact.Name]; exists {
			return artifactError("stage", errors.New("duplicate artifact"))
		}
		if len(payload.data) > w.limits.MaxFileBytes {
			return artifactError("stage", errors.New("file byte limit exceeded"))
		}
		additional += len(payload.data)
	}
	if len(w.payloads)+len(payloads)+2 > MaximumArtifactFiles {
		return artifactError("stage", errors.New("file count limit exceeded"))
	}
	used := 0
	for _, payload := range w.payloads {
		used += len(payload.data)
	}
	if used+additional > w.limits.MaxTotalBytes {
		return artifactError("stage", errors.New("total byte limit exceeded"))
	}
	for _, payload := range payloads {
		payload.data = append([]byte(nil), payload.data...)
		w.payloads[payload.artifact.Name] = payload
	}
	return nil
}

func (w *ArtifactWriter) canStageLocked() error {
	if w.finished {
		return artifactError("state", errors.New("writer is finished"))
	}
	return nil
}

func (w *ArtifactWriter) artifactsLocked() []Artifact {
	artifacts := make([]Artifact, 0, len(w.payloads))
	for _, payload := range w.payloads {
		artifacts = append(artifacts, payload.artifact)
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Name < artifacts[j].Name })
	return artifacts
}

func (w *ArtifactWriter) fileLimit() int {
	if w == nil || w.limits.MaxFileBytes <= 0 {
		return DefaultMaxArtifactFileBytes
	}
	return w.limits.MaxFileBytes
}

type stagedFile struct {
	name      string
	temp      string
	backup    string
	existed   bool
	published bool
}

func (w *ArtifactWriter) publishLocked(files map[string][]byte) error {
	if err := ensurePrivateDirectory(w.fs, w.directory); err != nil {
		return err
	}
	desired := make(map[string]bool, len(files))
	names := make([]string, 0, len(files))
	for name := range files {
		if !validArtifactName(name) {
			return errors.New("invalid artifact name")
		}
		desired[name] = true
		names = append(names, name)
	}
	existing, err := ownedArtifactNames(w.fs, w.directory)
	if err != nil {
		return err
	}
	for _, name := range existing {
		if !desired[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	staged := make([]stagedFile, 0, len(names))
	cleanupTemps := func(cause error) error {
		errorsFound := []error{cause}
		for _, file := range staged {
			if file.temp != "" {
				if err := w.fs.Remove(file.temp); err != nil && !errors.Is(err, fs.ErrNotExist) {
					errorsFound = append(errorsFound, err)
				}
			}
		}
		return errors.Join(errorsFound...)
	}
	for _, name := range names {
		entry := stagedFile{name: name}
		staged = append(staged, entry)
		if !desired[name] {
			continue
		}
		target := filepath.Join(w.directory, name)
		if err := rejectSymlinkTarget(w.fs, target); err != nil {
			return cleanupTemps(err)
		}
		temporary, err := w.fs.CreateTemp(w.directory, ".guitest-stage-")
		if err != nil {
			return cleanupTemps(err)
		}
		staged[len(staged)-1].temp = temporary.Name()
		if err := writePrivateFile(temporary, files[name]); err != nil {
			return cleanupTemps(err)
		}
	}

	rollback := func(cause error) error {
		var rollbackErrors []error
		for i := len(staged) - 1; i >= 0; i-- {
			file := &staged[i]
			target := filepath.Join(w.directory, file.name)
			if file.published {
				if err := w.fs.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
					rollbackErrors = append(rollbackErrors, err)
				}
			}
			if file.existed && file.backup != "" {
				if err := w.fs.Rename(file.backup, target); err != nil {
					rollbackErrors = append(rollbackErrors, err)
				} else {
					file.backup = ""
				}
			}
			if !file.existed && file.backup != "" {
				if err := w.fs.Remove(file.backup); err != nil && !errors.Is(err, fs.ErrNotExist) {
					rollbackErrors = append(rollbackErrors, err)
				}
			}
			if file.temp != "" {
				if err := w.fs.Remove(file.temp); err != nil && !errors.Is(err, fs.ErrNotExist) {
					rollbackErrors = append(rollbackErrors, err)
				}
			}
		}
		if len(rollbackErrors) > 0 {
			return errors.Join(append([]error{cause}, rollbackErrors...)...)
		}
		return cause
	}

	// Remove the old commit marker first. Until result.json is published last,
	// readers can reliably identify the directory as incomplete.
	backupOrder := make([]int, len(staged))
	for i := range staged {
		backupOrder[i] = i
	}
	sort.SliceStable(backupOrder, func(i, j int) bool {
		left, right := staged[backupOrder[i]].name, staged[backupOrder[j]].name
		if left == "result.json" || right == "result.json" {
			return left == "result.json"
		}
		return left < right
	})
	for _, index := range backupOrder {
		file := &staged[index]
		target := filepath.Join(w.directory, file.name)
		info, err := w.fs.Lstat(target)
		switch {
		case err == nil:
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return rollback(errors.New("artifact target is not a regular file"))
			}
			placeholder, err := w.fs.CreateTemp(w.directory, ".guitest-backup-")
			if err != nil {
				return rollback(err)
			}
			file.backup = placeholder.Name()
			if err := placeholder.Close(); err != nil {
				return rollback(err)
			}
			if err := w.fs.Remove(file.backup); err != nil {
				return rollback(err)
			}
			if err := w.fs.Rename(target, file.backup); err != nil {
				return rollback(err)
			}
			file.existed = true
		case errors.Is(err, fs.ErrNotExist):
		default:
			return rollback(err)
		}
	}

	publishOrder := make([]int, 0, len(files))
	for i := range staged {
		if desired[staged[i].name] {
			publishOrder = append(publishOrder, i)
		}
	}
	sort.SliceStable(publishOrder, func(i, j int) bool {
		left, right := staged[publishOrder[i]].name, staged[publishOrder[j]].name
		if left == "result.json" || right == "result.json" {
			return right == "result.json"
		}
		return left < right
	})
	for _, index := range publishOrder {
		file := &staged[index]
		target := filepath.Join(w.directory, file.name)
		if err := w.fs.Rename(file.temp, target); err != nil {
			return rollback(err)
		}
		file.temp = ""
		file.published = true
	}

	var cleanupErrors []error
	for i := range staged {
		file := &staged[i]
		if file.backup != "" {
			if err := w.fs.Remove(file.backup); err != nil && !errors.Is(err, fs.ErrNotExist) {
				cleanupErrors = append(cleanupErrors, err)
			} else {
				file.backup = ""
			}
		}
	}
	if len(cleanupErrors) > 0 {
		return errors.Join(append([]error{errors.New("artifact publication completed but backup cleanup failed")}, cleanupErrors...)...)
	}
	return nil
}

func normalizeArtifactLimits(limits ArtifactLimits) (ArtifactLimits, error) {
	if limits.MaxFileBytes == 0 {
		limits.MaxFileBytes = DefaultMaxArtifactFileBytes
	}
	if limits.MaxTotalBytes == 0 {
		limits.MaxTotalBytes = DefaultMaxArtifactTotalBytes
	}
	if limits.MaxFileBytes < 1 || limits.MaxTotalBytes < 1 {
		return ArtifactLimits{}, errors.New("artifact limits must be positive")
	}
	if limits.MaxTotalBytes < limits.MaxFileBytes {
		return ArtifactLimits{}, errors.New("total byte limit is less than file byte limit")
	}
	return limits, nil
}

func canonicalArtifactJSON(data []byte, maxBytes int) ([]byte, error) {
	if len(data) == 0 || len(data) > maxBytes {
		return nil, errors.New("JSON artifact byte limit exceeded")
	}
	if !utf8.Valid(data) {
		return nil, errors.New("invalid UTF-8 JSON artifact")
	}
	if err := rejectDuplicateFields(data); err != nil {
		return nil, errors.New("invalid JSON artifact")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, errors.New("invalid JSON artifact")
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("JSON artifact has trailing input")
	}
	canonical, err := json.Marshal(value)
	if err != nil || len(canonical)+1 > maxBytes {
		return nil, errors.New("JSON artifact byte limit exceeded")
	}
	return append(canonical, '\n'), nil
}

func marshalArtifactJSON(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func validatePNG(data []byte, maxBytes int) error {
	if len(data) == 0 || len(data) > maxBytes {
		return errors.New("PNG byte limit exceeded")
	}
	configuration, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return errors.New("invalid PNG")
	}
	if configuration.Width <= 0 || configuration.Height <= 0 ||
		int64(configuration.Width)*int64(configuration.Height) > MaximumScreenshotPixels {
		return errors.New("PNG pixel limit exceeded")
	}
	return nil
}

func validatePayloadLimits(files map[string][]byte, limits ArtifactLimits) error {
	if len(files) > MaximumArtifactFiles {
		return errors.New("file count limit exceeded")
	}
	total := 0
	for _, data := range files {
		if len(data) > limits.MaxFileBytes {
			return errors.New("file byte limit exceeded")
		}
		total += len(data)
		if total > limits.MaxTotalBytes {
			return errors.New("total byte limit exceeded")
		}
	}
	return nil
}

func validateArtifactResult(result Result) error {
	if result.Version != Version {
		return errors.New("invalid result version")
	}
	if err := validString("name", result.Name, true); err != nil {
		return errors.New("invalid result name")
	}
	if result.Steps < 0 || result.Steps > DefaultMaxSteps {
		return errors.New("invalid result step count")
	}
	switch result.Status {
	case StatusPassed:
		if result.Failure != nil {
			return errors.New("passed result has failure")
		}
	case StatusFailed:
		if result.Failure == nil {
			return errors.New("failed result has no failure")
		}
	default:
		return errors.New("invalid result status")
	}
	if failure := result.Failure; failure != nil {
		if failure.StepIndex < 0 || failure.StepIndex >= DefaultMaxSteps ||
			(failure.StepID != "" && validString("stepID", failure.StepID, false) != nil) ||
			!validOperation(failure.Operation) || !validErrorCode(failure.Code) {
			return errors.New("invalid result failure")
		}
	}
	return nil
}

func validateTraceResultConsistency(trace Trace, result Result) error {
	if trace.Status != result.Status || len(trace.Steps) != result.Steps {
		return errors.New("status or step count differs")
	}
	if result.Status == StatusPassed {
		if result.Failure != nil {
			return errors.New("passed result has failure")
		}
		return nil
	}
	if result.Failure == nil || len(trace.Steps) == 0 {
		return errors.New("failed metadata is missing")
	}
	last := trace.Steps[len(trace.Steps)-1]
	failure := result.Failure
	if last.Status != StatusFailed || failure.StepIndex != last.StepIndex || failure.StepID != last.StepID ||
		failure.Operation != last.Operation || failure.Code != last.Code || failure.Frame != last.FrameAfter {
		return errors.New("failure identity differs")
	}
	return nil
}

func stepArtifactName(stepIndex int, suffix string) (string, error) {
	if stepIndex < 0 || stepIndex >= DefaultMaxSteps {
		return "", artifactError("stage", errors.New("invalid step index"))
	}
	name := fmt.Sprintf("step-%03d-%s", stepIndex, suffix)
	if !validArtifactName(name) {
		return "", artifactError("stage", errors.New("invalid artifact suffix"))
	}
	return name, nil
}

func artifactError(operation string, err error) error {
	return &ArtifactError{Operation: operation, Err: err}
}

func ensurePrivateDirectory(fileSystem artifactFS, directory string) error {
	clean := filepath.Clean(directory)
	if filepath.Dir(clean) == clean {
		return errors.New("artifact directory must not be a filesystem root")
	}
	if err := rejectSymlinkPath(fileSystem, directory); err != nil {
		return err
	}
	if err := fileSystem.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	if err := rejectSymlinkPath(fileSystem, directory); err != nil {
		return err
	}
	info, err := fileSystem.Lstat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("artifact path is not a directory")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return errors.New("existing artifact directory is not private")
	}
	_, err = ownedArtifactNames(fileSystem, directory)
	return err
}

func ownedArtifactNames(fileSystem artifactFS, directory string) ([]string, error) {
	entries, err := fileSystem.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".guitest-stage-") || strings.HasPrefix(name, ".guitest-backup-") {
			return nil, errors.New("artifact directory contains an incomplete prior publication")
		}
		if !validArtifactName(name) {
			return nil, errors.New("artifact directory contains an unrelated entry")
		}
		info, err := fileSystem.Lstat(filepath.Join(directory, name))
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
			return nil, errors.New("artifact directory contains an unsafe entry")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func rejectSymlinkPath(fileSystem artifactFS, path string) error {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	remainder := strings.TrimPrefix(clean[len(volume):], string(filepath.Separator))
	current := volume + string(filepath.Separator)
	for _, component := range strings.Split(remainder, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := fileSystem.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("artifact path contains a symlink")
		}
	}
	return nil
}

func rejectSymlinkTarget(fileSystem artifactFS, target string) error {
	info, err := fileSystem.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("artifact target is a symlink")
	}
	if !info.Mode().IsRegular() {
		return errors.New("artifact target is not a regular file")
	}
	return nil
}

func writePrivateFile(file artifactFile, data []byte) (returnErr error) {
	closed := false
	defer func() {
		if !closed {
			if err := file.Close(); err != nil {
				returnErr = errors.Join(returnErr, err)
			}
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	written := 0
	for written < len(data) {
		n, err := file.Write(data[written:])
		if n < 0 || n > len(data)-written {
			return errors.New("invalid write count")
		}
		written += n
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	closed = true
	return nil
}

type artifactFile interface {
	io.Writer
	Name() string
	Chmod(os.FileMode) error
	Sync() error
	Close() error
}

type artifactFS interface {
	Lstat(string) (os.FileInfo, error)
	ReadDir(string) ([]os.DirEntry, error)
	MkdirAll(string, os.FileMode) error
	CreateTemp(string, string) (artifactFile, error)
	Rename(string, string) error
	Remove(string) error
}

type osArtifactFS struct{}

func (osArtifactFS) Lstat(name string) (os.FileInfo, error)       { return os.Lstat(name) }
func (osArtifactFS) ReadDir(name string) ([]os.DirEntry, error)   { return os.ReadDir(name) }
func (osArtifactFS) MkdirAll(path string, mode os.FileMode) error { return os.MkdirAll(path, mode) }
func (osArtifactFS) CreateTemp(directory, pattern string) (artifactFile, error) {
	return os.CreateTemp(directory, pattern)
}
func (osArtifactFS) Rename(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }
func (osArtifactFS) Remove(name string) error             { return os.Remove(name) }
