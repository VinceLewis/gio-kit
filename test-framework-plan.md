# Gio Test Framework Plan

Implementation status, verification evidence, and restart instructions are in
[test-framework-progress.md](test-framework-progress.md).

## Purpose

Add a repository-owned testing framework that lets applications using `gio-kit` inspect and drive their real Gio UI quickly and deterministically. The framework should support automated tests, developer diagnostics, and LLM-assisted debugging without coupling the reusable module to the demonstration application's data or policies.

The primary diagnostic output should be structured JSON. Screenshots should be optional supporting artifacts.

The primary development host is an Android phone running Termux on arm64. The
core workflow must therefore work locally without a desktop display, desktop
GPU libraries, ADB, an emulator, or a second computer. Android packaging and
user-performed device checks remain separate verification gates.

## Goals

- Exercise the same Gio widget input path used by real pointer and keyboard events.
- Drive taps, typing, scrolling, long presses, keyboard actions, and Android back behavior.
- Inspect the current frame as a semantic tree with geometry and interaction state.
- Include useful logical state for virtualized or clipped components that cannot be inferred from drawing operations alone.
- Make asynchronous behavior deterministic and safe against stale completion.
- Keep the core framework pure Go and usable without a window or Android device.
- Keep the core package buildable and testable under Termux without importing
  `gioui.org/app`, `gioui.org/gpu`, or a platform graphics backend.
- Provide optional screenshot capture where a supported headless GPU backend is available.
- Eventually allow a developer or LLM to inspect and drive an already running debug APK from Termux.

## Proposed Packages

### `guitest`

The main in-process driver. It owns:

- `input.Router`
- `op.Ops`
- viewport dimensions and device metrics
- locale and deterministic clock
- the application's root layout function
- frame settling and asynchronous-idle coordination
- semantic queries and action injection

A target application should expose its root UI as a layout function independent of `app.Window`, for example:

```go
type LayoutFunc func(layout.Context) layout.Dimensions
```

`LayoutFunc` is the minimum rendering seam, not the complete application
contract. Real applications also need injectable invalidation, deterministic
time, pending-work/idle reporting, and cleanup. The framework should define a
small optional harness contract for those concerns while keeping plain layout
functions convenient for component tests. It must not require an application
to construct `app.Window`, call `app.DataDir`, or start uncontrolled goroutines.
The driver owns frame scheduling; application callbacks request another frame
through the injected invalidator.

Illustrative test usage:

```go
ui := guitest.New(rootLayout, guitest.Size(420, 820))

ui.Tap(guitest.Label("New incident"))
ui.Type(guitest.Label("Short description"), "Network outage")
ui.Tap(guitest.Label("Save"))
ui.AssertText("Incident saved")
ui.DumpJSON(testOutput)
ui.Screenshot("saved-incident.png")
```

The exact API should follow normal Go error handling and integrate cleanly with `testing.T`; the example only describes the intended ergonomics.

### Optional component snapshot interfaces

Frame semantics only describe nodes that Gio laid out. Stateful components should optionally implement a generic debug snapshot interface so the dump can explain state that is not represented by current drawing operations.

Likely providers include:

- router: route, stack, modals, restoration version, and active guard
- grid: total rows, rendered and loaded ranges, paging, selection, sorting, filtering, loading, and errors
- form: field values, visibility, read-only state, validation, dirty state, submission, and errors
- shell: active navigation item and open overlays
- dialog and picker: open state, available actions, selection, loading, and errors

The interfaces must remain generic and must not expose demo-specific record or backend types.

## Frame and Input Model

Each driver action should follow Gio's normal immediate-mode sequence:

1. Lay out a frame so widgets register handlers and semantic operations.
2. Pass the operations to `input.Router.Frame`.
3. Resolve the target from the semantic tree and its bounds.
4. Queue the appropriate Gio input events.
5. Lay out subsequent frames so widgets consume the events and update state.
6. Continue until an explicit stable/idle condition or timeout is reached.

