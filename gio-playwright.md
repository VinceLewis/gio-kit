# Gio Playwright-Style Script Runner and ADL-Gio Acceptance Plan

## Status

**Status:** Approved and in progress. On 2026-09-12 the user authorized
execution through completion, with all automated work completed before any
final user-owned device acceptance. The user previously approved the
application-authoring policy: once the v1 runner contract is published, each
ADLJ application must create and maintain its declarative UI suite as the
application is authored or changed.

**Primary repositories:**

- Gio-Kit: `/data/data/com.termux/files/home/projects/gio-kit`
- ADL-Gio: `/data/data/com.termux/files/home/projects/adl-gio`
- Authoritative Giggle Band source: `/data/data/com.termux/files/home/projects/adl`

**Objective:** Add a strict, deterministic JSON scenario runner over Gio-Kit's
existing in-process `guitest.Driver`, integrate it with ADL-Gio's real
production composition root and temporary real SQLite storage, and finish with
Giggle Band ADLJ scenarios as the last and strongest automated acceptance gate.

This plan does not implement a live-APK control bridge. It does not require a
Gio fork. It exercises real Gio widgets and `input.Router` routing in process;
Android platform behavior remains a separate device gate.

An identical copy of this plan must exist at `../adl-gio/gio-playwright.md`.
The two copies are one plan, not independent forks: any status, contract, task,
or sequencing edit must update both, and repository checkpoints must compare
them byte-for-byte. Both repositories' `AGENTS.md` files make the local copy
mandatory reading for runner, UI-test, and ADLJ application-suite work.

## Decisions to Freeze Before Coding

- The script format is versioned JSON. YAML, JavaScript, CSS selectors, XPath,
  arbitrary expressions, arbitrary SQL, shell commands, plugins, and dynamic Go
  loading are excluded.
- The reusable interpreter lives in Gio-Kit. A consumer-owned executable links
  its application factory into the same process; `cmd/guitest` cannot discover
  or import arbitrary Go applications dynamically.
- Scripts interact through semantic selectors and normal driver actions.
  Controller calls may construct fixtures or independently verify results, but
  they do not count as routed UI evidence.
- Selector actions wait read-only for a unique compatible target. Ambiguity is
  an immediate authoring error. A mutating action is dispatched once and is
  never retried after an uncertain result.
- Assertions may poll invalidated or immediately due frames. They must not
  advance future virtual timers; scripts use an explicit `advance` step.
- Settling remains explicit so scripts can assert loading, error, retry, and
  other intermediate states.
- Semantic JSON and bounded component/probe output are the primary evidence.
  Screenshots are optional, unredacted diagnostics used only at selected visual
  checkpoints.
- The runner never owns or closes a caller's driver. The application launcher
  owns cancellation, joined cleanup, storage closure, and reconstruction.
- Giggle Band scripts are implemented and run only after Gio-Kit is published,
  ADL-Gio pins that published version with `GOWORK=off`, and all generic ADL-Gio
  integration gates pass.
- The authoritative scenario, fixture, and probe-declaration assets for an
  ADLJ application live in that application's source directory beside its
  `.adlj` declarations. ADL-Gio consumes a provenance-pinned test-only copy;
  application assets do not become production Go code or enter release APKs.
- After v1 publication, any change to an ADLJ application's routes, views,
  forms, policies, commands, storage-visible behavior, or model version must
  update its suite in the same change, or record why existing scenarios prove
  there is no behavioral change. Fast affected scenarios run during authoring;
  the full application suite remains the final integration gate.

## Intended Coverage

The runner should make roughly 80–85% of application UI acceptance automatic:
navigation, forms, validation, grids, related records, policies, loading,
errors, retry, persistence, restoration, cancellation, and database/service
integrity. JSON checkpoints should normally take milliseconds and replace most
screenshot inspection.

The following remain device-only evidence: real Android IME/composition,
TalkBack traversal and speech, touch timing and physics, GPU/display output,
system bars and insets, native Back dispatch, rotation/split-window delivery,
document providers, clipboard policy, background/resume, OS process death,
installation/signing behavior, ANRs, and visual quality.

## Architecture

```text
Giggle Band JSON scenarios beside authoritative ADLJ source
          |
          v
ADL-Gio consumer launcher
  - real newClient production root
  - named fixtures
  - bounded integrity probes
  - temporary SQLite/session/attachments
          |
          v
gio-kit/guitest/script
  - strict decoder and selector compiler
  - action dispatcher and deterministic waits
  - assertions, trace, JSON artifacts
          |
          v
existing guitest.Driver + Gio input.Router
```

`guitest/script` runs an already-created `*guitest.Driver`. A small reusable
`guitest/scriptcli` package handles common consumer CLI flags and lifecycle but
receives an application-supplied opener. The existing `cmd/guitest` gains only
schema validation and trace inspection commands; it does not gain a universal
`run` command.

ADL-Gio's launcher calls its existing `newClient`, which already builds the
same root used by Android with a real temporary SQLite database, runtime,
adapters, widgets, virtual clock, diagnostic providers, idle reporting, and
joined cleanup. No parallel test UI or storage implementation is permitted.

## Version 1 Script Contract

### Top-level document

```json
{
  "version": 1,
  "name": "guarded edit",
  "fixture": "synthetic.basic.v1",
  "defaults": {"timeout": "2s"},
  "steps": []
}
```

- `version`, `name`, and `steps` are required.
- `fixture` is an opaque consumer-owned name. The generic runner never knows
  record schemas or application vocabulary.
- Default limits: 1 MiB input, 500 steps, selector depth 16, 64 selector terms,
  2 KiB strings, and two seconds per step.
- A step timeout is hard-capped at the driver's existing five-second safety
  boundary. The caller context bounds the entire run.
- Unknown fields, duplicate step IDs, invalid UTF-8, non-finite coordinates,
  negative durations, excess complexity, trailing JSON, and incompatible step
  fields fail before the first action.

### Selector AST

Each selector object has exactly one operator:

- Leaves: `label`, `description`, `name`, `role`, `text`, `id`, `enabled`, and
  `selected`.
- Composition: `all`, `within`, `containing`, and `nth`.
- Roles: `unknown`, `button`, `checkbox`, `editor`, `radio`, and `switch`.
- `within` selects a target under an ancestor; `containing` selects a target
  containing a descendant. Their argument order must match the existing Go
  selector API.
- `id` refers only to application-provided `BindID` options established when
  the driver is constructed. Scripts cannot inject new bindings.
- Zero or multiple matches fail unless `nth` or an exact count assertion makes
  the author's intent explicit.

