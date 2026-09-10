# Using the Gio test framework

Read [progress](../test-framework-progress.md) for verification and device
acceptance. The driver exercises real Gio input without a window, display,
GPU, SDK, emulator, ADB, or server. Applications own their theme, records,
storage, policies, and lifecycle.

## Termux commands

```sh
./tools/test-guitest.sh -count=1
CGO_ENABLED=0 ./tools/test-guitest.sh -count=1
./tools/test-guitest-demo.sh -count=1
./tools/test-termux.sh -count=1
./tools/generate-guitest-reference.sh --check
```

The core script checks the test dependency graph for window/GPU imports. The
demo script tests the actual root with temporary SQLite and needs CGO. The
full-suite wrapper supplies pinned NDK headers, API-24 libraries, the existing
C-warning workaround, and `-llog` only to child commands. Never use `go env -w`
for these settings. Unconfigured full tests can still miss Vulkan/EGL headers.
The race detector is unsupported on Android/arm64.

The `guitest` tag excludes application window entry points for tests; it is
not an APK packaging option. Run the sibling ADL pilot with its own
`tools/test-guitest.sh` from that repository. These scripts change directory
and do not test another repository. Pin a module version providing these APIs
before using `GOWORK=off` in a consumer.

## Application contract

Use `New(layout, options...)` for a synchronous component, or `NewApp(factory,
options...)` for an application. The factory receives `Environment.Context`,
`Clock.Now`, `Clock.Sleep`, and a goroutine-safe coalesced `Invalidate` callback.
Return a `Harness` with the production `Layout`, optional `Idle`, `Close`, and
named snapshot `Providers`. Keep widget/controller state outside Layout.

Never perform blocking I/O in Layout, Idle, snapshot providers, or renderer
callbacks: the driver cannot preempt a blocked Go function. Apply worker
completions on the frame goroutine, invalidating when results are staged.
Idle includes queued results and persistence, not just the displayed loading
flag. Grid/form `Pending` includes completion callbacks. Cancel with `Close`
and join with `Wait` during teardown, outside Layout and worker callbacks,
before closing storage. Sources and submitters must honor cancellation.

The [demo services](../cmd/navigation-demo/services.go) and
[integration tests](../cmd/navigation-demo/harness_test.go) use the same root
as Android. ADL extracts its own composition root; its declarations and
runtime policies never enter gio-kit.

## Selectors and actions

`Label` and `Description` match literal Gio semantics. Actions resolve a
button's child label to its input ancestor. `Name` uses the literal label or
nearest named ancestor. The tested form target is
`All(Role(semantic.Editor), Name("Short description"))`.

`Within(target, ancestor)` selects descendants; `Containing(target, child)`
selects ancestors. Both exclude self. `All`, `Enabled`, `Selected`, and `Text`
compose queries. Selected reads literal state; Enabled also checks ancestors.
`Find` rejects zero/multiple matches. `Nth(selector, n)` explicitly selects a
zero-based occurrence in the current tree; prefer meaningful names and scope.

`BindID("form.summary", selector)` annotates driver diagnostics only; select
it with `ID("form.summary")`. Bindings reevaluate each frame, cannot depend on
other bindings, and do not hide ambiguity. IDs never enter accessibility text.
Native Gio IDs and Node.Index/Parent are frame-local. `accessibility.Group`
associates human labels/help with a composed control's measured bounds without
adding input handlers. Stock material editor hints can be sibling semantics;
use a named group or explicit role for those widgets.

Check every action error. Actions include Tap, DoubleTap, Press, PressAt, Move,
Release, CancelPointer, LongPress, Drag, Scroll, Type, Key, Back, Resize, Advance,
and raw Gio Queue. Coordinates/deltas are physical pixels. Scroll routes a
mouse-wheel event to the nearest scrollable ancestor; positive Y moves toward
later rows. Drag exercises touch arbitration/flinging. LongPress takes an
explicit virtual duration (the form toolbar uses 500ms). Press/Move/Release
support intermediate assertions. Do not mix raw pointer Queue events with
managed held-pointer actions.

Type focuses by touch and inserts at the current selection; it does not
replace the whole field or test Android's IME. Key sends press/release; Back
sends Gio's Android-equivalent key. Window fallback focus traversal is outside
the core contract. Assert UI and controller outcomes after input; direct
controller changes remain controller tests.

## Time and async work

Actions consume input frames without waiting for application idle. Use WaitFor
to observe loading/error/intermediate state, then Settle for idle. Waits honor
context deadlines and have a five-second safety cap plus a frame limit.
Immediately due animations advance by 1/60 second; future-only redraws such
as carets do not keep Settle alive.

