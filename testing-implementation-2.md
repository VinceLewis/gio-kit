# Testing Implementation 2 — Framework-First Development and Device Gates

## Status

**Status:** Approved for execution by the user on 2026-09-10.

**Execution result:** GK-1 through GK-6 and ADL-1 through ADL-6 are complete.
ADL-7 through ADL-9 remain held by B-001 and the resulting absence of a frozen
M8 candidate; their unchecked device/release items are intentionally pending.

**Repositories:**

- Gio-Kit: `/data/data/com.termux/files/home/projects/gio-kit`
- ADL-Gio: `/data/data/com.termux/files/home/projects/adl-gio`

**Objective:** Use Gio-Kit's in-process driver as the primary UI development
gate, add inexpensive platform-adjacent coverage, and concentrate manual Android
testing into an early platform smoke test and a comprehensive frozen-candidate
gate.

## Approved Decision

1. Do not implement the debug bridge or maintain a private Gio input adapter.
   Gio v0.10.2 has no supported live-window event injection seam under the
   bridge constraints.
2. Treat in-process tests of the production composition root as the normal UI
   development loop.
3. Build, sign, and inspect Android artifacts mechanically at delivery
   checkpoints and whenever packaging or platform-facing code changes.
4. Run a short device smoke test when a platform seam is first established or
   materially changed.
5. Run full device acceptance only on a frozen release candidate. Any fix
   creates a new candidate and repeats the affected gates.

Expected coverage is approximately 85–92% automated after the tasks below.
The remaining Android-only risk is concentrated in the real IME, touch timing
and physics, accessibility services, GPU/device rendering, window insets,
Android lifecycle delivery, and OS process recreation. Percentages are planning
estimates, not acceptance evidence.

## Gate Model

### Gate A — Continuous development

Run focused package tests for each change. For UI work, run the production-root
in-process suite before considering the change complete. Tests must exercise
real Gio widgets and `input.Router` routing, not mutate controllers as a
substitute for interaction assertions.

### Gate B — Repository checkpoint

Run the complete repository-owned test, vet, generated-artifact, dependency
isolation, cancellation, and restoration checks. Preserve exact failures;
passing unaffected packages is only a partial result.

### Gate C — Platform smoke

Use a signed arm64 APK after a new or changed window, lifecycle, storage,
clipboard, attachment, IME/focus, accessibility, GPU, packaging, or Android
platform seam. The user checks launch/render, one tap-and-type workflow, Android
Back, rotation/resize, large-list scrolling, background/resume, and relaunch.
Pure widget or application-policy changes do not automatically repeat this gate.

### Gate D — Frozen-candidate device acceptance

Freeze source and dependency versions, build and inspect one signed candidate,
record its identity and hash, then execute the complete manual checklist. Do not
change code or dependencies during acceptance. A fix produces a new identified
candidate.

## Gio-Kit Tasks

### GK-1 — Approve and record the testing policy — Complete

- [ ] Review and approve, amend, or reject this proposal.
- [ ] If approved, update `AGENTS.md` to replace its per-material-UI-change and
      three historical device pauses with Gates A–D for future work.
- [ ] Preserve the rule that agents never install or launch APKs automatically;
      the user owns installation, launch, lifecycle actions, and visual checks.
- [ ] Update `test-framework-progress.md` with the approved policy and current
      checkpoint.
- [ ] Keep `testing-debug-bridge.md` and `docs/guitest-bridge.md` as the negative
      feasibility record; remove the bridge from active work queues.

### GK-2 — Audit before extending the driver — Complete

- [ ] Map current tests to platform-adjacent risks and mark each risk as covered,
      partially covered, or device-only.
- [ ] Confirm existing coverage before adding APIs: resize/metrics, Back, Unicode
      edits, focus, long-press, drag, scroll, clipboard seams, fresh-root
      restoration, delayed work, cancellation, joined close, bounded semantics,
      component snapshots, and optional headless pixels.
- [ ] Add only missing cases; do not duplicate existing action or component
      tests.
- [ ] Keep an explicit device-only list in `docs/guitest-acceptance.md`.

