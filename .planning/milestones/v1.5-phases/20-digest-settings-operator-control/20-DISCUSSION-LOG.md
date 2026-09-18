# Phase 20: Digest Settings & Operator Control - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-11
**Phase:** 20-digest-settings-operator-control
**Areas discussed:** UI placement, Interaction model, Feedback & errors, Cadence field visibility

---

## UI Placement

| Option | Description | Selected |
|--------|-------------|----------|
| Add to /system view | New section in the existing System panel, alongside per-source health/recent-runs/About | ✓ |
| New dedicated settings page | A separate route/nav tab just for digest config | |

**User's choice:** Add to /system view
**Notes:** None — recommended option accepted directly.

---

## Interaction Model

| Option | Description | Selected |
|--------|-------------|----------|
| Instant-apply | Toggling on/off or changing cadence fires the PUT immediately | ✓ |
| Explicit Save button | Both fields batch into one PUT on an explicit Save click | |

**User's choice:** Instant-apply
**Notes:** None — recommended option accepted directly.

---

## Feedback & Errors

| Option | Description | Selected |
|--------|-------------|----------|
| Inline status, keep-stale on failure | Inline confirmation on success; revert + inline error on failure | ✓ |
| Toast notification | A transient toast for success/failure | |

**User's choice:** Inline status, keep-stale on failure
**Notes:** None — recommended option accepted directly.

---

## Cadence Field Visibility

| Option | Description | Selected |
|--------|-------------|----------|
| Always visible, disabled when off | Cadence picker visible but greyed out while digest mode is off | ✓ |
| Hidden until digest is on | Cadence picker only appears after toggle is flipped on | |

**User's choice:** Always visible, disabled when off
**Notes:** None — recommended option accepted directly.

---

## Claude's Discretion

- Exact migration column types/constraints beyond what ROADMAP.md already names
- Component decomposition within the `/system` view
- Exact inline confirmation/error copy strings

---

## Post-Discussion Grilling Session (2026-09-11)

A `/mattpocock-skills:grill-with-docs` session challenged the full v1.5 plan (milestone + this phase) after the above discussion had already been captured. It surfaced ten risks/assumptions ranked by impact and put five genuinely open decisions to the user; all five were resolved and folded into `20-CONTEXT.md` and `ROADMAP.md` directly (with explicit user sign-off to bypass the normal `/gsd-discuss-phase` flow for this doc sync). This phase's only share of that outcome:

| Open item | Resolution | Where it landed |
|-----------|------------|-----------------|
| Singleton enforcement mechanism (previously Claude's Discretion, above) | `CHECK (id = 1)` + migration-time seed `INSERT`, not upsert-on-read | `20-CONTEXT.md` D-05; `ROADMAP.md` Phase 20 notes |

The other four resolved decisions (settings-read failure posture, restart catch-up UX, pulling DGST-10's grouping forward into Phase 22, and shipping Phase 21+22 in the same release) belong to Phases 21-23, which have no `discuss-phase` context of their own yet — they're recorded directly in `ROADMAP.md`'s per-phase planner notes for those phases to pick up when their own discuss-phase runs.

## Deferred Ideas

- Toast notification component — no toast primitive exists in the SPA today; inline status chosen instead (see Feedback & Errors above)
- A separate dedicated settings page — considered under UI Placement, rejected in favor of extending `/system`
- Digest scheduling, mutual exclusion with real-time, grouping/chunking — later phases (21-23)

### Reviewed Todos (not folded)
- Move `shadcn` out of `web/package.json` dependencies — unrelated, weak keyword match
- Resolve D-15 prev-release query files from `--prev-tag` — unrelated, backend tooling
- Unify `internal/sqlscan`'s two quote state machines — unrelated, backend tooling
