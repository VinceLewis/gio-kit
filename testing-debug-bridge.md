# Testing Debug Bridge Plan

## Status and Decision Gate

**Status:** Phase 1 stopped negative on 2026-09-10; see
`docs/guitest-bridge.md`. Phase 2 is not authorized under the current Gio
v0.10.2 API and spike constraints.
**Scope:** Gio-Kit testing infrastructure and its Android demo only.
**Objective:** Let a repository-owned Termux CLI inspect and drive a manually
launched debug APK through Gio-Kit's existing semantic test vocabulary, without
putting a control surface in release artifacts.

This plan refines the deferred bridge evaluation in `docs/guitest-bridge.md`.
It does not reverse that decision by itself. Before implementation starts,
record user authorization in `test-framework-progress.md`. Phase 1 must pass
before choosing or publishing a protocol or public Go API.

Use GPT-5.6 Sol at high effort for the transport, lifecycle, security, and
release-exclusion work. Stop at every numbered phase boundary, update the
progress ledger, and preserve exact failures.

## Why Build It

The in-process `guitest` driver already provides deterministic selectors,
actions, idle handling, bounded snapshots, and application lifecycle hooks.
Installed Gio applications, however, appear to Android automation as a single
`SurfaceView`. A bridge can remove fragile coordinate tapping and make repeated
device scenarios faster and more reproducible.

The bridge is worthwhile only if it drives the same live application root and
Gio input path that renders the Android window. A shadow renderer, direct
controller mutation, or app-specific command API would give misleading device
results and must not be accepted as a substitute.

## Non-Goals

- Do not install, launch, stop, rotate, resize, or grant permissions to an APK.
  Repository rules continue to require user-mediated installation and launch;
  Android lifecycle actions remain separate device checks.
- Do not require ADB, an emulator, a desktop host, a network service, or an
  account.
- Do not expose arbitrary Go calls, files, SQL, shell commands, application
  records, or unbounded diagnostics.
- Do not replace pure-Go tests, real IME/accessibility checks, pixel inspection,
  release verification, or user device acceptance.
- Do not add application-specific records, policies, selectors, or behavior to
  Gio-Kit.
- Do not make bridge types dependencies of reusable widgets.

## Mandatory Safety Boundary

The bridge is remote control of the application. All of these constraints are
release blockers:

- Compile bridge code only with an explicit `guitestbridge` build tag.
- Use a distinct debug-only application ID and artifact name.
- Keep the default demo and every release build free of the bridge package,
  listener, debug UI, token strings, and bridge-only permissions.
- Bind only to `127.0.0.1` on an ephemeral port. Never bind wildcard, LAN, USB,
  or public interfaces.
- Generate a cryptographically random, short-lived session token at launch.
  Do not persist, log, commit, cache, or place it in command-line arguments.
- Reveal address and token only in an explicit, visually obvious debug screen.
  The user manually transfers the token to a CLI prompt.
- Authenticate before accepting any command; compare tokens in constant time,
  limit failed attempts, and close rejected connections.
- Allow one active controller. Expire credentials on shutdown and generate new
  credentials after process recreation.
- Enforce request deadlines, input sizes, output limits, selector complexity,
  frame limits, and bounded queues.
- Reuse the existing snapshot redaction and limits. Pixel output remains
  unredacted and is not part of the MVP.
- Close the listener and fail pending requests on application teardown. No
  bridge goroutine may outlive the application root.

## Phase 1 — Time-Boxed Device Feasibility Spike

Budget two hours. Add no stable protocol and no public reusable API during the
spike. Build a disposable, tagged demo variant and prove each item on this
Android/arm64 Termux device:

1. A manually launched debug APK can listen on Android loopback and Termux can
   connect to its ephemeral port without ADB or a broad network bind.
2. The generated Android manifest contains only the permissions actually
   needed by the tagged variant.
3. A short-lived token displayed by the debug APK can authenticate one Termux
   client; missing, stale, and incorrect tokens are rejected.
