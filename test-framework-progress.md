# Test Framework Progress

Plan: [test-framework-plan.md](test-framework-plan.md).

## Current checkpoint

- 2026-09-10: framework-first extension implementation is complete through the
  Gio-Kit Gate B checkpoint. Added viewport/keyboard-contraction matrices,
  focus/selection/composition-shaped editing, a bounded asynchronous adapter
  behind Gio's real clipboard commands, pointer-edge/threshold/cancel/recovery
  cases, repeated real-root reconstruction, rapid-navigation ordering, and
  direct component snapshot/semantic contracts. `./tools/test-checkpoint.sh
  -count=1` passed, including CGO-on/off core tests, the production demo root,
  full Termux tests/vet, and generated-reference verification. The production
  demo dependency graph does not import `guitest`; no rendering, packaging, or
  Android platform seam changed, so no redundant APK or Gate C run is required.
  Real IME/clipboard policy/insets, TalkBack, Android lifecycle/process, touch
  physics, and device GPU/display behavior remain device-only.
- Publication checkpoint: Gio-Kit `7b6c07acf625` is pushed to `origin/main`.
  ADL-Gio `b010b31` selects the published pseudo-version
  `v0.0.0-20260910084443-7b6c07acf625`; its workspace-independent production
  root, full verification, module verification, Android-tagged tests/vet, and
  signed arm64 build passed. The local evidence APK SHA-256 is
  `5544f9a26b9eef794f2c115ebde1ccf59e8ee9878f4a49891c97f7fe17ea887e`.
  It was not copied, installed, launched, or offered as a frozen candidate
  because ADL B-001 remains open.
- 2026-09-10: user approved `testing-implementation-2.md` for execution. Gates
  A–D replace the historical per-material-change and three-requirement device
  pauses. In-process production-root tests are the normal UI development gate;
  platform smoke is triggered by a new or materially changed platform seam;
  comprehensive device acceptance is performed against a frozen candidate.
  The user continues to own APK installation, launch, lifecycle actions, and
  visual acceptance; agents do not automate them through ADB or equivalent
  mechanisms.
- 2026-09-10: user authorized execution of `testing-debug-bridge.md` through
  the definitive Phase 1 feasibility decision. Phase 1 stopped negative at its
  public Android event-path condition: Gio v0.10.2 exposes no supported way to
  queue synthetic pointer/editor events into the live window router. No tagged
  server or APK was retained, and Phase 2 must not proceed.
- Started: 2026-09-09. User confirmed GPT-6 Astra / max and authorized commit,
  push, and execution of the plan.
- Status: **coding and automated verification complete; device acceptance pending**. On
  2026-09-10 the user reported installing and testing the latest APK: all well.
- Termux follow-up: full default-package tests and vet now pass through
  `./tools/test-termux.sh -count=1`. This does not advance the device gate.
- Resume here: collect the combined device results below. Both APKs are
  installed through user-authorized ADB. Do not repeat completed
  feasibility checks or advance device acceptance without user results.
- 2026-09-10 user steering: finish all required coding and automated checks
  before building APKs, then perform one combined user device test. This
  supersedes the intermediate packaging pauses for this framework work.
  Device acceptance is still required; later steering permits ADB installation.
- Later 2026-09-10 steering explicitly authorizes installation through wireless
  ADB. Launch and visual acceptance remain user steps. Pairing and connection
  succeeded using screenshots from Android's `Pictures/Screenshots` folder;
  both verified APKs installed successfully. Framework tests remain independent
  of ADB. No app was automatically launched.

## Delivery checklist

| Step | Deliverable | Status |
| --- | --- | --- |
| 1 | Termux input.Router pointer/editor/semantics proof | Complete |
| 2 | Layout, invalidation, time, idle, cleanup contract; demo root | Complete |
| 3 | Deterministic core frame driver | Complete |
| 4 | Reusable component semantics audit and improvements | Automated checks passed; combined device gate pending |
| 5 | Selectors and interaction actions | Complete |
| 6 | Versioned JSON dumps, JSON Schema, safe limits | Complete |
| 7 | Component snapshot providers | Complete |
| 8 | Tested human/LLM guide and adl-gio consumer pilot | Complete; published dependency verified with local overrides disabled |
| 9 | Separately built optional screenshots | Complete; headless pixel readback passed on this host |
| 10 | N*, G*, and form acceptance scenarios | Automated checks passed; combined device gate pending |
| 11 | Evaluate manually launched device control bridge | Complete; Phase 1 negative on the required live input seam |

