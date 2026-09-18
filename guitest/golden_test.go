package guitest_test

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"github.com/VinceLewis/gio-kit/guitest"
)

var whiteColor = color.NRGBA{R: 255, G: 255, B: 255, A: 255}

type fakeT struct {
	*testing.T
	failed string
}

func (f *fakeT) Fatalf(format string, args ...any) { f.failed = f.T.Name() }

func TestAssertGoldenDumpCreatesAndMatches(t *testing.T) {
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		paint.Fill(gtx.Ops, whiteColor)
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}, guitest.Size(64, 64))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	dump, err := d.Capture(guitest.DumpOptions{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "golden.json")
	fake := &fakeT{T: t}
	t.Setenv("GUITEST_UPDATE_GOLDEN", "1")
	guitest.AssertGoldenDump(fake, dump, path)
	if fake.failed != "" {
		t.Fatalf("unexpected failure creating golden file: %s", fake.failed)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("golden file was not written: %v", err)
	}
	t.Setenv("GUITEST_UPDATE_GOLDEN", "")
	fake = &fakeT{T: t}
	guitest.AssertGoldenDump(fake, dump, path)
	if fake.failed != "" {
		t.Fatalf("identical dump unexpectedly failed golden comparison")
	}
}

func TestAssertGoldenDumpDetectsMismatch(t *testing.T) {
	d, err := guitest.New(func(gtx layout.Context) layout.Dimensions {
		paint.Fill(gtx.Ops, whiteColor)
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}, guitest.Size(64, 64))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	dump, err := d.Capture(guitest.DumpOptions{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "golden.json")
	if err := os.WriteFile(path, []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fake := &fakeT{T: t}
	guitest.AssertGoldenDump(fake, dump, path)
	if fake.failed == "" {
		t.Fatal("expected a mismatch against a deliberately different golden file")
	}
}
