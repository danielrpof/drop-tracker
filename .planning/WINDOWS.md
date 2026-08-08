---
schema_version: 1
open_count: 2
waived_count: 1
fixed_count: 0
total_count: 3
last_updated: 2026-08-08T00:21:10.539Z
---

# Broken Windows Ledger

> Cross-phase defect register. `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | 02 | stub | internal/watchlist/service.go |  | List/UpdatePreferences/Remove return errNotImplemented; no route registered against any of them until plans 02-03/02-04 fill the bodies | open |  | 2026-08-06T00:53:49.438Z |  |
| 2 | quick-260806-hfn | deviation | internal/db/migrate_test.go |  | gitleaks history scan found 4 findings (fake DSN test-fixture password, no real credential) in commits fc3c02d/1dc505a/25c285c; accepted per human decision, not suppressed via .gitleaksignore -- see 260806-hfn-SUMMARY.md | open |  | 2026-08-06T18:05:12.264Z |  |
| 3 | 03 | unrun-verify | internal/httpserver/search.go |  | Live happy-path human-check (curl against running binary + real musicbrainz.org, confirming real Drake artist results) could not run in this sandbox: no outbound TLS egress to musicbrainz.org. D-03 degraded-path behavior was confirmed live instead (200 with status:error, no leaked text). | waived | Confirmed via /gsd-verify-work 03: reproduced with plain curl (bypassing drop-tracker's Go client entirely) on a real dev WSL2 machine with genuine internet access -- TLS handshake fails identically (server decode_error alert after ClientHello, IPv6 route unreachable falling back to IPv4). Third independent environment to hit this, Deezer unaffected each time. Confirmed environmental (WSL2 network path to musicbrainz.org), not a drop-tracker defect -- see 03-UAT.md gap G-03-1. | 2026-08-07T22:00:10.701Z | 2026-08-08T00:21:10.539Z |

````json
[
  {
    "id": 1,
    "kind": "stub",
    "phase": "02",
    "file": "internal/watchlist/service.go",
    "line": null,
    "description": "List/UpdatePreferences/Remove return errNotImplemented; no route registered against any of them until plans 02-03/02-04 fill the bodies",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-06T00:53:49.438Z",
    "resolved_at": null
  },
  {
    "id": 2,
    "kind": "deviation",
    "phase": "quick-260806-hfn",
    "file": "internal/db/migrate_test.go",
    "line": null,
    "description": "gitleaks history scan found 4 findings (fake DSN test-fixture password, no real credential) in commits fc3c02d/1dc505a/25c285c; accepted per human decision, not suppressed via .gitleaksignore -- see 260806-hfn-SUMMARY.md",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-08-06T18:05:12.264Z",
    "resolved_at": null
  },
  {
    "id": 3,
    "kind": "unrun-verify",
    "phase": "03",
    "file": "internal/httpserver/search.go",
    "line": null,
    "description": "Live happy-path human-check (curl against running binary + real musicbrainz.org, confirming real Drake artist results) could not run in this sandbox: no outbound TLS egress to musicbrainz.org. D-03 degraded-path behavior was confirmed live instead (200 with status:error, no leaked text).",
    "status": "waived",
    "reason": "Confirmed via /gsd-verify-work 03: reproduced with plain curl (bypassing drop-tracker's Go client entirely) on a real dev WSL2 machine with genuine internet access -- TLS handshake fails identically (server decode_error alert after ClientHello, IPv6 route unreachable falling back to IPv4). Third independent environment to hit this, Deezer unaffected each time. Confirmed environmental (WSL2 network path to musicbrainz.org), not a drop-tracker defect -- see 03-UAT.md gap G-03-1.",
    "recorded_at": "2026-08-07T22:00:10.701Z",
    "resolved_at": "2026-08-08T00:21:10.539Z"
  }
]
````
