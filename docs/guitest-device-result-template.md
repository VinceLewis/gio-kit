# Device gate result template

Copy this file for each Gate C or Gate D run. Store only sanitized text evidence
in the repository; keep APKs, extracted native files, signing keys, databases,
attachments, private logs, and screenshots with private data outside commits.

## Candidate identity

| Field | Value |
| --- | --- |
| Gate | C / D |
| Candidate status | smoke / frozen |
| Application ID | `<app-id>` |
| Artifact name/path | `<artifact>` |
| Version name | `<version-name>` |
| Version code | `<version-code>` |
| Source commit | `<full-commit>` |
| Dependency lock/module versions | `<identity-or-file>` |
| Gio version | `<gioui.org-version>` |
| ABI(s) | `<expected-arm64-v8a>` |
| Minimum SDK | `<min-sdk>` |
| Target SDK | `<target-sdk>` |
| Signer/status | `<debug-or-release-identity>; v2=<pass/fail>; v3=<pass/fail>` |
| EGL result | `<libEGL.so present; libEGL.so.1 absent; pass/fail>` |
| APK SHA-256 | `<64-hex-digest>` |
| Build/verification command | `<repository-owned-command>` |
| Build/verification date | `<ISO-8601>` |

## Device and execution

| Field | Value |
| --- | --- |
| Device/model | `<device>` |
| Android version/API | `<version/api>` |
| Install/update result | PASS / FAIL / BLOCKED |
| Launch result | PASS / FAIL / BLOCKED |
| Checklist used | `<Gate-C-or-Gate-D-document-and-revision>` |
| Tester/date | `<name-or-role>; <ISO-8601>` |
| Overall device result | PASS / FAIL / BLOCKED |

## Checklist results

| Item | Result | Notes/evidence |
| --- | --- | --- |
| `<checklist heading/item>` | PASS / FAIL / BLOCKED / NOT APPLICABLE | `<sanitized observation or reproduction>` |

## Findings

| ID | Severity | Reproduction | Expected/actual | Owner/status |
| --- | --- | --- | --- | --- |
| `<id>` | `<severity>` | `<steps>` | `<expected versus actual>` | `<repository/package; status>` |

Known exclusions (not exercised or unsupported; distinguish these from passes):

- `<exclusion and reason>`

User decision: **PASS / FAIL / BLOCKED**

Decision date and notes: `<ISO-8601; notes>`

If candidate source or dependencies changed after this run, replacement
artifact/version/SHA-256: `<new identity or none>`.
