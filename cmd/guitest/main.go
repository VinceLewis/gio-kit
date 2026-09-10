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
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 || args[0] == "help" {
		return json.NewEncoder(out).Encode(map[string]any{"version": 1, "commands": []any{
			map[string]any{"name": "help", "description": "Machine-readable command help (also accepts --json)"},
			map[string]any{"name": "schema", "description": "Print the versioned JSON Schema"},
			map[string]any{"name": "inspect", "required": []string{"-file"}, "options": []string{"-label", "-component"}, "description": "Validate and inspect a saved JSON dump; no live app connection"},
		}})
	}
	if args[0] == "schema" {
		_, err := out.Write(guitest.JSONSchema())
		return err
	}
	if args[0] != "inspect" {
		return errors.New("guitest: unknown command; use help")
	}
	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	file := flags.String("file", "", "dump file")
	label := flags.String("label", "", "literal label")
	component := flags.String("component", "", "component name")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *file == "" || flags.NArg() != 0 {
		return errors.New("guitest: inspect requires -file")
	}
	r, err := os.Open(*file)
	if err != nil {
		return err
	}
	defer r.Close()
	dump, err := guitest.ReadDump(r, 4<<20)
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
