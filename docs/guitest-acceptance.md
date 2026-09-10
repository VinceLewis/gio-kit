# Framework acceptance coverage

Audited against the repository tests on 2026-09-10. Controller tests remain
separate from real-input UI tests. The latter route input through Gio and assert
UI or controller outcomes. This inventory describes test scope; it is not a
record of a test run.

For the risk audit, **covered** means the deterministic in-process boundary has
the required assertions. **Partial** means useful coverage exists but a planned
case or seam is absent. **Device-only** means the public in-process driver cannot
establish the fact. A covered simulation never proves equivalent Android
behavior.

## Requirements coverage

| Requirements | Automated coverage |
| --- | --- |
| N1–N4, N6–N7 | Demo harness modal/deep-link/guarded Back/resize; routed dirty-form STAY/DISCARD and retained list state |
| N5 | Router serialization tests and demo fresh-root restoration from temporary persisted state |
| N8 | Demo delayed external navigation/disposal; virtual-time, cancellation, teardown, and stale-grid-result tests |
| G1–G2 | 10,000-item wheel/touch list tests, 10,000-row grid selection/open/scroll, bounded virtualized snapshot, and responsive controller/layout tests |
| G3–G4 | Routed grid sort; controller multi-sort, filter construction, and fake/SQLite source tests |
| G5 | Routed checkbox versus row opening; controller selection across page boundaries |
| G6 | Grid responsive/table minimum-width tests; card/table visual behavior remains a device check |
| G7–G8 | Routed error/retry/empty scenario; controller paging and out-of-order completion tests |
| G9 | Controller serialized preferences round trip; demo retained list state |
| Form metadata/validation | Named field routing, read-only rejection, choice selection, required error after blur, rule reevaluation, reference ID/display, and attachment/time value retention |
| Form dirty/submission | Routed submit failure/retry, guarded Back immediately after editing, edit-during-submit, shared validation, and cancel/join teardown tests |
| Related/composed controls | Picker search/disabled/select, scoped presentation actions, selected shell navigation, and confirmation dialog input |
| Diagnostics | Checked schema, deterministic ordering, default/custom redaction, cycles/depth/byte caps, short writers, and bounded grid/form samples |
| Optional graphics | Default unavailable fallback and isolated `guitestgpu` headless red-pixel readback; tolerant image comparison |

## Platform-adjacent risk audit