Supported actions should include:

- tap and double tap
- pointer press, move, and release
- long press using the deterministic clock
- drag and scrolling
- editor focus and text insertion using key/edit events
- key press and submission
- Android back-equivalent key events
- resize, rotation-equivalent viewport changes, and metric changes
- explicit time advancement

Tests should not depend on arbitrary sleeps. The framework should offer `Advance`, `WaitFor`, and an application-provided asynchronous-idle hook. Timeouts remain necessary to prevent deadlocks.

Actions must allow inspection of intermediate loading/error states: process
their input frames, then let callers explicitly choose `WaitFor` or `Settle`.
Settling may advance immediately due animation frames by a fixed interval, but
must not fast-forward endlessly through future-only redraws such as caret
blinking. Application delays use the injected clock and explicit advancement.

## Selectors and Semantics

Selectors should primarily use the accessibility semantics already emitted by Gio:

- semantic class or role
- label and description
- enabled, disabled, and selected state
- supported gestures
- bounds and tree ancestry

Expected selectors include:

- label or accessible name
- role plus label
- text presence
- descendant or ancestor scope
- occurrence index for unavoidable duplicates
- stable component/test ID where available

Custom grid rows, form choices, validation errors, modal actions, and other composed controls must emit complete semantics. Improving this coverage benefits both tests and Android accessibility.

Stable test IDs may be needed for duplicated, localized, or dynamic labels. They must not degrade screen-reader output. The implementation should first prefer meaningful accessible names, then design a test-only annotation mechanism rather than placing machine-oriented IDs in user-facing semantic descriptions.

Semantic IDs assigned by Gio must not be persisted or treated as stable across frames.

## JSON Diagnostic Dump

`DumpJSON` should be the primary representation for developer and LLM debugging. It should combine the actual current frame with optional component snapshots.

Illustrative output:

```json
{
  "version": 1,
  "viewport": {"width": 420, "height": 820},
  "route": "/incident/123",
  "focus": "form.short_description",
  "frame": [
    {
      "id": "form.short_description",
      "role": "editor",
      "label": "Short description",
      "value": "Network outage",
      "bounds": {"x": 16, "y": 214, "width": 388, "height": 56},
      "visibility": "visible",
      "enabled": true,
      "selected": false,
      "actions": ["focus", "type"]
    }
  ],
  "components": {
    "form": {
      "dirty": true,
      "submitting": false,
      "errors": {}
    },
    "grid": {
      "totalRows": 10000,
      "renderedRange": [40, 56],
      "scrollOffset": 1832,
      "loading": false
    }
  }
}
```

The schema should include, where applicable:

- schema version and capture timestamp or deterministic frame time
- viewport, metrics, locale, and orientation
- route, navigation stack, modal stack, and focused control
- semantic hierarchy and stable diagnostic IDs
- role, label, description, value, validation message, and state
- absolute bounds, clip bounds, and viewport intersection
- supported actions and whether the node is currently interactable
- component state such as loading, retry, empty, dirty, selected, or submitting
- scroll positions, rendered ranges, loaded ranges, and total item counts
- pending asynchronous work and the last relevant error

### Visibility terminology

- `visible`: the node intersects the viewport and its effective clip.
- `partially_visible`: only part of the node is inside the effective visible region.
- `clipped`: the node was laid out but lies outside its effective clip or viewport.
- `covered`: known to be behind a modal or overlay.
- `virtualized`: the logical item exists but was not laid out in this frame.

The dump must not claim exact occlusion where it cannot be proven. Gio v0.10.2
public semantics expose bounds but not the full clip stack or an exact mapping
to input handlers. Labels can also be sibling semantics over a control (for
example, a material editor hint). Report clipping/coverage as unknown unless a
component provider or a supported public API establishes it. Arbitrary paint
order and translucent overlays make pixel-level visibility ambiguous.

### Virtualized content