### GK-3 — Add missing platform-adjacent simulation — Complete

- [ ] Add a deterministic viewport matrix covering compact phone, tall phone,
      landscape, tablet-like, and narrow split-screen constraints.
- [ ] Model keyboard-like viewport contraction and restoration. If an inset API
      is needed, keep it optional, deterministic, and independent of `app.Window`.
- [ ] Test focus acquisition, focus loss, editor selection/replacement,
      composition-shaped Unicode edits, and immediate Back after editing.
- [ ] Test clipboard behavior through an injected bounded adapter, including
      unavailable, denied, empty, Unicode, and stale-result cases.
- [ ] Add repeated close/recreate tests that reconstruct the real demo root from
      persisted bytes and prove workers, timers, and queued completions are
      cancelled and joined.
- [ ] Add rapid navigation and out-of-order completion cases across forms,
      grids, pickers, dialogs, and presentation components.
- [ ] Exercise touch arbitration at target edges, held pointers during disposal,
      long-press thresholds, drag cancellation, and large-list scroll recovery.
- [ ] Assert semantic role, name, state, enabled state, grouping, and bounded
      diagnostics for every material reusable control.
- [ ] Keep Android IME, TalkBack traversal, real window insets, GPU rendering,
      and lifecycle callbacks explicitly device-only; simulations must not be
      reported as platform proof.

### GK-4 — Strengthen repository-owned gates — Complete

- [ ] Keep `./tools/test-guitest.sh -count=1` as the pure-Go driver and
      dependency-isolation gate.
- [ ] Keep `CGO_ENABLED=0 ./tools/test-guitest.sh -count=1` passing.
- [ ] Keep `./tools/test-guitest-demo.sh -count=1` as the production-root demo
      integration gate.
- [ ] Run `./tools/test-termux.sh -count=1` at repository checkpoints.
- [ ] Run `./tools/generate-guitest-reference.sh --check` whenever the public
      testing API or schema changes.
- [ ] Run `./tools/test-guitest-gpu.sh -count=1` when screenshot or rendering
      code changes; do not treat it as Android display proof.
- [ ] Add a single repository-owned checkpoint script only if it reduces missed
      commands without weakening the separate dependency-isolation checks.
- [ ] Continue to report the unsupported Android/arm64 race detector; run race
      tests on a supported host before calling new concurrency code hardened.

### GK-5 — Device-test assets — Complete

- [ ] Create a concise reusable smoke checklist for Gate C.
- [ ] Create a full Gate D checklist covering navigation/restoration, 10,000-row
      grids, forms/validation, references, dialogs, loading/error/empty/retry,
      Back, rotation/split-screen, keyboard/clipboard, touch/long-press,
      background/resume, process recreation, accessibility, and visual defects.
- [ ] Add a result template recording app ID, artifact name, version name/code,
      source commit, Gio version, ABI, min/target SDK, signer status, EGL result,
      SHA-256, device result, and known exclusions.
- [ ] Keep APKs, native extraction files, keys, databases, and build directories
      ignored and outside commits.

### GK-6 — Gio-Kit completion gate — Complete

- [ ] Run all Gate B commands after the last framework change.
- [ ] Build, sign, verify, and copy the demo APK only if framework work changes
      production widgets, the demo root, rendering, packaging, or another
      platform-facing seam.
- [ ] If the changes are test/docs-only, mechanically prove the production
      dependency graph remains unaffected and do not require a redundant APK.
- [ ] Publish Gio-Kit before updating any ADL-Gio dependency.
- [ ] Record the exact Gio-Kit commit or module version selected by ADL-Gio.

## ADL-Gio Tasks

The initial ADL-Gio work is planning and test inventory. Production changes are
allowed only when the inventory finds an actual coverage or testability gap.

### ADL-1 — Update governance after approval — Complete

- [ ] Add a reviewed decision to `adl-gio.md` adopting Gates A–D. Because the
      existing release-gate language is signed off, mark affected sections
      `Revisit` until the user approves the amendment.
- [ ] Update `AGENTS.md` so production-root `guitest` tests are the normal UI
      development gate, Gate C is triggered by platform-facing changes, and
      Gate D is required for a frozen release candidate.