### Step operations

- Actions: `tap`, `doubleTap`, `longPress`, `press`, `move`, `release`,
  `cancelPointer`, `drag`, `scroll`, `focus`, `type`, `key`, `back`,
  `clearFocus`, `setSelection`, `setComposition`, `edit`, and `resize`.
- Time: `advance` and `settle`.
- Semantic assertions: `expect` with `present`, `absent`, `enabled`, `disabled`,
  `selected`, `unselected`, exact `count`, `viewportVisible`,
  `viewportPartiallyVisible`, or `viewportClipped`.
- Component assertion: `expectSnapshot`, using a component name, a small RFC
  6901 JSON Pointer, and one of `equals`, `notEquals`, or `exists`.
- Consumer integrity assertion: `expectProbe`, using a pre-registered probe
  name, bounded JSON arguments, and canonical JSON equality. Probe names are an
  allowlist; the runner cannot execute SQL, access arbitrary files, or invoke
  application commands.
- Diagnostics: `capture`, with bounded JSON always available and screenshot
  mode `never`, `optional`, or `required`.
- Raw `Router.Queue`, unrestricted coordinates, `PressAt`, action retries,
  automatic scrolling, parallel steps, and multi-driver orchestration are not
  part of v1.

### Auto-wait and failure behavior

- Before an action, wait read-only for exactly one compatible and interactable
  target. Absence or temporary disablement may retry until the deadline.
- Ambiguity fails immediately. It is not treated as transient.
- Dispatch the action exactly once. Never retry input after dispatch.
- Assertions retry read-only until true or timed out.
- Do not automatically settle after actions and do not advance future timers.
- Virtualized rows require explicit bounded scrolling; absent semantics cannot
  be treated as proof that a logical record does not exist.
- On failure, capture the current redacted JSON before best-effort pointer
  cancellation. Successful scripts must end with no held pointer.
- Stable error codes: `invalid_script`, `timeout`, `cancelled`, `not_found`,
  `ambiguous`, `not_interactable`, `assertion_failed`, `probe_error`,
  `frame_limit`, `closed`, `artifact_error`, and `internal`.

### Result and artifacts

- `result.json`: bounded final result and failing step identity.
- `trace.json`: step index/ID/operator, status, attempts, frames before/after,
  virtual time before/after, stable error code, and artifact names.
- `step-NNN-failure.json`: existing bounded/redacted frame capture.
- Explicit numbered capture JSON files.
- Optional PNG files through an injected screenshot callback.

Traces must not copy typed text, selector values, expected values, clipboard
contents, application errors, raw semantic nodes, or SQL results. Artifact file
names derive from numeric step indexes, not user text. Use private permissions,
reject symlink targets, write atomically, and enforce both per-file and total
run size limits. PNGs are explicitly unredacted and use synthetic fixtures.

## Agent Execution Rules

- Every implementation agent must use `gpt-5.6-sol` with reasoning effort
  `high`, or a lower model/effort. No task may target Astra or any model above
  Sol/high.
- Every agent first reads the target repository's `AGENTS.md` and the exact
  prerequisite documents named by its task.
- An agent editing either copy of this plan owns both copies for that task and
  runs `cmp -s gio-playwright.md ../adl-gio/gio-playwright.md` from Gio-Kit.
- Workers are not alone in the codebase. They preserve unrelated changes, do
  not revert other agents' work, and adapt to completed prerequisite changes.
- Each worker owns only the files listed for its task. Shared generated files,
  module files, scripts, progress ledgers, and central documentation have one
  integration owner.
- Parallel work starts only where the dependency graph and file ownership below
  permit it. No two active workers edit the same file or generated artifact.
- Agents do not commit, push, publish, install, or launch unless separately
  authorized. The lead records exact checks and failures before handoff.

## Master Task Tracker

| Task | Repository | Depends on | Status |
| --- | --- | --- | --- |
| P0 Contract and governance approval | All | None | Complete |
| GK-1 Protocol/schema | Gio-Kit | P0 | Complete |
| GK-2 Selector compiler | Gio-Kit | GK-1 types frozen | Complete |
| GK-3 Read-only actionability seam | Gio-Kit | P0 | Complete |
| GK-4 Execution engine | Gio-Kit | GK-1, GK-2, GK-3 | Complete |
| GK-5 Trace/artifact layer | Gio-Kit | GK-1; integrate after GK-4 | Complete |
| GK-6 CLI and consumer launcher | Gio-Kit | GK-4, GK-5 | Complete |
| GK-7 Production demo scenario | Gio-Kit | GK-6 | Complete |
| GK-8 Docs, gates, publication | Gio-Kit | GK-1–GK-7 | Complete |
| ADL-1 Production-root injection seam | ADL-Gio | P0 | Complete |
| ADL-2 Reusable fixture harness | ADL-Gio | ADL-1 | Complete |
| ADL-3 Deterministic runtime decorator | ADL-Gio | ADL-1 | Complete |
| ADL-4 Published runner pin and launcher | ADL-Gio | GK-8, ADL-2 | Complete |
| ADL-5 Bounded integrity probes | ADL-Gio | ADL-2 | Complete |
| ADL-6 Synthetic routed scenarios | ADL-Gio | ADL-3–ADL-5 | Complete |
| ADL-7 Async/retry/restoration scenarios | ADL-Gio | ADL-3–ADL-6 | Complete |
| ADL-8 Isolation, documentation, Gate B | ADL-Gio | ADL-1–ADL-7 | Complete |
| GB-0 Giggle governance/provenance gate | ADL + ADL-Gio | ADL-8 | Complete |
| GB-1 Giggle fixtures | ADL source; pinned by ADL-Gio | GB-0 | Complete |
| GB-2 Giggle probes | ADL declarations + ADL-Gio engine | GB-1 | Complete |
| GB-3 Giggle scenarios | ADL source; pinned by ADL-Gio | GB-1, GB-2 | Complete |
| GB-4 Final Giggle wrapper and ultimate run | ADL-Gio | GB-3 | Complete |
| F-1 Frozen candidate/device handoff | ADL-Gio | GB-4 | Complete |

## Phase 0 — Approval and Contract Freeze

### P0 — Contract and governance approval

**Owner:** lead agent only. No implementation worker starts before completion.

- [x] Approve or amend this plan and record the decision in both repositories'
      active progress documents.
- [x] Freeze the v1 JSON structure, selector grammar, operations, wait rules,
      error codes, limits, artifact policy, fixture name, and probe contract.
- [x] Confirm that the work is in-process only and does not reopen the rejected
      live bridge or authorize a Gio fork.