4. Disconnect/reconnect, background/resume, rotation, and orderly shutdown do
   not leak listeners, goroutines, requests, or credentials.
5. A command can be queued from the transport, executed on the Gio frame
   goroutine, and answered after a resulting frame without blocking Layout.
6. The command can inspect the live semantic tree used by the Android root.
7. At least one semantic tap and one editor insertion traverse the same live Gio
   input path as the rendered application. Confirm their visible/controller
   outcomes; transport success alone is insufficient.
8. Process recreation invalidates the old endpoint/token and allows an explicit
   new connection after the user reopens the app.
9. A normal untagged build has no bridge dependency or recognizable bridge
   endpoint/token implementation.

### Spike stop conditions

Stop and retain the current ADB/manual workflow if any of these is true:

- Gio's public Android event path cannot safely accept synthetic input for the
  live root.
- The only workable design lays out the same state through a second router,
  mutates controllers directly, or requires application-specific hooks.
- Loopback isolation or user-mediated authentication cannot be established.
- The listener or request dispatcher cannot be cancelled and joined cleanly.
- Release exclusion cannot be demonstrated mechanically.
- The spike exceeds its time box without a complete tap-and-type vertical
  slice.

Record the result in `docs/guitest-bridge.md` and
`test-framework-progress.md`. Delete disposable spike code if the result is
negative; keep the evidence and exact limitation.

## Phase 2 — Minimal Bridge Architecture

Proceed only after Phase 1 passes and the spike design is reviewed.

### Package boundaries

- `guitest/bridge`: tagged, application-neutral transport and frame dispatcher.
- `cmd/guitest-device`: repository-owned Termux client; it must remain useful
  without importing Gio window or GPU packages.
- `cmd/navigation-demo`: tagged composition only, owning the debug screen,
  listener lifetime, and integration with its existing root.
- Reusable router, grid, form, shell, picker, dialog, and presentation packages
  remain unaware of the bridge.

Do not export a stable bridge API until the demo and one consumer prove the
minimum seam. Keep transport details private while they are still changing.

### Execution model

The socket reader validates and bounds a request, then places it on a bounded
channel and requests a frame. The frame goroutine performs the action at a
documented safe point against the live test adapter. Results are collected
only after the action frame and any requested idle/settle frames complete.
The socket goroutine serializes the bounded response.

No socket read, write, wait, JSON encoding, selector parsing, screenshot work,
or application I/O may run inside Layout. The bridge must preserve Gio-Kit's
existing cancellation, coalesced invalidation, stale-result, and joined-close
contracts. A timed-out request is cancelled and cannot apply later.

### Initial command surface

Keep the MVP intentionally small:

- `hello`: negotiate one protocol version and authenticate.
- `health`: report bridge readiness, session identity, viewport, and limits.
- `dump`: return the existing bounded/redacted schema-v1 capture.
- `find`: resolve an existing selector and return bounded diagnostics.
- `tap`: use an existing selector.
- `type`: use an editor selector and bounded UTF-8 text.
- `back`: dispatch Gio's Android-equivalent Back input.
- `scroll`: use an existing selector and bounded physical-pixel delta.
- `wait`: wait for an existing selector/state predicate or application idle.
- `close`: close only the authenticated bridge session, not the application.

Defer double-tap, long-press, drag, raw input queues, resize, lifecycle control,
and screenshots until the minimum surface is stable. Never add arbitrary
coordinates as a fallback for a failed semantic selector.

### Protocol constraints

After the spike, specify a small versioned request/response envelope with:

- request ID, protocol version, command, deadline, and bounded payload;
- exactly one response per accepted request;
- structured error code, safe message, and retryability;
- explicit busy, unauthenticated, unsupported, timeout, cancelled, stale-session,
  selector, action, and output-limit errors;
- deterministic JSON fields suitable for the existing inspection tooling;
- a maximum request of 64 KiB and maximum response no larger than the existing
  4 MiB diagnostic hard cap;
