# Test Framework Progress

Plan: [test-framework-plan.md](test-framework-plan.md).

## Current checkpoint

- Started: 2026-09-09. User confirmed GPT-6 Astra / max and authorized commit,
  push, and execution of the plan.
- Status: preparing the plan checkpoint, then the Termux input/semantics proof.
- Resume here: complete step 1 before settling the public API. Read this file,
  repository guidance, and the current working tree before continuing.
- Device gates remain mandatory. Stop after packaging material UI changes for
  the user to install, launch, and test; do not advance through a pending gate.

## Delivery checklist

| Step | Deliverable | Status |
| --- | --- | --- |
| 1 | Termux input.Router pointer/editor/semantics proof | Pending |
| 2 | Layout, invalidation, time, idle, cleanup contract; demo root | Pending |
| 3 | Deterministic core frame driver | Pending |
| 4 | Reusable component semantics audit and improvements | Pending |
| 5 | Selectors and interaction actions | Pending |
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

## Device acceptance

No new APK or device result yet.

## Checkpoint history

- 2026-09-09: execution authorized; progress ledger created before coding.