- [x] Record the user-approved authoring policy: generic production code and
      ordinary tests remain vocabulary-blind, while application-specific
      declarative fixtures/scenarios and bounded probe declarations live beside
      each ADLJ application and evolve with it.
- [x] Freeze the exact isolation and provenance rules by which ADL-Gio consumes
      those assets test-only without putting application vocabulary in generic
      production Go or release APKs.
- [x] Confirm Giggle fixture data is synthetic and that no developer/device
      database, private UI tree, screenshot, or production credential is used.
- [x] Assign one integration owner per repository for module files, generated
      files, scripts, documentation, and progress ledgers.

**Frozen decision (2026-09-12):** The contract above is accepted without
amendment. The runner remains an in-process test facility and does not reopen
the rejected live bridge or authorize a Gio fork. ADL-Gio may consume only a
hash-verified, model-matched, test-only copy of declared suite assets; mutable
sibling paths, production package embedding, and release-APK inclusion are
forbidden. Giggle fixtures use synthetic data only. The lead agent is the
integration owner for shared module files, generated artifacts, scripts,
documentation, and progress ledgers in Gio-Kit, ADL-Gio, and the authoritative
ADL test-asset directory.

**Acceptance:** all disputed contracts are resolved in writing; no worker must
invent public behavior or broaden governance while coding.

## Phase 1 — Gio-Kit Runner

After GK-1 is frozen, GK-2, GK-3, and GK-5 may run concurrently because their
file ownership does not overlap. GK-4 then integrates those results.

### GK-1 — Protocol types, strict decoder, validator, and schema

**Owned files:**

- `guitest/script/protocol.go`
- `guitest/script/decode.go`
- `guitest/script/validate.go`
- `guitest/script/schema.go`
- `guitest/script/schema-v1.json`
- focused and fuzz tests beside those files

- [x] Define the public script, defaults, step, selector, assertion, probe,
      capture, result, and stable error-code types.
- [x] Decode one bounded JSON document with `DisallowUnknownFields` and reject
      trailing data.
- [x] Validate discriminated unions, enums, mutual exclusion, unique step IDs,
      durations, coordinate finiteness, UTF-8, and complexity limits.
- [x] Generate or deterministically check a schema that properly represents
      unions; do not reuse reflection machinery that cannot express them.
- [x] Add malformed, oversized, boundary, unknown-field, recursion, and fuzz
      regression tests proving failure occurs before execution.

**Acceptance:** schema and Go validation agree; every invalid input is bounded
and returns `invalid_script` before any UI mutation.

### GK-2 — Selector compiler and parity tests

**Owned files:** `guitest/script/selector.go` and its tests. Do not edit
`guitest/input.go`.

- [x] Compile the validated selector AST into existing public Gio-Kit selectors.
- [x] Map roles exactly and preserve inherited names/enabled state, ancestry,
      occurrence, and strict ambiguity behavior.
- [x] Compare every compiled selector with its direct-Go equivalent across
      reorder, resize, duplicated labels, ancestry, disabled ancestors, and
      bound IDs.

**Acceptance:** scripted and direct-Go selectors return the same nodes and
errors for the same frame.

### GK-3 — Minimal read-only actionability seam

**Owned files:** new `guitest/actionability.go` and tests. This is the only task
allowed to touch core target-resolution behavior.

- [x] Add a narrow read-only API such as `CheckTarget(selector, capability)` for
      tap, edit, and scroll readiness.
- [x] Delegate to existing target resolution without sending input or claiming
      unknown occlusion/clip/focus facts.
- [x] Prove it distinguishes not found, ambiguity, disabled, off-viewport, and
      capability mismatch without changing widget state.

**Acceptance:** the runner can poll readiness without probing through a
possibly repeated mutating action.

### GK-4 — Execution engine, actions, assertions, probes, and auto-wait

**Owned files:**

- `guitest/script/runner.go`
- `guitest/script/actions.go`
- `guitest/script/assert.go`
- `guitest/script/probe.go`
- `guitest/script/jsonpointer.go`
- focused runner tests

- [x] Implement sequential `Run(ctx, driver, script, options)` on the caller
      goroutine without closing the driver.
- [x] Dispatch every v1 action through existing driver APIs.
- [x] Implement read-only readiness waits and exactly-once mutation.
- [x] Implement explicit settle/advance behavior and assertion polling.
- [x] Evaluate snapshots only through bounded/redacted `Capture`.
- [x] Define a bounded named-probe callback returning normalized JSON. Reject
      unknown probes and cap arguments/results before decoding or comparing.
- [x] Implement the small JSON Pointer/equality vocabulary without a general
      expression evaluator.
- [x] Return stable step errors with index, optional ID, operator, code, and
      frame number while preserving useful `errors.Is` behavior.
- [x] Track held pointer state; capture before best-effort cancellation.
- [x] Test real material widgets and router input, delayed enablement,
      exact-once callbacks, ambiguity fail-fast, cancellation, frame limits,
      virtual-time advancement, intermediate loading, and 10,000-row scrolling.

**Acceptance:** a JSON scenario drives real Gio input deterministically; no
action is retried after dispatch; probes cannot escape their registered
allowlist.

### GK-5 — Deterministic trace and safe artifact writer

**Owned files:**

- `guitest/script/trace.go`
- `guitest/script/artifact.go`
- `guitest/script/trace_schema.go`
- `guitest/script/trace-schema-v1.json`
- focused artifact tests

- [x] Produce the result, trace, failure dump, and explicit capture artifacts
      described above.
- [x] Exclude real-time values and sensitive script/application values from the
      trace.
- [x] Enforce private permissions, atomic replacement, symlink rejection,
      deterministic names, and per-file/total limits.
- [x] Accept an optional screenshot callback so the core dependency graph stays
      graphics-free.
- [x] Mark every PNG unredacted and fail correctly when a required screenshot
      is unavailable.

**Acceptance:** deterministic artifact comparison passes; injected write,
short-write, symlink, oversize, and screenshot failures leave no misleading
partial success.

### GK-6 — Reusable consumer CLI and artifact CLI additions

**Owned files:**

- `guitest/scriptcli/cli.go` and tests
- `cmd/guitest/main.go` and tests

- [x] Add reusable consumer `run -script -artifacts` parsing around an injected
      application opener and named fixture/probe registry.
- [x] Guarantee deferred joined closure on success, validation failure, runtime
      failure, cancellation, and artifact failure.
- [x] Add `script-schema`, `trace-schema`, `validate-script`, and
      `inspect-trace` to `cmd/guitest`.
