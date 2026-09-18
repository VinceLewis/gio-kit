# Gio-Kit JSON Dump Upgrade Plan — Tree-Shape Assertions

## Status and repository split

Mirrors `gio-playwright.md`'s convention: this is one plan with a copy in
each repository, not two independent plans.

- **Primary repositories:** Gio-Kit
  `/data/data/com.termux/files/home/projects/gio-kit`; ADL-Gio
  `/data/data/com.termux/files/home/projects/adl-gio`.
- The sections **"Grounding for the sections below"** through **"Testing-
  instructions update"** are the shared plan and must stay byte-identical
  between `gio-kit/gio-json-test-upgrade-plan.md` and
  `adl-gio/gio-json-test-upgrade-plan.md`, the same way `gio-playwright.md`
  requires its two copies to match. Edit both together.
- ADL-Gio's copy has its own leading sections (ADL-Gio-only: the
  `assertTreeShape` helper and F-002/F-003/F-005 draft tests, which are
  `cmd/client` test code, not framework code) with no counterpart here.
- This file instead has its own trailing **"Gio-Kit implementation"** section
  (framework-side tasks actually done in this repository) with no
  counterpart in the ADL-Gio copy.
- Consumption order: Gio-Kit implements and publishes first; ADL-Gio re-pins
  the published version (`GOWORK=off`, no local replace) before its
  Role-enum-dependent regression tests (F-002/F-003) can compile.

## Grounding for the sections below

Read before the rest of this doc: `guitest/dump.go` (`Dump`/`FrameNode`
fields, and that `Coverage`, `Interactable`, and `Focus` are populated with
literal `"unknown"` placeholders today — they are declared in the schema but
never computed), `guitest/schema.go` (the `Role` enum is currently
`unknown|button|checkbox|editor|radio|switch` — nothing for headers, cards,
pickers, tabs, dialogs), `guitest/screenshot/screenshot.go` (pixel capture
exists but only under the opt-in `guitestgpu` build tag, off by default), and
`gio-playwright.md` (the existing, already extensive testing-instructions
document — script contract, device-only boundary list, coverage ledger
pointer). Anything below that duplicates something already in
`gio-playwright.md` or ADL-Gio's `docs/testing-coverage.md` is flagged as
such rather than repeated.

## Expert panel — what's missing from the framework

Five reviewers, each grounded in a real testing tradition, reviewed the
current `guitest` framework (dump-based tree assertions, no default
accessibility checks, no visual diffing, no property-based generation, no
golden files) plus ideas pulled from other frameworks: Espresso's
`AccessibilityChecks` (auto contrast/touch-target/label checks on every
interaction), Compose UI Testing's semantics tree and richer role/matcher
vocabulary, Playwright's accessibility-tree snapshots and time-travel trace
viewer, Percy/Chromatic/Applitools perceptual visual diffing, Flutter's
`matchesGoldenFile` pixel goldens, axe-core's WCAG rule engine, and
QuickCheck/Hypothesis-style property-based/generative testing.

- **Dr. A. Whitfield** (accessibility test automation, Espresso/axe-core
  lineage): the framework has the right primitive — a semantic tree, not
  pixels — but runs zero accessibility checks on it. Espresso fails a test by
  default if touch targets are under 48dp, contrast is too low, or a control
  has no accessible name; `guitest` computes none of this today even though
  `FrameNode.Bounds` and `Label` already carry what's needed for two of the
  three. Wants an opt-out (not opt-in) `AssertAccessible(dump)` run on every
  capture.
- **M. Reyes** (visual regression, Percy/Chromatic/Applitools lineage): the
  JSON tree can't catch a wrong color, a clipped icon glyph, or a font that
  silently fails to load — only real pixels can. Wants perceptual screenshot
  diffing (tolerant to anti-aliasing noise, not exact-byte) turned on for a
  curated set of visual-checkpoint scenarios, not everywhere — it needs the
  `guitestgpu` headless backend, which is slower and currently off.
