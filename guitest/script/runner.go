package script

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/VinceLewis/gio-kit/guitest"
)

var (
	ErrAssertion = errors.New("guitest/script: assertion failed")
	ErrProbe     = errors.New("guitest/script: probe failed")
	ErrInternal  = errors.New("guitest/script: internal error")
)

// Options supplies consumer-owned, allowlisted probes and optional artifact
// output. Run never closes Driver.
type Options struct {
	Probes      map[string]ProbeFunc
	Artifacts   *ArtifactWriter
	DumpOptions guitest.DumpOptions
}

// StepError identifies one failed step without copying script or application
// values. Unwrap preserves the useful underlying error category.
type StepError struct {
	StepIndex int
	StepID    string
	Operation Operation
	Code      ErrorCode
	Frame     uint64
	Err       error
}

func (e *StepError) Error() string {
	if e == nil {
		return "guitest/script: step failed"
	}
	return fmt.Sprintf("guitest/script: step %d (%s) failed: %s", e.StepIndex, e.Operation, e.Code)
}

func (e *StepError) Unwrap() error { return e.Err }

type preparedStep struct {
	selector guitest.Selector
	expected any
}

type runner struct {
	ctx       context.Context
	driver    *guitest.Driver
	script    Script
	options   Options
	prepared  []preparedStep
	trace     *TraceBuilder
	startedAt time.Time
	held      bool
}

// Run executes a validated script sequentially on the caller goroutine. It
// does not close or otherwise take ownership of driver.
func Run(ctx context.Context, driver *guitest.Driver, document Script, options Options) (Result, error) {
	result := Result{Version: Version, Name: document.Name, Status: StatusFailed}
	if ctx == nil {
		return result, fmt.Errorf("%w: nil context", ErrInternal)
	}
	if driver == nil {
		return result, fmt.Errorf("%w: nil driver", ErrInternal)
	}
	prepared, err := prepare(document, options)
	if err != nil {
		return result, err
	}
	r := &runner{ctx: ctx, driver: driver, script: document, options: options, prepared: prepared,
		trace: NewTraceBuilder(), startedAt: driver.Clock().Now()}
	for index := range document.Steps {
		if err := r.runStep(index); err != nil {
			return r.finishFailure(index, err)
		}
		result.Steps = index + 1
	}
	if err := ctx.Err(); err != nil {
		return r.interrupted(result, err)
	}
	result.Status = StatusPassed
	return r.finish(result, nil)
}

func prepare(document Script, options Options) ([]preparedStep, error) {
	if err := Validate(document); err != nil {
		return nil, err
	}
	prepared := make([]preparedStep, len(document.Steps))
	held := false
	for index := range document.Steps {
		step := &document.Steps[index]
		path := "steps[" + quoteIndex(index) + "]"
		if step.Selector != nil {
			selector, err := CompileSelector(*step.Selector)
			if err != nil {
				return nil, err
			}
			prepared[index].selector = selector
		}
		if step.Op == OpExpectSnapshot {
			raw := step.Equals
			if rawPresent(step.NotEquals) {
				raw = step.NotEquals
			}
			if rawPresent(raw) {
				value, err := decodeBoundedJSON(raw, false)
				if err != nil {
					return nil, invalid(path, "snapshot comparison is invalid")
				}
				prepared[index].expected = value
			}
		}
		if step.Op == OpExpectProbe {
			probe, ok := options.Probes[step.Probe]
			if !ok || probe == nil {
				return nil, invalid(path+".probe", "probe is not registered")
			}
			args := step.Args
			if !rawPresent(args) {
				args = json.RawMessage("{}")
			}
			if _, err := decodeBoundedJSON(args, true); err != nil {
				return nil, invalid(path+".args", err.Error())
			}
			value, err := decodeBoundedJSON(step.Equals, false)
			if err != nil {
				return nil, invalid(path+".equals", "probe comparison is invalid")
			}
			prepared[index].expected = value
		}
		if step.Op == OpCapture && options.Artifacts == nil {
			return nil, invalid(path, "capture requires an artifact writer")
		}
		switch step.Op {
		case OpPress:
			if held {
				return nil, invalid(path, "press cannot begin while a pointer is held")
			}
			held = true
		case OpMove:
			if !held {
				return nil, invalid(path, "move requires a held pointer")
			}
		case OpRelease, OpCancelPointer:
			if !held {
				return nil, invalid(path, "pointer end requires a held pointer")
			}
			held = false
		case OpTap, OpDoubleTap, OpLongPress, OpDrag, OpScroll, OpFocus, OpType:
			if held {
				return nil, invalid(path, "action is incompatible with a held pointer")
			}
		}
	}
	if held {
		return nil, invalid("steps", "script must not finish with a held pointer")
	}
	return prepared, nil
}

