# Gate D frozen-candidate acceptance checklist

Freeze source and dependencies before starting. Complete the [device result
template](guitest-device-result-template.md) for the exact signed APK. Do not
change code or dependencies during acceptance; a fix creates a new candidate
and requires the affected gates again.

Mark every item PASS, FAIL, BLOCKED, or NOT APPLICABLE with a reason.

## Candidate and startup

- [ ] Artifact identity, SHA-256, source commit, Gio version, ABI, SDK levels,
      signature, and Android EGL checks match the frozen candidate.
- [ ] Install/update and first launch succeed without crash, ANR, blank frame,
      persistent redraw, or unexpected loss of supported existing data.
- [ ] Loading, ready, empty, filtered-empty, error, and RETRY states are
      distinct and usable; repeat one retry after correcting its cause.

## Navigation and restoration

- [ ] Exercise primary navigation, deep links/external navigation, modal routes,
      and dialogs; cancel and confirm dialogs and verify the intended outcome.
- [ ] Navigate list to record and back. Preserve documented filter, scroll,
      selection, and route state; do not expect unsaved values unless the app
      explicitly promises them.
- [ ] Edit a form and immediately use Android Back. Verify STAY retains the
      edit and DISCARD leaves only after confirmation.
- [ ] Navigate rapidly while loading. Late results must not reopen stale
      screens, overwrite current state, or corrupt the session.
- [ ] Background/resume with a list, edited form, dialog, and loading state
      where practical; verify redraw and documented state retention.
- [ ] Stop the process manually through Android UI and relaunch. Verify the
      documented route/session/theme/state restoration and missing-record
      recovery behavior.

## 10,000-row grid

- [ ] In card and table modes where supported, search/filter, sort (including
      repeated direction changes), select rows without opening them, open a key
      cell/row, page/load more, and return without losing relevant state.
- [ ] Sustain slow and fast touch scrolling through the 10,000-row data set,
      reverse direction, interact after the scroll settles, and check for
      stalls, jumps, duplicates, missing rows, blank regions, or stale data.
- [ ] Exercise fetch failure, RETRY, unfiltered empty, and filtered-empty states.

## Forms, references, and clipboard

- [ ] Use the real keyboard for Unicode, caret movement, selection,
      replacement, deletion, multiline input where supported, and focus changes.
- [ ] Exercise required/format/range validation on blur and submit. Invalid or
      busy actions must reject input; errors must identify the responsible
      field and clear after correction.
- [ ] Exercise choice and reference lookup/search/disabled/select flows. Verify
      the displayed reference and saved record identity remain consistent.
- [ ] Exercise Save and Save & Close, submission failure, correction, and retry;
      verify dirty state and committed values after returning/relaunching.
- [ ] Long-press editable text and exercise SELECT ALL, COPY, CUT, and PASTE
      with Unicode and empty clipboard content where Android permits. Verify
      selection handles, menu placement, denial/unavailability behavior, and no
      stale pasted result.
- [ ] Confirm read-only and unsupported attachment/connected controls are
      visibly unavailable, correctly described, and non-mutating.

## Layout, touch, accessibility, and visuals

- [ ] Rotate between portrait and landscape or use split-screen where
      supported with a list, edited form, dialog, and loading state open.
      Confirm keyboard avoidance, scrolling, focus, clipping, and redraw.
- [ ] Check controls at target edges, repeated taps, drag cancellation,
      long-press threshold/feel, fling/scroll arbitration, and interactions
      immediately before navigation/disposal.
- [ ] With TalkBack where available, traverse all material reusable controls.
      Verify useful role/name/state, grouping, order, disabled-state
      communication, error announcements, and no unlabeled actionable control.
- [ ] Check compact and large layouts, system font/density settings in scope,
      contrast, truncation, overlap, touch target size, stale frames, flicker,
      missing glyphs, and other visual defects.

## Result

- [ ] Record crashes, ANRs, relevant sanitized logs, exact reproduction steps,
      and screenshots where useful. Keep private data, keys, databases, APKs,
      and extracted native files outside commits.
- [ ] Record an explicit user PASS, FAIL, or BLOCKED result plus known
      exclusions. Installation, launch, host tests, and headless pixels alone do
      not pass Gate D.