- [x] Do not add application discovery, subprocess compilation, networking,
      ADB, dynamic plugins, or a universal run command.

**Acceptance:** a consumer needs only a small compiled launcher; the generic
artifact CLI remains application-independent and safe on untrusted structure.

### GK-7 — Real navigation-demo launcher and scenario

**Owned files:** `cmd/navigation-demo/**`, its scenario testdata, and only the
demo-specific test script if required.

- [x] Extract the existing tagged test factory so tests and a
      `guitestrunner`-tagged launcher use the same production root.
- [x] Keep default Android main excluded only when the runner tag is selected;
      prove the production graph does not import runner packages.
- [x] Add one real-root scenario covering startup delay, navigation, editor
      input, dirty Back with STAY/DISCARD, router/form snapshot assertions, grid
      interaction, and joined teardown.
- [x] Use a temporary data directory and real demo storage.

**Acceptance:** the first JSON scenario passes against the production root and
proves cleanup; no demo policy enters reusable packages.

### GK-8 — Documentation, verification, and publication checkpoint

**Single integration owner files:** `docs/guitest.md`, new
`docs/guitest-script.md`, `README.md`, `test-framework-progress.md`, generation
scripts, generated API/command references, `go.mod`, and `go.sum` if changed.

- [x] Document the DSL, application opener, fixture/probe boundary, auto-wait,
      explicit time, lifecycle ownership, trace privacy, screenshots, and
      platform exclusions.
- [x] Generate/check both schemas and all public references.
- [x] Run focused tests throughout, then:

```sh
./tools/test-guitest.sh -count=1
CGO_ENABLED=0 ./tools/test-guitest.sh -count=1
./tools/test-guitest-demo.sh -count=1
./tools/generate-guitest-reference.sh --check
./tools/test-checkpoint.sh -count=1
```

- [x] Verify core/script/scriptcli dependency graphs exclude `gioui.org/app`,
      GPU, EGL, GL, Vulkan, and window backends.
- [x] Run the GPU probe only if screenshot implementation or rendering changed.
- [x] Record exact failures; unaffected packages are only partial evidence.
- [x] After separate authorization, commit/push and publish or identify the
      exact Gio-Kit pseudo-version for ADL-Gio.

**Acceptance:** Gate B passes from a clean checkpoint and a published version is
available. Test-only work does not trigger Gate C unless it changes a platform
seam.

**Evidence (2026-09-12):** the five checkpoint commands were executed from a
clean `gio-kit` worktree at `3c66f2d8f9b1`. `./tools/test-guitest.sh -count=1`,
the same suite with `CGO_ENABLED=0`, `./tools/test-guitest-demo.sh -count=1`,
`./tools/generate-guitest-reference.sh --check` and
`./tools/test-checkpoint.sh -count=1` all pass. `go list -deps` over `guitest`,
`guitest/script` and `guitest/scriptcli` reports no `gioui.org/app`, GPU, EGL,
GL, Vulkan or window-backend package. The GPU probe was not re-run because
neither screenshot implementation nor rendering changed in this task. `main` is
level with `origin/main` at `3c66f2d8f9b1`, published as
`v0.0.0-20260912110610-3c66f2d8f9b1`, which is the version ADL-4 pins.

## Phase 2 — ADL-Gio Generic Integration

ADL-1 may begin after P0 while Gio work proceeds. After ADL-1, ADL-2 and ADL-3
may run concurrently. Runner-dependent work waits for GK-8 publication.

### ADL-1 — Narrow production-root injection seam

**Owned files:** constructor/service sections of `cmd/client/root.go` and
`cmd/client/services.go`, with focused tests. No other agent edits them.

- [x] Add private, defaulted SQLite opener and runtime factories.
- [x] Define the minimum private runtime interface needed by the client:
      execute, setup, context resolution, inspection, policy inspection, and
      attachment recovery.
- [x] Keep production defaults as the existing real SQLite opener and
      `runtime.New`; retain the concrete SQLite handle where required.
- [x] Add compile-time interface assertions.
- [x] Preserve startup cleanup and the existing close order: cancel, prevent new
      work, capture state, close adapters, join workers, drain persistence,
      write final state, then close SQLite.
- [x] Prove opener/factory failure closes everything already acquired.

**Acceptance:** production behavior is unchanged, while tests can decorate the
real runtime and induce deterministic startup failure without a second storage
architecture.

### ADL-2 — Reusable real-root fixture harness

**Owned files:** a new `cmd/client/harness_support_test.go` and the helper region
of `harness_test.go`; one worker owns both.

- [x] Extract options for declarations, named fixture, seeded session, SQLite
      path, runtime decorator, clock, BindIDs, providers, and artifacts.
- [x] Always construct the actual `newClient`; never reproduce its UI tree.
- [x] Return a handle containing driver, root, resolved model, database path,
      temporary attachment/session paths, and service recorder.
- [x] Register cleanup immediately and prove repeated close is safe.
- [x] Clearly label direct fixture seeding as setup rather than UI evidence.

**Acceptance:** existing tests migrate without loss, and fresh-root
reconstruction can reuse the same temporary persistent files intentionally.

### ADL-3 — Deterministic real-runtime decorator

**Owned files:** new `cmd/client/script_services_test.go` and focused tests.

- [x] Wrap and delegate to the real application runtime.
- [x] Match faults by generic intent/object/view and invocation number.
- [x] Support virtual-clock delay with cancellation, one-shot failure followed
      by delegation, copied call recording, cancellation observation, and
      required lifecycle hooks.
- [x] Never fabricate successful records or mutate SQLite directly.
- [x] Prove ordering, call counts, copied contexts, cancellation, and that
      injected failures leave SQLite unchanged.

**Acceptance:** scripts can exercise real loading/retry/stale-result behavior
without wall-clock sleeps or fake success paths.

### ADL-4 — Pin published runner and add consumer launcher

**Single integration owner files:** `go.mod`, `go.sum`, new test-only client
launcher files, and relevant tool wrapper.

- [x] Update ADL-Gio to the exact GK-8 published version.
- [x] Prove selection with `GOWORK=off`; no `replace` or sibling checkout may be
      acceptance evidence.
- [x] Add a consumer launcher that selects only registered named fixtures and
      probes, creates the actual client root, runs the script, and closes/join
      all resources.
- [x] Add a generic suite loader for provenance-pinned application test assets.
      It must validate the runner schema and reject path traversal, undeclared
      files, model mismatches, and mutable external input.
- [x] Keep the launcher and runner dependencies out of the `gioandroid`
      production graph.
