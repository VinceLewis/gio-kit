# gio-kit

## Working here

Every tool call re-uploads the whole conversation, so ten one-line calls cost
ten full contexts. Batch independent reads, greps and shell steps into one
call; read ranges (`sed -n`, `grep -n`) rather than whole files; run the gate
once rather than watching it. A hook says so at the moment it matters, so this
note stays short deliberately.

A turn costs a full context whether or not it carries work, so do not spend
turns on words. No progress commentary, no restating the task, no summarising a
file just read, no announcing the next step. Act, then report once at the end.

Run long builds in the foreground with a generous timeout. Backgrounding a
build and then polling it turns one wait into dozens of round trips at full
context each — reliably more expensive than the build.

Canonical copy of these rules, and the measurements behind them, in
`~/projects/dev-kit` (`claude/CLAUDE.md`, `docs/measurements.md`).
