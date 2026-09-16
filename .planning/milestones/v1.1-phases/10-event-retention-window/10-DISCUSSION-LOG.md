# Phase 10: Event Retention Window - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-13
**Phase:** 10-Event Retention Window
**Areas discussed:** Retention basis, Env var shape & edge cases, Empty-state copy, Near-boundary behavior

---

## Retention basis

| Option | Description | Selected |
|--------|-------------|----------|
| created_at | Detection time — reliable timestamptz, already the effective sort/pagination key; a freshly-seeded artist's back-catalog stays visible together for a full window from add-time | ✓ |
| release_date | Actual release date — feels more "correct" but is a nullable free-text string (sometimes just a year, sometimes missing), unreliable for date arithmetic; would make a freshly-seeded artist's old catalog vanish immediately | |

**User's choice:** created_at (recommended option)
**Notes:** No further questions — settled in one round.

---

## Env var shape & edge cases

| Option | Description | Selected |
|--------|-------------|----------|
| Plain integer days | `EVENT_RETENTION_DAYS=90` — matches DATA-01's literal "90 days" wording, most operator-friendly unit | ✓ |
| Go time.Duration string | `EVENT_RETENTION_WINDOW=2160h` — consistent with `PollInterval`'s existing convention, but a much less obvious way to write "90 days" | |

**User's choice:** Plain integer days (recommended option)

| Option | Description | Selected |
|--------|-------------|----------|
| Fail fast at startup | 0 or negative treated as invalid config, refuses to boot — same posture as `DatabaseURL`'s notEmpty | ✓ |
| 0 means unlimited | Explicit opt-out of retention filtering, but overloads one value with two meanings and negatives still need separate handling | |

**User's choice:** Fail fast at startup (recommended option)
**Notes:** No further questions — both sub-decisions settled in one round each.

---

## Empty-state copy

| Option | Description | Selected |
|--------|-------------|----------|
| New third message | Neither existing message ("No release activity yet" / "No matching events") is accurate for retention-caused emptiness | ✓ |
| Reuse "No matching events" | Treats retention as just another filter axis, but implies the user chose a filter, which isn't true | |
| Reuse "No release activity yet" | Simplest, but actively misleading — there IS release activity, just hidden | |

**User's choice:** New third message (recommended option)

| Option | Description | Selected |
|--------|-------------|----------|
| API signal | Add a `hasOlderEvents`-style indicator to the `/events` response so the frontend can pick the right message precisely | ✓ |
| Heuristic: watchlist non-empty | No backend change, but imprecise — misfires for a brand-new artist with zero detections yet | |

**User's choice:** API signal (recommended option)
**Notes:** This decision surfaces new API surface beyond the existing `Page{Events, NextCursor}` shape — exact field name/shape left to Claude's discretion in CONTEXT.md.

---

## Near-boundary behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Inclusive: created_at >= cutoff shows | "Older than the window" means strictly older; an event exactly at the edge stays visible | ✓ |
| Exclusive: created_at > cutoff shows | Stricter reading, but the boundary is practically unobservable to a real user either way | |

**User's choice:** Inclusive (recommended option)
**Notes:** No further questions — settled in one round. User confirmed ready to write context after this area.

---

## Claude's Discretion

- Exact SQL mechanism for the `created_at` cutoff filter in `ListEvents` (Go-computed cutoff parameter vs. in-SQL interval expression)
- Exact field name/shape of the "are there older events" API signal (D-06)
- Exact `EVENT_RETENTION_DAYS` validation error message/mechanism
- Whether the cutoff is computed once per request or via DB-side `now() - interval`, and any `events.created_at` index needed for performance
- Exact empty-state copy wording for the new third message

## Deferred Ideas

None — discussion stayed within phase scope. No scope-creep suggestions came up during discussion.
