# Repository Guidelines

## Mission and Source of Truth

Build the requirements in `gio-crud-requirements.md` as a reusable Go module that other Gio applications can import. The module, rather than the demonstration app, is the primary deliverable. Treat the requirements document and its acceptance criteria as authoritative. Keep public APIs independent of any one application's records, storage layer, theme, or backend.

This repository must also contain a functional Gio mobile test application. It is an integration harness and visual demo for the module, not a second implementation. It must exercise navigation and restoration, virtualized/paged grids, metadata-driven forms, validation, related-record flows, loading/error/empty states, and the composition scenarios from the requirements on an Android device.

## Local Reference Project

The only prior Gio project on this device is `/data/data/com.termux/files/home/projects/vingo`. Inspect it before designing Gio lifecycle code, Android packaging, release verification, or the mobile test UI. Useful reference material includes:

- `main.go` for Gio window/event-loop, immediate-mode state, navigation, redraw, and responsive layout patterns.
- `build-notes.md` for Termux-specific testing and device-verification lessons.
- `tools/build-release-apk.sh` and `tools/giocmd-termux/` for the working Android build pipeline and patched `gogio` implementation.
- `go.mod` for a known working Gio dependency baseline (`gioui.org v0.10.2`).

Reuse or adapt Vingo code where it genuinely reduces risk, but do not couple this module to Vingo. Check provenance and preserve applicable copyright or license notices when copying non-trivial code. Do not copy application-specific language-learning, SQLite, content, signing identity, package ID, version, or output names.

## Architecture

Keep reusable packages importable and free of `internal` visibility restrictions that would prevent downstream use. Put the mobile harness in a separate command/application package so importing the library never pulls in application policy or demo data. Keep platform-specific code behind small interfaces or build-tagged files. Public state must be explicit and owned outside Gio frame layout calls; layout code must not perform uncontrolled blocking I/O.

Design APIs for cancellation, deterministic tests, and asynchronous data sources. Avoid package globals. Keep route and persisted state versionable and serializable. Preserve list scroll/filter/selection state across navigation. Form and grid editing must share validation rather than duplicate it.

## Termux Android Toolchain

This repository is built directly under Termux on Android/arm64. Preserve the known-working pinned baseline unless an intentional, documented migration is requested and verified on-device:

- SDK root: `/data/data/com.termux/files/usr/opt/android-sdk`
- Android build-tools: `35.0.0`
- compile/target SDK: API 35
- minimum SDK: API 24
- NDK: `29.0.14206865` (r29)
- NDK host-toolchain directory: `toolchains/llvm/prebuilt/linux-x86_64` even though Termux runs on Android/arm64; this is a patched layout assumption.
- Termux `aapt2`: `/data/data/com.termux/files/usr/bin/aapt2`, package version `16.0.0.4-2`; the SDK build-tools `aapt2` must resolve to this Termux-native executable.
- Termux-native `/data/data/com.termux/files/usr/bin/d8`, `apksigner`, and `zipalign` are required by the patched build tooling.
- Android build target: `arm64`; use the NDK sysroot and API-24 aarch64 library path as demonstrated by Vingo.

Do not replace the patched `gogio` flow with an upstream binary and do not upgrade SDK, NDK, build-tools, Gio, Go language version, or packaging tools casually. Record any necessary deviation here and in the build script, and prove the build and signing steps locally. The user is responsible for installing and launching APKs on the device. Never commit keystores or signing secrets; debug signing is acceptable only for local testing.

## Build and Verification

Provide repository-owned scripts derived from the working Vingo approach; callers should not need to remember environment flags. The Android build script must validate tool paths and versions, build a signed arm64 APK, verify its signature, and verify that `libgio.so` depends on Android `libEGL.so`, not desktop `libEGL.so.1`. Use a project-specific application ID and artifact name.

Run focused pure-Go package tests throughout development. For full default-package tests and vet on this Termux host, run `./tools/test-termux.sh -count=1`. Vulkan and EGL headers are already in the pinned NDK but are absent from the default compiler search path. The script scopes the APK build's include/library paths and C-warning workaround, plus `-llog` for standalone CGO test executables, to its child commands; do not persist these flags with `go env -w` or change the toolchain. Keep running the separate core/demo framework scripts for dependency-isolation checks and tagged demo tests. If a check fails, preserve the exact failure and run unaffected packages separately; a partial suite is not a pass. Successful compilation does not prove a usable GPU/headless backend or Android UI behavior. The Go race detector remains unsupported on Android/arm64.

Use the testing gates below for testing-framework and UI work:

- Gate A, continuous development: run focused tests for each change and the production-root in-process suite for UI work. Real Gio widgets and `input.Router` input must drive important interaction assertions.
- Gate B, repository checkpoint: run the complete repository-owned test, vet, generated-artifact, dependency-isolation, cancellation, and restoration checks. Preserve exact failures; unaffected packages are only a partial result.
- Gate C, platform smoke: build, sign, inspect, and copy a stable arm64 APK after establishing or materially changing a window, lifecycle, storage, clipboard, attachment, IME/focus, accessibility, GPU, packaging, or Android platform seam. The user checks launch/render, tap/type, Android Back, resize/rotation, large-list scrolling, background/resume, and relaunch.
- Gate D, frozen-candidate device acceptance: freeze source and dependencies, build and inspect one identified signed candidate, and give the user the complete acceptance checklist. Any fix produces a new candidate and repeats affected gates.

Pure widget or application-policy changes do not automatically trigger Gate C. Build and inspect Android artifacts mechanically at delivery checkpoints and whenever packaging or platform-facing code changes. Android-only behavior—including the real IME, touch timing and physics, accessibility services, GPU/device rendering, window insets, lifecycle delivery, and OS process recreation—remains device-only evidence. In-process simulation must not be reported as platform proof.

Agents must never install or launch APKs automatically through ADB, `pm`, `am`, `monkey`, Termux intents, or equivalent mechanisms, even when ADB is available. The user owns installation, launch, lifecycle actions, and visual acceptance. Unit and integration tests remain required but do not replace applicable device testing.

## Coding and Test Standards

For testing-framework implementation or use, read `docs/guitest.md` and
`test-framework-progress.md`. Keep the ledger current at checkpoints and before
pausing so another session can resume without repeating completed work.

Use idiomatic Go and `gofmt`. Keep Gio layout functions small; retain widget and interaction state across frames and explicitly invalidate when external state changes require redraw. Never block the frame loop on data fetching. Make async completion safe against stale requests and disposed screens.

Map automated tests to the requirement IDs (`N*`, `G*`, and form requirements). Add unit tests for routing, URL parsing, guards, serialization, sorting/filter construction, paging, selection, validation, dirty state, and policy evaluation. Add integration tests using deterministic fake data sources, including 10,000-row virtualization scenarios, delayed responses, cancellation, retry, and out-of-order completion. The mobile harness must expose each acceptance scenario without needing a real backend.

Inspect the working tree before edits and preserve unrelated user changes. Treat generated APKs and build directories as disposable artifacts and keep them out of version control.
