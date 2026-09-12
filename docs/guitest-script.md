# Declarative Gio test scripts

Package `guitest/script` executes strict, versioned JSON scenarios against an
already-created `guitest.Driver`. It uses the same Gio widgets and
`input.Router` path as direct Go tests. It does not control a live APK, provide
application discovery, or own the driver lifecycle.

## Contract and validation

A v1 document contains `version`, `name`, optional consumer-owned `fixture`,
optional default timeout, and an ordered `steps` array. Decode scripts with
`script.Decode`; it applies the 1 MiB input limit, rejects duplicate or unknown
fields and trailing JSON, and runs the complete Go validator before execution.
The checked-in JSON Schema describes the closed structural unions. Aggregate
limits such as total selector depth/terms, UTF-8 byte counts, duration bounds,
and duplicate step IDs are enforced by `script.Decode` and remain authoritative.

Selectors are literal semantic queries: label, description, inherited name,
role, text, bound test ID, enabled/selected state, conjunction, ancestry,
containment, and explicit zero-based occurrence. A target action waits
read-only for one compatible, interactable match. Ambiguity fails immediately;
the input action is dispatched exactly once and is never retried.

Assertions poll frames until true or until the step deadline. Actions do not
implicitly settle. `settle` waits only for due frames and application idle;
`advance` is the only operation that moves future virtual timers. Large and
virtualized collections therefore require explicit bounded scroll steps.

## Consumer launcher

Package `guitest/scriptcli` supplies `Run` for a compiled consumer executable:

```go
result, err := scriptcli.Run(ctx, os.Args[1:], scriptcli.Config{
    Open:     openApplication,
    Fixtures: scriptcli.Names{"synthetic.basic.v1": {}},
    Probes:   scriptcli.Names{"records": {}},
})
```

The command syntax is `run -script FILE -artifacts DIRECTORY`. The artifact
directory must be a dedicated private directory created by the runner or
already owned by runner output; shared, symlinked, root, unrelated, or
crash-remnant directories are rejected. Fixture and probe names are checked
before the opener runs. The opener returns the actual application root and
per-instance probe callbacks; it must not construct a parallel test UI.

`script.Run` never closes its caller's driver. `scriptcli.Run` owns the instance
returned by its opener: it closes the driver, then invokes the optional
opener-owned cleanup, joining all errors. Application `Harness.Close` remains
responsible for cancellation, worker joins, persistence drain, and storage
closure.

## Probes, snapshots, and privacy

`expectSnapshot` reads one bounded, redacted component snapshot through
`Driver.Capture` and a small RFC 6901 JSON Pointer. `expectProbe` can call only
the consumer's pre-registered names, with bounded JSON arguments and results.
Probe callbacks must be read-only, deterministic, cancellation-aware, and
must never expose SQL, arbitrary files, commands, network calls, or application
discovery.

When configured, a run publishes bounded `result.json`, privacy-safe
`trace.json`, numbered captures, and a failure dump. Trace entries contain
indices, operators, statuses, frame/virtual-time movement, stable error codes,
and numeric artifact names; they never copy typed text, selector/expected
values, clipboard data, application errors, semantic trees, or probe results.
Files are private and individually atomically replaced, with `result.json`
published last as the generation marker and stale deterministic files removed.
A portable filesystem cannot make a multi-file set power-loss atomic; detected
crash remnants cause later runs to fail closed.

Screenshots use an injected callback and are explicitly unredacted. Keep them
optional unless the consumer provides rendering, use synthetic fixtures, and
store them outside version control.

## Generic artifact commands

`cmd/guitest` provides `script-schema`, `trace-schema`, `validate-script`, and
`inspect-trace`. It deliberately has no universal `run` command; only a
consumer-owned compiled launcher can construct an application.

This runner provides in-process UI evidence only. Android IME behavior,
TalkBack, real touch physics, GPU/display output, insets, lifecycle delivery,
process recreation, installation, signing, and visual acceptance remain the
separate platform gates described in [the main guide](guitest.md).
