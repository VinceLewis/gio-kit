// Command guitest inspects saved diagnostic artifacts. It has no live control
// endpoint and never installs, launches or connects to an Android application.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/VinceLewis/gio-kit/guitest"
	"github.com/VinceLewis/gio-kit/guitest/script"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if out == nil {
		return errors.New("guitest: nil output")
	}
	if len(args) == 0 || args[0] == "help" {
		if len(args) > 1 && (len(args) != 2 || args[1] != "--json") {
			return errors.New("guitest: help accepts only --json")
		}
		return json.NewEncoder(out).Encode(map[string]any{"version": 1, "commands": []any{
			map[string]any{"name": "help", "description": "Machine-readable command help (also accepts --json)"},
			map[string]any{"name": "schema", "description": "Print the versioned JSON Schema"},
			map[string]any{"name": "inspect", "required": []string{"-file"}, "options": []string{"-label", "-component"}, "description": "Validate and inspect a saved JSON dump; no live app connection"},
			map[string]any{"name": "script-schema", "description": "Print the versioned script JSON Schema"},
			map[string]any{"name": "trace-schema", "description": "Print the versioned privacy-safe trace JSON Schema"},
			map[string]any{"name": "validate-script", "required": []string{"-file"}, "description": "Strictly validate a bounded script without running an application"},
			map[string]any{"name": "inspect-trace", "required": []string{"-file"}, "description": "Strictly validate and inspect a bounded privacy-safe trace"},
		}})
	}
	switch args[0] {
	case "schema":
		if len(args) != 1 {
			return errors.New("guitest: schema accepts no arguments")
		}
		_, err := out.Write(guitest.JSONSchema())
		return err
	case "script-schema":
		if len(args) != 1 {
			return errors.New("guitest: script-schema accepts no arguments")
		}
		_, err := out.Write(script.JSONSchema())
		return err
	case "trace-schema":
		if len(args) != 1 {
			return errors.New("guitest: trace-schema accepts no arguments")
		}
		_, err := out.Write(script.TraceJSONSchema())
		return err
	case "validate-script":
		return validateScriptCommand(args[1:], out)
	case "inspect-trace":
		return inspectTraceCommand(args[1:], out)
	case "inspect":
		return inspectDumpCommand(args[1:], out)
	default:
		return errors.New("guitest: unknown command; use help")
	}
}

func inspectDumpCommand(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	file := flags.String("file", "", "dump file")
	label := flags.String("label", "", "literal label")
	component := flags.String("component", "", "component name")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *file == "" || flags.NArg() != 0 {
		return errors.New("guitest: inspect requires -file")
	}
	r, err := os.Open(*file)
	if err != nil {
		return err
	}
	dump, err := guitest.ReadDump(r, 4<<20)
	err = errors.Join(err, r.Close())
	if err != nil {
		return err
	}
	var result any = dump
	if *component != "" {
		value, ok := dump.Components[*component]
		if !ok {
			return guitest.ErrNotFound
		}
		result = value
	}
	if *label != "" {
		nodes := []guitest.FrameNode{}
		for _, node := range dump.Nodes {
			if node.Label == *label {
				nodes = append(nodes, node)
			}
		}
		result = nodes
	}
	return json.NewEncoder(out).Encode(result)
}

func validateScriptCommand(args []string, out io.Writer) error {
	fileName, err := requiredFileFlag("validate-script", args)
	if err != nil {
		return err
	}
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	document, decodeErr := script.Decode(file)
	if err := errors.Join(decodeErr, file.Close()); err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(struct {
		Valid   bool `json:"valid"`
		Version int  `json:"version"`
		Steps   int  `json:"steps"`
	}{Valid: true, Version: document.Version, Steps: len(document.Steps)})
}

func inspectTraceCommand(args []string, out io.Writer) error {
	fileName, err := requiredFileFlag("inspect-trace", args)
	if err != nil {
		return err
	}
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	trace, readErr := script.ReadTrace(file, script.DefaultMaxArtifactFileBytes)
	if err := errors.Join(readErr, file.Close()); err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(trace)
}

func requiredFileFlag(command string, args []string) (string, error) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	fileName := flags.String("file", "", "JSON file")
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	if *fileName == "" || flags.NArg() != 0 {
		return "", fmt.Errorf("guitest: %s requires -file", command)
	}
	return *fileName, nil
}
