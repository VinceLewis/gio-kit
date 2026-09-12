package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/layout"
	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/guitest/script"
)

func TestMachineHelpSchemaAndInspect(t *testing.T) {
	for _, args := range [][]string{{"help", "--json"}, {"schema"}} {
		var out bytes.Buffer
		if err := run(args, &out); err != nil || !json.Valid(out.Bytes()) {
			t.Fatalf("%v: %v", args, err)
		}
	}
	d, err := guitest.New(func(layout.Context) layout.Dimensions { return layout.Dimensions{} })
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var dump bytes.Buffer
	if err = d.DumpJSON(&dump, guitest.DumpOptions{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "frame.json")
	if err = os.WriteFile(path, dump.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = run([]string{"inspect", "-file", path}, &out); err != nil {
		t.Fatal(err)
	}
	if !json.Valid(out.Bytes()) {
		t.Fatal("invalid output")
	}
}

func TestScriptAndTraceCommands(t *testing.T) {
	for _, test := range []struct {
		command string
		want    []byte
	}{
		{command: "script-schema", want: script.JSONSchema()},
		{command: "trace-schema", want: script.TraceJSONSchema()},
	} {
		var out bytes.Buffer
		if err := run([]string{test.command}, &out); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out.Bytes(), test.want) {
			t.Fatalf("%s output differs from generated schema", test.command)
		}
	}

	scriptPath := filepath.Join(t.TempDir(), "script.json")
	document := `{"version":1,"name":"private scenario name","fixture":"private fixture","steps":[]}`
	if err := os.WriteFile(scriptPath, []byte(document), 0o600); err != nil {
		t.Fatal(err)
	}
	var validated bytes.Buffer
	if err := run([]string{"validate-script", "-file", scriptPath}, &validated); err != nil {
		t.Fatal(err)
	}
	if !json.Valid(validated.Bytes()) || bytes.Contains(validated.Bytes(), []byte("private")) {
		t.Fatalf("unsafe validation output: %s", validated.Bytes())
	}

	tracePath := filepath.Join(t.TempDir(), "trace.json")
	trace := script.Trace{Version: script.TraceVersion, Status: script.StatusPassed, Steps: []script.TraceStep{}}
	traceJSON, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tracePath, traceJSON, 0o600); err != nil {
		t.Fatal(err)
	}
	var inspected bytes.Buffer
	if err := run([]string{"inspect-trace", "-file", tracePath}, &inspected); err != nil {
		t.Fatal(err)
	}
	if !json.Valid(inspected.Bytes()) || !bytes.Contains(inspected.Bytes(), []byte(`"status":"passed"`)) {
		t.Fatalf("invalid inspected trace: %s", inspected.Bytes())
	}
}

func TestScriptCommandsRejectUntrustedStructureAndUniversalRun(t *testing.T) {
	invalidScript := filepath.Join(t.TempDir(), "invalid-script.json")
	if err := os.WriteFile(invalidScript, []byte(`{"version":1,"name":"bad","steps":[],"unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"validate-script", "-file", invalidScript}, io.Discard); !errors.Is(err, script.ErrInvalidScript) {
		t.Fatalf("validate invalid script error = %v", err)
	}
	invalidTrace := filepath.Join(t.TempDir(), "invalid-trace.json")
	if err := os.WriteFile(invalidTrace, []byte(`{"version":1,"status":"passed","steps":[],"unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"inspect-trace", "-file", invalidTrace}, io.Discard); !errors.Is(err, script.ErrInvalidTrace) {
		t.Fatalf("inspect invalid trace error = %v", err)
	}
	if err := run([]string{"run", "-script", invalidScript, "-artifacts", t.TempDir()}, io.Discard); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("generic run command error = %v", err)
	}

	var help bytes.Buffer
	if err := run([]string{"help", "--json"}, &help); err != nil {
		t.Fatal(err)
	}
	var description struct {
		Commands []struct {
			Name string `json:"name"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(help.Bytes(), &description); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"script-schema": true, "trace-schema": true, "validate-script": true, "inspect-trace": true}
	for _, command := range description.Commands {
		if command.Name == "run" {
			t.Fatal("generic command unexpectedly advertises run")
		}
		delete(want, command.Name)
	}
	if len(want) != 0 {
		t.Fatalf("help omitted commands: %v", want)
	}
}
