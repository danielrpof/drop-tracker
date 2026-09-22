# Feature Research

**Domain:** Watchlist organization for a self-hosted single-operator release tracker (v1.6) — tags, notes, search/sort/filter, bulk edit, bulk import review
**Researched:** 2026-09-22
**Confidence:** MEDIUM (websearch-sourced, cross-checked across 2+ independent sources per claim per `classify-confidence --verified`; no official docs/Context7 coverage exists for this UX-pattern question — it's a design-convention survey, not an API-fact lookup)

## Feature Landscape

### Table Stakes (Users Expect These)

Features an operator managing a 50+ artist watchlist assumes exist once tags/notes/bulk-add ship at all. Missing these makes the new surface feel half-built rather than absent.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Case/whitespace-insensitive tag matching | Every tag system researched (Gmail, GitHub labels) treats `"Hip-Hop"`, `" hip-hop "`, and `"hip-hop"` as the same tag at the storage/comparison layer, even where display casing varies. Users expect autocomplete to catch near-duplicates, not create four variants of one tag. | LOW | Normalize to a canonical key (trim + collapse internal whitespace + lowercase) for storage/uniqueness/comparison; keep first-used casing (or lowercase-only) for display. GitHub's own label-matching bugs (case-sensitive raw comparison) are a documented anti-pattern to avoid. |
| Tag rename affects all tagged artists atomically | Gmail's label rename is a single global edit reflected everywhere the label appears — users expect the same: renaming a tag once updates every artist carrying it, not a per-artist re-tag. | LOW-MEDIUM | Natural fit for a normalized many-to-many `tags` + `artist_tags` join table; rename is an UPDATE on the `tags` row, not N row edits. |
| Tag delete removes the tag from all artists, artists survive | Deleting a label/tag in every system researched (Gmail, GitHub) removes the *label*, never the underlying item. | LOW | Cascading delete on the join rows only; watchlist entries are untouched. |
| Empty states for every new surface | Every list/filter UI researched (bookmark managers, label systems) has a defined "nothing here yet" state distinct from "no results for this filter" — conflating the two reads as a bug. | LOW | Needs at least: no tags exist yet (Watchlist/autocomplete), an artist has no tags, an artist has no note, a tag/mute/release-type filter matches zero artists, History has no events for the selected tag. |
| Deterministic sort order (tie-breaking) | Users researched via SQL `ORDER BY` convention docs expect that re-sorting or re-filtering an unchanged list never silently reshuffles rows with equal primary-sort values — that reads as broken pagination/rendering, not "expected ties." | LOW | Every sort mode needs a stable secondary key (see UX Pattern Findings below). |
| Bulk destructive action requires an explicit, count-stated confirmation | Universal across the bulk-action UX guides researched (SaaS destructive-action patterns, eBay/HashiCorp/Basis design-system bulk-edit patterns): a bulk remove must show the affected count in the confirmation itself ("Remove 12 artists from watchlist?"), not a generic "Are you sure?". | LOW | Matches PROJECT.md's already-stated "confirmed bulk remove." |
| Paste-list bulk add never silently drops or silently adds anything | *arr-family tools (Lidarr/Sonarr, the closest domain analog — self-hosted media watchlist bulk-add) and Letterboxd's importer both stage matches for review before committing; nothing is added to the watchlist until the user confirms the reviewed batch. Already locked in PROJECT.md ("nothing added until confirmed") — research confirms this is the domain-standard behavior, not a gold-plated addition. | MEDIUM | The review screen itself (best match + alternates + skip/exclude per row) is the feature, not a side detail. |

### Differentiators (Competitive Advantage)

Not required by domain convention, but meaningfully raise usability for a single-operator instance managing a non-trivial watchlist. These are where v1.6 earns the "manageable at 50+ artists" goal PROJECT.md states.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Tag-based filtering reused identically across Watchlist and History | Most competitor tag systems (Gmail, GitHub) let you filter by label in one place (inbox, issue list) but rarely carry the same tag taxonomy into a second, differently-shaped view. Drop-tracker's History-filterable-by-tag plus Watchlist-filterable-by-tag sharing one tag vocabulary is a genuine differentiator for a personal tool. | LOW (once tags backend exists) | Reuses the accessible combobox already built in Phase 11.1 (`history.tsx`) rather than a new control. |
| Match-confidence-driven review screen, not a flat "did we find it" list | The Lidarr/Sonarr pattern (MusicBrainz-ID-backed exact match ranked above fuzzy text match) is more rigorous than most bulk importers researched (bookmark dedup tools, for instance, still show ~17-34% silent duplication rates from naive raw-string matching). Applying tiered confidence (exact ID > exact name > fuzzy > not found) to the paste-a-list review screen directly avoids that failure class. | MEDIUM | internal/musicbrainz and internal/deezer search clients already exist (Phase 03) — this is UI/aggregation work on top of an existing capability, not a new external integration. |
| Tags on Discord notification surfaces | No competitor researched (music trackers, RSS readers) surfaces user-defined tags inside the *notification* itself — tags are typically a browse/filter-only construct. Putting them on real-time embeds and digest lines is a drop-tracker-specific differentiator that closes the loop between organization and the app's actual output channel. | LOW-MEDIUM | Read-only rendering concern — digest grouping explicitly stays by event type per PROJECT.md, so this is additive text, not a new grouping dimension. |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|------------------|-------------|
| Hierarchical tags / nested folders (à la old-school bookmark folder trees) | Feels like better organization at first glance; Spotify users have long requested folders for the same reason. | Every flat-tag system researched (Gmail, GitHub) deliberately avoids hierarchy — nesting adds a second data model (parent/child), ambiguous multi-parent cases, and a rename/move UX cost, for a personal 50-100-artist list where flat tags plus filter already solve findability. Spotify's own unresolved multi-year folder request thread is cited by researchers as evidence hierarchy is expensive to do well, not that it's missing by oversight. | Flat free-form multi-tags (already the v1.6 design) — multiple tags per artist already gives most of the practical benefit of folders (an artist can be in "reggaeton" and "new-2026" simultaneously, which a strict folder tree can't do without duplication). |
| Fuzzy/semantic duplicate detection on bulk-add ("AI similarity matching") | Bookmark-manager research shows this is a real pain point (17-34% silent duplicate rates) and vendors increasingly market simhash/AI dedup as the fix. | Overkill for this domain: artists are already keyed by stable external IDs (MusicBrainz MBID / Deezer artist ID) once matched, so "is this a duplicate" is an exact-ID comparison against the existing watchlist, not a fuzzy content-similarity problem. Building semantic dedup here solves a harder version of a problem the ID match already solves exactly. | Exact-ID collision check against the current watchlist at review-screen time (flag "already watched", don't re-add) — see UX Pattern Findings below. |
| Silent auto-merge on tag rename collision | Feels convenient — "why ask, just merge them." | A silent merge is a data-loss-adjacent action (two previously-distinct groupings collapse into one with no way to tell which artists came from which tag afterward) — every bulk-destructive-action guide researched insists on an explicit confirmation whenever an action is irreversible-in-effect, even if not "destructive" in the delete sense. | Detect the collision, show a confirmation naming both tags and the resulting merged artist count, require explicit confirm (see UX Pattern Findings below) — cheap to implement, avoids a surprise. |
| Undo for bulk *remove from watchlist* | Undo is the modern default for bulk actions per the SaaS/eBay/HashiCorp bulk-action guides researched, and it's tempting to apply it uniformly. | Removing a watchlist entry likely cascades or orphans per-artist history/detection state (dedup keys, deluxe-change baselines) that a UI-layer "undo" can't cheaply restore without re-adding and re-seeding — the same class of problem the app's `EVENT_RETENTION_DAYS` soft-delete design already had to solve deliberately (Phase 10). A UI toast-undo would either lie about full restoration or require its own state-preservation design. | Confirm-with-count modal instead (already the v1.6 design decision) — matches the "explicit confirm for high-consequence irreversible actions" branch of the researched guidance, not the "offer undo" branch reserved for cheap-to-reverse actions. |

## UX Pattern Findings (direct answers to the research question)

**Tag normalization (case, whitespace, max length/count):**
Storage/comparison layer should be case-insensitive and whitespace-normalized (trim + collapse internal runs to a single space) — this is the convention in both systems researched (Gmail merge-by-name, GitHub label-matching bug reports that exist precisely because raw casing wasn't normalized). Cap per-tag length modestly (researched implementations range ~24-100 chars; something in the 30-40 char range is generous for a short label like "reggaeton" or "deluxe-watch" without inviting note-length text). Cap tag count per artist at a small number (researched examples split between ~5 and ~50 depending on whether tags are a primary or secondary UI element — for a *secondary* organizational aid on an artist card, a low cap, e.g. 10-15, keeps the card readable; there is no single universal number, so treat this as a product choice, not a researched constant).

**Rename-merge collisions:**
When renaming tag A to a name that normalizes to an existing tag B, the domain-standard behavior (Gmail explicitly supports "merge to an existing label" as the resolution path, rather than erroring or silently creating a duplicate) is to detect the collision pre-commit and offer an explicit merge: show both tags' names and the resulting merged artist count, require confirmation, then union the artist memberships into one tag row and delete the renamed-from tag. Do not silently auto-merge (see Anti-Features) and do not block the rename with a bare "name taken" error — that's worse UX than the tool being renamed from already offers.

**Empty states:**
Treat "no tags exist in the system yet" (autocomplete has nothing to suggest, first-run state) as distinct from "this artist has no tags" (normal steady-state) and distinct from "this filter/search matched zero artists" (needs a "clear filter" affordance, not just blank space) — all three are different empty states in every list/filter UI researched and conflating them (e.g., showing the same blank card for "no tags anywhere" and "filtered to zero") reads as a bug to users. Same three-way split applies to History filtered by tag.

**Bulk-import line parsing (commas vs newlines, duplicates, already-watched, "Artist - extra" noise):**
- **Delimiter:** newline-per-entry is the domain-standard shape for a "paste a list" box (this matches how *arr-family import-list tools and CSV/bookmark bulk importers researched all expect one item per line); treat commas as noise/part of the name rather than a second delimiter, since artist names themselves can legitimately contain commas in edge cases (featuring credits, "The Weeknd, Pt. 2"-style entries) — splitting on commas risks mid-name breaks that splitting on newlines doesn't.
- **Duplicates within the pasted batch:** dedupe case-insensitively before searching (searching the same string twice wastes the per-name rate-limited lookup budget for no benefit) and show the deduped count to the user.
- **Already-watched artists:** flag rather than silently drop — bookmark-import research is explicit that *silent* dedup/skip is what causes trust problems (users can't tell if their paste "worked"); the review screen should mark a matched artist as "already on your watchlist" and let the user see and explicitly exclude it, matching the "nothing added until confirmed" design already locked for this milestone. Matching should be by the stable external ID once the search resolves a candidate, not by fuzzy name string, since ID-based comparison is exact where the domain already has one (see Anti-Features: no need for fuzzy/semantic dedup here).
- **"Artist - extra" noise:** don't auto-strip suffixes/noise algorithmically before searching (risks stripping something that was actually part of the name); instead surface each raw pasted line, run it through the same per-name search the existing search-proxy already does, and let the confidence-ranked match results absorb the noise naturally (a search for "Bad Bunny - Topic" will usually still surface Bad Bunny as the top/best match via the underlying MusicBrainz/Deezer search relevance ranking) — treat any line that fails to produce a confident match as a "not found, edit or skip" row rather than trying to out-guess arbitrary paste noise with regex heuristics.

**Review-screen confidence signals:**
Tiered confidence, ranked (based on the Lidarr/Sonarr and Letterboxd import patterns researched, both of which rank exact-ID matches above text-based best-guess matches):
1. **High** — exact/near-exact name match against the search API's top result (already the existing search-proxy's own ranking signal from Phase 03/12's popularity-ranking work) → pre-select as the default, still visible and changeable.
2. **Medium** — a plausible but not exact match (partial string match, or multiple close candidates) → show the top candidate plus a visible list of alternates, require the user to actively confirm rather than defaulting to auto-accept.
3. **Not found** — search returned nothing usable → flag the row distinctly (not blank/absent), offer manual re-search or skip.
Never auto-commit a Medium or Not-found row; only High-confidence rows are reasonable to pre-check for a "confirm all" convenience action, and even then the user must hit one final confirm for the whole batch (matches "nothing added until confirmed").

**Bulk action undo vs confirm:**
Split by reversibility/consequence, per the SaaS/eBay/HashiCorp bulk-action guidance researched: reversible, low-consequence bulk edits (add/remove tags, set release-type/mute preferences) can apply immediately with a lightweight success indication (toast), since re-editing is cheap; the bulk *remove from watchlist* action — which interacts with existing detection state the app already treats carefully (dedup keys, deluxe baselines, retention filtering) — should use an explicit, count-stated confirmation modal rather than an apply-then-offer-undo pattern, because a UI-level "undo" can't cheaply/correctly restore whatever cascade a real remove triggers. This mirrors the anti-feature note above.

**Sort ties:**
Every sort mode needs a defined, stable secondary key so re-renders/re-filters never visibly reorder equal-valued rows: name sort → name then artist ID; date-added sort → date-added timestamp then artist ID; latest-release sort → release date then artist ID. (No single "correct" secondary key exists in the literature — this is the SQL/UX convention of "always fully order, never leave ties to incidental row order," confirmed by the ORDER BY research above, applied here as a concrete recommendation.)

**Latest-release sort with no events:**
Postgres natively supports `NULLS LAST`/`NULLS FIRST` and the researched UX convention (missing/unknown values trail known values, e.g., "products with prices before unknown-price products") favors always placing artists with zero detected events at the end of the list regardless of ascending/descending direction — i.e., don't let `NULLS FIRST` surface never-released artists at the top on a descending sort, which would look like they have the *newest* release. Recommend an explicit `NULLS LAST` in both directions for this sort mode rather than relying on Postgres's per-direction default (`NULLS LAST` on ASC, `NULLS FIRST` on DESC), since the per-direction default is exactly the case that would surprise a user on descending sort.

## Feature Dependencies

```
Tags backend (tags table + artist_tags join, normalized-key uniqueness)
    ├──requires──> none new (additive schema only)
    ├──enables──> Tag autocomplete on Watchlist add/edit
    ├──enables──> Global tag rename/delete
    ├──enables──> Watchlist filter by tag
    ├──enables──> History filter by tag  ──reuses──> existing accessible combobox (Phase 11.1, history.tsx)
    ├──enables──> Tags on Discord real-time embeds
    └──enables──> Tags on digest lines  ──constrained by──> digest grouping stays by event type (PROJECT.md)

Notes field (per-artist plain text, length-capped)
    └──requires──> none new (single column on existing watchlist row)

Watchlist search/sort/filter
    ├──requires──> Tags backend (for the tag filter facet)
    └──requires──> existing release-type filter / muted-event-type columns (already in internal/watchlist, Phase 02)

Multi-select bulk edit (tags, prefs, remove)
    ├──requires──> Tags backend (for bulk tag add/remove)
    ├──requires──> existing per-artist preference mutation endpoints (Phase 02) extended to batch
    └──requires──> new multi-select UI affordance on Watchlist (not present today)

Paste-a-list bulk add + review screen
    ├──requires──> existing search-proxy (internal/musicbrainz, internal/deezer clients — Phase 03)
    ├──requires──> existing per-request rate limiters (Phase 03/11) — "no new background API polling" constraint means each pasted name's lookup rides the same interactive-search rate budget, not a new poller
    ├──requires──> exact-ID collision check against current watchlist (already-watched detection)
    └──enables──> nothing added to internal/watchlist until user confirms the reviewed batch
```

### Dependency Notes

- **Tags backend is the single shared dependency** for five of the seven v1.6 target features (autocomplete, rename/delete, Watchlist filter, History filter, Discord display). It should land first/early in the phase sequence — every other tag-touching feature is blocked on it.
- **History filter by tag reuses, not replaces**, the accessible combobox already built and validated in Phase 11.1 (Windows Chromium legibility fix) — no new filter-control component should be built from scratch.
- **Paste-a-list bulk add has no new external-API surface** — it is UI/orchestration on top of the existing Phase 03 search-proxy and Phase 03/11 rate limiters, which is what keeps it compatible with the "no new background API polling" v1.6 constraint. The per-name search must stay synchronous/foreground (user is watching the review screen build), not a background poller job.
- **Multi-select bulk edit conflicts with nothing** existing but is the one genuinely new UI mechanism (row selection state, action bar) — every other v1.6 feature extends an existing surface (Watchlist card, History filter, Discord embed) rather than introducing a new interaction pattern.
- **Tags on Discord surfaces depend on the tags backend existing with real data** — sequencing this after tags are usable in the UI (so there's something meaningful to test against) rather than in parallel is lower-risk.

## MVP Definition

### Launch With (v1.6)

All seven target features are already locked as in-scope in PROJECT.md; nothing here is negotiable MVP scoping in the traditional sense. Sequencing priority based on the dependency graph above:

- [ ] Tags backend (schema + normalize/dedupe/rename-merge logic) — blocks 5 of 7 downstream features
- [ ] Tag autocomplete + global rename/delete UI
- [ ] Per-artist notes (length-capped, Watchlist-only) — no dependency, can land independently/in parallel
- [ ] Watchlist search/sort/filter (name search, sort ties, tag/mute/release-type filter)
- [ ] History filter by tag (reuse existing combobox)
- [ ] Tags on Discord real-time embeds and digest lines
- [ ] Multi-select bulk edit (tags, prefs, confirmed remove)
- [ ] Paste-a-list bulk add + review screen (confidence-tiered matches, already-watched flagging, nothing committed until confirmed)

### Add After Validation (v1.x)

Not part of this research's scope to invent — PROJECT.md's Out of Scope section already covers longer-horizon items (producer tracking, upcoming-release calendar per the `2026-09-08` note's Option D). Nothing surfaced in this research suggests deferring any of the seven locked features further; the open question is sequencing, not scope.

### Future Consideration (v2+)

- Hierarchical/nested tag groups — explicitly an anti-feature per research above; flat tags already cover the practical need.
- Semantic/fuzzy dedup on bulk-add — explicitly an anti-feature; exact-ID collision check is sufficient given the domain already has stable external IDs.
- Cross-device undo history for bulk actions — not requested in PROJECT.md and the researched guidance places watchlist-remove in the "confirm, don't undo" bucket anyway.

## Competitor Feature Analysis

| Feature | Gmail Labels | Lidarr/Sonarr (closest domain analog) | Our Approach |
|---------|--------------|-----------------------------------------|--------------|
| Tag/label rename | In-place rename, merges if target name exists | N/A (no tagging) | Same merge-on-collision behavior, with explicit confirm rather than silent merge |
| Bulk import matching | N/A | MusicBrainz-ID match preferred over text match; falls back to artist/album text matching | Same tiered approach: ID/exact match (High) → fuzzy text match (Medium, shows alternates) → not found |
| Filter by tag across multiple views | Labels filter Inbox only (single view) | N/A | Differentiator: same tag vocabulary filters both Watchlist and History |
| Duplicate detection on import | N/A | Reads existing library, matches by ID where present | Exact-ID collision check against current watchlist, not fuzzy similarity |

## Sources

- [Required-label matching is case-sensitive against raw GitHub label names — GitHub Issue #70](https://github.com/StGerman/crewd/issues/70) — MEDIUM
- [How to Keep Your Inbox (Super) Tidy With Gmail Labels — Drag Blog](https://www.dragapp.com/blog/gmail-labels-everything/) — MEDIUM
- [Users Gmail Labels — GAM-team/GAM Wiki](https://github.com/GAM-team/GAM/wiki/Users-Gmail-Labels) — MEDIUM
- [How to clean up your bookmarks: duplicates, dead links — Stashr](https://stashr.me/blog/clean-up-bookmarks) — MEDIUM (duplicate-rate figures corroborated across multiple bookmark-tool sources in the same search pass)
- [Import Bookmarks — Bookmarkjar Documentation](https://docs.bookmarkjar.com/bookmarks/import) — MEDIUM
- [Bookmark managers with duplicate resolver — Bookmark OS](https://bookmarkos.com/bookmark-manager-finder/Duplicate%20resolver) — MEDIUM
- [muspy.com](https://muspy.com/) and [Muspy documentation — Read the Docs](https://muspy.readthedocs.io/en/latest/doc/muspy.html) — MEDIUM (confirms Muspy has no tag/watchlist-organization layer; drop-tracker's v1.6 scope has no direct precedent in this specific competitor)
- [SaaS Destructive Actions & Confirmation UX Patterns (2026)](https://www.saasui.design/blog/saas-destructive-actions-confirmation-ux-patterns) — MEDIUM
- [Bulk action UX: 8 design guidelines with examples for SaaS — Eleken](https://www.eleken.co/blog-posts/bulk-actions-ux) — MEDIUM
- [Bulk Edit: Design | Patterns — eBay Playbook](https://playbook.ebay.com/design-system/patterns/bulk-edit) — MEDIUM
- [Table multi-select — Helios Design System (HashiCorp)](https://helios.hashicorp.design/patterns/table-multi-select) — MEDIUM
- [Bulk Editing — Basis Design System](https://design.basis.com/patterns/bulk-editing) — MEDIUM
- [A UX guide to destructive actions — Medium/Bootcamp](https://medium.com/design-bootcamp/a-ux-guide-to-destructive-actions-their-use-cases-and-best-practices-f1d8a9478d03) — MEDIUM
- [Ark UI — Tags Input component docs](https://ark-ui.com/docs/components/tags-input) — MEDIUM
- [Chakra UI — Tags Input component docs](https://chakra-ui.com/docs/components/tags-input) — MEDIUM
- [How ORDER BY and NULL Work Together in SQL — LearnSQL.com](https://learnsql.com/blog/how-to-order-rows-with-nulls/) — MEDIUM
- [How to Sort SQL Results With NULL Values at the End — Baeldung](https://www.baeldung.com/sql/sort-ascending-null-values-last) — MEDIUM
- [Placement of NULL values for ORDER BY with nullable columns — sqlfordevs.com](https://sqlfordevs.com/order-by-with-null) — MEDIUM
- [Importing data — Letterboxd](https://letterboxd.com/about/importing-data/) — MEDIUM
- [Lidarr Importing an Existing Library — Servarr Wiki](https://wiki.servarr.com/lidarr/importing-existing-library) — MEDIUM
- Internal: `.planning/PROJECT.md` (v1.6 target features, existing Validated requirements for release-type filters/muted event types/search-proxy) — HIGH (primary source)
- Internal: `.planning/notes/2026-09-08-feature-module-ideas-post-v1.3.md` (Option E framing, "no new background API polling" constraint) — HIGH (primary source)
- Internal: `internal/watchlist/service.go`, `web/app/routes/history.tsx` (confirmed existing filter/preference plumbing to build on) — HIGH (codebase read)

---
*Feature research for: drop-tracker v1.6 Watchlist Organization*
*Researched: 2026-09-22*