Virtualized grid or list rows outside the laid-out range are not part of the frame and cannot be discovered from `op.Ops` or the semantic tree. Their existence must be reported by the owning component snapshot provider.

Large collections must be bounded. The default dump should report totals, loaded/rendered ranges, and small relevant samples. A targeted API or CLI option should allow requests such as a particular component, row range, or record ID without emitting all 10,000 rows.

### Determinism and safety

- Version the JSON schema from the first release.
- Use deterministic field and node ordering.
- Avoid volatile internal pointers and Gio semantic IDs.
- Mark unavailable or unknown facts rather than guessing.
- Redact passwords, tokens, signing data, attachment contents, and fields marked sensitive by default.
- Allow applications to add redaction policies without replacing safe defaults.
- Put output size and traversal-depth limits on dumps.

## Screenshot Support

Where supported, the framework can render the same operation list with `gpu/headless` and write a PNG. Useful behaviors include:

- explicit screenshot requests
- screenshot-on-failure
- optional golden-image comparison with configurable tolerance
- pairing each PNG with its JSON dump

Screenshots should remain diagnostic rather than the primary correctness assertion. Font rasterization, GPU behavior, platform differences, and animation timing can make pixel tests brittle.

The current Termux host may not provide a compatible `gpu/headless` backend. The pinned NDK already supplies Vulkan/EGL headers; `tools/test-termux.sh` resolves the default compiler search-path blocker for full default-package tests and vet without changing toolchain versions. That compilation result does not establish headless rendering support. Screenshot support must therefore be optional and degrade gracefully; failure to initialize a headless renderer must not prevent semantic interaction tests.

Graphics-dependent screenshot support must be isolated in a separate package
or behind explicit build tags. Runtime fallback alone is insufficient: merely
importing a GPU or window package can make `go test` fail to compile in Termux.
The default core test command must not select that dependency graph or require
the NDK environment supplied by the full-suite wrapper.

## Termux and Consumer Integration

The first consumer pilot should be the sibling `adl-gio` application. Its
current Android composition root is build-tagged, constructs state through
`app.Window`/`app.DataDir`, and passes `window.Invalidate` directly into async
grid, form, and presentation adapters. End-to-end in-process testing therefore
requires `adl-gio` to extract a window-independent composition root with
injected storage paths, clock, invalidator, and lifecycle cleanup. That
consumer refactor belongs in `adl-gio`; no ADL types or policies may enter
`gio-kit`.

Before committing to the public harness API, prove a small vertical slice on
this phone: lay out a real reusable widget with `input.Router`, read its
semantic tree, inject one pointer action and one editor action, and run the
test under Termux with no graphics backend. Then pilot route/back, async idle,
and teardown through the extracted `adl-gio` root using temporary storage and
deterministic fixtures.

## On-Device Debug Control Bridge

A later phase can add a debug-build-only control bridge to the mobile harness. It should be compiled out of release builds and expose a small protocol to a repository-owned Termux CLI. This is not Android Debug Bridge (ADB), must not require ADB, and must not install or launch the application; the user starts the debug APK manually.

Candidate commands:

- `tree` or `dump`
- `find`
- `tap`
- `type`
- `scroll`
- `long-press`
- `back`
- `resize` where supported
- `wait`
- `screenshot` where technically available

The bridge should bind only to loopback, use a short-lived authentication token, serialize commands onto the UI/frame loop, and enforce timeouts and output limits. A device spike must prove the app-to-Termux transport, Android permissions, port discovery, token handoff, reconnect behavior, and shutdown before the protocol is designed. The token and endpoint may be surfaced only through an explicit debug UI or another user-mediated channel. The bridge must never be enabled in release artifacts because it is effectively remote control of the application.

The first implementation should not depend on this bridge. The in-process driver and semantic coverage provide most of the value and establish the foundation for safe on-device control.

