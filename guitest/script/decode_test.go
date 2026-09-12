package script_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/VinceLewis/gio-kit/guitest/script"
)

func TestDecodeValidOperations(t *testing.T) {
	documents := []string{
		`{"op":"tap","selector":{"label":"Save"}}`,
		`{"op":"doubleTap","selector":{"description":"Open"}}`,
		`{"op":"longPress","selector":{"name":"Summary"},"duration":"500ms"}`,
		`{"op":"press","selector":{"role":"button"}}`,
		`{"op":"move","position":{"x":-1.25,"y":999.5}}`,
		`{"op":"release"}`,
		`{"op":"cancelPointer"}`,
		`{"op":"drag","selector":{"id":"grid"},"delta":{"x":0,"y":-40},"duration":"200ms"}`,
		`{"op":"scroll","selector":{"enabled":true},"delta":{"x":0,"y":120}}`,
		`{"op":"focus","selector":{"selected":false}}`,
		`{"op":"type","selector":{"all":[{"role":"editor"},{"text":"Summary"}]},"text":""}`,
		`{"op":"key","key":{"name":"Enter","modifiers":["shift","control"]}}`,
		`{"op":"back"}`,
		`{"op":"clearFocus"}`,
		`{"op":"setSelection","range":{"start":2,"end":0}}`,
		`{"op":"setComposition","range":{"start":0,"end":2}}`,
		`{"op":"edit","range":{"start":0,"end":2},"text":"é"}`,
		`{"op":"resize","size":{"width":420,"height":820},"metric":{"pxPerDp":1,"pxPerSp":1.25}}`,
		`{"op":"advance","duration":"0s"}`,
		`{"op":"settle","timeout":"5s"}`,
		`{"op":"expect","selector":{"within":{"target":{"name":"Save"},"ancestor":{"id":"form"}}},"expect":"present"}`,
		`{"op":"expect","selector":{"containing":{"target":{"role":"button"},"descendant":{"label":"Save"}}},"expect":"count","count":1}`,
		`{"op":"expect","selector":{"nth":{"selector":{"name":"row"},"index":3}},"expect":"viewportClipped"}`,
		`{"op":"expectSnapshot","component":"form","pointer":"/fields/0/value","equals":null}`,
		`{"op":"expectSnapshot","component":"grid","pointer":"","notEquals":{"loading":true}}`,
		`{"op":"expectSnapshot","component":"shell","pointer":"/route","exists":false}`,
		`{"op":"expectProbe","probe":"records","args":{"kind":"song"},"equals":{"count":2}}`,
		`{"op":"capture","name":"final","components":["router","form"],"screenshot":"optional"}`,
	}
	steps := strings.Join(documents, ",")
	input := fmt.Sprintf(`{"version":1,"name":"all operations","fixture":"synthetic.basic.v1","defaults":{"timeout":"2s"},"steps":[%s]}`, steps)
	decoded, err := script.Decode(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Steps) != len(documents) {
		t.Fatalf("steps = %d, want %d", len(decoded.Steps), len(documents))
	}
}

func TestDecodeRejectsMalformedInputs(t *testing.T) {
	valid := `{"version":1,"name":"x","steps":[]}`
	tests := map[string][]byte{
		"empty":                  nil,
		"malformed":              []byte(`{"version":`),
		"unknown top field":      []byte(`{"version":1,"name":"x","steps":[],"extra":true}`),
		"unknown nested field":   []byte(`{"version":1,"name":"x","steps":[{"op":"tap","selector":{"label":"x","extra":1}}]}`),
		"trailing":               []byte(valid + `{}`),
		"invalid UTF-8":          append([]byte(`{"version":1,"name":"`), 0xff),
		"duplicate field":        []byte(`{"version":1,"name":"x","name":"y","steps":[]}`),
		"wrong version":          []byte(`{"version":2,"name":"x","steps":[]}`),
		"missing name":           []byte(`{"version":1,"steps":[]}`),
		"missing steps":          []byte(`{"version":1,"name":"x"}`),
		"numeric duration":       []byte(`{"version":1,"name":"x","steps":[{"op":"advance","duration":1}]}`),
		"unparseable duration":   []byte(`{"version":1,"name":"x","steps":[{"op":"advance","duration":"later"}]}`),
		"negative duration":      []byte(`{"version":1,"name":"x","steps":[{"op":"advance","duration":"-1ns"}]}`),
		"timeout above cap":      []byte(`{"version":1,"name":"x","defaults":{"timeout":"5001ms"},"steps":[]}`),
		"unknown operation":      []byte(`{"version":1,"name":"x","steps":[{"op":"click"}]}`),
		"incompatible field":     []byte(`{"version":1,"name":"x","steps":[{"op":"back","text":"secret"}]}`),
		"two selector operators": []byte(`{"version":1,"name":"x","steps":[{"op":"tap","selector":{"label":"x","role":"button"}}]}`),
		"bad role":               []byte(`{"version":1,"name":"x","steps":[{"op":"tap","selector":{"role":"link"}}]}`),
		"empty all":              []byte(`{"version":1,"name":"x","steps":[{"op":"tap","selector":{"all":[]}}]}`),
		"negative nth":           []byte(`{"version":1,"name":"x","steps":[{"op":"tap","selector":{"nth":{"selector":{"name":"x"},"index":-1}}}]}`),
		"duplicate step ID":      []byte(`{"version":1,"name":"x","steps":[{"id":"same","op":"back"},{"id":"same","op":"back"}]}`),
		"bad comparison union":   []byte(`{"version":1,"name":"x","steps":[{"op":"expectSnapshot","component":"x","pointer":"","equals":1,"exists":true}]}`),
		"bad JSON pointer":       []byte(`{"version":1,"name":"x","steps":[{"op":"expectSnapshot","component":"x","pointer":"not/a/pointer","equals":1}]}`),
		"probe args not object":  []byte(`{"version":1,"name":"x","steps":[{"op":"expectProbe","probe":"x","args":[],"equals":1}]}`),
		"duplicate raw key":      []byte(`{"version":1,"name":"x","steps":[{"op":"expectProbe","probe":"x","args":{"a":1,"a":2},"equals":1}]}`),
		"duplicate modifier":     []byte(`{"version":1,"name":"x","steps":[{"op":"key","key":{"name":"A","modifiers":["shift","shift"]}}]}`),
		"invalid range":          []byte(`{"version":1,"name":"x","steps":[{"op":"setSelection","range":{"start":-1,"end":0}}]}`),
		"invalid size":           []byte(`{"version":1,"name":"x","steps":[{"op":"resize","size":{"width":0,"height":1},"metric":{"pxPerDp":1,"pxPerSp":1}}]}`),
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := script.Decode(bytes.NewReader(input))
			if !errors.Is(err, script.ErrInvalidScript) {
				t.Fatalf("error = %v, want ErrInvalidScript", err)
			}
			var detail *script.InvalidScriptError
			if !errors.As(err, &detail) || detail.Code() != script.CodeInvalidScript {
				t.Fatalf("error detail = %#v", err)
			}
		})
	}
}

