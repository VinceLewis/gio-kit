package presentation

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/VinceLewis/gio-kit/theme"
)

func densityTestPage() Page {
	return Page{Sections: []Section{{
		Heading: "Section",
		Controls: []Control{
			{ID: "toggle", Kind: "toggle", Label: "Toggle", Enabled: true},
		},
		Lists: []List{{ID: "list", Rows: []Row{
			{ID: "r1", Fragments: []Fragment{{Kind: "field", Text: "Row one", Style: "bold"}}},
			{ID: "r2", Fragments: []Fragment{{Kind: "field", Text: "Row two", Style: "bold"}}},
		}}},
	}}}
}

func measureHeight(w *Widget, width int) int {
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Constraints: layout.Constraints{Max: image.Pt(width, 4000)}, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Now: time.Now()}
	return w.Layout(gtx, material.NewTheme()).Size.Y
}

// Proves requirement 1: the same neutral page renders measurably different
// compact vs comfortable vs spacious spacing while the page's own content and
// semantics (row/control identity and count) are untouched by density.
func TestDensityChangesSpacingButNotContentOrSemantics(t *testing.T) {
	page := densityTestPage()
	heightAt := func(density string) int {
		w := NewWidget(page)
		w.Density = density
		return measureHeight(w, 400)
	}
	compact := heightAt(theme.DensityCompact)
	comfortable := heightAt(theme.DensityComfortable)
	spacious := heightAt(theme.DensitySpacious)
	if !(compact < comfortable && comfortable < spacious) {
		t.Fatalf("density did not measurably change spacing: compact=%d comfortable=%d spacious=%d", compact, comfortable, spacious)
	}
	for _, density := range []string{theme.DensityCompact, theme.DensityComfortable, theme.DensitySpacious} {
		w := NewWidget(page)
		w.Density = density
		measureHeight(w, 400)
		section := w.Page.Sections[0]
		if len(section.Lists[0].Rows) != 2 || section.Lists[0].Rows[0].ID != "r1" || section.Lists[0].Rows[1].ID != "r2" {
			t.Fatalf("density %q altered row content/identity: %#v", density, section.Lists[0].Rows)
		}
		if len(section.Controls) != 1 || section.Controls[0].Label != "Toggle" {
			t.Fatalf("density %q altered control content: %#v", density, section.Controls)
		}
	}
}

// Proves requirement 8: a zero theme.Set and zero density strings resolve to
// exactly theme.Comfortable(), and a widget left at its zero value lays out
// identically to one explicitly configured with the default Set and the
// comfortable density name — the regression guard for other Gio-Kit
// consumers that supply no configuration.
func TestZeroMetricsAndDensityReproduceComfortableGeometry(t *testing.T) {
	page := densityTestPage()
	zero := NewWidget(page)
	if got, want := zero.metrics(), theme.Comfortable(); got != want {
		t.Fatalf("zero Set/density resolved to %+v, want Comfortable %+v", got, want)
	}
	explicit := NewWidget(page)
	explicit.Metrics = theme.DefaultSet()
	explicit.Density = theme.DensityComfortable
	if zeroHeight, explicitHeight := measureHeight(zero, 400), measureHeight(explicit, 400); zeroHeight != explicitHeight {
		t.Fatalf("zero widget diverged from explicit comfortable widget: %d vs %d", zeroHeight, explicitHeight)
	}
}

// Proves requirement 2: compactFeed and default/card styles render provably
// differently for identical row content.
func TestCardAndFeedRowStylesRenderDifferently(t *testing.T) {
	row := Row{
		ID: "row-1", StatusLabel: "Ready", StatusIcon: "check", ColorToken: "positive",
		Fragments: []Fragment{{Kind: "field", Text: "Title", Style: "bold"}, {Kind: "field", Text: "Detail about the row"}},
	}
	w := NewWidget(Page{})
	th := material.NewTheme()
	m := theme.Comfortable()
	measure := func(render func(gtx layout.Context) layout.Dimensions) layout.Dimensions {
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Constraints: layout.Constraints{Max: image.Pt(360, 4000)}, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Now: time.Now()}
		return render(gtx)
	}
	cardDims := measure(func(gtx layout.Context) layout.Dimensions { return w.layoutCardRow(gtx, th, m, row) })
	feedDims := measure(func(gtx layout.Context) layout.Dimensions { return w.layoutFeedRow(gtx, th, m, "", row) })
	if cardDims.Size.Y == feedDims.Size.Y {
		t.Fatalf("card and feed rows measured the same height for identical content: %v", cardDims)
	}
}