Android's real accessibility integration may later support a separate UIAutomator-style layer. That would be valuable for platform-level verification but is heavier, depends on Android instrumentation and packaging, and does not replace deterministic pure-Go tests. Repository rules still require the user to install and launch APKs manually; agents must not automate installation or launch.

## Documentation and LLM Use

Documentation is a release deliverable. Add `docs/guitest.md` for humans and
LLM coding agents before declaring the framework ready for downstream use. It
should cover:

- Termux setup and the exact core test commands that do not require desktop
  graphics
- application integration, including layout, invalidation, clock, idle, and
  cleanup hooks
- selectors, ambiguity handling, actions, settling, timeouts, and async test
  patterns
- JSON dump interpretation, unknown/unavailable values, virtualization,
  redaction, and output limits
- tested examples for component tests and an application-level workflow
- optional screenshots and the manually launched on-device debug workflow,
  with its security restrictions and troubleshooting guidance

Publish the dump format as a checked-in versioned JSON Schema and give the CLI
machine-readable command help. Generate API/command/schema reference sections
from source where practical, and verify all prose examples in tests, so the LLM
guide cannot silently drift from the implementation. Hand-written guidance is
still required for workflow, safety, and interpretation; generated Go API
documentation alone is not sufficient.

## Test Strategy

The framework itself needs focused tests for:

- pointer targeting and clipped bounds
- focus and text editing
- scrolling and long-press timing
- semantic selection and ambiguity errors
- modal blocking and Android back behavior
- resize and rotation-equivalent relayout
- deterministic settling of asynchronous updates
- stale and out-of-order completion
- JSON schema stability, ordering, redaction, and size limits
- virtualized 10,000-row component snapshots
- graceful operation without a headless GPU backend
- a Termux-safe dependency check proving the core package does not pull in
  window or graphics backends
- documentation examples and JSON Schema compatibility

Application tests should assert semantic and controller outcomes. Examples include route changes, focused field value, validation message, dirty state, selected rows, visible loaded range, and retry state. Assertions that call controller methods directly should remain controller tests rather than UI-driver tests.

## Suggested Delivery Order

1. Prove the minimal `input.Router` semantic/input vertical slice under Termux.
2. Define the layout plus invalidation/time/idle/cleanup integration contract and formalize a window-independent root in the mobile harness.
3. Add the core frame driver with fixed viewport, metrics, locale, and time.
4. Audit and improve semantics for reusable grid, form, shell, picker, dialog, and presentation widgets.
5. Implement semantic selectors and tap, type, scroll, long-press, back, and resize actions.
6. Add versioned JSON frame dumps, JSON Schema, redaction, and deterministic ordering.
7. Add component snapshot providers, beginning with router, grid, and form.
8. Publish and test the human/LLM usage guide, then pilot the integration contract in `adl-gio` without coupling the repositories.
9. Add separately built optional headless screenshots and screenshot-on-failure.
10. Use the framework for focused acceptance tests mapped to the existing `N*`, `G*`, and form requirements.
11. Evaluate the manually launched, debug-only on-device control bridge after the in-process API and JSON schema are stable.

Every material mobile UI change remains subject to the repository's release APK build, signature and dependency verification, copy to Downloads, and user-performed on-device test gate.

## Main Risks

- Incomplete semantics will make selectors ambiguous and dumps misleading.
- A test-ID scheme could harm accessibility if mixed into user-facing descriptions.
- Virtualized content requires explicit cooperation from components.
- “Rendered,” “visible,” “clipped,” and “covered” must remain precisely distinguished.
- Async settling without explicit application hooks can become flaky.
- Headless rendering is platform-dependent in the current Termux environment.
- A layout-only adapter will miss invalidation, pending async work, persistence,
  and teardown behavior in real applications.
- A debug control server is a security risk unless isolated, authenticated, and absent from release builds.
- A generated reference without tested workflows can be accurate at the API
  level but still ineffective for LLM use; generated and hand-written content
  must be kept in sync.
- Tests can give false confidence if they bypass Gio event routing or assert only controller state.
