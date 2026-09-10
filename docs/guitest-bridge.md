# Device control bridge evaluation

Decision (2026-09-10): **Phase 1 failed; do not implement the bridge on Gio
v0.10.2.** The in-process driver, bounded JSON artifacts, and local inspection
CLI remain the supported delivery.

The spike stopped before APK construction at its explicit public-event-path
stop condition. The live Android window owns the `input.Router`. Public
`app.Window` has `Event`, `Invalidate`, `Option`, `Perform`, and `Run`, but no
event injection method. Public `input.Source` can consume events and execute
`input.Command` values, but cannot queue `pointer.Event`, `key.Event`, or
`key.EditEvent`. Only `input.Router.Queue` can do that, and the router pointer
inside `FrameEvent.Source` is unexported. Gio's backend callbacks that process
events and expose live semantics are also unexported.

A disposable compile probe confirmed the boundary with:

```text
source.Queue undefined (type input.Source has no field or method Queue)
```

Reflection/unsafe access, `go:linkname`, or patching Gio would couple the bridge
to private internals. A second router or shadow layout would not drive the live
Android root. Controller hooks would be application-specific. These are all
excluded by the spike rules, so loopback and authentication work cannot rescue
the required tap-and-type vertical slice. No spike server or APK was retained.

Reconsider only if Gio gains a supported API for injecting input into, and
reading semantics from, the live window router, or if the requirement is
explicitly changed to permit a maintained Gio fork/private-internals adapter.
Loopback, authentication, lifecycle cleanup, and release exclusion would still
need a new device spike.
