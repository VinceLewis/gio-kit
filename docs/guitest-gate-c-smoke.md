# Gate C platform smoke checklist

Use this short gate after a new or materially changed window, lifecycle,
storage, clipboard, attachment, IME/focus, accessibility, GPU, packaging, or
Android platform seam. Test the exact signed arm64 APK recorded in the [device
result template](guitest-device-result-template.md). Installation, launch, and
device interaction are user actions.

- [ ] Confirm the recorded app ID, version/code, source commit, SHA-256,
      signature status, ABI, SDK levels, and EGL dependency match the APK.
- [ ] Install or update it, launch from Android, and confirm the first screen
      renders without a crash, ANR, clipping, blank frame, or persistent redraw.
- [ ] Tap into one representative editor, type and edit Unicode with the real
      keyboard, dismiss/reopen the keyboard, and complete the workflow.
- [ ] Use Android Back from the editor and from the next screen; confirm the
      expected guard/navigation behavior and no lost committed state.
- [ ] Rotate or use split-screen/resize where supported; confirm content,
      controls, keyboard avoidance, and redraw remain usable.
- [ ] Sustain touch scrolling in the large list/grid, open an item, return, and
      confirm list position and relevant filter/selection state are retained.
- [ ] Background and resume the app, then stop it manually through Android UI
      and relaunch; confirm the documented route/state restoration contract.
- [ ] Record PASS, FAIL, or BLOCKED for every item, including device details,
      reproduction steps, and explicit exclusions. Do not infer a pass from
      installation or launch alone.

Any source or dependency fix creates a new APK identity and invalidates this
result for the changed behavior.
