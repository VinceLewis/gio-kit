// Package diagnostic defines bounded, application-independent component
// inspection. Providers must return promptly and never start I/O or mutate UI.
package diagnostic

import "strings"

// Request selects a small logical sample. Limit is clamped to [1,128], with a
// default of 20. RecordID requests a particular loaded record when supported.
type Request struct {
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
	RecordID string `json:"recordId,omitempty"`
}

func (r Request) Bounded() Request {
	if r.Offset < 0 {
		r.Offset = 0
	}
	if r.Limit <= 0 {
		r.Limit = 20
	}
	if r.Limit > 128 {
		r.Limit = 128
	}
	return r
}

// Component.State contains JSON-like values. Consumers must bound and redact
// it before serialization; raw snapshots, like raw Gio semantics, can be private.
// SensitiveLabels names control groups whose descendants contain private data.
type Component struct {
	Kind            string         `json:"kind"`
	State           map[string]any `json:"state"`
	SensitiveLabels []string       `json:"-"`
}

type Provider interface{ DebugSnapshot(Request) Component }
type ProviderFunc func(Request) Component

func (f ProviderFunc) DebugSnapshot(r Request) Component { return f(r) }

// Sensitive is unconditionally redacted by guitest diagnostics. Wrapping a
// value cannot be overridden by a consumer's additional redaction policy.
type Sensitive struct{ Value any }

const Redacted = "[redacted]"

// SensitiveName is a conservative default for common credential and binary
// field names. Applications should additionally mark their own private fields.
func SensitiveName(value string) bool {
	normal := strings.ToLower(value)
	for _, word := range []string{"password", "passwd", "secret", "token", "credential", "privatekey", "private_key", "signing", "keystore", "attachment"} {
		if strings.Contains(normal, word) {
			return true
		}
	}
	return false
}
