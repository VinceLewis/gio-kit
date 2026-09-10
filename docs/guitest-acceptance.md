# Framework acceptance coverage

Controller tests remain separate from real-input UI tests. The latter assert
both UI and controller results after routing input through Gio.

| Requirements | Automated coverage |
| --- | --- |
| N1–N4, N6–N7 | Demo harness modal/deep-link/guarded Back/resize; ADL real-root add/edit/discard/back pilot |
| N5 | Demo and ADL fresh-root restoration using temporary persisted state |
| N8 | Demo delayed external navigation/disposal; virtual-time, cancellation and teardown driver tests |
| G1–G2 | 10,000-item wheel/touch list tests, 10,000-row grid selection/open/scroll and bounded virtualized snapshot |
| G3–G4 | Routed grid sort; controller multi-sort/filter-construction and fake-source tests |
| G5 | Routed checkbox versus row opening; controller selection across page boundaries |
| G6 | Existing grid layout/responsive tests plus card/table mobile gate |
| G7–G8 | Routed error/retry/empty scenario; controller paging and out-of-order completion tests |
| G9 | Controller serialized preferences round trip; demo retained list state |
| Form metadata/validation | Named field routing, read-only rejection, choice selection, required error after blur |
| Form dirty/submission | Routed submit failure/retry, guarded Back after edits, controller edit-during-submit and shared validation tests |
| Related/composed controls | Picker search/disabled/select, scoped presentation actions, selected shell navigation and confirmation dialog |
| Diagnostics | Checked schema, deterministic ordering, sensitive fields and custom redaction, cycles/depth/byte caps and short writers |
| Optional graphics | Default unsupported fallback and isolated tagged headless pixel readback; tolerant image comparison |

Real Android Back dispatch, IME, accessibility, rotation, process termination,
long-press/touch feel, and large-data visual behavior require the combined user
device checklist in the progress ledger. No host result certifies those facts.