| Risk or seam | Status | Existing evidence | Precise remainder |
| --- | --- | --- | --- |
| Resize and metrics | Covered | The named compact/tall/landscape/tablet/split-screen matrix includes keyboard-like contraction/restoration; demo and responsive package tests also exercise changed constraints and metrics. | Real Android insets and window delivery are device-only. |
| Back | Covered | The driver routes `key.NameBack`; the demo exercises guarded Back immediately after an edit and restored-root Back. | Android system Back dispatch, keyboard interaction, and gesture navigation are device-only. |
| Unicode editing | Covered | Routed tests cover Unicode insertion, rune-based selection/replacement, composition-shaped events, deletion, blur, and immediate Back. | Android IME composition and selection remain device-only. |
| Focus | Covered | `TestTermuxRealWidgetInput` proves pointer focus; routed form validation proves focus loss/blur. | Top-level Android focus, IME visibility, and focus behavior across lifecycle/insets are device-only. |
| Long press | Covered | Virtual tests cover exact holds, threshold boundaries, cancellation, target edges, the form text toolbar, and word selection. | OS selection handles, touch timing, and physical feel are device-only. |
| Drag and pointer cancellation | Covered | Managed gestures cover move/cancel, drag cancellation, held-pointer disposal, edge arbitration, and list recovery. | Android gesture arbitration and physics are device-only. |
| Wheel/touch scroll and virtualization | Covered | Wheel/touch tests scroll away and recover through 10,000 logical items while keeping semantics bounded. | Sustained physical scrolling, fling behavior, visual recovery, and touch feel are device-only. |
| Clipboard | Covered | A bounded asynchronous adapter is wired behind Gio's real read/write commands and covers unavailable, denied, oversized, empty, Unicode, and stale reads. | Android clipboard permissions, lifecycle, and system UI are device-only. |
| Fresh-root restoration | Covered | Repeated demo-root recreation restores persisted bytes across generations and proves queued work is cancelled and joined. | OS process death and lifecycle delivery are device-only. |
| Delayed work | Covered | Virtual clock tests expose pending state before advance; the demo external event and async form/grid tests apply completion on later frames. | Wall-clock scheduling and OS suspension are outside the deterministic proof. |
| Cancellation and stale results | Covered | Driver, form, grid, and demo-root tests cover joined cancellation, rapid navigation, and stale/out-of-order work. Picker, shell, dialog, and presentation are synchronous renderers/callback emitters with no owned async completion seam. | OS suspension/termination remains device-only. |
| Joined close | Covered | `TestCloseCancelsVirtualSleeperAndIsIdempotent`, `TestCloseCancelsAndWaitJoinsSubmission`, demo external disposal, and database-startup close assert joined teardown. | Forced Android termination cannot run application cleanup and remains device-only behavior. |
| Bounded semantics and dumps | Covered | Dump tests assert deterministic bounded/redacted output, invalid schema rejection, cycles, output limits, and no partial writes; large-grid semantics stay bounded. | Gio public APIs cannot prove exact occlusion, clip stacks, interactability, or top-level focus. |
| Component snapshots | Covered | Router, grid, form, shell, picker, dialog, and presentation providers have direct bounded capture/state assertions. | Provider evidence cannot establish platform rendering or accessibility behavior. |
| Reusable-control semantics | Covered | The reusable control inventory has routed assertions for roles, names, selected state, enabled/disabled behavior, grouping/scoping, and actual outcomes. | TalkBack order and announcements are device-only. |
| Optional headless pixels | Covered | The tagged screenshot test reads back the expected red pixel and image comparison is tested; the default build remains graphics-free. | Backend availability is host-specific. Android GPU/display rendering, font rasterization, density, and driver defects are device-only. |
| Android window/lifecycle/process delivery | Device-only | No supported live-window input bridge exists; fresh-root simulation covers only application-owned reconstruction. | System Back dispatch, rotation/split-screen callbacks, background/resume, window focus, and OS process recreation require device acceptance. |
| Android IME and real insets | Device-only | Synthetic key/edit events and `Resize` do not involve an Android IME or window manager. | Composition, selection handles, action keys, autocorrect, keyboard visibility, and real inset delivery require device acceptance. |
| Android accessibility services | Device-only | Semantic trees and state are inspectable in process, but no accessibility service participates. | TalkBack traversal, announcements, focus movement, and platform presentation require device acceptance. |
| Android display/GPU output | Device-only | The optional probe uses Gio's headless backend on this host. | Android surface creation, device GPU/display output, font rasterization, and driver-specific defects require device acceptance. |

## Explicit device-only list

Only device acceptance can establish:

- Android Back dispatch, gesture navigation, window focus, lifecycle callbacks,
  background/resume delivery, and OS process recreation;
- real IME composition, selection handles, keyboard resize/insets, action keys,
  autocorrect, and keyboard-specific focus behavior;
- Android clipboard integration, permission/policy behavior, and system clipboard
  UI;
- TalkBack traversal order, announcements, focus movement, disabled-state
  communication, and platform accessibility services;
- physical touch target usability, long-press thresholds/feel, gesture
  arbitration, scrolling/fling physics, and edge interactions;
- real window insets, rotation, split-screen delivery, density/font scaling,
  clipping, redraw, and visual stability; and
- Android GPU/display output, device-driver behavior, startup/ANR/crash behavior,
  and large-data visual performance.

Use the reusable [Gate C smoke checklist](guitest-gate-c-smoke.md), [Gate D
acceptance checklist](guitest-gate-d-acceptance.md), and [result
template](guitest-device-result-template.md) for those claims.