- [x] Reject unknown fixtures/probes before mutation; never expose arbitrary
      filesystem paths, SQL, shell, or network operations.

**Acceptance:** `GOWORK=off` can execute a synthetic JSON scenario against the
real client root using the published module.

### ADL-5 — Bounded database/service integrity probes

**Owned files:** new `cmd/client/script_integrity_test.go` or a dedicated
test-only acceptance package; do not edit the runtime/storage implementation.

- [x] Register generic probes for record counts/aliases, revisions, audit,
      operations, lookup outcomes, transaction coherence, selected context,
      route/theme/session, and persistence.
- [x] Normalize unstable IDs to fixture aliases, sort results, cap rows/depth/
      strings/bytes, and redact sensitive fields.
- [x] Return data only; probes must not mutate storage or call application
      commands.
- [x] Add positive, negative, malformed-argument, unknown-alias, and oversize
      tests.

**Acceptance:** scripts can confirm UI, runtime-call, and SQLite truth without
embedding SQL or schema-specific verbs in Gio-Kit.

### ADL-6 — Synthetic routed core scenarios

**Owned files:** new `cmd/client/script_runner_test.go` and application-neutral
JSON under `cmd/client/testdata/guitest/`.

- [x] Add routed Add → editor input → Save/Save & Close.
- [x] Add invalid/denied submit → routed correction → successful retry.
- [x] Add dirty Back → STAY → Back → DISCARD.
- [x] Add reference picker search/select and related child add/edit/remove.
- [x] Add grid selection/sort/filter/scroll/open/Back using UI input, replacing
      controller mutations where the script API can now route them.
- [x] After each mutation assert semantic/component state, exactly one expected
      runtime intent with user/channel/context/time, and real SQLite/audit/
      operation outcomes.
- [x] Prove denied and failed requests add no record, revision, audit row, or
      operation; successful writes produce one coherent transaction.

**Acceptance:** important local workflows are proven at all three layers—UI,
runtime boundary, and real SQLite—without direct controller action substitutes.

### ADL-7 — Async, retry, teardown, and restoration scenarios

**Owned files:** separate JSON scenario files and a new
`cmd/client/script_async_test.go`.

- [x] Delay a real search/read, assert loading, navigate away, advance virtual
      time, and prove cancellation/no stale UI application.
- [x] Inject first-call failure, activate RETRY through Gio input, delegate the
      second call to the real runtime, and assert calls/database outcome.
- [x] Close with work pending and prove cancellation and joined teardown.
- [x] Reopen the same temporary files and verify committed records, route,
      context, session, theme, and model fingerprint as applicable.
- [x] Repeat reconstruction without wall-clock sleeps.

**Acceptance:** no stale result reopens an old screen; successful closure means
workers, adapters, completion queues, and persistence are finished.

### ADL-8 — Isolation, ledgers, and Gate B

**Single integration owner files:** test scripts, dependency checks,
`docs/testing-coverage.md`, `implementation-plan.md`,
`implementation-evidence.md`, and module files.

- [x] Record routed versus fixture/controller evidence and the device-only
      remainder for each enabled generic workflow.
- [x] Add a mechanical check that every bundled ADLJ acceptance application
      carries a suite compatible with its model version, and that changed
      behavioral declarations update the suite or an explicit no-impact record.
- [x] Prove the `guitest` graph excludes window/GPU dependencies and the
      `gioandroid` production graph excludes script-runner packages.
- [x] Prove scripts cannot invoke shell, network, arbitrary filesystem, SQL, or
      unregistered probes.
- [x] Run focused tests and then:

```sh
GOWORK=off ./tools/test-guitest.sh -count=1
GOWORK=off ./tools/verify.sh
```

- [x] Run documented `gioandroid` tagged tests/vet with command-local NDK flags,
      module verification, generated checks, and `git diff --check`.
- [x] Re-evaluate Gate C only if the root construction/lifecycle/storage seam
      materially changed. Record why strictly test-only changes do not trigger
      it.

**Acceptance:** all generic ADL-Gio gates pass using the published dependency.
No Giggle-specific script may start before this checkpoint.

**Evidence (2026-09-12):** routed versus fixture evidence and the device-only
remainder are recorded per workflow in `docs/testing-coverage.md`, together with
two explicit gaps: the grid renders no filter affordance, so filtering is proven
only through a view's declared search; and `runtime_calls` carries a virtual
timestamp that is stable only for a fixed step sequence.

`TestBundledApplicationCarriesACompatibleSuite` is the mechanical check that the
bundled acceptance application ships a suite pinned to the model version it
resolves to, that the suite records a source revision, and that every registered
fixture is exercised by at least one scenario. It failed exactly as intended
while the pinned copy lagged the authoritative source.

`tools/test-guitest.sh` proves the tagged client graph excludes
`gioui.org/app`, GPU, EGL, GL and Vulkan, and that the `gioandroid` production
graph contains no `gio-kit/guitest` package. `script_isolation_test.go` proves a
scenario cannot reach a shell, a network, an arbitrary file, SQL or an
unregistered probe or fixture, and that the suite loader refuses traversal,
absolute paths, symlinks, digest mismatches, model mismatches and undeclared
files.

`GOWORK=off ./tools/verify.sh` passes every check, including input hashes,
formatting, the vocabulary guard, conformance accounting, the generated ledger,
tests, vet, workspace-independent module resolution and `git diff --check`.
`GOWORK=off go test -count=1 ./...`, `GOWORK=off go vet ./...`, the documented
`gioandroid` tagged tests and vet, and `GOWORK=off ./tools/test-guitest.sh
-count=1` all pass. Gate C is not re-triggered by this work: no root
construction, lifecycle or storage seam changed, and the three production
corrections are behavioural fixes inside existing seams.

## Phase 3 — Giggle Band Ultimate Acceptance Suite

Every task in this phase is deliberately last. These tasks must not run against
a sibling Gio-Kit worktree or an unverified ADL-Gio integration.

### GB-0 — Governance, provenance, and model freeze

- [x] Confirm the P0 governance refinement permits isolated Giggle declarative
      acceptance assets while production Go code remains vocabulary-blind.
- [x] Compare the authoritative sources `src/reference/giggle-band/app.yaml`,
      `domain.adlj`, and `ui.adlj` with ADL-Gio's pinned embedded snapshot.
- [x] Define and verify the test-only pin from
      `src/reference/giggle-band/tests/**` into ADL-Gio. The final suite must
      consume the pinned copy, not mutable sibling files.
