---
schema_version: 1
open_count: 4
waived_count: 1
fixed_count: 3
total_count: 8
last_updated: 2026-09-02T22:44:55.675Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | 02 | stub | internal/watchlist/service.go |  | List/UpdatePreferences/Remove return errNotImplemented; no route registered against any of them until plans 02-03/02-04 fill the bodies | open |  | 2026-08-06T00:53:49.438Z |  |
| 2 | quick-260806-hfn | deviation | internal/db/migrate_test.go |  | gitleaks history scan found 4 findings (fake DSN test-fixture password, no real credential) in commits fc3c02d/1dc505a/25c285c; accepted per human decision, not suppressed via .gitleaksignore -- see 260806-hfn-SUMMARY.md | open |  | 2026-08-06T18:05:12.264Z |  |
| 3 | 03 | unrun-verify | internal/httpserver/search.go |  | Live happy-path human-check (curl against running binary + real musicbrainz.org, confirming real Drake artist results) could not run in this sandbox: no outbound TLS egress to musicbrainz.org. D-03 degraded-path behavior was confirmed live instead (200 with status:error, no leaked text). | waived | Confirmed via /gsd-verify-work 03: reproduced with plain curl (bypassing drop-tracker's Go client entirely) on a real dev WSL2 machine with genuine internet access -- TLS handshake fails identically (server decode_error alert after ClientHello, IPv6 route unreachable falling back to IPv4). Third independent environment to hit this, Deezer unaffected each time. Confirmed environmental (WSL2 network path to musicbrainz.org), not a drop-tracker defect -- see 03-UAT.md gap G-03-1. | 2026-08-07T22:00:10.701Z | 2026-08-08T00:21:10.539Z |
| 4 | 14 | deviation | .env.example |  | INSTANCE_PASSPHRASE / TRUST_PROXY_HEADERS added to .env.example by operator during 14-01 (file is denied to agent tools; plan 14-04 formalizes wording) | fixed |  | 2026-08-29T16:16:47.738Z | 2026-08-29T17:20:29.066Z |
| 5 | 14 | lint-warning | web/app/root.tsx | 19 | tsc --noEmit fails on stale react-router typegen artifact (./+types/root TS2307); pre-existing, react-router build passes; needs a typegen pretypecheck step | open |  | 2026-09-01T03:23:20.970Z |  |
| 6 | 15 | deviation | cmd/coverage-report/testdata/baseline-metrics-backend.json |  | Task 1 baseline sidecar fixtures carried 41-char sha fields; corrected to valid 40-char in 20e6d68 (resolved) | fixed |  | 2026-09-02T22:19:13.272Z | 2026-09-02T22:19:34.900Z |
| 7 | 15 | deviation | cmd/coverage-report/main.go |  | backendTotalPct summed numStmts per profile line, inflating the denominator ~10x on a real merged go-test profile (reported 7.97% vs go tool cover 90.0%); fixed in b1967f6 by merging blocks by position (resolved) | fixed |  | 2026-09-02T22:30:17.794Z | 2026-09-02T22:30:26.122Z |
| 8 | 15 | unrun-verify | .github/workflows/full-pipeline.yml |  | SC #1-#5 (one sticky comment; three pushes -> one comment; no-baseline degrades to absolute+footer; sub-gate coverage still posts + stays mergeable; merge publishes baseline, no PR recompute) require a live scratch-branch PR against a throwaway target branch -- not automatable, not runnable from this box. actionlint + all per-task static gates pass; the cache/artifact/comment runtime behavior is unverified until the walkthrough. | open |  | 2026-09-02T22:44:55.675Z |  |

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
  },
  {
    "id": 4,
    "kind": "deviation",
    "phase": "14",
    "file": ".env.example",
    "line": null,
    "description": "INSTANCE_PASSPHRASE / TRUST_PROXY_HEADERS added to .env.example by operator during 14-01 (file is denied to agent tools; plan 14-04 formalizes wording)",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-08-29T16:16:47.738Z",
    "resolved_at": "2026-08-29T17:20:29.066Z"
  },
  {
    "id": 5,
    "kind": "lint-warning",
    "phase": "14",
    "file": "web/app/root.tsx",
    "line": 19,
    "description": "tsc --noEmit fails on stale react-router typegen artifact (./+types/root TS2307); pre-existing, react-router build passes; needs a typegen pretypecheck step",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-01T03:23:20.970Z",
    "resolved_at": null
  },
  {
    "id": 6,
    "kind": "deviation",
    "phase": "15",
    "file": "cmd/coverage-report/testdata/baseline-metrics-backend.json",
    "line": null,
    "description": "Task 1 baseline sidecar fixtures carried 41-char sha fields; corrected to valid 40-char in 20e6d68 (resolved)",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-09-02T22:19:13.272Z",
    "resolved_at": "2026-09-02T22:19:34.900Z"
  },
  {
    "id": 7,
    "kind": "deviation",
    "phase": "15",
    "file": "cmd/coverage-report/main.go",
    "line": null,
    "description": "backendTotalPct summed numStmts per profile line, inflating the denominator ~10x on a real merged go-test profile (reported 7.97% vs go tool cover 90.0%); fixed in b1967f6 by merging blocks by position (resolved)",
    "status": "fixed",
    "reason": "",
    "recorded_at": "2026-09-02T22:30:17.794Z",
    "resolved_at": "2026-09-02T22:30:26.122Z"
  },
  {
    "id": 8,
    "kind": "unrun-verify",
    "phase": "15",
    "file": ".github/workflows/full-pipeline.yml",
    "line": null,
    "description": "SC #1-#5 (one sticky comment; three pushes -> one comment; no-baseline degrades to absolute+footer; sub-gate coverage still posts + stays mergeable; merge publishes baseline, no PR recompute) require a live scratch-branch PR against a throwaway target branch -- not automatable, not runnable from this box. actionlint + all per-task static gates pass; the cache/artifact/comment runtime behavior is unverified until the walkthrough.",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-02T22:44:55.675Z",
    "resolved_at": null
  }
]
````
