# Phase 6: Frontend & Release History - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-10
**Phase:** 06-Frontend & Release History
**Areas discussed:** Layout & navigation, History feed shape, Add-artist & preferences flow, Visual style & album art

---

## Layout & navigation

| Option | Description | Selected |
|--------|-------------|----------|
| Two tabs/routes: Watchlist + History | Clean separation — manage vs. browse | ✓ |
| Single dashboard, both sections stacked | No routing, gets cramped with many items | |
| Three-way split: Watchlist / History / per-artist detail | More surface area, dedicated artist page | |

**User's choice:** Two tabs/routes: Watchlist + History

| Option | Description | Selected |
|--------|-------------|----------|
| Search bar lives at the top of the Watchlist tab | Search-to-add and manage-existing in one visual context | ✓ |
| Add-artist modal triggered by a button | Keeps base list uncluttered | |

**User's choice:** Search bar lives at the top of the Watchlist tab

| Option | Description | Selected |
|--------|-------------|----------|
| Purely read-only feed, no cross-links | Simplest, no scroll-and-highlight logic | ✓ |
| Clicking an event's artist name jumps to Watchlist tab, scrolled to that artist | Connective tissue, but not required by any locked requirement | |

**User's choice:** Purely read-only feed, no cross-links

| Option | Description | Selected |
|--------|-------------|----------|
| Purely a management list — no history hints | No duplicated/derived counts to keep in sync | ✓ |
| Small badge/count of unseen events per artist | Requires a "last viewed" concept that doesn't exist | |

**User's choice:** Purely a management list — no history hints
**Notes:** Layout settled after 4 questions; user moved to next area without further discussion.

---

## History feed shape

| Option | Description | Selected |
|--------|-------------|----------|
| Global chronological feed, newest first | Simplest API, matches HIST-01's wording | ✓ |
| Grouped by artist, expandable sections | More structure, loses at-a-glance cross-artist view | |

**User's choice:** Global chronological feed, newest first

| Option | Description | Selected |
|--------|-------------|----------|
| No filters for v1 | Simplest | |
| Filter by event type only | Small added scope | |
| Filter by both artist and event type | Most flexible, matches HIST-01's "per artist" wording | ✓ |

**User's choice:** Filter by both artist and event type
**Notes:** This was the higher-scope option, but confirmed in-scope since HIST-01's requirement text explicitly says "per artist" — not scope creep.

| Option | Description | Selected |
|--------|-------------|----------|
| Infinite scroll / "load more" button | Natural for an activity feed | ✓ |
| Numbered pagination (page 1, 2, 3...) | More explicit but less natural for a feed | |
| No pagination — fetch everything | Will degrade as history grows | |

**User's choice:** Infinite scroll / "load more" button

| Option | Description | Selected |
|--------|-------------|----------|
| Type-specific detail: cover art + delta info per event type | Mirrors Discord embed shapes from Phase 5 | ✓ |
| Uniform minimal card: title, artist, date, type badge only | Simpler, loses track-count-delta detail | |

**User's choice:** Type-specific detail: cover art + delta info per event type

---

## Add-artist & preferences flow

| Option | Description | Selected |
|--------|-------------|----------|
| Two labeled columns/sections, side by side | Matches existing GET /search response shape | ✓ |
| Single merged list, source shown as a small tag per result | More visual work to interleave two independently-ranked lists | |

**User's choice:** Two labeled columns/sections, side by side

| Option | Description | Selected |
|--------|-------------|----------|
| Add with defaults, edit preferences afterward | One preference-editing UI to build, not two | ✓ |
| Preference picker shown inline before confirming add | More upfront friction, duplicates edit UI | |

**User's choice:** Add with defaults, edit preferences afterward

| Option | Description | Selected |
|--------|-------------|----------|
| Search result shows "Already watching" (disabled/greyed), no add action | Client-side prevention, avoids hitting 409 | ✓ |
| Let the click proceed, show the API's 409 as an error toast | Simpler client logic, surfaces a preventable server error | |

**User's choice:** Search result shows "Already watching" (disabled/greyed), no add action

| Option | Description | Selected |
|--------|-------------|----------|
| Inline checkboxes/toggles directly in each watchlist row | No modal, matches existing PATCH partial-update semantics | ✓ |
| Edit button opens a settings panel/modal per artist | More clicks per change, simpler base list | |

**User's choice:** Inline checkboxes/toggles directly in each watchlist row

---

## Visual style & album art

| Option | Description | Selected |
|--------|-------------|----------|
| Clean, minimal, dark-themed — functional but polished | Signals competence without competing with CI/CD focus | ✓ |
| Bare-bones/utilitarian — default browser styling, function only | Fastest, but may undercut portfolio impression | |
| High-design, custom branded look (logo, custom palette, animations) | Most impressive but competes with Phase 7's time budget | |

**User's choice:** Clean, minimal, dark-themed — functional but polished

| Option | Description | Selected |
|--------|-------------|----------|
| Small thumbnail per item (both feed and watchlist) | Modest visual anchor without dominating layout | |
| Large hero-style art (feed reads like an album-cover grid/wall) | More visually striking, bigger layout commitment | ✓ |
| No art at all — text-only rows | Simplest, but leaves captured cover_art_url/image_url unused | |

**User's choice:** Large hero-style art (feed reads like an album-cover grid/wall)
**Notes:** This is a deliberate tension against the "minimal" framing of the previous answer — user wants restrained chrome/controls but visually striking album art specifically. Captured explicitly in CONTEXT.md D-14's rationale so downstream planning doesn't flatten this into generic minimalism.

| Option | Description | Selected |
|--------|-------------|----------|
| Tailwind CSS utility classes | Fast, consistent dark theme, no runtime CSS-in-JS overhead | ✓ |
| Plain CSS / CSS Modules, hand-written | Full control, slower to iterate | |
| A component library (e.g. Mantine, Chakra) | Fastest for polished widgets, heavier dependency | |

**User's choice:** Tailwind CSS utility classes

| Option | Description | Selected |
|--------|-------------|----------|
| Friendly empty-state message + clear call-to-action | Styled consistently with dark theme | ✓ |
| Minimal placeholder text only, no special styling | Less time spent, less polished | |

**User's choice:** Friendly empty-state message + clear call-to-action

---

## Claude's Discretion

- Exact history API route/param names and pagination mechanism (cursor vs. offset)
- Exact new sqlc query/queries and any new index needed for the paginated, filterable history read
- React project layout, routing library choice, data-fetching approach, component decomposition
- Whether the frontend polls automatically or relies on manual refresh (default: manual refresh unless research suggests otherwise)
- Exact Tailwind color palette/dark theme tokens, spacing scale, hero-art grid/card sizing
- Exact `go:embed` wiring point and SPA-serving/routing-fallback mechanism

## Deferred Ideas

None — discussion stayed fully within phase scope. The one option that added scope beyond the minimum (artist+event-type filtering in the History feed) was confirmed to already be covered by HIST-01's literal requirement text ("per artist"), not new capability.