- [x] Resolve the current provenance inconsistency: ADL-Gio
      `third_party/adl/PROVENANCE.md` and `docs/source-inputs.md` report different
      ADL commits.
- [x] Record the exact ADL source commit, model version/fingerprint, Gio-Kit
      module version, and ADL-Gio source checkpoint.
- [x] Do not use connected features as successful local behaviors; invitation,
      sync, streaming, and other `onlineRequired` operations must fail closed
      until a real authority/transport exists.

**Acceptance:** one authoritative, licensed, reproducible Giggle model is
frozen, and app-specific automated assets are explicitly permitted.

**Evidence (2026-09-12):** the P0 authoring policy is now enforced mechanically
rather than by convention. `tools/check-vocabulary.sh` still forbids application
vocabulary everywhere, with one narrow exception for an application's own
declarative acceptance assets, and it verifies that exception rather than
trusting it: an exempt Go file must be a `_test.go` file carrying the `guitest`
build tag, and an exempt generator under `tools/` must carry `//go:build
ignore`. `go list -tags gioandroid -deps ./cmd/client` contains no
`gio-kit/guitest` package, so no shipped build can reach the runner or any
scenario asset.

The authoritative `domain.adlj` and `ui.adlj` at
`adl/src/reference/giggle-band/` are byte-identical to the pinned snapshot in
`third_party/adl/reference/`, and all 43 entries of
`third_party/adl/SHA256SUMS` verify. The test-only pin is defined by
`tools/sync-giggle-suite.sh`, which refuses to pin from a dirty authoritative
tree, and by `tools/generate-giggle-manifest.go`, which records the source
revision and a digest per scenario. `loadClientSuite` verifies the manifest,
every digest, the runner schema, the declared model version and the absence of
undeclared files before the first scenario runs, and rejects path traversal,
absolute paths and symlinks.

The reported provenance inconsistency is resolved. `docs/source-inputs.md` had
recorded ADL revision `af886659771a` while `third_party/adl/PROVENANCE.md`
recorded `b386b9d1b37f`, and both documents named a stale Gio-Kit module
version. Both now record the true pins, and `PROVENANCE.md`'s own entry in
`SHA256SUMS` was regenerated. No source-input hash changed.

Frozen identity for this gate: ADL source `b386b9d1b37f37ffee9787156698e76d90230da9`;
model version `1.13.3`, fingerprint
`sha256-3e74e9fefe0a19ded04b03ec40647753323bc09ba503c36137394c2c17d92a8c`;
Gio-Kit `v0.0.0-20260912110610-3c66f2d8f9b1`; ADL-Gio checkpoint `0098da677592`.

Connected behaviour is not treated as working local behaviour anywhere:
`BandInvitation` and its commands are `onlineRequired`, and the shell renders
them disabled with the catalog reason "Available when connected".

### GB-1 — Named deterministic Giggle fixtures

**Authoritative owned files:**
`/data/data/com.termux/files/home/projects/adl/src/reference/giggle-band/tests/fixtures/**`
and their schema/loader tests. ADL-Gio owns only a generated or copied test-only
provenance-pinned input. Start from the reference data in ADL's
`src/reference/band-app.ts`, but copy only data whose provenance permits it.

- [x] `giggle.blank.v1`: bundled model, empty SQLite, attachment temp directory,
      empty session, and frozen clock.
- [x] `giggle.admin-seeded.v1`: Casey Morgan; The Alphas as administrator; The
      Betas as member; representative gigs, rehearsal, conflicts, songs, set
      lists, ordered children, and event/set-list links.
- [x] `giggle.invitee-seeded.v1`: pending, accepted, revoked, other-person, and
      role variants for fail-closed checks.
- [x] `giggle.large-library.v1`: 10,000 deterministic songs or events.
- [x] Use aliases and deterministic IDs. Fixture-only policy bypass is clearly
      marked and never counted as proof that a user action is allowed.

**Acceptance:** fixtures schema-validate, create only temporary storage, load
deterministically, and expose no production/private data.

### GB-2 — Giggle-specific bounded probes

**Owned files:** declarative probe requests beside the authoritative Giggle
suite plus isolated test-only ADL-Gio probe adapters/tests where the generic
probe vocabulary cannot express an invariant. Application names and business
rules must not enter production Go.

- [x] `giggle.contexts`: labels, selected context, and roles.
- [x] `giggle.records`: bounded alias/title summaries for one declared object
      and context.
- [x] `giggle.set_list_integrity`: ordered aliases, contiguous positions,
      unique song membership, common band, and total duration.
- [x] `giggle.event_set_list_integrity`: unique links and contiguous positions.
- [x] `giggle.atomic_outcome`: before/after counts, revisions, audit/operation
      outcome, and transaction consistency.
- [x] `giggle.persistence`: model version/fingerprint, context, route/theme, and
      named-record presence.
- [x] `giggle.no_cross_context_leak`: expected/forbidden aliases only.

**Acceptance:** every probe is read-only, allowlisted, alias-normalized,
deterministic, bounded, and independently unit-tested.

### GB-3 — Ordered Giggle scenario suite

**Authoritative owned files:**
`/data/data/com.termux/files/home/projects/adl/src/reference/giggle-band/tests/scenarios/*.json`
and `tests/suite.json`. ADL-Gio consumes the test-only pinned copy. Each
scenario gets a fresh fixture unless it explicitly tests reopen persistence.

- [x] `00-startup-contract.json`: clean bundle diagnostics, model identity,
      navigation reachability, and connected controls unavailable.
- [x] `10-first-run-and-bands.json`: local identity, first band, second band,
      context switching, and founder membership.
- [x] `20-admin-crud-validation.json`: create/edit/search/sort/delete songs and
      gigs; invalid date/time/email/phone/amount/mode leaves all stores unchanged.
- [x] `30-set-list-atomicity.json`: add/edit/reorder/remove songs, duration,
      uniqueness, compact positions, cancellation, and one-transaction commit.
- [x] `40-event-set-lists.json`: attach/reorder/detach, reject duplicate link,
      atomic rollback, and position compaction.
- [x] `50-role-and-context-isolation.json`: administrator versus member policy,
      scoped lists/pickers/dashboards, and no cross-context disclosure.
- [x] `60-presentations.json`: home toggles, calendar navigation, availability
      names/conflict precedence, empty states, and row actions.
- [x] `70-connected-fail-closed.json`: invitation/sync/streaming/attachment
      controls unavailable or safely non-mutating.
- [x] `80-close-reopen-restoration.json`: joined close, fresh root, committed
      data, route/context/theme, dirty guard, and missing-record recovery.
