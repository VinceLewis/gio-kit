# Test Framework Progress

Plan: [test-framework-plan.md](test-framework-plan.md).

## Current checkpoint

- Started: 2026-09-09. User confirmed GPT-6 Astra / max and authorized commit,
  push, and execution of the plan.
- Status: step 1 complete; defining the application contract and core driver.
- Resume here: steps 2–3. Read this file, repository guidance, and the current
  working tree before continuing. Step 1 does not need to be repeated unless
  the Gio dependency changes.
- Device gates remain mandatory. Stop after packaging material UI changes for
  the user to install, launch, and test; do not advance through a pending gate.

## Delivery checklist

| Step | Deliverable | Status |
| --- | --- | --- |
| 1 | Termux input.Router pointer/editor/semantics proof | Complete |
| 2 | Layout, invalidation, time, idle, cleanup contract; demo root | In progress |
| 3 | Deterministic core frame driver | Automated checks passed |
| 4 | Reusable component semantics audit and improvements | Pending |
| 5 | Selectors and interaction actions | In progress: tap/type/key/back/resize |
| 6 | Versioned JSON dumps, JSON Schema, safe limits | Pending |
| 7 | Component snapshot providers | Pending |
| 8 | Tested human/LLM guide and adl-gio consumer pilot | Pending |
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

## Device acceptance

No new APK or device result yet.

## Checkpoint history

- 2026-09-09: execution authorized; progress ledger created before coding.
- `b73fab4`: plan and initial progress ledger committed and pushed.
- 2026-09-09: Termux feasibility proof passed without window/GPU dependencies.
