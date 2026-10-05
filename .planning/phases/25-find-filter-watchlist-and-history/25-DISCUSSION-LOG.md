# Phase 25: Find & Filter — Watchlist and History - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-10-04
**Phase:** 25-find-filter-watchlist-and-history
**Areas discussed:** Toolbar & two search boxes, Tag filter semantics, State persistence, Latest-release semantics

---

## Toolbar & two search boxes

| Option | Description | Selected |
|--------|-------------|----------|
| Separate list toolbar | Add-artist search stays on top; filter/sort/count toolbar above the list | ✓ |
| Single shared input | One box filters and offers "Search MusicBrainz for X" | |
| Collapse add-search | Add-artist search behind a button/dialog | |

| Option (default sort) | Description | Selected |
|--------|-------------|----------|
| Name A–Z | Matches today's server order | ✓ |
| Latest release, newest | Active artists first | |
| Date added, newest | Recent additions first | |

| Option (card date) | Description | Selected |
|--------|-------------|----------|
| Only when sorted by it | Shown in latest-release sort only | |
| Always show | Muted line on every card; reverses Phase 06 D-03/D-04 | ✓ |
| Never show | Sort-only signal | |

| Option (search match) | Description | Selected |
|--------|-------------|----------|
| Name substring, accent-insensitive | NFD + strip diacritics | ✓ |
| Name + disambiguation | Also matches disambiguation | |
| Plain case-insensitive substring | No accent folding | |

## Tag filter semantics

| Option (multi-tag) | Description | Selected |
|--------|-------------|----------|
| Any-of (OR) | Either tag | ✓ |
| All-of (AND) | Every tag | |
| Any/All toggle | User switches | |

| Option (Watchlist combobox) | Description | Selected |
|--------|-------------|----------|
| base-ui Combobox | Phase 24 vendored, `multiple` with chips | ✓ |
| Hand-rolled HistoryFilters | Would need multi-select built | |
| Mixed by use | base-ui for Watchlist, hand-rolled for History | |

| Option (chip click) | Description | Selected |
|--------|-------------|----------|
| Yes, toggles filter | Chip body toggles tag in filter; × still detaches | ✓ |
| No, chips stay labels | Toolbar-only filtering | |

| Option (preference filters) | Description | Selected |
|--------|-------------|----------|
| Two separate toggles | Has muted events / Custom release types | ✓ |
| One "Customized" toggle | Either condition | |
| Per-type filters | Per event/release type | |

| Option (History tag) | Description | Selected |
|--------|-------------|----------|
| Single tag | Scalar `tag_id` narg | ✓ |
| Multi-tag any-of | `tag_ids` array | |

| Option (History tag control) | Description | Selected |
|--------|-------------|----------|
| Hand-rolled, match siblings | Reuse HistoryFilters `Combobox<T>` | ✓ |
| base-ui everywhere | Mismatch within History row | |
| Migrate all History controls | Out-of-scope refactor | |

## State persistence

| Option (Watchlist state) | Description | Selected |
|--------|-------------|----------|
| URL query params | `useSearchParams`, replace while typing | ✓ |
| Component state, resets | useState only | |
| localStorage | Per-browser memory | |

| Option (History to URL) | Description | Selected |
|--------|-------------|----------|
| Yes, same treatment | Consistent across routes | ✓ |
| No, Watchlist only | Smaller diff | |

| Option (cross-link) | Description | Selected |
|--------|-------------|----------|
| Independent | Tabs stay decoupled | ✓ |
| "View history" link | Single-tag filter links to /history?tag=N | |

| Option (stale params) | Description | Selected |
|--------|-------------|----------|
| Drop silently, rewrite URL | Delete/merge in Manage tags updates active filter | ✓ |
| Show as "unknown tag" chip | Keep id, matches nothing | |

## Latest-release semantics

| Option (retention) | Description | Selected |
|--------|-------------|----------|
| Yes, within retention | Same `created_at >= cutoff` as ListEvents | ✓ |
| No, all-time | Every stored new_release event | |

| Option (partial/future dates) | Description | Selected |
|--------|-------------|----------|
| Lexicographic, show as-is precision | Future dates count | ✓ |
| Exclude future dates | Only already-released | |
| Treat partials as end-of-period | Diverges from History ordering | |

| Option (tag filter source) | Description | Selected |
|--------|-------------|----------|
| Derived from watchlist payload | Zero extra requests; SC5 holds | ✓ |
| Lazy GET /tags on picker open | Full vocabulary, dead options | |

**Notes:** Claude confirmed while discussing that retention keys on `events.created_at` (detection time), so seed-mode back catalogues count toward the latest release until they age out, the same way History treats them.

## Claude's Discretion

- Sort control shape, toolbar wrapping, and all copy (pinned in the UI phase)
- Search debounce
- LATERAL join form and whether an index is needed
- How the cutoff reaches `ListWatchlist`

## Deferred Ideas

- A cross-route "See releases for #tag" link
- An any/all toggle for the multi-tag filter
