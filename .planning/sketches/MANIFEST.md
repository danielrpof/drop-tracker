# Sketch Manifest

## Design Direction
The shipped drop-tracker look: dark only, quiet and dense. Zinc surfaces in three layers (950 page, 900 cards, 800 secondary). Indigo-500 is reserved for the primary CTA, the active state, and focus rings. Inter throughout, with 4 sizes and 2 weights (06-UI-SPEC). Phase 27 adds a paste-a-list dialog on the Watchlist page. Its review screen has to sort up to 100 rows into decisions without becoming a wall, and work at 375px. UI choices follow ui-ux-pro-max: color is never the only signal, there are no wide tables on phones, one atomic status region carries live updates, and touch targets are at least 44px.

## Reference Points
- The existing Watchlist page and Phase 26 bulk action bar (`web/app/components/watchlist/BulkActionBar.tsx`)
- `.planning/phases/27-paste-a-list-bulk-add/27-CONTEXT.md` D-10..D-20 (locked behavior)

## Sketches

| # | Name | Design Question | Winner | Tags |
|---|------|----------------|--------|------|
| 001 | review-screen | How do status groups, row cards, and the confirm footer hold up inside a dialog at 375px with 40 rows? | C: Segmented tabs | phase-27, review, dialog, mobile |
| 002 | dialog-flow | How do paste, resolving, review, and result feel as one flow, and where does progress live? | B: Progress screen, then review | phase-27, flow, progress |