func TestDecodeLimitsAndBoundaries(t *testing.T) {
	tooLarge := bytes.Repeat([]byte{' '}, script.DefaultMaxInput+1)
	if _, err := script.Decode(bytes.NewReader(tooLarge)); !errors.Is(err, script.ErrInvalidScript) {
		t.Fatalf("oversized input error = %v", err)
	}

	steps := make([]script.Step, script.DefaultMaxSteps)
	for i := range steps {
		steps[i] = script.Step{ID: fmt.Sprintf("s-%d", i), Op: script.OpBack}
	}
	boundary, err := json.Marshal(validScript(steps...))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = script.DecodeBytes(boundary); err != nil {
		t.Fatalf("boundary steps rejected: %v", err)
	}
	steps = append(steps, script.Step{ID: "too-many", Op: script.OpBack})
	over, _ := json.Marshal(validScript(steps...))
	if _, err = script.DecodeBytes(over); !errors.Is(err, script.ErrInvalidScript) {
		t.Fatalf("step overflow error = %v", err)
	}

	longName := strings.Repeat("x", script.DefaultMaxString+1)
	input, _ := json.Marshal(script.Script{Version: 1, Name: longName, Steps: []script.Step{}})
	if _, err = script.DecodeBytes(input); !errors.Is(err, script.ErrInvalidScript) {
		t.Fatalf("string overflow error = %v", err)
	}
}

func TestValidateSelectorComplexity(t *testing.T) {
	name := "x"
	selector := script.Selector{Name: &name}
	for range script.DefaultSelectorDepth {
		selector = script.Selector{Nth: &script.NthSelector{Selector: selector}}
	}
	s := validScript(script.Step{Op: script.OpTap, Selector: &selector})
	if err := script.Validate(s); !errors.Is(err, script.ErrInvalidScript) {
		t.Fatalf("depth error = %v", err)
	}

	all := make([]script.Selector, script.DefaultSelectorTerms)
	for index := range all {
		all[index] = script.Selector{Name: &name}
	}
	selector = script.Selector{All: all}
	s = validScript(script.Step{Op: script.OpTap, Selector: &selector})
	if err := script.Validate(s); !errors.Is(err, script.ErrInvalidScript) {
		t.Fatalf("term error = %v", err)
	}
}

func TestDecodeRejectsRecursiveJSONValue(t *testing.T) {
	value := `0`
	for range script.DefaultJSONDepth + 1 {
		value = `[` + value + `]`
	}
	input := []byte(`{"version":1,"name":"x","steps":[{"op":"expectProbe","probe":"x","equals":` + value + `}]}`)
	if _, err := script.DecodeBytes(input); !errors.Is(err, script.ErrInvalidScript) {
		t.Fatalf("recursive value error = %v", err)
	}
}

func TestValidateNonFiniteMetric(t *testing.T) {
	s := validScript(script.Step{Op: script.OpResize, Size: &script.Size{Width: 1, Height: 1}, Metric: &script.Metric{PxPerDp: 1, PxPerSp: math.Inf(1)}})
	if err := script.Validate(s); !errors.Is(err, script.ErrInvalidScript) {
		t.Fatalf("metric error = %v", err)
	}
}

func FuzzDecode(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`{"version":1,"name":"x","steps":[]}`),
		[]byte(`{"version":1,"name":"x","steps":[{"op":"tap","selector":{"label":"Save"}}]}`),
		[]byte(`null`), nil, {0xff},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := script.DecodeBytes(data)
		if err != nil && !errors.Is(err, script.ErrInvalidScript) {
			t.Fatalf("unstable error category: %T %v", err, err)
		}
	})
}
