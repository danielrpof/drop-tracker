# Phase 19 — UI Review

**Audited:** 2026-09-11
**Baseline:** `.planning/phases/19-frontend-system-view/19-UI-SPEC.md` (approved design contract)
**Screenshots:** not captured — no dev server detected on localhost:3000 / 5173 / 8080; code-only audit against `web/app/routes/system.tsx`, `web/app/components/system/*`, `web/app/lib/format.ts`, `web/app/lib/sources.ts`, `web/app/app.css`, `web/app/root.tsx`, `web/app/routes.ts`, `web/app/components/ui/table.tsx`

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 4/4 | Every locked string (badges, empty states, About rows, failed-refresh line, caption text) matches the UI-SPEC verbatim, including pluralization and em-dash null handling. |
| 2. Visuals | 4/4 | Hierarchy matches contract exactly (About → callout → panels); Interrupted vs Completed-with-errors amber tiers correctly disambiguated with a leading icon; no icon-only controls without text. |
| 3. Color | 4/4 | Zero hardcoded hex/rgb in the phase's files; `text-primary` used only on the Watchlist link, matching the closed accent list; status palette used only in the 5 spec-declared locations. |
| 4. Typography | 3/4 | The vendored `Table` component silently introduces a third font weight (`font-medium`, 500) on `<th>` and raw `text-sm` classes on the table/caption, bypassing the "exactly 2 weights, named tokens only" contract. |
| 5. Spacing | 3/4 | Table cells render at `p-3`/`h-12` (12px/48px) from the unmodified vendored component, contradicting the UI-SPEC's own stated implementation ("`p-4` table cell rhythm", 16px) and the declared spacing scale, which has no 12px step. |
| 6. Experience Design | 4/4 | All 5 render states, every empty/partial/overflow permutation from the UI Considerations table, re-entrancy guard, and unmount-safety are implemented and unit-tested exactly as specified. |

**Overall: 22/24**

---

## Top 3 Priority Fixes

1. **Table header cells carry an undocumented 500 font-weight** — `web/app/components/ui/table.tsx`'s `TableHead` ships `font-medium` unmodified, and neither `SourceHistoryTable.tsx` nor any wrapper strips it. This silently violates the UI-SPEC's explicit "Weights available: 400 and 600 only… not a new weight" rule (that exception is scoped to Button/Badge only, not Table). User impact: none functionally, but it's a real design-system drift a future contributor will copy forward. Fix: add `className="font-normal"` (or equivalent) to each `TableHead` in `SourceHistoryTable.tsx`, or override in a project-level table style.

2. **Table cell padding doesn't match the spec's own stated value** — the UI-SPEC's Spacing Scale row literally says "`p-4` table cell rhythm" (16px), but the unmodified vendored `TableCell`/`TableHead` render at `p-3` (12px) and `h-12` (48px) — neither value appears in the declared spacing scale (xs4/sm8/md16/lg24/xl32/2xl48/3xl64), and 48px is explicitly marked "not used in this phase" elsewhere in the same document. Fix: override cell/head padding via `className` on `SourceHistoryTable.tsx`'s `TableHead`/`TableCell` usages to `p-4`, or amend the UI-SPEC's Spacing Scale note if the vendored default is intentionally being kept.

3. **`TableCaption` and `<table>` use raw Tailwind `text-sm` instead of the `text-label` design token** — visually near-identical (0.875rem) but the named-token system bundles line-height (1.5) and is the single source of truth every other component in this phase used correctly; `text-sm`'s default line-height (1.25) diverges silently. Fix: pass `className="text-label"` on `TableCaption` and drop the base `text-sm` from `Table`'s wrapper `className`, or update the component-level `cn()` default in `table.tsx` once for every future table caller.

---

## Detailed Findings

### Pillar 1: Copywriting (4/4)
- `system.tsx:227-235` — error EmptyState heading/body match spec exactly ("Couldn't load system status." / "The server didn't return a status. Try again in a moment.").
- `system.tsx:249-263` — empty-watchlist Alert title/description/link text match D-05/Copywriting Contract verbatim, including the em dash character.
- `system.tsx:266-273` — first-run heading/body and the `schema_applied === null` DB-unreachable variant both match the Copywriting Contract's exact wording.
- `AboutInstance.tsx:43-73` — Schema/Database/Watchlist/Poll-interval row values follow the D-01/D-02/D-06 rules precisely, including the `schema_applied === null` em-dash Schema row.
- `SourcePanel.tsx:64-113` — verbatim `summary` line, conditional counts line gated on `artists_errored > 0`, "No clean run in recent history" fallback, skip line, and escalation line copy all match the locked strings character-for-character.
- `SourceHistoryTable.tsx:29-34` — the three-condition caption (cap / count / absent-at-zero) matches the contract's exact wording and pluralization.
- No generic `Submit`/`OK`/`Click Here` labels found anywhere in the phase's files.

