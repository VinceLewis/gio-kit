# Test Framework Progress

Plan: [test-framework-plan.md](test-framework-plan.md).

## Current checkpoint

- Started: 2026-09-09. User confirmed GPT-6 Astra / max and authorized commit,
  push, and execution of the plan.
- Status: foundation and demo integration pass automated checks; preparing the
  first release APK/device gate. See verification below for host limitations.
- Resume here: finish APK verification, then wait for the user's device result.
  After acceptance, continue the step-4 component semantics audit and remaining
  step-5 actions. Do not repeat the completed feasibility proof.
- Device gates remain mandatory. Stop after packaging material UI changes for
  the user to install, launch, and test; do not advance through a pending gate.

## Delivery checklist

| Step | Deliverable | Status |
| --- | --- | --- |
| 1 | Termux input.Router pointer/editor/semantics proof | Complete |
| 2 | Layout, invalidation, time, idle, cleanup contract; demo root | Automated checks passed; APK gate next |
| 3 | Deterministic core frame driver | Automated checks passed |
| 4 | Reusable component semantics audit and improvements | Pending |
| 5 | Selectors and interaction actions | In progress: tap/type/key/back/resize |
| 6 | Versioned JSON dumps, JSON Schema, safe limits | Pending |
| 7 | Component snapshot providers | Pending |
| 8 | Tested human/LLM guide and adl-gio consumer pilot | Foundation guide added; complete reference/schema and pilot pending |
| 9 | Separately built optional screenshots | Pending |
| 10 | N*, G*, and form acceptance scenarios | Pending |
| 11 | Evaluate manually launched device control bridge | Pending |

Status values: Pending, In progress, Automated checks passed, Awaiting device
test, Complete, or Deferred with an explicit reason. A host check or signed APK
does not constitute device acceptance.

## Verification and decisions

- Baseline `gio-kit`: `481a224`, branch `main`, tracking `origin/main`; only the
  untracked framework plan existed before execution.
- `adl-gio` has pre-existing modifications to `implementation-plan.md` and
  `implementation-evidence.md`. Preserve them while doing the framework work;
  inspect its commit rules again at the consumer integration checkpoint.
- Pinned baseline: Go language 1.24.0 and Gio v0.10.2. No toolchain migration is
  part of this plan.
- Core testing must exclude window/GPU dependencies at compile time. Graphics
  and Android behavior have separate, explicitly reported verification gates.
- Step 1: `./tools/test-guitest.sh -count=1 -v` passed on Android/arm64 under
  Termux. It checks the test dependency graph, drives a real material button
  and Unicode editor through `input.Router`, reads semantics, and runs vet.
- Gio v0.10.2 public semantics expose class, label, description, bounds,
  selected/disabled state, and click/scroll gestures. They do not expose a
  complete clip stack or a semantic-to-focus-tag mapping. Do not invent those
  facts or access Gio internals; use explicit component cooperation later.
- Core driver: `./tools/test-guitest.sh -count=1 -v` and its `CGO_ENABLED=0`
  variant passed. Coverage includes selectors/ambiguity, real pointer/editor/
  back input, disabled/offscreen targets, resize, virtual timers, queued
  completion/idle, cancellation, cleanup on factory failure, frame limits,
  and future-only redraws.
- Actions consume input frames without waiting for all external work; call
  `WaitFor` to observe loading or `Settle` for application idle. Both waits have
  a five-second safety cap and honor earlier context deadlines. Long press,
  drag/scroll, scoped selectors, stable IDs, and dumps remain pending.
- Stock material editor hints are sibling semantics over their editor, not
  descendants. Semantic hit-testing cannot prove an input sink or exact
  occlusion. The core ignores paint-only labels during targeting; stronger
  component diagnostics belong to the semantics/snapshot milestones.
- The form also registers a coincident, pass-through long-press area as a
  sibling of each editor. Targeting permits that unnamed area; a regression
  test asserts that real input reaches the form controller. Exact input-sink
  ownership is still not exposed by Gio public semantics.
- Demo tests: `./tools/test-guitest-demo.sh -count=1 -v` passed, including
  N1/N2/N3/N4/N6/N7 routed modal, editor, guarded Back, and resize; N5 fresh-root
  reconstruction from persisted state; N8 delayed navigation and disposal;
  database-startup cleanup; and coalesced persistence/error propagation.
- Those tests exposed and fixed a stale dirty-flag bug when Back immediately
  follows typing. The guard now reads the form controller's current state.
- Demo lifecycle tests also passed ten consecutive runs after the cancellation
  and guard fixes. This exercises scheduling variability but is not a race
  detector result.
- Application services now isolate the window, inject artificial delays,
  apply external navigation on the frame goroutine, cancel/join startup and
  external work, and write navigation state asynchronously with atomic replace.
- Focused tests and vet passed for router, grid (including SQLite), form,
  shell, presentation, dialog, picker, and guitest.
- Normal `go test ./...` and `go vet ./...` fail at
  `gioui.org@v0.10.2/internal/vk/vulkan_android.go:12:10`:
  `fatal error: 'vulkan/vulkan.h' file not found` (followed by
  `1 error generated.`). This is an incomplete host gate, not a pass.
- `go test -race ./guitest` cannot run: `-race is not supported on android/arm64`.
  Concurrent lifecycle tests remain required; they do not replace race testing
  on a supported host or device acceptance here.

## Device acceptance

No new APK or device result yet.

## Checkpoint history

- 2026-09-09: execution authorized; progress ledger created before coding.
- `b73fab4`: plan and initial progress ledger committed and pushed.
- `2be5127`: standalone Termux proof committed and pushed.
- `945d489`: deterministic core driver and lifecycle hooks committed and pushed.
- 2026-09-09: Termux feasibility proof passed without window/GPU dependencies.
