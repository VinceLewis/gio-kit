package guitest_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"gioui.org/layout"
	"github.com/VinceLewis/gio-kit/diagnostic"
	"github.com/VinceLewis/gio-kit/guitest"
)

func TestDumpRedactsDynamicMapKeysWithoutCollisions(t *testing.T) {
	state := map[string]any{
		diagnostic.Redacted:                    "value-1",
		"alice@example.test":                   "value-2",
		"api-token-live-secret":                "value-3",
		"record-000042":                        "value-4",
		"018f47a2-72bc-7def-8123-123456789abc": "value-5",
		"01ARZ3NDEKTSV4RRFFQ69G5FAV":           "value-6",
		"customer-key-alpha":                   "value-7",
		"customer-key-beta":                    "value-8",
		"ordinary":                             "value-9",
	}
	var callbackPaths []string
	options := guitest.DumpOptions{Redact: func(path, value string) string {
		callbackPaths = append(callbackPaths, path)
		if strings.HasSuffix(path, ".<map-key>") {
			if strings.HasPrefix(value, "customer-key-") {
				return diagnostic.Redacted
			}
		}
		return value
	}}
	driver := mapStateDriver(t, state)
	var first, second bytes.Buffer
	if err := driver.DumpJSON(&first, options); err != nil {
		t.Fatal(err)
	}
	callbackPaths = nil
	if err := driver.DumpJSON(&second, options); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("map-key redaction is not deterministic")
	}
	if !json.Valid(first.Bytes()) {
		t.Fatal("redacted dump is not valid JSON")
	}
	for _, private := range []string{
		"alice@example.test", "api-token-live-secret", "record-000042",
		"018f47a2-72bc-7def-8123-123456789abc", "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		"customer-key-alpha", "customer-key-beta",
	} {
		if bytes.Contains(first.Bytes(), []byte(private)) {
			t.Fatalf("dump leaked private map key %q", private)
		}
	}
	var dump guitest.Dump
	if err := json.Unmarshal(first.Bytes(), &dump); err != nil {
		t.Fatal(err)
	}
	clean := dump.Components["audit"].State
	if len(clean) != len(state) {
		t.Fatalf("redaction collision lost entries: got %d, want %d", len(clean), len(state))
	}
	redactedKeys := 0
	for key := range clean {
		if strings.HasPrefix(key, diagnostic.Redacted) {
			redactedKeys++
		}
	}
	if redactedKeys != len(state)-1 {
		t.Fatalf("clean state keys = %v", clean)
	}
	for _, path := range callbackPaths {
		for private := range state {
			if private != "ordinary" && private != diagnostic.Redacted && strings.Contains(path, private) {
				t.Fatalf("caller key-redaction path leaked key %q: %q", private, path)
			}
		}
	}
	if _, err := guitest.ReadDump(bytes.NewReader(first.Bytes()), 0); err != nil {
		t.Fatalf("redacted dump failed schema validation: %v", err)
	}
}

func TestDumpMapKeyCollisionSuffixesStayBoundedAndUTF8(t *testing.T) {
	state := map[string]any{
		"first-private-key":  1,
		"second-private-key": 2,
		"third-private-key":  3,
		"fourth-private-key": 4,
	}
	driver := mapStateDriver(t, state)
	dump, err := driver.Capture(guitest.DumpOptions{
		MaxStringBytes: 12,
		Redact: func(path, value string) string {
			if strings.HasSuffix(path, ".<map-key>") {
				return "同じキー名前"
			}
			return value
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	clean := dump.Components["audit"].State
	if len(clean) != len(state) {
		t.Fatalf("collision suffixes lost entries: %v", clean)
	}
	for key := range clean {
		if len(key) > 12 || !utf8.ValidString(key) {
			t.Errorf("unbounded or invalid key %q (%d bytes)", key, len(key))
		}
		if strings.Contains(key, "private") {
			t.Errorf("key retained original fragment: %q", key)
		}
	}
}

func mapStateDriver(t *testing.T, state map[string]any) *guitest.Driver {
	t.Helper()
	driver, err := guitest.NewApp(func(guitest.Environment) (guitest.Harness, error) {
		return guitest.Harness{
			Layout: func(layout.Context) layout.Dimensions { return layout.Dimensions{} },
			Providers: map[string]diagnostic.Provider{
				"audit": diagnostic.ProviderFunc(func(diagnostic.Request) diagnostic.Component {
					return diagnostic.Component{Kind: "audit", State: state}
				}),
			},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := driver.Close(); err != nil {
			t.Error(err)
		}
	})
	return driver
}