### Pillar 2: Visuals (4/4)
- Content order in `system.tsx` (About → empty-watchlist callout → panels) matches the "Visual Hierarchy" section exactly.
- `OutcomeBadge.tsx:38-46` — the two amber tiers ("Completed with errors" vs "Interrupted") are correctly disambiguated by the `Ban` icon on Interrupted only, per D-07's glanceability requirement.
- Refresh button and its Loading/Refreshing states swap icon+text together (`RefreshCw`/`Loader2`), never icon-only, with `aria-hidden` on decorative icons.
- No focal-point ambiguity: `<h1>System</h1>` at `text-display`, panel/About titles at `text-heading`, single clear reading order.

### Pillar 3: Color (4/4)
- `grep` for hex/rgb across all phase files returned zero hits.
- `text-primary` used exactly once, on the Watchlist link inside the empty-watchlist Alert — matching the Color table's closed accent list ("Only: the active tab underline and focus rings…").
- Status palette (`status-ok`/`status-warn`) used only in the 5 UI-SPEC-declared locations: outcome badges, DB-reachable pill, schema-drift text, skip line, escalation line. No stray use on buttons, nav, or borders.
- `app.css:47-48` confirms the two new tokens were added exactly as specified, with the required coexistence comment.

### Pillar 4: Typography (3/4)
- Route/panel/About titles correctly use `text-display`/`text-heading` named tokens (bare `<h2>` overriding `CardTitle`, per the UI-SPEC's own instruction).
- `format.ts`/panel/table body and label text consistently use `text-body`/`text-label` — no stray `text-xs`/`text-lg` etc. found in the phase's own components.
- **Defect:** `web/app/components/ui/table.tsx` (vendored this phase, hand-written per its own header comment) ships `font-medium` on `TableHead` (a third weight beyond the contracted 400/600) and `text-sm` on `Table`/`TableCaption` (a non-token size/line-height pair) — neither is overridden at any call site in `SourceHistoryTable.tsx`. The UI-SPEC's font-weight exception clause names only Button/Badge, not Table, so this is an unaccounted deviation, not a pre-existing carve-out.

### Pillar 5: Spacing (3/4)
- Page/card gutters (`p-8`, `gap-6`, `gap-2`, `gap-1`) all map cleanly onto the declared scale.
- **Defect:** the vendored `Table`'s default `TableHead`/`TableCell` padding (`p-3` = 12px, `h-12` = 48px) is left unmodified in `SourceHistoryTable.tsx`. 12px has no place in the documented scale (nearest named steps are sm=8px, md=16px) and the UI-SPEC's own Spacing Scale table asserts "`p-4` table cell rhythm" for this exact table — the implementation does not match the contract's own stated intention, and 48px is separately marked "not used in this phase" in the same table.

### Pillar 6: Experience Design (4/4)
- All 5 states (`loading`/`error`/`session-expired`/`first-run`/`loaded`) implemented exactly per D-04, verified by `system.test.tsx` (16 files / 205 tests, 90%+ coverage per 19-05-SUMMARY.md).
- `loading`: card-skeleton mirroring loaded layout, Refresh present-but-disabled — not a bare spinner.
- `error`: header Refresh fully hidden (`!loadError &&` gate at `system.tsx:182`), Retry is sole recovery affordance.
- Refresh re-entrancy uses a `useRef` guard (not state), verified against a real double-click race per 19-05-SUMMARY's documented bug fix — a materially more rigorous state-management choice than the naive state-boolean approach.
- `mountedRef` prevents state writes after unmount/401-during-refresh — closes the "stray request behind the login screen" ROADMAP criterion.
- `deriveLoadedShape` recomputed on every render (not latched in state), correctly handling the loaded→first-run buffer-reset transition.
- Every UI-SPEC "UI Considerations" row (empty/loading/populated/error/partial/overflow/long-text/zero-one-many) has a corresponding implementation and unit test per the 19-04/19-05 coverage tables.
- No destructive actions exist in this read-only view — correctly nothing to confirm.

---

## Registry Safety

`web/components.json` confirms `registries: {}` — no third-party registries declared, matching the UI-SPEC's Registry Safety table (shadcn official only). Note: `web/app/components/ui/table.tsx` was hand-written locally (per its own header comment) rather than pulled via the CLI, because the CLI mis-resolved the `cn` dependency as an npm package during the researched offline fallback — this is a supply-chain-safe outcome (no third-party code pulled in), but it is the direct cause of the Typography/Spacing defects above, since the hand-written file was never re-styled to match this project's token system after being copied from the registry's raw shape.

Registry audit: 0 third-party blocks checked (none declared), no flags.

---

## Files Audited

- `web/app/routes/system.tsx`
- `web/app/components/system/AboutInstance.tsx`
- `web/app/components/system/SourcePanel.tsx`
- `web/app/components/system/OutcomeBadge.tsx`
- `web/app/components/system/SourceHistoryTable.tsx`
- `web/app/components/common/EmptyState.tsx`
- `web/app/components/ui/table.tsx`
- `web/app/lib/format.ts`
- `web/app/lib/sources.ts`
- `web/app/app.css` (theme tokens)
- `web/app/root.tsx` (nav tab)
- `web/app/routes.ts` (route registration)
- `web/components.json` (registry safety)
