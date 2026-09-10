package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/layout"
	"github.com/VinceLewis/gio-kit/guitest"
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
