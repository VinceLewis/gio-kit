package script_test

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/VinceLewis/gio-kit/guitest/script"
)

func TestProtocolConstantsAndDuration(t *testing.T) {
	wantCodes := []script.ErrorCode{
		script.CodeInvalidScript, script.CodeTimeout, script.CodeCancelled, script.CodeNotFound,
		script.CodeAmbiguous, script.CodeNotInteractable, script.CodeAssertionFailed,
		script.CodeProbeError, script.CodeFrameLimit, script.CodeClosed, script.CodeArtifactError,
		script.CodeInternal,
	}
	seen := map[script.ErrorCode]bool{}
	for _, code := range wantCodes {
		if code == "" || seen[code] {
			t.Fatalf("invalid stable error code %q", code)
		}
		seen[code] = true
	}
	duration := script.Duration(1500 * time.Millisecond)
	encoded, err := json.Marshal(duration)
	if err != nil || string(encoded) != `"1.5s"` {
		t.Fatalf("marshal duration: %s, %v", encoded, err)
	}
	var decoded script.Duration
	if err = json.Unmarshal(encoded, &decoded); err != nil || decoded.Duration() != 1500*time.Millisecond {
		t.Fatalf("unmarshal duration: %v, %v", decoded, err)
	}
}

func TestValidateRejectsProgrammaticNonFiniteGeometry(t *testing.T) {
	s := validScript(script.Step{Op: script.OpMove, Position: &script.Point{X: math.NaN(), Y: 1}})
	if err := script.Validate(s); !errors.Is(err, script.ErrInvalidScript) {
		t.Fatalf("Validate error = %v", err)
	}
}

func TestInvalidScriptErrorDoesNotExposeRawJSONKeys(t *testing.T) {
	const secretKey = "customer-password-column"
	step := script.Step{
		Op:     script.OpExpectProbe,
		Probe:  "privacy",
		Args:   json.RawMessage(`{"` + secretKey + `":"` + strings.Repeat("x", script.DefaultMaxString+1) + `"}`),
		Equals: json.RawMessage(`true`),
	}
	err := script.Validate(validScript(step))
	if !errors.Is(err, script.ErrInvalidScript) {
		t.Fatalf("Validate error = %v", err)
	}
	if strings.Contains(err.Error(), secretKey) {
		t.Fatalf("validation error exposed script JSON key: %v", err)
	}
}

func validScript(steps ...script.Step) script.Script {
	return script.Script{Version: script.Version, Name: "test", Steps: steps}
}