- [ ] Preserve genericity: automated tests use synthetic ADL fixtures; they may
      load Giggle Band only for vocabulary-blind declaration coverage and
      runtime invariants. Giggle-specific UI judgment remains user-driven.
- [ ] Preserve required Android packaging, signature, ABI, EGL, SQLite, and
      clean dependency-resolution checks.
- [ ] State that the negative bridge result closes bridge work for the current
      Gio baseline.

### ADL-2 — Repair the active implementation plan — Complete; B-001 retained

- [ ] Update `implementation-plan.md` Progress and Resume Notes to remove the
      obsolete instruction to decide or execute the debug-bridge spike.
- [ ] Resolve Q-001 as “Phase 1 negative in Gio-Kit; no ADL-Gio implementation.”
- [ ] Record the result in the active Evidence Log without claiming device
      transport testing.
- [ ] Keep B-001, the missing redistribution terms for pinned ADL inputs, open.
      This testing plan does not authorize release or solve licensing.
- [ ] Retain the current M7 candidate as evidence only; do not present it as a
      releasable final artifact while B-001 is open.
- [ ] Once B-001 is resolved, complete M7 Batch 15 and transition to M8 under the
      approved Gate D procedure.

### ADL-3 — Build a production-root coverage ledger — Complete

- [ ] Inventory `cmd/client/harness_test.go` and related UI/runtime integration
      tests against every enabled local workflow and every M8 acceptance item.
- [ ] For each item record: automated assertion, synthetic fixture, component
      provider, device-only remainder, and responsible package.
- [ ] Cover navigation, guarded Back, route/session/theme restoration, context
      selection, list/search/filter/sort/paging, forms, validation, references,
      permissions, policies, commands/workflows, dashboards, calendars,
      matrices, attachments, audit-visible outcomes, and unsupported connected
      controls.
- [ ] Distinguish a controller/runtime assertion from a real-root input
      assertion. Important enabled workflows require both where practical.
- [ ] Keep application vocabulary out of Go test names, fixtures, setup, and
      assertions.

### ADL-4 — Fill only demonstrated automated gaps — Complete for enabled local workflows

- [ ] Add synthetic real-root tests for any enabled navigation or form workflow
      missing semantic input coverage.
- [ ] Add loading, empty, retry, permission-denied, validation-denied,
      submission-failed, cancellation, and stale/out-of-order completion cases.
- [ ] Add 10,000-row paging and scrolling coverage through the ADL-Gio adapters,
      including selection and filter/sort state preservation.
- [ ] Add immediate guarded Back after editing and Back while work is pending.
- [ ] Add viewport/keyboard-contraction matrices using the Gio-Kit APIs approved
      in GK-3.
- [ ] Add repeated close/fresh-root reconstruction for route, context, session,
      theme, and persisted data; do not promise persistence of state the product
      intentionally does not serialize.
- [ ] Exercise exact decimal, date, time, null, and reference values through
      semantic form input where possible, then assert SQLite and rendered
      round-trips.
- [ ] Test unsupported connected-only controls as visibly unavailable and
      non-mutating.
- [ ] Add bounded/redacted diagnostic captures for failure triage without
      exposing real application data.

### ADL-5 — ADL-Gio automated gates — Complete

- [ ] Run focused tests while changing the responsible package.
- [ ] Run `GOWORK=off ./tools/verify.sh` at checkpoints.
- [ ] Run `GOWORK=off ./tools/test-guitest.sh -count=1` for the real client root.
- [ ] Run the documented `gioandroid` tagged tests and vet with command-local
      NDK flags; never persist them with `go env -w`.
- [ ] Verify the selected published Gio-Kit version with workspace and replace
      overrides disabled.
- [ ] Preserve exact failures and do not report a partial suite as passing.
- [ ] If Gio-Kit changed, publish it first, update ADL-Gio's module version, and
      repeat all dependency-isolated tests and the Android build.

### ADL-6 — Current platform smoke status — Complete; no changed platform seam