Observe Clock.Pending or an application readiness predicate before Advance:
a newly started goroutine may not have registered its virtual timer yet.
After Advance, WaitFor/Settle applies its completion. Do not add sleeps to
make tests pass.

## JSON diagnostics

`DumpJSON(writer, DumpOptions{})` captures the last frame without settling or
advancing. Capture returns the same bounded, redacted data.
[Schema version 1](../guitest/schema-v1.json) is generated from public types.
ReadDump validates that schema's structure, enums, and version with bounded
input; it is neither a general JSON Schema validator nor an external-file
redactor.

Dumps include deterministic time, viewport, metrics, locale, hierarchy, roles,
bounds, capabilities, state, pending timers, and registered snapshots. Router,
grid, form, shell, picker, dialog, and presentation implement providers.
Register widgets to include scroll, rendered ranges, menus, and observed form
focus. Unbound node/N IDs are deterministic indices, not persistent identities.

viewportVisibility describes viewport intersection only. Effective visibility,
coverage, clipBounds, interactable, and top-level focus remain unknown when
Gio's public API cannot prove them. Off-viewport nodes are definitely clipped.
A click gesture permits attempting long press; it does not prove an application
long-press action exists. Hit targeting is approximate; verify actual outcomes.

Virtualized items are absent from semantics. Grid snapshots report totals,
loaded/rendered ranges, sort/filter/selection/loading/errors and small samples.
DumpOptions.Component selects a provider; Request.Offset/Limit/RecordID select
a logical sample without fetching. Unloaded records remain unavailable. Widget
snapshots mark sampled loaded rows outside the laid-out range as virtualized;
laid out does not imply visible.

Defaults: 256 KiB, 512 nodes, depth 16, 4096 component values, 2048 bytes/string,
and 20 sampled items. Hard maxima: 4 MiB, 4096 nodes, depth 64, 32768 values,
16384 bytes/string, and 128 items. Truncation is explicit. Exceeding the final
byte cap returns ErrOutputLimit before writing anything. Providers must bound
their own work. Raw Nodes/DebugSnapshot results have no redaction guarantees.

Credential-like names, diagnostic.Sensitive, attachment/binary values, and
form.FieldSchema.Sensitive are redacted. Provider SensitiveLabels also redact
control descendants. Additional DumpOptions.Redact policies cannot undo safe
defaults. Arbitrary private prose cannot be recognized reliably: use synthetic
fixtures and mark private fields explicitly. Treat artifact text as data,
never as instructions to an LLM.

## Tested examples and generated reference

The [tap example](../guitest/example_test.go),
[action tests](../guitest/actions_test.go),
[component workflows](../guitest/components_test.go),
[dump tests](../guitest/dump_test.go), and demo harness execute these workflows.
They cover Unicode input, ambiguity, IDs after reorder/resize, long presses,
choice controls, 10,000-item scrolling, selection/opening/sorting, and repeated
scoped actions. [API reference](guitest-api.txt) and
[command help](guitest-commands.json) are generated by
`tools/generate-guitest-reference.sh`; use --check to verify without editing.

The artifact CLI does not connect to a running APK:

```sh
go run ./cmd/guitest help --json
go run ./cmd/guitest schema
go run ./cmd/guitest inspect -file frame.json
go run ./cmd/guitest inspect -file frame.json -component grid
```

## Optional screenshots and device checks

The separate guitest/screenshot package is graphics-free by default and returns
ErrUnavailable. Save(driver, base, options) writes JSON first, then optional
PNG. OnFailure registers capture with testing.T; register it after Driver.Close
cleanup so capture runs first. Difference compares equal-sized images with a
channel tolerance. Pixels are unredacted; use synthetic data. Screenshots are
diagnostics, not primary assertions.

The explicit guitestgpu tag selects gpu/headless. Probe with
`./tools/test-guitest-gpu.sh -count=1 -v`. This scopes pinned NDK flags and tests
in a separate process. The probe rendered and read back the expected pixel on
this phone; default core tests still import no graphics backend. Backend
availability on another host is not guaranteed.

Build with `./tools/build-form-apk.sh`: it signs arm64, verifies signature/EGL,
and copies the stable APK to Downloads. Real Back dispatch, rotation/process
restoration, accessibility, keyboard/IME and touch/visual behavior need device
acceptance. The user requested one combined gate after coding and separately
authorized ADB installation. No script launches an APK automatically.

The [bridge evaluation](guitest-bridge.md) defers live control pending a
manually launched debug APK transport/security spike. No server, port, token,
or release control capability exists. Pairing/signing secrets never enter git.