- [x] `90-large-library.json`: 10,000-row paging/filter/sort/select/open/Back,
      restored scroll, bounded capture, and cancellation.
- [x] `99-final-integrity.json`: all bounded invariants, no orphan/duplicate
      child links, no cross-context leak, no unexpected failure, and no pending
      worker/persistence activity.
- [x] Every positive mutation has a denial/failure non-mutation counterpart.
- [x] Absence checks include a present ancestor/anchor so an unloaded screen
      cannot create a false pass.
- [x] Screenshots appear only at a small viewport/theme matrix and atomic final
      states; functional assertions use semantics/probes.

**Acceptance:** all scripts pass independently and in the declared order using
only the pinned published Gio-Kit dependency and the actual ADL-Gio root.

**Defects found by this suite (2026-09-12).** The suite exists to find these,
so they are recorded as results rather than obstacles.

Fixed in this repository, because each is a crash or a silently wrong outcome a
user would hit immediately:

- **Confirming DELETE in any record editor crashed the client.**
  `editorContent` dereferenced `u.editor` immediately after
  `receiveEditorResult`, which clears it on a successful delete, so every
  confirmed delete ended in a nil-pointer panic. The editor is the only delete
  affordance in the application, so no record could be deleted at all.
  `editorContent` now returns immediately when the editor has already left.
- **A record created through Save & Close never appeared in its own list.**
  The post-commit refresh was queued behind a guard requiring the editor to
  still be open, and Save & Close closes it first, so `invalidateViews` never
  ran. The record was written correctly and stayed invisible until the next
  cold start — including after navigating away and back. The refresh now runs
  on every committed write, independently of the editor's lifetime.
- **The `grid` and `presentation` diagnostics reported an arbitrary view.**
  Both scanned a Go map, whose iteration order is randomised, so a scenario
  could be handed a different screen's grid on each run. They now resolve the
  view the router is actually showing, walking the stack from the top down. A
  diagnostic that describes a surface the user is not looking at is worse than
  none, and two scenarios had unknowingly been written against the stale value.

Recorded, not changed, because each needs a decision this plan does not carry:

- **A read-model source declaring `scope: currentUser` returns every user's
  rows.** `runtime/query.go` implements the scope as "the record is the user, or
  the user created it", while `docs/spec/runtime-semantics.md` defines it as
  "records whose lookup to the user object holds the signed-in user". In the
  acceptance application the two agree for every reachable case, because its
  policy only lets a person create their own rows, so this is a conformance
  deviation rather than a live disclosure. Correcting it changes production
  query semantics across every `currentUser`-scoped source and needs an explicit
  decision plus conformance review.
- **A declared `displayField` is not applied to grid cells or read-model
  dashboard rows**, so raw record identifiers reach the user: `band-alphas` on
  My Invitations, `user-casey` on the availability board, `song-neon-map` on
  Streaming. The affected read-model fields declare no lookup, so the correction
  is a definition change to the bundled ADL and carries the same authorization
  and migration requirement as any other.
- **A list toolbar's ADD is not policy-gated.** A plain member is offered ADD on
  every band-scoped list; the create form opens and the refusal arrives only on
  save. The editor path is gated correctly, so this is a UI inconsistency that
  fails closed, not a policy hole.
- **A declared child-collection `summary` aggregate and a declared picker
  `emptyState` are both dropped** by the adapter, and a staged child row is
  labelled differently from the same row after commit.
- **The declared calendar empty state is unreachable**, because the adapter
  always fills every day of the month, so the widget's empty branch never runs.

The scenarios assert the true current behaviour in each recorded case, with the
correction noted in the scenario itself, rather than asserting an outcome the
application does not yet produce.

### GB-4 — Final wrapper and ultimate automated run

**Single integration owner files:** `tools/test-giggle-e2e.sh`, coverage/evidence
ledgers, and final suite manifest.

- [x] Force `GOWORK=off`, verify the exact Gio-Kit version, reject `replace`,
      allocate clean temporary roots, and use synthetic fixtures only.
- [x] Verify the pinned suite hashes and source provenance match GB-0; never
      treat a run against mutable sibling files as final evidence.
- [x] Preserve real exit status and bounded artifacts; do not turn partial suite
      success into a pass.
- [x] Run only after ordinary ADL-Gio verification succeeds:

```sh
GOWORK=off ./tools/test-giggle-e2e.sh -count=1
```

- [x] Record scenario count, checks, exact source/module/model identities, and
      artifact location without committing databases, traces containing private
      data, screenshots, or build output.
- [x] If a Gio-Kit defect is found, fix and republish Gio-Kit, repin ADL-Gio,
      repeat ADL-4 through ADL-8, then restart GB-0. Do not mask it locally.

**Acceptance:** this is the final automated gate. It passes from clean,
published inputs with no workspace assistance and all earlier gates green.

**Evidence (2026-09-12):** `GOWORK=off ./tools/test-giggle-e2e.sh -count=1`
passes. The wrapper forces `GOWORK=off`, refuses a `replace` directive, verifies
the resolved Gio-Kit version, refuses to run without a pinned suite, and reports
the provenance it is running: Gio-Kit `v0.0.0-20260912110610-3c66f2d8f9b1`,
12 scenarios from ADL `60809cf60e83f6ace2317ea69634709de3392bf0`, model
`1.13.3`. The pin was produced by `tools/sync-giggle-suite.sh` from a clean
authoritative tree; it refuses to pin from a dirty one. All 12 scenarios pass
in the declared manifest order, and the tagged suite passed 9 consecutive runs.

One caveat on scheduling: scenario steps carry wall-clock deadlines capped at
five seconds, so running this gate concurrently with another heavy build on the
same device can time a step out. The single failure observed during this work
had that cause and did not reproduce in isolation.

## Starter Giggle Band Script

This is the initial target for `30-set-list-atomicity.json`. Exact accessible
names and BindIDs must be confirmed against the frozen model during GB-3; the
contract and intent must not be weakened merely to match current labels.

