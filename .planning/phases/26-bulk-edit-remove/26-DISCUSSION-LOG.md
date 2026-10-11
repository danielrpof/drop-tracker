# Phase 26: Bulk Edit & Remove - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-11
**Phase:** 26-bulk-edit-remove
**Areas discussed:** Partial outcomes, Bulk preference semantics, Selection UX, Bulk remove & undo

---

## Partial outcomes

| Question | Options | Selected |
|----------|---------|----------|
| Bulk tag add with some artists at the 10-tag cap | Apply what fits, report capped by name / All-or-nothing with blocker list | Apply what fits |
| Concurrently removed id in tag/pref actions | Skip as "gone" / Fail the batch | Skip as "gone" |
| Already-gone id in bulk remove | Treat as already removed / Fail the whole batch | Treat as already removed |
| Response shape for reversible endpoints | Per-id outcomes / Counts only + refetch / You decide | Per-id outcomes |

## Bulk preference semantics

| Question | Options | Selected |
|----------|---------|----------|
| Effect on existing settings | Per-type delta / Overwrite whole set | Per-type delta |
| Mixed-state display | Tri-state checkbox / Set-Clear-Leave per type | Tri-state checkbox |
| When applied | Popover + Apply / Immediately per click | Popover + Apply |
| Empty release-type set | Same rule as single PATCH / You decide | Same rule as PATCH |

**Notes:** Checked during discussion: PATCH validates only membership, so an empty release-type set is allowed.

## Selection UX

| Question | Options | Selected |
|----------|---------|----------|
| Entry | Always-visible checkbox / "Select" mode toggle | Always-visible checkbox |
| Action bar | Sticky bottom bar / Replaces list toolbar | Sticky bottom bar |
| Hidden selections on filter change | Prune hidden / Keep with "(K hidden)" | Prune hidden |
| Bulk tag picker | Reuse TagCombobox with create / Existing tags only | Reuse TagCombobox with create |
| Bulk-edited cards that stop matching filters | Stay sticky (25 G4) / Vanish immediately | Stay sticky |

## Bulk remove & undo

| Question | Options | Selected |
|----------|---------|----------|
| Undo after bulk remove | Confirm only / Undo toast too | Confirm only |
| Confirm dialog content | Count + first few names / Count only / Full list | Count + first few names |
| Seed-mode gap copy | One plain sentence / You decide | One plain sentence |

## Claude's Discretion

- Endpoint paths and verbs, the request id-count and body bounds, and whether remove-tag targets by id or by name.
- Toast and confirm copy, the action-bar phone layout, and keyboard and shift-click selection (UI phase).

## Deferred Ideas

- Undo for bulk remove, to revisit after Phase 27's bulk-add endpoint exists.
