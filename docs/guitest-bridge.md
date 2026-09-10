# Device control bridge evaluation

Decision (2026-09-10): defer bridge implementation. The in-process driver,
bounded JSON artifacts, and local inspection CLI form the first delivery.
Wireless ADB installation is a separate user-authorized device aid; pairing
does not prove an app-to-Termux transport or authorize release remote control.

The plan requires a device spike before protocol design. A manually launched
debug APK must establish loopback reachability, Android permissions, endpoint
discovery, explicit user-mediated short-lived token handoff, reconnect, and
shutdown. Those facts are unverified. No transport/authentication protocol is
chosen and no server is included in this release.

After API/schema and mobile acceptance stabilize, a separate debug-only spike
can evaluate those points. Any future commands must serialize onto the frame
loop, enforce deadlines/output limits, and be excluded at compile time from
release APKs. Actual Android surface capture and accessibility instrumentation
remain separate from the working in-process headless renderer.