Status values: Pending, In progress, Automated checks passed, Awaiting device
test, Complete, or Deferred with an explicit reason. A host check or signed APK
does not constitute device acceptance.

## Completion verification (2026-09-10)

- Implemented measured accessibility groups and audited form, grid, shell,
  picker, dialog and presentation semantics. Removed positional demo targeting.
  Invalid/submitting form action buttons now disable real input as well as
  appearing disabled; this was exposed by the new real-input tests.
- Added scoped/name/state/occurrence selectors, test-only ID bindings, managed
  pointer phases, double tap, long press, drag and wheel scroll. Tests exercise
  touch arbitration and 10,000-item virtualization without graphics imports.
- Added bounded redacted schema-v1 captures, schema validation, snapshot
  providers for all seven component kinds, local artifact CLI and generated
  human/LLM references. Unknown clip/coverage/focus facts remain explicit.
- Added optional tagged headless rendering, JSON/PNG artifact pairing,
  screenshot-on-failure, and tolerant image comparison. The real tagged probe
  rendered/read the expected red pixel on this Termux host; PNGs are unredacted.
- Extended routed acceptance coverage for grid selection/sort/retry/empty,
  stale completion, form validation/submission/retry, and composed widgets.
  See [coverage matrix](docs/guitest-acceptance.md).
- ADL pilot extracts its existing root behind storage, clock, invalidation,
  provider and cleanup hooks. Tests route add/edit/guarded Back and reconstruct
  state from a temporary database/session; a virtual worker proves joined
  cancellation. No ADL policy enters gio-kit. Default and Android-tagged ADL
  tests/vet passed with the temporary workspace and the published dependency,
  with `GOWORK=off`, no replacement, and successful module verification.
- Core checks (including CGO disabled), demo checks, full Termux tests/vet,
  generated-reference checks and the isolated GPU test passed. Final
  controller cleanup and the consumer pilot also passed before publication.
- Screenshot artifact failure now removes stale PNGs before writing a new tree
  and removes partial images. Core tests/vet and the real tagged GPU probe pass
  after this final helper fix.
- ADB paired and connected successfully using the user-provided screenshots;
  both APK installations succeeded. The user confirmed both manual launches.
  Both processes were running; their error-level logs showed only Android's
  ashmem deprecation warning. Workflow acceptance remains pending.
  Pairing codes/screenshots are outside the repositories and are not committed.
- Bridge Phase 1 stopped negative at the supported live-input seam; see
  [the decision note](docs/guitest-bridge.md). `input.Source` cannot inject
  events into the window-owned router, and private-router, shadow-router, and
  application-controller substitutes violate the spike rules. Release builds
  contain no control bridge.
- Consumer integration is committed as `adl-gio@c5d29eb`, pinning published
  `gio-kit@v0.0.0-20260910002538-1a65d354f88e`. Full consumer verification,
  tagged root tests/vet, module verification and the signed Android build pass
  with `GOWORK=off`. No local workspace or replacement is required.
- That final-pin ADL build has SHA-256
  `7a6224f2d375db7850ae6bc4f262f31a358099a85e83081644734aff75fd8072`;
  the verification copy is `/data/data/com.termux/files/usr/tmp/adl-framework-final-pin.apk`.
  The installed/Downloads APK below remains the device-test checkpoint. Its
  production dependency graph excludes the only subsequent library code change,
  the optional screenshot helper. No further installation interrupted testing.

## Foundation verification and decisions (historical)

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
- Unconfigured `go test ./...` and `go vet ./...` failed at
  `gioui.org@v0.10.2/internal/vk/vulkan_android.go:12:10`:
  `fatal error: 'vulkan/vulkan.h' file not found` (followed by
  `1 error generated.`). The headers already exist in the pinned NDK; the
  default compiler search path omitted them.
- 2026-09-09: `./tools/test-termux.sh -count=1` passed full default-package
  tests and vet, including when invoked from outside the repository. It uses
  the APK build's NDK include/API-24 library paths and C-warning workaround,
  plus `-llog` for standalone CGO test executables. The wrapper validates
  Android/arm64, CGO, headers, and libraries; its CGO-disabled rejection was
  checked. No global Go settings, packages, or pinned toolchain versions changed.
  Some NDK/compiler warnings remain on compilation. This resolves the header
  compilation blocker, not GPU/headless runtime support or device acceptance.