```json
{
  "version": 1,
  "name": "giggle.set-list-atomicity",
  "fixture": "giggle.admin-seeded.v1",
  "defaults": {"timeout": "2s"},
  "steps": [
    {
      "id": "open-navigation",
      "op": "tap",
      "selector": {"all": [{"role": "button"}, {"name": "Open navigation"}]}
    },
    {
      "id": "open-set-lists",
      "op": "tap",
      "selector": {
        "within": {
          "target": {"all": [{"role": "button"}, {"name": "Set Lists"}]},
          "ancestor": {"id": "shell.nav-drawer"}
        }
      }
    },
    {"id": "wait-list", "op": "settle"},
    {
      "id": "open-headline",
      "op": "tap",
      "selector": {"name": "Open August headline"}
    },
    {"id": "wait-form", "op": "settle"},
    {
      "id": "initial-integrity",
      "op": "expectProbe",
      "probe": "giggle.set_list_integrity",
      "args": {"setList": "headline"},
      "equals": {
        "songAliases": ["neon-map", "late-signal", "harbour-lights"],
        "positions": [1, 2, 3],
        "uniqueSongs": true,
        "contiguousPositions": true,
        "totalDurationSeconds": 638
      }
    },
    {
      "id": "add-song",
      "op": "tap",
      "selector": {
        "within": {
          "target": {"name": "Add to Songs"},
          "ancestor": {"id": "edit-section.Songs"}
        }
      }
    },
    {
      "id": "choose-slow-tide",
      "op": "tap",
      "selector": {
        "within": {
          "target": {"name": "Slow Tide"},
          "ancestor": {"id": "picker.SongPicker"}
        }
      }
    },
    {
      "id": "no-write-before-save",
      "op": "expectProbe",
      "probe": "giggle.set_list_integrity",
      "args": {"setList": "headline"},
      "equals": {
        "songAliases": ["neon-map", "late-signal", "harbour-lights"],
        "positions": [1, 2, 3],
        "uniqueSongs": true,
        "contiguousPositions": true,
        "totalDurationSeconds": 638
      }
    },
    {
      "id": "save",
      "op": "tap",
      "selector": {"all": [{"role": "button"}, {"name": "Save"}]}
    },
    {"id": "wait-save", "op": "settle"},
    {
      "id": "saved-integrity",
      "op": "expectProbe",
      "probe": "giggle.set_list_integrity",
      "args": {"setList": "headline"},
      "equals": {
        "songAliases": ["neon-map", "late-signal", "harbour-lights", "slow-tide"],
        "positions": [1, 2, 3, 4],
        "uniqueSongs": true,
        "contiguousPositions": true,
        "totalDurationSeconds": 883
      }
    },
    {
      "id": "one-atomic-outcome",
      "op": "expectProbe",
      "probe": "giggle.atomic_outcome",
      "args": {"workflow": "headline-add-slow-tide"},
      "equals": {
        "status": "committed",
        "transactions": 1,
        "partialWrites": 0,
        "unexpectedFailures": 0
      }
    },
    {
      "id": "final-capture",
      "op": "capture",
      "name": "headline-four-songs",
      "components": ["router", "shell", "form", "picker"],
      "screenshot": "optional"
    }
  ]
}
```

## Final Publication and Verification Order

1. Complete GK-1 through GK-8.
2. Run Gio-Kit Gate B and generated-reference checks.
3. After authorization, publish/select one exact Gio-Kit version.
4. Complete ADL-1 through ADL-3 without weakening production behavior.
5. Pin the published version with `GOWORK=off` in ADL-4.
6. Complete ADL-5 through ADL-7.
7. Pass ADL-8 generic verification and dependency-isolation gates.
8. Complete GB-0 governance/provenance/model freeze.
9. Implement GB-1 fixtures, then GB-2 probes, then GB-3 scripts.
10. Run GB-4 as the ultimate automated test, last.
11. If platform seams changed, build/sign/inspect one identified arm64 APK.
12. Freeze source and dependencies, then give the user the Gate C/D device
    checklist. Any fix creates a new candidate and repeats affected gates.

## F-1 — Frozen candidate and device handoff

**Evidence (2026-09-12).** Clean consumer source `e43988ce0f24` with ADL
declaration snapshot `b386b9d1b37f`, Giggle suite pin
`60809cf60e83f6ace2317ea69634709de3392bf0` and Gio-Kit
`v0.0.0-20260912110610-3c66f2d8f9b1` produced
`/storage/emulated/0/Download/adl-client-acceptance-suite-arm64.apk`.

It is `app.adl.client` version `0.1.0.9` (code 9), min SDK 24, target and
compile SDK 35, `arm64-v8a` only, four-byte aligned, and signed by the
continuing certificate SHA-256
`36e2e7900ff3bcf3ea6286f5a38aa8596092815a43aa7a9bc9be20a3747c88f8`. APK
Signature Schemes v2 and v3 verify; v1 is absent by design. The native library
links Android `libEGL.so` rather than desktop `libEGL.so.1`, and contains the
statically linked SQLite engine. Build and Downloads copies share SHA-256
`adcacb1708391b11cfd8a1dafbf7e0b1e5d5eb4781cc03538086888019875f99`.

The candidate was not installed, launched or driven. All device actions remain
user-owned.

### Gate C/D device checklist

This build changes three behaviours the automated suite could not have caught
before, and each needs a human to confirm it on the device. Existing app data is
preserved; install as an update without clearing it.

1. **Delete a record.** Open any song, gig, streaming link or availability
    entry, choose DELETE and confirm. Before this build that crashed the app
    every time. Expect the record to disappear and the app to stay running.
2. **Create a record with Save & Close.** Add a song, then use SAVE & CLOSE
    rather than SAVE. Before this build the new row was invisible in its own
    list until the app was restarted. Expect it to appear immediately.
3. **Edit a song, streaming link or availability entry.** Tap a row in Songs,
    Streaming or Availability. Before this build those opened a read-only
    screen with no way to edit or delete. Expect an editable form with SAVE,
    SAVE & CLOSE and DELETE.

Then the standing regression checks: both themes; the drawer's touch, scroll,
close and Android Back; rotation and short-height reachability; enlarged text;
TalkBack labels and focus on the drawer, the context selector, the theme
control and sign out; the context selector with the real band names; and that
sign out still shows an exit-style glyph rather than a close cross.

Three known defects are deliberately still present and are **not** regressions
to report: raw identifiers appear instead of names on My Invitations, the band
availability board and Streaming; a plain member is offered ADD on band lists
and is refused only on save; and the calendar shows an empty month grid rather
than its declared empty-state text. Each needs a definition or semantics
decision recorded in GB-3 above.

## Estimate

- Gio-Kit runner and demo proof: 7–10 working days.
- ADL-Gio production-root integration, probes, and generic scenarios: 5–10
  working days.
- Giggle fixtures and final scenario suite: 5–10 working days.
- Total: approximately 17–30 engineer-days. With the safe parallelism above,
  elapsed implementation time is approximately 3–5 weeks, excluding review,
  publication authorization, unresolved provenance/licensing work, and device
  acceptance.