- no unsolicited application data and no token echo.

Prefer length-delimited messages only after the spike proves that they simplify
bounded reads and reconnect behavior. Do not design streaming or multiplexing
for the MVP.

## Phase 3 — Verification and Hardening

Add focused tests before any consumer pilot:

- pure-Go framing, authentication, expiry, replay, size, deadline, cancellation,
  queue saturation, reconnect, and malformed-input tests;
- frame-dispatch tests proving ordering, one-at-a-time execution, timeout
  cancellation, invalidation, idle behavior, and joined shutdown;
- selector/action parity tests against the existing in-process driver;
- fuzz tests for the bounded decoder and selector payloads;
- ten repeated connect/action/disconnect cycles with no goroutine growth;
- tagged demo tests for navigation, Unicode typing, guarded Back, grid scrolling,
  validation, retry, and bounded dumps;
- a negative build/import test proving default packages do not depend on bridge
  or window/GPU code;
- APK inspection proving the normal release lacks the bridge package, debug UI,
  listener markers, and any bridge-only manifest permission;
- tagged APK signature, Android EGL, ABI, installation, manual launch, and
  user-executed device checklist results reported separately.

Run the existing core, demo, generated-reference, Termux, and GPU checks where
applicable. The bridge suite supplements them; no existing gate is weakened.
The race detector remains unavailable on Android/arm64, so also run race tests
on a supported Go host before treating the transport as hardened.

Target an idle semantic-command round trip below 250 ms on this phone, excluding
application work requested by `wait`. Record measurements rather than turning a
device-specific observation into a portable guarantee. Verify that stalled or
malicious clients do not block frames.

## Phase 4 — Consumer Pilot

Publish Gio-Kit first and select a real module version before integrating a
consumer. Pilot only through an application-neutral composition seam:

- the consumer owns its tagged debug artifact, app ID, bridge screen, and root;
- Gio-Kit owns protocol, authentication, dispatch, selectors, and diagnostics;
- no consumer vocabulary, policy, storage schema, or fixture enters Gio-Kit;
- no bridge code is linked by the consumer's default/release APK;
- local workspace replacements remain uncommitted and final checks use the
  published module with workspace overrides disabled.

The pilot must repeat fresh launch, navigation, form editing, guarded Back,
background/resume, process recreation/reconnect, rotation, and error recovery.
ADB or user actions still perform lifecycle operations; the bridge reconnects
and verifies application state afterward.

## Delivery Gates

- [ ] User authorizes implementation and the progress ledger records it.
- [ ] Phase 1 proves loopback, authentication, live semantic inspection, live
      tap/type injection, lifecycle cleanup, and release absence.
- [ ] The spike decision and evidence update `docs/guitest-bridge.md`.
- [ ] Phase 2 MVP passes focused tests and generated documentation checks.
- [ ] Security review confirms loopback-only binding, token lifecycle, bounds,
      cancellation, redaction, and one-controller behavior.
- [ ] Default and tagged dependency graphs remain correctly isolated.
- [ ] Full supported tests, vet, formatting, generated outputs, and diff checks
      pass with exact limitations recorded.
- [ ] Normal release and tagged debug APKs are separately built and inspected.
- [ ] The user manually installs, launches, and accepts the tagged demo workflow.
- [ ] A published Gio-Kit version passes the consumer pilot without a local
      replacement.
- [ ] Documentation explains setup, threat model, commands, reconnect behavior,
      limitations, and proof that release artifacts exclude the bridge.

## Estimate and Payoff Check

- Phase 1 spike: approximately two hours.
- Narrow MVP after a successful spike: four to eight additional hours.
- Hardened reusable bridge, documentation, and consumer pilot: one to three
  working days, depending on Android live-input constraints.

At the end of Phase 1, compare the remaining repeated device scenarios against
the implementation cost. Continue only if the bridge demonstrates semantic
accuracy and materially reduces interaction time. Do not justify completion by
counting transport or controller-level tests as Android UI control.
