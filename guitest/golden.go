package guitest

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// goldenUpdateEnv is checked instead of a flag.Bool so this works uniformly
// across every package that calls AssertGoldenDump, without each of them
// having to register (and collide on) the same flag: `go test` doesn't
// namespace flags per package, so two packages both declaring `-update`
// would conflict. `GUITEST_UPDATE_GOLDEN=1 go test ./...` regenerates every
// checked-in golden file in one run.
const goldenUpdateEnv = "GUITEST_UPDATE_GOLDEN"

// FailureReporter is the subset of *testing.T golden-file assertions need.
type FailureReporter interface {
	Helper()
	Fatalf(format string, args ...any)
}

// AssertGoldenDump compares dump against the checked-in JSON file at path,
// byte-for-byte after canonical re-marshalling (so an existing file written
// by a different DumpOptions/field order still compares meaningfully).
// Per gio-json-test-upgrade-plan.md item 8's resolved decision, this is
// deliberately a plain checked-in testdata file reviewed via `git diff`, not
// a dedicated golden-diff tool — a mismatch's failure message includes both
// documents so the reviewer can already see the diff shape via git status,
// and re-running with GUITEST_UPDATE_GOLDEN=1 writes the new baseline.
//
// A golden dump proves the tree shape did not change; it must never be the
// sole assertion for a behavioral change (per the same plan section) since a
// meaningless whitespace-only diff is just as loud as a real regression.
func AssertGoldenDump(t FailureReporter, dump Dump, path string) {
	t.Helper()
	got, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		t.Fatalf("guitest: marshal dump for golden comparison: %v", err)
	}
	got = append(got, '\n')
	if os.Getenv(goldenUpdateEnv) != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("guitest: create golden directory: %v", err)
		}
		if err := os.WriteFile(path, got, 0644); err != nil {
			t.Fatalf("guitest: write golden file %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("guitest: golden file %s does not exist; run with %s=1 to create it:\n%s", path, goldenUpdateEnv, got)
		return
	}
	if err != nil {
		t.Fatalf("guitest: read golden file %s: %v", path, err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf("guitest: dump does not match golden file %s (re-run with %s=1 after confirming the change is intended):\n--- want\n%s\n--- got\n%s", path, goldenUpdateEnv, want, got)
	}
}
