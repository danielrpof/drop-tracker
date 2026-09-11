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

- Singleton enforcement mechanism (`CHECK (id = 1)` vs. seed-in-migration vs. upsert-on-read)
- Exact migration column types/constraints beyond what ROADMAP.md already names
- Component decomposition within the `/system` view
- Exact inline confirmation/error copy strings

## Deferred Ideas

- Toast notification component — no toast primitive exists in the SPA today; inline status chosen instead (see Feedback & Errors above)
- A separate dedicated settings page — considered under UI Placement, rejected in favor of extending `/system`
- Digest scheduling, mutual exclusion with real-time, grouping/chunking — later phases (21-23)

### Reviewed Todos (not folded)
- Move `shadcn` out of `web/package.json` dependencies — unrelated, weak keyword match
- Resolve D-15 prev-release query files from `--prev-tag` — unrelated, backend tooling
- Unify `internal/sqlscan`'s two quote state machines — unrelated, backend tooling