// Proves requirement 4: a typical compact row (title, date, time, venue,
// status) renders within two visual lines: its measured content height,
// after removing the constant row padding, is at most twice a single
// fragment's own line height.
func TestCompactFeedRowRendersWithinTwoVisualLines(t *testing.T) {
	w := NewWidget(Page{})
	th := material.NewTheme()
	m := theme.Comfortable()
	measure := func(fragments []Fragment) layout.Dimensions {
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Constraints: layout.Constraints{Max: image.Pt(360, 4000)}, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Now: time.Now()}
		row := Row{ID: "row", StatusLabel: "Confirmed", StatusIcon: "check", Fragments: fragments}
		return w.layoutFeedRow(gtx, th, m, "", row)
	}
	single := measure([]Fragment{{Kind: "field", Text: "Weekly Sync", Style: "bold"}})
	typical := measure([]Fragment{
		{Kind: "field", Text: "Weekly Sync", Style: "bold"},
		{Kind: "field", Text: "2026-09-14"},
		{Kind: "field", Text: "14:00"},
		{Kind: "field", Text: "Main Hall"},
	})
	var padOps op.Ops
	padGtx := layout.Context{Ops: &padOps, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	padding := 2 * padGtx.Dp(m.RowPaddingY)
	singleLine := single.Size.Y - padding
	if singleLine <= 0 {
		t.Fatalf("could not establish a single-line reference height: %v", single)
	}
	maxTwoLines := padding + 2*singleLine + padGtx.Dp(4)
	if typical.Size.Y > maxTwoLines {
		t.Fatalf("compact row exceeded two visual lines: got %dpx, single line %dpx, want <= %dpx", typical.Size.Y, singleLine, maxTwoLines)
	}
}

