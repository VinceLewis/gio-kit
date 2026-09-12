package script

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTraceBuilderCopiesAndBoundsPrivacySafeMetadata(t *testing.T) {
	artifacts := []Artifact{{Name: "step-000-failure.json"}}
	builder := NewTraceBuilder()
	step := TraceStep{
		StepIndex: 0, StepID: "login", Operation: OpTap, Status: StatusFailed,
		Attempts: 2, FrameBefore: 7, FrameAfter: 10,
		VirtualBefore: Duration(time.Second), VirtualAfter: Duration(1500 * time.Millisecond),
		Code: CodeNotInteractable, Artifacts: artifacts,
	}
	if err := builder.Add(step); err != nil {
		t.Fatal(err)
	}
	step.Artifacts[0].Name = "mutated"
	trace, err := builder.Finish(StatusFailed)
	if err != nil {
		t.Fatal(err)
	}
	if got := trace.Steps[0].Artifacts[0].Name; got == "mutated" {
		t.Fatal("builder retained caller artifact slice")
	}
	encoded, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"typed secret", "selector secret", "clipboard secret", "sqlite secret"} {
		if bytes.Contains(encoded, []byte(secret)) {
			t.Fatalf("trace exposed %q", secret)
		}
	}
	decoded, err := ReadTrace(bytes.NewReader(encoded), int64(len(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, trace) {
		t.Fatalf("decoded trace differs:\n got %#v\nwant %#v", decoded, trace)
	}
}

func TestTraceRejectsUnknownDuplicateAndNondeterministicInput(t *testing.T) {
	valid := Trace{
		Version: TraceVersion, Status: StatusPassed,
		Steps: []TraceStep{{StepIndex: 0, Operation: OpSettle, Status: StatusPassed, Attempts: 1}},
	}
	if err := ValidateTrace(valid); err != nil {
		t.Fatal(err)
	}
	bad := valid
	bad.Steps = cloneTraceSteps(valid.Steps)
	bad.Steps[0].Operation = OpCapture
	bad.Steps[0].Artifacts = []Artifact{{Name: "step-000-capture.json"}, {Name: "step-000-capture-unredacted.png", Unredacted: true}}
	bad.Artifacts = append([]Artifact(nil), bad.Steps[0].Artifacts...)
	if err := ValidateTrace(bad); !errors.Is(err, ErrInvalidTrace) {
		t.Fatalf("nondeterministic artifact order error = %v", err)
	}
	failedEmpty := Trace{Version: TraceVersion, Status: StatusFailed, Steps: []TraceStep{}}
	if err := ValidateTrace(failedEmpty); !errors.Is(err, ErrInvalidTrace) {
		t.Fatalf("empty failed trace error = %v", err)
	}
	misleading := valid
	misleading.Steps = cloneTraceSteps(valid.Steps)
	misleading.Steps[0].Artifacts = []Artifact{{Name: "step-000-failure.json"}}
	misleading.Artifacts = append([]Artifact(nil), misleading.Steps[0].Artifacts...)
	if err := ValidateTrace(misleading); !errors.Is(err, ErrInvalidTrace) {
		t.Fatalf("passed failure-artifact error = %v", err)
	}

	inputs := []string{
		`{"version":1,"version":1,"status":"passed","steps":[]}`,
		`{"version":1,"status":"passed","steps":[],"secret":"value"}`,
		`{"version":1,"status":"passed","steps":[]} {}`,
	}
	for _, input := range inputs {
		if _, err := ReadTrace(strings.NewReader(input), int64(len(input))); !errors.Is(err, ErrInvalidTrace) {
			t.Errorf("ReadTrace(%q) error = %v", input, err)
		}
	}
}

func TestTraceJSONSchemaCheckedInAndClosed(t *testing.T) {
	checked, err := os.ReadFile("trace-schema-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if generated := TraceJSONSchema(); !bytes.Equal(checked, generated) {
		index := 0
		for index < len(checked) && index < len(generated) && checked[index] == generated[index] {
			index++
		}
		t.Fatalf("trace-schema-v1.json is stale at byte %d (checked %d bytes, generated %d bytes)", index, len(checked), len(generated))
	}
	var schema map[string]any
	if err := json.Unmarshal(checked, &schema); err != nil {
		t.Fatal(err)
	}
	if closed, ok := schema["additionalProperties"].(bool); !ok || closed {
		t.Fatal("trace schema is not closed")
	}
	defs := schema["$defs"].(map[string]any)
	step := defs["step"].(map[string]any)
	if got := len(step["oneOf"].([]any)); got != 2 {
		t.Fatalf("step status branches = %d, want 2", got)
	}
	if !validArtifactName("step-499-failure.json") || validArtifactName("step-500-failure.json") {
		t.Fatal("artifact index bounds differ from the schema's 000..499 range")
	}
}

func TestArtifactWriterPublishesDeterministicPrivateArtifacts(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "private", "artifacts")
	pngBytes := testPNG(t)
	writer, err := NewArtifactWriter(directory, ArtifactOptions{
		Screenshot: func(ctx context.Context, step int) ([]byte, error) {
			if step != 0 {
				t.Fatalf("screenshot step = %d", step)
			}
			return append([]byte(nil), pngBytes...), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	capture, err := writer.StageCapture(context.Background(), 0, []byte(`{ "z": 1, "redacted": true }`), ScreenshotRequired)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := artifactNames(capture), []string{"step-000-capture-unredacted.png", "step-000-capture.json"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("capture names = %v, want %v", got, want)
	}
	step := TraceStep{StepIndex: 0, Operation: OpCapture, Status: StatusPassed, Attempts: 1, FrameBefore: 2, FrameAfter: 3, Artifacts: capture}
	builder := NewTraceBuilder()
	if err := builder.Add(step); err != nil {
		t.Fatal(err)
	}
	trace, err := builder.Finish(StatusPassed)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.StageTrace(trace); err != nil {
		t.Fatal(err)
	}
	result := Result{Version: Version, Name: "scenario", Status: StatusPassed, Steps: 1, Artifacts: []string{"user-secret-name"}}
	if err := writer.StageResult(result); err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	wantFiles := []string{"result.json", "step-000-capture-unredacted.png", "step-000-capture.json", "trace.json"}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	gotFiles := make([]string, len(entries))
	for i, entry := range entries {
		gotFiles[i] = entry.Name()
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("%s mode = %o, want 600", entry.Name(), got)
		}
	}
	if !reflect.DeepEqual(gotFiles, wantFiles) {
		t.Fatalf("files = %v, want %v", gotFiles, wantFiles)
	}
	if info, err := os.Stat(directory); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("directory info = %v, error = %v", info, err)
	}

	resultBytes, err := os.ReadFile(filepath.Join(directory, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(resultBytes, []byte("user-secret-name")) {
		t.Fatal("result retained caller-provided artifact name")
	}
	if !bytes.Contains(resultBytes, []byte("capture-unredacted.png")) {
		t.Fatal("result does not explicitly mark the unredacted image filename")
	}
	traceBytes, err := os.ReadFile(filepath.Join(directory, "trace.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(traceBytes, []byte(`"unredacted":true`)) {
		t.Fatal("trace does not mark the image unredacted")
	}
	if got, err := os.ReadFile(filepath.Join(directory, "step-000-capture.json")); err != nil {
		t.Fatal(err)
	} else if string(got) != `{"redacted":true,"z":1}`+"\n" {
		t.Fatalf("canonical capture = %q", got)
	}
}

func TestArtifactWriterScreenshotSemanticsAreTransactional(t *testing.T) {
	for _, test := range []struct {
		name       string
		mode       ScreenshotMode
		screenshot ScreenshotFunc
		wantError  bool
		wantCount  int
	}{
		{name: "optional unavailable", mode: ScreenshotOptional, screenshot: func(context.Context, int) ([]byte, error) { return nil, ErrScreenshotUnavailable }, wantCount: 1},
		{name: "required unavailable", mode: ScreenshotRequired, screenshot: func(context.Context, int) ([]byte, error) { return nil, ErrScreenshotUnavailable }, wantError: true},
		{name: "optional backend failure", mode: ScreenshotOptional, screenshot: func(context.Context, int) ([]byte, error) { return nil, io.ErrUnexpectedEOF }, wantError: true},
		{name: "required invalid png", mode: ScreenshotRequired, screenshot: func(context.Context, int) ([]byte, error) { return []byte("not png"), nil }, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			writer, err := NewArtifactWriter(t.TempDir(), ArtifactOptions{Screenshot: test.screenshot})
			if err != nil {
				t.Fatal(err)
			}
			artifacts, err := writer.StageCapture(context.Background(), 0, []byte(`{"safe":true}`), test.mode)
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, wantError %v", err, test.wantError)
			}
			if err != nil && (!errors.Is(err, ErrArtifact) || (test.name == "required unavailable" && !errors.Is(err, ErrScreenshotUnavailable))) {
				t.Fatalf("error identity = %v", err)
			}
			if got := len(artifacts); got != test.wantCount {
				t.Fatalf("returned artifacts = %d, want %d", got, test.wantCount)
			}
			if got := len(writer.Artifacts()); got != test.wantCount {
				t.Fatalf("staged artifacts = %d, want %d", got, test.wantCount)
			}
		})
	}
}

func TestArtifactWriterScreenshotObservesCancellation(t *testing.T) {
	called := false
	writer, err := NewArtifactWriter(t.TempDir(), ArtifactOptions{Screenshot: func(context.Context, int) ([]byte, error) {
		called = true
		return testPNG(t), nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := writer.StageCapture(ctx, 0, []byte(`{"safe":true}`), ScreenshotOptional); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled screenshot error = %v", err)
	}
	if called {
		t.Fatal("screenshot callback was called after cancellation")
	}
	if len(writer.Artifacts()) != 0 {
		t.Fatal("canceled screenshot staged a partial JSON capture")
	}
}

func TestArtifactWriterPublishesBoundedFailureResult(t *testing.T) {
	directory := t.TempDir()
	writer, err := NewArtifactWriter(directory, ArtifactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	failureArtifact, err := writer.StageFailure(0, []byte(`{"redacted":true,"nodes":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	trace := Trace{
		Version: TraceVersion, Status: StatusFailed,
		Steps: []TraceStep{{
			StepIndex: 0, StepID: "failed-step", Operation: OpTap, Status: StatusFailed,
			Attempts: 1, FrameBefore: 1, FrameAfter: 2, Code: CodeNotInteractable,
			Artifacts: []Artifact{failureArtifact},
		}},
		Artifacts: []Artifact{failureArtifact},
	}
	if err := writer.StageTrace(trace); err != nil {
		t.Fatal(err)
	}
	result := Result{
		Version: Version, Name: "failure", Status: StatusFailed, Steps: 1,
		Failure: &Failure{StepIndex: 0, StepID: "failed-step", Operation: OpTap, Code: CodeNotInteractable, Frame: 2},
	}
	if err := writer.StageResult(result); err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var published Result
	if err := json.Unmarshal(data, &published); err != nil {
		t.Fatal(err)
	}
	if published.Failure == nil || published.Failure.Code != CodeNotInteractable {
		t.Fatalf("published failure = %#v", published.Failure)
	}
	if got, want := published.Artifacts, []string{"trace.json", "step-000-failure.json"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("published artifacts = %v, want %v", got, want)
	}
}

func TestArtifactWriterRejectsSymlinksAndLimits(t *testing.T) {
	root := t.TempDir()
	realDirectory := filepath.Join(root, "real")
	if err := os.Mkdir(realDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(realDirectory, link); err != nil {
		t.Fatal(err)
	}
	if _, err := NewArtifactWriter(link, ArtifactOptions{}); !errors.Is(err, ErrArtifact) {
		t.Fatalf("symlink error = %v", err)
	}
	if _, err := NewArtifactWriter(string(filepath.Separator), ArtifactOptions{}); !errors.Is(err, ErrArtifact) {
		t.Fatalf("filesystem-root error = %v", err)
	}
	shared := filepath.Join(root, "shared")
	if err := os.Mkdir(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewArtifactWriter(shared, ArtifactOptions{}); !errors.Is(err, ErrArtifact) {
		t.Fatalf("shared-directory error = %v", err)
	}
	if info, err := os.Stat(shared); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("shared directory was modified: info=%v err=%v", info, err)
	}
	unowned := filepath.Join(root, "unowned")
	if err := os.Mkdir(unowned, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unowned, "unrelated.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewArtifactWriter(unowned, ArtifactOptions{}); !errors.Is(err, ErrArtifact) {
		t.Fatalf("unowned-directory error = %v", err)
	}

	writer, err := NewArtifactWriter(filepath.Join(root, "limited"), ArtifactOptions{Limits: ArtifactLimits{MaxFileBytes: 16, MaxTotalBytes: 32}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.StageFailure(0, []byte(`{"value":"too long"}`)); !errors.Is(err, ErrArtifact) {
		t.Fatalf("oversize error = %v", err)
	}
	if len(writer.Artifacts()) != 0 {
		t.Fatal("oversize stage left artifact metadata")
	}

	totalWriter, err := NewArtifactWriter(filepath.Join(root, "total-limited"), ArtifactOptions{Limits: ArtifactLimits{MaxFileBytes: 64, MaxTotalBytes: 80}})
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"value":"1234567890123456789012345678901234567890"}`)
	if _, err := totalWriter.StageFailure(0, payload); err != nil {
		t.Fatal(err)
	}
	if _, err := totalWriter.StageFailure(1, payload); !errors.Is(err, ErrArtifact) {
		t.Fatalf("total limit error = %v", err)
	}
	if got := len(totalWriter.Artifacts()); got != 1 {
		t.Fatalf("failed total-limit stage left %d artifacts, want prior 1", got)
	}
}

func TestArtifactWriterReconcilesStaleGenerationAndPublishesResultLast(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"result.json", "trace.json", "step-000-capture.json", "step-000-capture-unredacted.png", "step-499-failure.json"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("stale"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fileSystem := &recordingRenameFS{artifactFS: osArtifactFS{}}
	writer, err := newArtifactWriter(directory, ArtifactOptions{}, fileSystem)
	if err != nil {
		t.Fatal(err)
	}
	stagePassingEmpty(t, writer)
	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(entries))
	for index, entry := range entries {
		got[index] = entry.Name()
	}
	if want := []string{"result.json", "trace.json"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("reconciled files=%v, want %v", got, want)
	}
	if len(fileSystem.published) < 2 || fileSystem.published[len(fileSystem.published)-1] != "result.json" {
		t.Fatalf("publication order=%v, result.json must be last", fileSystem.published)
	}
	if fileSystem.markerVisibleEarly {
		t.Fatal("result.json was visible before the complete generation was published")
	}
}

func TestArtifactWriterSurfacesBackupCleanupAndRejectsCrashRemnants(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "result.json"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileSystem := &backupCleanupFailureFS{artifactFS: osArtifactFS{}}
	writer, err := newArtifactWriter(directory, ArtifactOptions{}, fileSystem)
	if err != nil {
		t.Fatal(err)
	}
	stagePassingEmpty(t, writer)
	if err := writer.Commit(); !errors.Is(err, errInjectedBackupCleanup) || !errors.Is(err, ErrArtifact) {
		t.Fatalf("cleanup error=%v", err)
	}
	for _, name := range []string{"result.json", "trace.json"} {
		if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
			t.Fatalf("complete generation %s: %v", name, err)
		}
	}
	if _, err := NewArtifactWriter(directory, ArtifactOptions{}); !errors.Is(err, ErrArtifact) {
		t.Fatalf("crash-remnant error=%v", err)
	}
}

func TestArtifactWriterRejectsTraceResultMismatch(t *testing.T) {
	writer, err := NewArtifactWriter(t.TempDir(), ArtifactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	trace := Trace{Version: TraceVersion, Status: StatusFailed, Steps: []TraceStep{{
		StepIndex: 0, StepID: "expected", Operation: OpTap, Status: StatusFailed, Attempts: 1,
		FrameBefore: 1, FrameAfter: 2, Code: CodeTimeout,
	}}}
	if err := writer.StageTrace(trace); err != nil {
		t.Fatal(err)
	}
	result := Result{Version: Version, Name: "mismatch", Status: StatusFailed, Steps: 1,
		Failure: &Failure{StepIndex: 0, StepID: "different", Operation: OpTap, Code: CodeInternal, Frame: 2}}
	if err := writer.StageResult(result); err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit(); !errors.Is(err, ErrArtifact) {
		t.Fatalf("mismatch commit error=%v", err)
	}
}

func TestTraceResultConsistencyRequiresExactFailureIdentity(t *testing.T) {
	trace := Trace{Version: TraceVersion, Status: StatusFailed, Steps: []TraceStep{{
		StepIndex: 0, StepID: "step", Operation: OpTap, Status: StatusFailed, Attempts: 1,
		FrameBefore: 1, FrameAfter: 2, Code: CodeTimeout,
	}}}
	matching := Result{Version: Version, Name: "match", Status: StatusFailed, Steps: 1,
		Failure: &Failure{StepIndex: 0, StepID: "step", Operation: OpTap, Code: CodeTimeout, Frame: 2}}
	if err := validateTraceResultConsistency(trace, matching); err != nil {
		t.Fatalf("matching metadata error = %v", err)
	}
	tests := map[string]func(*Result){
		"step index": func(result *Result) { result.Failure.StepIndex = 1 },
		"step id":    func(result *Result) { result.Failure.StepID = "other" },
		"operation":  func(result *Result) { result.Failure.Operation = OpFocus },
		"code":       func(result *Result) { result.Failure.Code = CodeInternal },
		"frame":      func(result *Result) { result.Failure.Frame++ },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			result := matching
			failure := *matching.Failure
			result.Failure = &failure
			mutate(&result)
			if err := validateTraceResultConsistency(trace, result); err == nil {
				t.Fatal("mismatched failure metadata was accepted")
			}
		})
	}
}

func TestArtifactWriterCommitRollbackRestoresPriorFiles(t *testing.T) {
	directory := t.TempDir()
	oldResult := []byte("old result\n")
	if err := os.WriteFile(filepath.Join(directory, "result.json"), oldResult, 0o600); err != nil {
		t.Fatal(err)
	}
	fileSystem := &renameFailureFS{artifactFS: osArtifactFS{}, failPublish: 2}
	writer, err := newArtifactWriter(directory, ArtifactOptions{}, fileSystem)
	if err != nil {
		t.Fatal(err)
	}
	stagePassingEmpty(t, writer)
	if err := writer.Commit(); !errors.Is(err, ErrArtifact) {
		t.Fatalf("commit error = %v", err)
	}
	got, err := os.ReadFile(filepath.Join(directory, "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, oldResult) {
		t.Fatalf("restored result = %q, want %q", got, oldResult)
	}
	if _, err := os.Lstat(filepath.Join(directory, "trace.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new trace survived rollback: %v", err)
	}
	assertNoTemporaryFiles(t, directory)
}

func TestArtifactWriterShortWriteLeavesNoPublishedFiles(t *testing.T) {
	directory := t.TempDir()
	fileSystem := &shortWriteFS{artifactFS: osArtifactFS{}}
	writer, err := newArtifactWriter(directory, ArtifactOptions{}, fileSystem)
	if err != nil {
		t.Fatal(err)
	}
	stagePassingEmpty(t, writer)
	if err := writer.Commit(); !errors.Is(err, io.ErrShortWrite) || !errors.Is(err, ErrArtifact) {
		t.Fatalf("commit error = %v", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("short write left files: %v", entries)
	}
}

func TestArtifactWriterSurfacesTemporaryCleanupFailure(t *testing.T) {
	directory := t.TempDir()
	fileSystem := &tempCleanupFailureFS{artifactFS: osArtifactFS{}}
	writer, err := newArtifactWriter(directory, ArtifactOptions{}, fileSystem)
	if err != nil {
		t.Fatal(err)
	}
	stagePassingEmpty(t, writer)
	err = writer.Commit()
	if !errors.Is(err, io.ErrShortWrite) || !errors.Is(err, errInjectedTempCleanup) || !errors.Is(err, ErrArtifact) {
		t.Fatalf("commit cleanup error = %v", err)
	}
	if _, err := NewArtifactWriter(directory, ArtifactOptions{}); !errors.Is(err, ErrArtifact) {
		t.Fatalf("temporary-remnant error = %v", err)
	}
}

func stagePassingEmpty(t *testing.T, writer *ArtifactWriter) {
	t.Helper()
	trace := Trace{Version: TraceVersion, Status: StatusPassed, Steps: []TraceStep{}}
	if err := writer.StageTrace(trace); err != nil {
		t.Fatal(err)
	}
	if err := writer.StageResult(Result{Version: Version, Name: "test", Status: StatusPassed, Steps: 0}); err != nil {
		t.Fatal(err)
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func artifactNames(artifacts []Artifact) []string {
	names := make([]string, len(artifacts))
	for i, artifact := range artifacts {
		names[i] = artifact.Name
	}
	return names
}

func assertNoTemporaryFiles(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".guitest-") {
			t.Errorf("temporary file survived: %s", entry.Name())
		}
	}
}

type renameFailureFS struct {
	artifactFS
	failPublish int
	publishes   int
}

type recordingRenameFS struct {
	artifactFS
	published          []string
	markerVisibleEarly bool
}

func (f *recordingRenameFS) Rename(oldPath, newPath string) error {
	if strings.HasPrefix(filepath.Base(oldPath), ".guitest-stage-") {
		name := filepath.Base(newPath)
		f.published = append(f.published, name)
		if name != "result.json" {
			marker := filepath.Join(filepath.Dir(newPath), "result.json")
			if _, err := f.artifactFS.Lstat(marker); err == nil {
				f.markerVisibleEarly = true
			}
		}
	}
	return f.artifactFS.Rename(oldPath, newPath)
}

var errInjectedBackupCleanup = errors.New("injected backup cleanup failure")

type backupCleanupFailureFS struct {
	artifactFS
	backupRemoves int
}

func (f *backupCleanupFailureFS) Remove(name string) error {
	if strings.HasPrefix(filepath.Base(name), ".guitest-backup-") {
		f.backupRemoves++
		if f.backupRemoves == 2 {
			return errInjectedBackupCleanup
		}
	}
	return f.artifactFS.Remove(name)
}

func (f *renameFailureFS) Rename(oldPath, newPath string) error {
	if strings.HasPrefix(filepath.Base(oldPath), ".guitest-stage-") {
		f.publishes++
		if f.publishes == f.failPublish {
			return errors.New("injected rename failure")
		}
	}
	return f.artifactFS.Rename(oldPath, newPath)
}

type shortWriteFS struct {
	artifactFS
	used bool
}

func (f *shortWriteFS) CreateTemp(directory, pattern string) (artifactFile, error) {
	file, err := f.artifactFS.CreateTemp(directory, pattern)
	if err != nil {
		return nil, err
	}
	if !f.used && strings.HasPrefix(pattern, ".guitest-stage-") {
		f.used = true
		return &shortWriteFile{artifactFile: file}, nil
	}
	return file, nil
}

var errInjectedTempCleanup = errors.New("injected temporary cleanup failure")

type tempCleanupFailureFS struct {
	artifactFS
	used bool
}

func (f *tempCleanupFailureFS) CreateTemp(directory, pattern string) (artifactFile, error) {
	file, err := f.artifactFS.CreateTemp(directory, pattern)
	if err != nil {
		return nil, err
	}
	if !f.used && strings.HasPrefix(pattern, ".guitest-stage-") {
		f.used = true
		return &shortWriteFile{artifactFile: file}, nil
	}
	return file, nil
}

func (f *tempCleanupFailureFS) Remove(name string) error {
	if strings.HasPrefix(filepath.Base(name), ".guitest-stage-") {
		return errInjectedTempCleanup
	}
	return f.artifactFS.Remove(name)
}

type shortWriteFile struct{ artifactFile }

func (*shortWriteFile) Write([]byte) (int, error) { return 0, nil }
