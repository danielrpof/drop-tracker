---
id: SEED-261118-1bb
status: dormant
planted: 2026-11-18
planted_during: v1.6 Watchlist Organization, Phase 27 (deferred)
trigger_when: when the user wants to import a playlist or an existing artist list into the watchlist
scope: medium
---

# SEED-261118-1bb: Paste-a-list bulk add (deferred Phase 27)

## Why This Matters

The user pastes artist names one per line. The app resolves each name through `/search` and sorts the rows into confidence tiers: High means exactly one MusicBrainz result has the same name and scores ≥ 90. The user reviews the rows and adds the confirmed set in one request. It turns building a watchlist from an existing list into minutes instead of one search per artist.

It was deferred on 2026-11-18 after a grilling, spike 001 and an exploration. drop-tracker is a personal tool run locally, the feature would be used rarely (only when bootstrapping), and its cost (a four-step dialog, tiers, retries, a backfill-runner refactor, a new confirm endpoint) didn't justify building it now. It was on the roadmap because it "seemed like a natural feature to have", not because of a felt need.

## When to Surface

**Trigger:** the user wants to import a playlist or an existing artist list into the watchlist.

## Scope Estimate

**Medium.** About one phase. The design is complete, so revival goes straight to `/gsd-plan-phase`.

**Prerequisite:** GitHub issue #17 Layer 1 (MusicBrainz 503 retry in `internal/httpclient.Do`) must be in place first. Without it, a home IP sees about 38% Error rows at 1 req/s.

## Breadcrumbs

- `.planning/phases/27-paste-a-list-bulk-add/27-CONTEXT.md`: decisions D-01..D-20 and grilling amendments G-27-1..10 (backfill runner, concurrency 2, one-transaction confirm, the S90 High rule, one automatic retry)
- `.planning/phases/27-paste-a-list-bulk-add/27-UI-SPEC.md`: the approved UI contract, updated for G-27
- `.planning/sketches/001-review-screen/`, `.planning/sketches/002-dialog-flow/`: the chosen variants (segmented tabs; a progress screen, then review)
- `.planning/spikes/001-paste-confidence-tiers/`: tier measurements on 50 real names (`merged.md`) and the 503 finding
- `CONTEXT.md` glossary: **Same name**, **Bulk outcome** (applied / skipped / failed)
- `.planning/REQUIREMENTS.md`: IMPT-01..05
- GitHub issue #17

## Notes

On revival, re-check that `/search` still returns `limit=10` and whether #17 changed the MusicBrainz rate (it affects the resolving-time copy, G-27-2).