// Proves requirement 5: no rendered/semantic line starts or ends with an
// orphan separator, or consists only of punctuation or whitespace, including
// an empty middle field and an empty trailing field.
func TestFilterFeedFragmentsDropsEmptyFieldsAndOrphanSeparators(t *testing.T) {
	fragments := []Fragment{
		{Kind: "field", Text: "Title", Style: "bold"},
		{Kind: "text", Text: " - "},
		{Kind: "field", Text: "   "}, // empty middle field
		{Kind: "text", Text: " - "},
		{Kind: "field", Text: "Location"},
		{Kind: "text", Text: " - "},
		{Kind: "field", Text: ""}, // empty trailing field
	}
	got := filterFeedFragments(fragments)
	if len(got) == 0 {
		t.Fatal("filtered fragments unexpectedly empty")
	}
	if isSeparatorFragment(got[0]) || isSeparatorFragment(got[len(got)-1]) {
		t.Fatalf("run starts or ends with a separator: %#v", got)
	}
	for i := 1; i < len(got); i++ {
		if isSeparatorFragment(got[i]) && isSeparatorFragment(got[i-1]) {
			t.Fatalf("two adjacent separators survived filtering: %#v", got)
		}
	}
	texts := make([]string, len(got))
	for i, f := range got {
		texts[i] = f.Text
	}
	if want := []string{"Title", " - ", "Location"}; !equalStrings(texts, want) {
		t.Fatalf("filtered fragments = %v, want %v", texts, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Proves requirement 6: row semantics contain every available summary value
// and the status, even when a caller never sets AccessibleLabel.
func TestRowAccessibleLabelIncludesSummaryValuesAndStatus(t *testing.T) {
	row := Row{
		Fragments: []Fragment{
			{Kind: "field", Text: "Weekly Sync", Style: "bold"},
			{Kind: "field", Text: "Main Hall"},
		},
		StatusLabel: "Confirmed", StatusAccessibleLabel: "Confirmed and locked in",
	}
	label := rowAccessibleLabel(row, true)
	for _, want := range []string{"Weekly Sync", "Main Hall", "Confirmed and locked in"} {
		if !containsSubstring(label, want) {
			t.Fatalf("row accessible label %q missing %q", label, want)
		}
	}
}

func containsSubstring(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// Proves requirement 5's List.RowLayout wiring: "stack" renders one fragment
// per line while the inline default merges short fragments onto shared
// lines, so the stack rendering measures taller for identical content.
func TestRowLayoutStackRendersTallerThanInline(t *testing.T) {
	w := NewWidget(Page{})
	th := material.NewTheme()
	m := theme.Comfortable()
	fragments := []Fragment{
		{Kind: "field", Text: "Weekly Sync", Style: "bold"},
		{Kind: "field", Text: "2026-09-14"},
		{Kind: "field", Text: "14:00"},
	}
	measure := func(rowLayout string) layout.Dimensions {
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Constraints: layout.Constraints{Max: image.Pt(360, 4000)}, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Now: time.Now()}
		row := Row{ID: "row", Fragments: fragments}
		return w.layoutFeedRow(gtx, th, m, rowLayout, row)
	}
	inline := measure(RowLayoutInline)
	stacked := measure(RowLayoutStack)
	if stacked.Size.Y <= inline.Size.Y {
		t.Fatalf("stack row layout did not render taller than inline: stacked=%d inline=%d", stacked.Size.Y, inline.Size.Y)
	}
}

// Proves requirement 7: Page.Validate rejects an unknown density or list
// style and accepts every valid value, including empty.
func TestPageValidateRejectsUnknownDensityAndListStyle(t *testing.T) {
	for _, density := range []string{"", theme.DensityCompact, theme.DensityComfortable, theme.DensitySpacious} {
		page := Page{Density: density, Sections: []Section{{
			Density:   density,
			Lists:     []List{{Density: density, RowDensity: density, Style: ListStyleDefault, RowLayout: RowLayoutInline}},
			Calendars: []Calendar{{Density: density}},
			Matrices:  []Matrix{{Density: density}},
		}}}
		if err := page.Validate(); err != nil {
			t.Fatalf("valid density %q rejected: %v", density, err)
		}
	}
	for _, style := range []string{ListStyleDefault, ListStyleTable, ListStyleFeed, ListStyleCompactFeed, ListStyleCards} {
		page := Page{Sections: []Section{{Lists: []List{{Style: style}}}}}
		if err := page.Validate(); err != nil {
			t.Fatalf("valid list style %q rejected: %v", style, err)
		}
		if err := ValidateListStyle(style); err != nil {
			t.Fatalf("ValidateListStyle rejected valid style %q: %v", style, err)
		}
	}
	if err := (Page{Density: "bogus"}).Validate(); err == nil {
		t.Fatal("unknown page density accepted")
	}
	if err := (Page{Sections: []Section{{Density: "bogus"}}}).Validate(); err == nil {
		t.Fatal("unknown section density accepted")
	}
	if err := (Page{Sections: []Section{{Lists: []List{{Style: "bogus"}}}}}).Validate(); err == nil {
		t.Fatal("unknown list style accepted")
	}
	if err := (Page{Sections: []Section{{Lists: []List{{Density: "bogus"}}}}}).Validate(); err == nil {
		t.Fatal("unknown list density accepted")
	}
	if err := (Page{Sections: []Section{{Calendars: []Calendar{{Density: "bogus"}}}}}).Validate(); err == nil {
		t.Fatal("unknown calendar density accepted")
	}
	if err := (Page{Sections: []Section{{Matrices: []Matrix{{Density: "bogus"}}}}}).Validate(); err == nil {
		t.Fatal("unknown matrix density accepted")
	}
	if ValidateListStyle("bogus") == nil {
		t.Fatal("ValidateListStyle accepted an unknown style")
	}
}