- After adding the wrapper, `CGO_ENABLED=0 ./tools/test-guitest.sh -count=1`
  and `./tools/test-guitest-demo.sh -count=1` both passed, including their
  dependency-isolation checks and vet. The default suite does not replace them.
- `go test -race ./guitest` cannot run: `-race is not supported on android/arm64`.
  Concurrent lifecycle tests remain required; they do not replace race testing
  on a supported host or device acceptance here.

## Combined device acceptance (pending)

- Gio Kit: `/storage/emulated/0/Download/gio-kit-form-phase3-release.apk`,
  `app.giokit.cruddemo`, version `0.3.1.10`, code `10`.
- ADL pilot: `/storage/emulated/0/Download/adl-client-foundation-arm64.apk`,
  `app.adl.client`, version `0.1.0.2`, code `2`.
- Both build/sign/Downloads-copy pipelines passed. Signatures use the existing
  local debug key with v2/v3 verification. Both target only arm64 and Android
  EGL. The installed copies match these SHA-256 hashes:
  - Gio Kit: `1253136d0ebcd857320e2115238b0ebddc625c11fd4bcc10225d5653de442363`.
  - ADL pilot: `11a90b92984ee57cdf040c514672526d50e26e71a25823839b4c0b84c3271d57`.
- APK framework source: `f9ddc8da792e`, published before the ADL dependency pin
  `v0.0.0-20260910000804-f9ddc8da792e`. The subsequent screenshot artifact helper
  fix does not enter either application's production dependency graph.
- Both APKs installed successfully through user-authorized ADB; installed
  package versions were checked. The user confirmed launching both apps.
  Workflow acceptance remains pending; installation/launch do not pass this gate.

Launch both installed APKs, then check:

1. Gio Kit: edit Short description; confirm invalid/busy Save buttons reject
   input. Trigger required validation and the `server-error` submission,
   correct/retry, and check Save / Save & Close. Test keyboard Unicode input,
   long-press COPY/PASTE, choices, references and disabled attachment controls.
2. Gio Kit: select rows without opening them, open a key cell, sort/filter,
   scroll the 10,000-row grid in cards and table modes, and exercise fetch
   failure/RETRY/empty states. Return with Android Back and check retained state.
3. Both apps: navigate, open forms/dialogs, edit and use Android Back to test
   STAY/DISCARD. Check touch targets and accessible labels (TalkBack if used),
   including repeated actions in composed views and unavailable controls.
4. Both apps: rotate or resize where supported with a list, edited form or
   dialog open. Check software keyboard, scrolling and redraws. Background,
   manually stop, and relaunch to verify restoration. Gio Kit preserves routes;
   its serializer does not promise persistence of unsaved field values.
5. ADL: exercise normal list/form/presentation/reference flows and rapid
   navigation while loading, then relaunch. Check that late results do not
   reopen old screens or lose the current session/theme/navigation state.

Host semantic/GPU tests do not replace these Android checks. The live bridge
is deferred as evaluated in docs/guitest-bridge.md; no control server is shipped.

## Foundation device acceptance (accepted)

- Source checkpoint: `17e41f2` (pushed to `origin/main`).
- Build: `./tools/build-form-apk.sh` passed using the pinned Termux pipeline.
- Artifact: `/storage/emulated/0/Download/gio-kit-form-phase3-release.apk`.
- Application: `app.giokit.cruddemo`, version `0.3.1.9`, version code `9`.
- Signature: v2 and v3 verified; existing local debug signing key.
- Only packaged native ABI: `arm64-v8a`. ELF dependencies include Android
  `libEGL.so` and exclude desktop `libEGL.so.1`.
- SHA-256 of both build output and Downloads copy:
  `a40072ba00758c3cccb2a164f27235d8f08eaa6aa932846ddbb3a8d19290adea`.
- Installation, launch, and device behavior: **accepted by user 2026-09-10**.

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

User result (2026-09-10): installed and tested the latest version; all is well.

## Checkpoint history

- 2026-09-09: execution authorized; progress ledger created before coding.
- `b73fab4`: plan and initial progress ledger committed and pushed.
- `2be5127`: standalone Termux proof committed and pushed.
- `945d489`: deterministic core driver and lifecycle hooks committed and pushed.
- `17e41f2`: demo integration, current guide, regression tests, and APK version
  checkpoint committed and pushed. Release APK verified and copied to Downloads.
- 2026-09-09: Termux feasibility proof passed without window/GPU dependencies.
- 2026-09-09: verified full-suite Termux wrapper added and header-blocker notes
  corrected; framework implementation remains paused for user device acceptance.
