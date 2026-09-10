package guitest_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/diagnostic"
	formkit "github.com/VinceLewis/gio-kit/form"
	"github.com/VinceLewis/gio-kit/guitest"
)

func TestDumpDeterminismRedactionAndBounds(t *testing.T) {
	form, err := formkit.New([]formkit.FieldSchema{{ID: "password", Label: "Passphrase", DefaultValue: "never-print-me"}, {ID: "private", Label: "Personal detail", Sensitive: true, DefaultValue: "private-value"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(form.Close)
	w := formkit.NewWidget(form)
	th := theme()
	cycle := map[string]any{}
	cycle["cycle"] = cycle
	d, err := guitest.NewApp(func(guitest.Environment) (guitest.Harness, error) {
		return guitest.Harness{
			Layout: func(gtx layout.Context) layout.Dimensions { return w.Layout(gtx, th) },
			Providers: map[string]diagnostic.Provider{"form": w, "custom": diagnostic.ProviderFunc(func(diagnostic.Request) diagnostic.Component {
				return diagnostic.Component{Kind: "custom", State: map[string]any{"apiToken": "hidden-token-value", "bytes": []byte("never-bytes"), "wrapped": diagnostic.Sensitive{Value: "never-wrapped"}, "cycle": cycle, "text": strings.Repeat("à", 5000)}}
			})},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	var first, second bytes.Buffer
	options := guitest.DumpOptions{Redact: func(path, value string) string {
		if strings.HasSuffix(path, ".kind") {
			return "custom"
		}
		return value
	}}
	if err := d.DumpJSON(&first, options); err != nil {
		t.Fatal(err)
	}
	if err := d.DumpJSON(&second, options); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("same frame is not deterministic")
	}
	for _, secret := range []string{"never-print-me", "private-value", "hidden-token-value", "never-bytes", "never-wrapped"} {
		if strings.Contains(first.String(), secret) {
			t.Fatalf("dump leaked %s", secret)
		}
	}
	var dump guitest.Dump
	if err := json.Unmarshal(first.Bytes(), &dump); err != nil {
		t.Fatal(err)
	}
	if _, err := guitest.ReadDump(bytes.NewReader(first.Bytes()), 0); err != nil {
		t.Fatal("schema compatibility:", err)
	}
	if dump.Version != 1 || !dump.Truncated || dump.Focus != "unknown" {
		t.Fatalf("invalid dump metadata: %+v", dump)
	}
	for _, node := range dump.Nodes {
		if node.ClipBounds != nil || node.Coverage != "unknown" {
			t.Fatal("invented geometry")
		}
	}
	var untouched bytes.Buffer
	if err := d.DumpJSON(&untouched, guitest.DumpOptions{MaxBytes: 10}); !errors.Is(err, guitest.ErrOutputLimit) || untouched.Len() != 0 {
		t.Fatalf("partial output: %v", err)
	}
	if err := d.DumpJSON(shortWriter{}, options); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write: %v", err)
	}
	if _, err := d.Capture(guitest.DumpOptions{Component: "absent"}); !errors.Is(err, guitest.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestCheckedSchemaAndMalformedDumps(t *testing.T) {
	checked, err := os.ReadFile("schema-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(checked, guitest.JSONSchema()) {
		t.Fatal("schema stale: run go run ./cmd/guitest schema > guitest/schema-v1.json")
	}
	for _, input := range []string{`{}`, `{"version":99}`, `{"version":1,"nodes":"bad"}`} {
		if _, err := guitest.ReadDump(strings.NewReader(input), 0); err == nil {
			t.Fatal("invalid dump accepted")
		}
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestDumpCustomRedactionIsAdditive(t *testing.T) {
	th := theme()
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions { return material.Body1(th, "token: forbidden").Layout(gtx) })
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var out bytes.Buffer
	if err := d.DumpJSON(&out, guitest.DumpOptions{Redact: func(_, _ string) string { return "replacement" }}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "forbidden") || !strings.Contains(out.String(), diagnostic.Redacted) {
		t.Fatal("safe defaults were replaced")
	}
}