func (r *runner) runStep(index int) error {
	step := &r.script.Steps[index]
	timeout := DefaultStepTimeout
	if r.script.Defaults != nil && r.script.Defaults.Timeout != nil {
		timeout = r.script.Defaults.Timeout.Duration()
	}
	if step.Timeout != nil {
		timeout = step.Timeout.Duration()
	}
	ctx, cancel := context.WithTimeout(r.ctx, timeout)
	defer cancel()

	beforeFrame := r.driver.FrameNumber()
	beforeTime := r.virtualTime()
	attempts := 0
	var artifacts []Artifact
	err := ctx.Err()
	if err != nil {
		attempts = 1
	} else {
		artifacts, err = r.execute(ctx, index, &attempts)
		if contextErr := ctx.Err(); contextErr != nil {
			if err == nil {
				err = contextErr
			} else {
				err = errors.Join(contextErr, err)
			}
		}
	}
	code := ErrorCode("")
	status := StatusPassed
	var stepErr *StepError
	if err != nil {
		status = StatusFailed
		capture, captureErr := r.captureJSON(nil)
		if captureErr == nil && r.options.Artifacts != nil {
			artifact, stageErr := r.options.Artifacts.StageFailure(index, capture)
			captureErr = stageErr
			if stageErr == nil {
				artifacts = append(artifacts, artifact)
			}
		}
		if r.held {
			_ = r.driver.CancelPointer()
			r.held = false
		}
		if captureErr != nil && !errors.Is(err, guitest.ErrClosed) {
			err = errors.Join(err, fmt.Errorf("%w: failure capture", ErrArtifact))
		}
		code = codeFor(err)
		stepErr = &StepError{StepIndex: index, StepID: step.ID, Operation: step.Op,
			Code: code, Frame: r.driver.FrameNumber(), Err: err}
	}
	traceErr := r.trace.Add(TraceStep{
		StepIndex: index, StepID: step.ID, Operation: step.Op, Status: status, Attempts: attempts,
		FrameBefore: beforeFrame, FrameAfter: r.driver.FrameNumber(), VirtualBefore: beforeTime,
		VirtualAfter: r.virtualTime(), Code: code, Artifacts: artifacts,
	})
	if traceErr != nil {
		if stepErr != nil {
			return stepErr
		}
		err = fmt.Errorf("%w: trace step", ErrInternal)
		return &StepError{StepIndex: index, StepID: step.ID, Operation: step.Op,
			Code: CodeInternal, Frame: r.driver.FrameNumber(), Err: err}
	}
	if stepErr != nil {
		return stepErr
	}
	return nil
}

func (r *runner) virtualTime() Duration {
	return Duration(r.driver.Clock().Now().Sub(r.startedAt))
}

func (r *runner) finishFailure(index int, cause error) (Result, error) {
	step := r.script.Steps[index]
	stepErr, ok := cause.(*StepError)
	if !ok {
		stepErr = &StepError{StepIndex: index, StepID: step.ID, Operation: step.Op,
			Code: codeFor(cause), Frame: r.driver.FrameNumber(), Err: cause}
	}
	result := Result{Version: Version, Name: r.script.Name, Status: StatusFailed, Steps: index + 1,
		Failure: &Failure{StepIndex: index, StepID: step.ID, Operation: step.Op, Code: stepErr.Code, Frame: stepErr.Frame}}
	return r.finish(result, stepErr)
}