- **Prof. Hallgren** (property-based/generative testing, QuickCheck/Hypothesis
  lineage): every M8 finding this project has logged so far (missing
  controls, wrong editor for a field type, header/card mismatch) is a case a
  human tester happened to walk into by hand. A generator that produces many
  seeded-but-deterministic synthetic ADL declarations — varying field type
  combinations, view kinds, breakpoints — and asserts one general property
  ("every declared reachable operation has a reachable, distinctly-labeled
  control") would have found F-004/F-006 without anyone hand-writing that
  scenario.
- **K. Osei** (mobile native UI test architecture, Espresso/Compose/XCTest
  lineage): the `Role` enum is too narrow for a component library this size —
  `unknown|button|checkbox|editor|radio|switch` can't even express "this is a
  table column header" or "this is a date picker," which is exactly why the
  F-002/F-003 draft tests above had to guess at role names. Compose and
  Espresso both ship dozens of semantic roles/matchers for this reason. Also:
  `Focus` is a single top-level string, so there's no way to assert keyboard
  tab order, which native frameworks treat as a first-class accessibility
  check.
- **J. Vance** (test pyramid / flake economics, Testing Library/Kent Beck
  lineage): agrees with the direction but warns against two specific
  anti-patterns — (1) golden/snapshot files that get rubber-stamped on every
  diff without being read (the standard failure mode of snapshot testing
  everywhere it's been tried) and (2) letting `guitestgpu` visual diffing
  become the default path, which reintroduces the flakiness and GPU
  dependency this framework was built specifically to avoid. Wants any golden
  file paired with a specific, named structural assertion, not used as the
  only check.

### Consensus (unanimous or majority-with-minor-dissent) — added to the plan below

1. **Compute `Coverage` for real.** Unanimous. It's declared in the schema
   and always `"unknown"` today; z-order/occlusion is derivable from existing
   `Bounds` and paint order and would have caught F-003 (header rendered
   above card rows) directly as an overlap, not just an unwanted node.
2. **Extend the `Role` enum** with the composite kinds gio-kit's own widgets
   need (`columnHeader`, `card`, `picker`, `tab`, `dialog`, `drawer`, at
   minimum). Unanimous — every panelist independently hit this gap from a
   different angle (Osei on vocabulary, Hallgren on generic property
   assertions needing a stable role name, Whitfield on labeling checks).
3. **Add a default, opt-out accessibility check** (`guitest.AssertAccessible`
   or similar) that runs on every `Capture`/`DumpJSON`: minimum touch-target
   size from `Bounds`, non-empty accessible name for interactable nodes, and
   (once item 4 lands) contrast. Majority: Whitfield proposes it, Osei and
   Vance agree it belongs in the framework rather than per-test boilerplate;
   Reyes's only dissent is that contrast can't be fully proven without real
   pixels, so the color-based check should be marked "necessary but not
   sufficient," not a full a11y guarantee.
4. **Add per-node foreground/background color** (at least for text/icon
   nodes) to `FrameNode`, to make contrast checking possible at all. Majority
   with minor dissent: Reyes would rather spend the effort on real pixel
   diffing directly; the other four hold that color-in-JSON is orders of
   magnitude cheaper to run on every test and catches most theme/contrast
   regressions before they need the slower pixel path.
5. **Add per-node validation state** (`Valid *bool`, `ErrorMessage string`)
   on form-field nodes, not only in `Dump.Components` state. Unanimous —
   directly generalizes the F-002 acceptance gap ("hidden failing fields," see
   ADL-Gio's `implementation-plan.md` line 251) into a structural assertion
   every form test can make without knowing the form's component name.
6. **Add an explicit focus/tab-order field** — a top-level
   `Dump.FocusOrder []string` of node IDs in traversal order, replacing
   `Focus`'s single opaque string — so keyboard-only navigation has a
   first-class assertion. Unanimous; currently nothing in the framework can
   express "Tab visits these controls in this order."
7. **Add a seeded property-based ADL generator** for one general reachability
   property ("every declared, policy-permitted operation has exactly one
   reachable, distinctly-labeled control across all four responsive cases"),
   run with a small fixed set of seeds in CI (not truly random each run, per
   Osei's flakiness concern). Majority with minor dissent: Vance is fine with
   it as long as failures shrink to a minimal reproducible case (standard
   QuickCheck/Hypothesis behavior) rather than dumping a large random
   fixture.
8. **Add golden dumps, but only paired with a named structural assertion, never
   bare.** Majority with Vance's explicit condition attached: a checked-in
   golden JSON per (scenario, responsive case) is allowed as a *diff-review
   aid*, but every property it would catch must also have its own
   `assertTreeShape`-style assertion — the golden file is a "did anything
   change" tripwire for human review, never the sole pass/fail signal, so it
   can't be rubber-stamped away.
9. **Turn on `guitestgpu` perceptual screenshot diffing for a small, named set
   of visual checkpoints**, using tolerance-based (not exact-byte) comparison.
   Majority with Whitfield's dissent noted: she'd rank this below items 1–7
   since it reintroduces a GPU dependency; the panel agrees it should be
   staged last, after the structural checks above are in place, and scoped to
   checkpoints structural checks can't reach (font rendering, real color
   blending) — this is the direct answer to F-005/visual-only defects that
   are provably impossible to catch from the semantic tree alone.
10. **Give every gio-kit component a disabled-state contract**: any node
    marked not interactable must have a paired reason surfaced on the node or
    its owning component state, not just `Enabled: false`. Unanimous —
    generalizes the F-005 pattern (control present, disabled, no visible
    reason) found in ADL-Gio's earlier bug classification.

Items intentionally left out of the plan: full pixel-diffing as the default
test path (Vance's flake objection, and it duplicates the device-only Gate C/D
evidence `gio-playwright.md` already assigns to the user); a universal
snapshot-everything policy (Vance); dynamic/plugin selector languages such as
CSS/XPath (already excluded by `gio-playwright.md`'s frozen decisions, and no
panelist raised a reason to revisit that).

## Schema additions required (`guitest/dump.go`, `guitest/schema.go`)

Bump `SchemaVersion` and regenerate the checked-in JSON schema
(`JSONSchema()` in `schema.go`, verified against `docs/guitest-api.txt`) once
these land:

| Field | Where | Purpose |
| --- | --- | --- |
| `FrameNode.Coverage` computed, not hardcoded | `Capture` | occlusion detection (item 1) |
| New `Role` enum values: `columnHeader`, `card`, `picker`, `tab`, `dialog`, `drawer` | `schema.go` role enum | item 2 |
| `FrameNode.Foreground`, `FrameNode.Background` (color, redaction-exempt) | `FrameNode` | contrast checks (item 4) |
| `FrameNode.Valid *bool`, `FrameNode.ErrorMessage string` | `FrameNode` | per-field validation state (item 5) |
| `Dump.FocusOrder []string` (node IDs, traversal order); deprecate single-string `Focus` or keep both | `Dump` | keyboard tab order (item 6) |
| `FrameNode.DisabledReason string` | `FrameNode` | disabled-state contract (item 10) |

None of these require new redaction categories beyond what `redact.go`
already does for `Label`/`Description`; colors and booleans are not sensitive
text.

## Testing-instructions update — making "the correct way" the default

`gio-playwright.md` is the file `AGENTS.md` already makes mandatory reading
before any gio-kit test work. Add a new section near the top, immediately
after "Intended Coverage", titled **"Required checks for every new
production-root test"**, stating:

1. Any new or changed screen must call `assertTreeShape` (ADL-Gio's helper,
   once landed in `harness_support_test.go`) across all four responsive
   cases — not just the default device size.
2. Every `Capture`/`DumpJSON` call in a new test must also pass through the
   default accessibility check (item 3) — this happens automatically once it
   ships as a `guitest` default, so the instruction becomes "do not suppress
   it," not "remember to call it."
3. Any screen exposing a declared, policy-permitted operation must be
   covered by the reachability property test (item 7) rather than a
   hand-written "control exists" assertion, once the generator exists.
4. A golden dump may be added for review convenience but never as the only
   assertion for a change (item 8) — reviewers should reject a diff whose
   only evidence is an updated golden file with no accompanying structural
   assertion change.
5. Reach for `guitestgpu` screenshot checkpoints only for defects structural
   checks cannot express (color/font rendering) and name the checkpoint's
   purpose in the test — not as a default for new screens.

Also add one line to `AGENTS.md`'s existing pointer noting that
`gio-playwright.md`'s "Required checks" section, not this plan file, is the
enforced instruction set — this plan is the design record for how it got
there, so it does not need to be kept in sync after the checks land.

## Gio-Kit implementation

### Done (items 1, 2, 5, 6, 10 above)

1. `guitest/schema.go`: `columnHeader`, `card`, `picker`, `tab`, `dialog`,
   `drawer` added to the `role` enum choices. `gioui.org/io/semantic` has no
   distinct `ClassOp` for any of them (confirmed), so `Capture` in
   `guitest/dump.go` derives them from each component's own `DebugSnapshot`
   state (label-text matched — see the caveat below) instead of
   `n.Desc.Class`. `tab` is declared but never populated: gio-kit's shell has
   no widget backing a distinct tab bar (its wide-mode nav rail reuses
   drawer-item rendering).
2. `guitest/dump.go` `FrameNode`: `Valid *bool`, `ErrorMessage string`,
   `DisabledReason string` added, populated from existing `DebugSnapshot`
   state (`form` field errors, `shell`/`picker` `DisabledReason`) via the
   same label-text matching.
3. `guitest/dump.go` `Capture`: `Coverage` is now computed for real
   (`computeCoverage` in `guitest/dump.go`) — for each node, a later-painted
   node (Gio paints in document order) that is neither its ancestor nor its
   descendant and whose `ViewportIntersection` fully or partially contains
   the node's own is marked `"covered"`/`"partially_covered"`; otherwise
   `"uncovered"`. Covered by `TestCoverageDetectsOverlappingPaintOrder`
   (`guitest/occlusion_test.go`).
4. `guitest/dump.go` `Dump.FocusOrder`: added, approximated from paint order
   (documented caveat: not a verified platform tab order).
5. `SchemaVersion` bumped to 2, `docs/guitest-api.txt`/`schema-v1.json`
   regenerated, `guitest/dump_test.go` updated for the new fields.
6. `gio-playwright.md`'s "Required checks" section added, byte-identical in
   both repos.

**Known limitation, not yet fixed**: role/error/reason population above has
no structural link from a semantic node to its owning component — it matches
on label text (the same pattern `sensitiveLabels` already used), which is a
best-effort heuristic, not a guaranteed-correct ownership pointer. A grid-mode
instance of this was found and fixed: `grid.Widget.DebugSnapshot` now reports
`resolvedViewMode` ("table"/"cards") so `columnHeader` vs `card` role
assignment is keyed off the component's actual resolved mode
(`grid/diagnostic.go`, `grid/widget.go`) rather than guessing from the
ambiguous `"Open "`/`"Sort by "` label prefixes alone — see
`TestGridRoleReflectsResolvedViewMode` (`guitest/grid_role_test.go`). The
general label-matching fragility for other components (form/shell/picker/
dialog) remains open; a real provider→node ownership link would need a new
`diagnostic` API and is out of scope here.

### Remaining (items 3, 4, 7, 8, 9 above) — product decisions resolved, not yet implemented

The product owner has resolved all five open decisions; the items below are
now unblocked and ready to implement in gio-kit:

7. **Item 3 — default accessibility check.** `guitest.AssertAccessible` runs
   opt-out by default on every `Capture`/`DumpJSON` (decision: opt-out, not
   opt-in) — minimum touch-target size from `Bounds`, non-empty accessible
   name for interactable nodes, and (once item 4 lands) contrast.
8. **Item 4 — per-node color.** Capture `Foreground`/`Background` by
   instrumenting Gio paint ops (decision: read `paint.ColorOp`/
   `paint.LinearGradientOp` at capture time, not a separate theme-introspection
   path), scoped to text/icon nodes.
9. **Item 7 — reachability generator.** Scope the seeded property-based
   generator to field types and view kinds only (decision: not full ADL
   operation semantics) — "every declared, policy-permitted operation has
   exactly one reachable, distinctly-labeled control across all four
   responsive cases," fixed seed set, shrink-on-failure.
10. **Item 8 — golden dumps.** Plain checked-in `testdata` JSON files,
    reviewed via `git diff` (decision: no dedicated golden-diff tool) — never
    the sole assertion for a change, per the existing consensus text above.
11. **Item 9 — GPU diffing.** Limit `guitestgpu` perceptual diffing to a
    handful of explicitly named checkpoints (decision: not a general
    screenshot-everything mode), tolerance-based comparison, staged last.
