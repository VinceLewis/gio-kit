package form

import (
	"testing"
	"time"
)

func TestTemporalPickerParsesAndFormatsEveryFieldType(t *testing.T) {
	fallback := time.Date(2026, 9, 10, 8, 0, 0, 0, time.FixedZone("CEST", 2*60*60))
	for _, test := range []struct {
		field FieldType
		input string
		want  string
	}{
		{FieldDate, "2028-02-29", "2028-02-29"},
		{FieldTime, "23:59:30", "23:59:30"},
		{FieldDateTime, "2028-02-29T23:59:30+02:00", "2028-02-29T23:59:30+02:00"},
	} {
		draft := temporalDraft(test.field, test.input, fallback)
		if got := temporalValue(test.field, draft, test.input); got != test.want {
			t.Fatalf("%v value = %q, want %q", test.field, got, test.want)
		}
	}
}

func TestTemporalPickerFallsBackWithoutAcceptingMalformedText(t *testing.T) {
	fallback := time.Date(2026, 9, 10, 8, 7, 0, 0, time.UTC)
	if got := temporalDraft(FieldDate, "2026-20-75", fallback); !got.Equal(fallback) {
		t.Fatalf("malformed date draft = %v", got)
	}
	if got := temporalValue(FieldTime, temporalDraft(FieldTime, "29:75", fallback), "29:75"); got != "08:07" {
		t.Fatalf("malformed time fallback = %q", got)
	}
}
