# Test Framework Progress

Plan: [test-framework-plan.md](test-framework-plan.md).

## Current checkpoint

- Started: 2026-09-09. User confirmed GPT-6 Astra / max and authorized commit,
  push, and execution of the plan.
- Status: **awaiting user device acceptance** of the foundation APK. Build,
  signing, signature, native dependency, and Downloads-copy checks passed.
- Resume here: obtain the user's result for the checklist below. After
  acceptance, continue the step-4 component semantics audit and remaining step-5
  actions, including replacing the demo test's positional field selector with
  an accessible/stable target. Do not repeat the completed feasibility proof.
- Device gates remain mandatory. Stop after packaging material UI changes for
  the user to install, launch, and test; do not advance through a pending gate.

## Delivery checklist

| Step | Deliverable | Status |
| --- | --- | --- |
| 1 | Termux input.Router pointer/editor/semantics proof | Complete |
| 2 | Layout, invalidation, time, idle, cleanup contract; demo root | Awaiting device test |
| 3 | Deterministic core frame driver | Automated checks passed |
| 4 | Reusable component semantics audit and improvements | Pending |
| 5 | Selectors and interaction actions | In progress: tap/type/key/back/resize |
| 6 | Versioned JSON dumps, JSON Schema, safe limits | Pending |
| 7 | Component snapshot providers | Pending |
| 8 | Tested human/LLM guide and adl-gio consumer pilot | Foundation guide added; complete reference/schema and pilot pending |
| 9 | Separately built optional screenshots | Pending |
| 10 | N*, G*, and form acceptance scenarios | Navigation integration coverage added; remaining scenarios pending |
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

- Source checkpoint: `17e41f2` (pushed to `origin/main`).
- Build: `./tools/build-form-apk.sh` passed using the pinned Termux pipeline.
- Artifact: `/storage/emulated/0/Download/gio-kit-form-phase3-release.apk`.
- Application: `app.giokit.cruddemo`, version `0.3.1.9`, version code `9`.
- Signature: v2 and v3 verified; existing local debug signing key.
- Only packaged native ABI: `arm64-v8a`. ELF dependencies include Android
  `libEGL.so` and exclude desktop `libEGL.so.1`.
- SHA-256 of both build output and Downloads copy:
  `a40072ba00758c3cccb2a164f27235d8f08eaa6aa932846ddbb3a8d19290adea`.
- Installation, launch, and device behavior: **not tested; user gate pending**.

Install and launch manually, then check:

1. List navigation: filter and scroll the list, open a record, and return with
   Android Back. The same list position and filter should remain.
2. Deep link, external event, and modal: exercise DEEP LINK and EXTERNAL EVENT,
   then NEW MODAL / CANCEL MODAL. Confirm correct routes and retained list state.
3. Immediate guarded Back: change Short description, dismiss the keyboard,
   and immediately press Android Back. STAY retains the edit; DISCARD returns
   to the list. Also exercise normal Save and Save & Close from a real grid row.
4. Rotation/resize: try portrait/landscape or split screen where supported,
   with a list, edited form, and modal open. Check touch targets and keyboard.
5. Restoration: leave a record open, background and manually stop/relaunch the
   app. Confirm the record route/parameters restore. This gate does not promise
   persistence of unsaved field values; the route serializer stores routes.
6. Large data/error paths: scroll the 10,000-row SQLite grid, try long presses,
   choice/reference controls, and the simulated fetch failure/retry. Test form
   input and the `server-error` submission scenario, then correct and retry.

Record the user's results here before advancing the implementation.

## Checkpoint history

- 2026-09-09: execution authorized; progress ledger created before coding.
- `b73fab4`: plan and initial progress ledger committed and pushed.
- `2be5127`: standalone Termux proof committed and pushed.
- `945d489`: deterministic core driver and lifecycle hooks committed and pushed.
- `17e41f2`: demo integration, current guide, regression tests, and APK version
  checkpoint committed and pushed. Release APK verified and copied to Downloads.
- 2026-09-09: Termux feasibility proof passed without window/GPU dependencies.