- [ ] Treat the completed M6 Android acceptance as the early platform smoke for
      the existing window, Gio, SQLite, navigation, form, and restoration seams.
- [ ] Repeat Gate C before M8 only if subsequent work changes one of those seams
      or the Android packaging/toolchain. Test-only and planning-only changes do
      not trigger another smoke installation.
- [ ] If repeated, record launch/render, tap/type, Back/guard, rotation,
      large-list scroll, background/resume, and relaunch results against the
      exact APK hash.

### ADL-7 — Prepare the frozen M8 candidate — Blocked by B-001

- [ ] Resolve B-001 before release handoff.
- [ ] Remove all temporary migration/fault-injection variants and markers.
- [ ] Account for the currently installed temporary 1.14.0 migration-probe data:
      use the already planned explicit disposable-data reset before installing
      the model-1.13.0 candidate.
- [ ] Confirm clean worktrees, published commits, the pinned Gio-Kit module
      version, and `GOWORK=off` resolution.
- [ ] Run Gate B and the clean reproducible Android build.
- [ ] Verify app ID, version, min/target SDK, arm64-only ABI, alignment, v2/v3
      signature, static SQLite, Android `libEGL.so`, absence of desktop
      `libEGL.so.1`, and artifact SHA-256.
- [ ] Copy to a non-conflicting Downloads filename and freeze that artifact.
- [ ] Provide the user its identity, install/reset caveat, connected-only
      exclusions, and the Gate D checklist.

### ADL-8 — M8 device acceptance — Blocked until a frozen candidate exists

- [ ] First-run/setup and declared context selection.
- [ ] Drawer, navigation, Android Back, dirty-form STAY/DISCARD, and retained
      list/filter/selection state.
- [ ] Large lists: search, filter, sort, select, open, and sustained scrolling.
- [ ] Forms: real keyboard, Unicode, selection, copy/paste, required validation,
      references, exact values, Save, Save & Close, failure, correction, retry.
- [ ] Permissions, policies, constraints, lifecycles, enabled commands, local
      workflows, and audit-visible outcomes.
- [ ] Dashboards, composed presentations, calendars, matrices, dialogs, and
      unsupported attachment/connected controls.
- [ ] Portrait/landscape or split-screen with a list, edited form, dialog, and
      loading state open.
- [ ] Background/resume, manual process stop, relaunch, route/context/session/
      theme restoration, and missing-record recovery.
- [ ] TalkBack traversal where available, semantic labels, disabled controls,
      touch targets, long-press behavior, clipping, redraw, and visual defects.
- [ ] Error/retry and rapid navigation while work is pending; late results must
      not reopen stale screens or corrupt the current session.
- [ ] Record crashes, ANRs, logs, reproduction steps, screenshots where useful,
      and an explicit user pass/fail result.

### ADL-9 — Findings and closure — Pending Gate D

- [ ] Route every finding to its responsible milestone and repository. Reusable
      widget defects are fixed in Gio-Kit first; ADL interpretation/runtime
      defects remain in ADL-Gio.
- [ ] Add an automated regression whenever the failure is reproducible without
      claiming unavailable platform proof.
- [ ] Rerun focused tests, Gate B, dependency-isolated verification, and the APK
      build after each fix.
- [ ] Assign a new version/hash to every changed candidate. Repeat the affected
      manual checks; repeat all Gate D checks for systemic lifecycle, storage,
      navigation, or rendering changes.
- [ ] Record final user sign-off in `implementation-plan.md` and archive the
      evidence without committing APKs, keys, databases, attachments, or logs
      containing private data.

## Explicit Non-Tasks

- No live debug bridge, private-router access, `go:linkname`, reflection/unsafe
  adapter, second router, shadow renderer, or application-specific control API.
- No Gio, Go, SDK, NDK, build-tools, or packaging-tool migration for this work.
- No automatic APK installation, launch, rotation, process control, or input
  injection.
- No weakening of ADL conformance, policy, transaction, migration, genericity,
  or clean-module-resolution gates.
- No claim that synthetic input proves Android IME, TalkBack, GPU, lifecycle,
  or real-device behavior.