func (r *runner) finish(result Result, runErr error) (Result, error) {
	if runErr == nil {
		if err := r.ctx.Err(); err != nil {
			return r.interrupted(result, err)
		}
	}
	trace, traceErr := r.trace.Finish(result.Status)
	if traceErr != nil && runErr == nil {
		runErr = fmt.Errorf("%w: finish trace", ErrInternal)
	}
	if r.options.Artifacts == nil {
		return result, runErr
	}
	if interrupted := r.interruptBeforeSuccessWrite(result, runErr); interrupted != nil {
		return interrupted.Result, interrupted.err
	}
	if traceErr == nil {
		traceErr = r.options.Artifacts.StageTrace(trace)
	}
	if traceErr == nil {
		if interrupted := r.interruptBeforeSuccessWrite(result, runErr); interrupted != nil {
			return interrupted.Result, interrupted.err
		}
	}
	if traceErr == nil {
		artifacts := r.options.Artifacts.Artifacts()
		result.Artifacts = make([]string, len(artifacts)+1)
		result.Artifacts[0] = "trace.json"
		for index := range artifacts {
			result.Artifacts[index+1] = artifacts[index].Name
		}
		traceErr = r.options.Artifacts.StageResult(result)
	}
	if traceErr == nil {
		if interrupted := r.interruptBeforeSuccessWrite(result, runErr); interrupted != nil {
			return interrupted.Result, interrupted.err
		}
	}
	if traceErr == nil {
		traceErr = r.options.Artifacts.Commit()
	}
	if traceErr != nil {
		_ = r.options.Artifacts.Abort()
		result.Artifacts = nil
		artifactErr := fmt.Errorf("%w: finalization failed", ErrArtifact)
		if runErr != nil && result.Status == StatusFailed && result.Failure != nil {
			return result, errors.Join(runErr, artifactErr)
		}
		if runErr != nil {
			artifactErr = errors.Join(runErr, artifactErr)
		}
		return r.artifactFailure(result, artifactErr)
	}
	return result, runErr
}

// interruptedResult carries a result and error together so finish can check
// cancellation between each in-memory artifact stage without publishing a
// successful result after cancellation was already observable.
type interruptedResult struct {
	Result
	err error
}

func (r *runner) interruptBeforeSuccessWrite(result Result, runErr error) *interruptedResult {
	if runErr != nil {
		return nil
	}
	if err := r.ctx.Err(); err != nil {
		interrupted, interruptErr := r.interrupted(result, err)
		return &interruptedResult{Result: interrupted, err: interruptErr}
	}
	return nil
}

func (r *runner) interrupted(result Result, cause error) (Result, error) {
	if r.options.Artifacts != nil {
		_ = r.options.Artifacts.Abort()
	}
	result.Status = StatusFailed
	result.Artifacts = nil
	if result.Steps == 0 || len(r.script.Steps) == 0 {
		return result, cause
	}
	index := result.Steps - 1
	if index >= len(r.script.Steps) {
		index = len(r.script.Steps) - 1
	}
	step := r.script.Steps[index]
	stepErr := &StepError{StepIndex: index, StepID: step.ID, Operation: step.Op,
		Code: codeFor(cause), Frame: r.driver.FrameNumber(), Err: cause}
	result.Failure = &Failure{StepIndex: index, StepID: step.ID, Operation: step.Op,
		Code: stepErr.Code, Frame: stepErr.Frame}
	return result, stepErr
}

func (r *runner) artifactFailure(result Result, cause error) (Result, error) {
	result.Status = StatusFailed
	result.Artifacts = nil
	if result.Steps == 0 || len(r.script.Steps) == 0 {
		return result, cause
	}
	index := result.Steps - 1
	if index >= len(r.script.Steps) {
		index = len(r.script.Steps) - 1
	}
	step := r.script.Steps[index]
	stepErr := &StepError{StepIndex: index, StepID: step.ID, Operation: step.Op,
		Code: CodeArtifactError, Frame: r.driver.FrameNumber(), Err: cause}
	result.Failure = &Failure{StepIndex: index, StepID: step.ID, Operation: step.Op,
		Code: stepErr.Code, Frame: stepErr.Frame}
	return result, stepErr
}

func codeFor(err error) ErrorCode {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrInvalidScript):
		return CodeInvalidScript
	case errors.Is(err, context.Canceled):
		return CodeCancelled
	case errors.Is(err, ErrAssertion):
		return CodeAssertionFailed
	case errors.Is(err, ErrProbe):
		return CodeProbeError
	case errors.Is(err, ErrArtifact):
		return CodeArtifactError
	case errors.Is(err, context.DeadlineExceeded):
		return CodeTimeout
	case errors.Is(err, guitest.ErrNotFound):
		return CodeNotFound
	case errors.Is(err, guitest.ErrAmbiguous):
		return CodeAmbiguous
	case errors.Is(err, guitest.ErrNotInteractable):
		return CodeNotInteractable
	case errors.Is(err, guitest.ErrFrameLimit):
		return CodeFrameLimit
	case errors.Is(err, guitest.ErrClosed):
		return CodeClosed
	default:
		return CodeInternal
	}
}

func marshalCapture(dump guitest.Dump) ([]byte, error) { return json.Marshal(dump) }
