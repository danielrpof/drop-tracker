# Requirements: drop-tracker

**Defined:** 2026-09-22
**Milestone:** v1.6 Watchlist Organization
**Core Value:** A single Go binary that reliably detects and notifies on new releases for watched artists, built and shipped through a CI/CD pipeline rigorous enough to demonstrate real DevOps practice.

## v1.6 Requirements

### Tags

- [ ] **TAG-01**: User can add free-form tags to a watchlist artist, with autocomplete from existing tags; new tags are created on the fly
- [ ] **TAG-02**: User can remove a tag from an artist
- [ ] **TAG-03**: Tags match case- and whitespace-insensitively (`Reggaeton ` = `reggaeton`), keeping the first-entered display casing
- [ ] **TAG-04**: Tags are capped at 32 characters and 10 per artist, enforced by both API and DB, with a clear error
- [ ] **TAG-05**: User can rename a tag globally; renaming onto an existing tag asks to confirm a merge
- [ ] **TAG-06**: User can delete a tag globally, with a confirmation stating how many artists carry it
- [ ] **TAG-07**: Tags belong to the artist (not the watchlist entry), so they survive removal and reappear on re-add

### Notes

- [ ] **NOTE-01**: User can add, edit, and clear a plain-text note (≤500 characters) on a watchlist artist, shown on its Watchlist card

### Watchlist View

- [ ] **WLVW-01**: User can search the watchlist by artist name
- [ ] **WLVW-02**: User can sort by name (A–Z/Z–A) or date added (newest/oldest), with a stable tie-break
- [ ] **WLVW-03**: User can sort by latest release (the artist's newest own `new_release` event date that is not upcoming); artists with none sort last in both directions
- [ ] **WLVW-04**: User can filter the watchlist by one or more tags
- [ ] **WLVW-05**: User can filter to artists with muted event types or non-default release-type filters
- [ ] **WLVW-06**: Search, sort, and filters combine; the view shows "N of M artists" and an empty state with a clear-filters action

### History

- [ ] **HIST-02**: User can filter the History feed by tag, combined with existing filters and respecting event retention

### Bulk Edit

- [ ] **BULK-01**: User can select multiple artists on the Watchlist, including "select all visible"
- [ ] **BULK-02**: User can add or remove a tag on all selected artists in one action
- [ ] **BULK-03**: User can set release-type filters and muted event types on all selected artists in one action
- [ ] **BULK-04**: User can remove all selected artists after a confirmation stating the count; the removal is all-or-nothing
- [ ] **BULK-05**: After a bulk action the selection clears and the watchlist reflects the result without a page reload

### Bulk Add

- [ ] **IMPT-01**: User can paste a list of artist names, one per line; blank lines and in-paste duplicates are ignored
- [ ] **IMPT-02**: The app searches each name within existing rate limits, shows progress, and can be cancelled
- [ ] **IMPT-03**: A review screen shows each line's best match with confidence (high / medium / not found) and lets the user pick an alternate or skip
- [ ] **IMPT-04**: Artists already on the watchlist are flagged and not re-added
- [ ] **IMPT-05**: Nothing is added until the user confirms; confirm adds all selected artists in one request and reports added / skipped / failed

### Notifications

- [ ] **NTFY-05**: Real-time Discord alerts show the artist's tags
- [ ] **NTFY-06**: Digest lines show the artist's tags; grouping stays by event type and chunking still never silently truncates or exceeds Discord limits
- [ ] **NTFY-07**: Tags and other user/artist-supplied text are markdown-escaped on real-time embeds (closing the existing gap in `internal/notifier/format.go`)

## Future Requirements

Deferred — tracked but not in the v1.6 roadmap.

- Apply tags to all artists at bulk-add confirm time
- Group the digest by tag
- Show notes on History cards
- Per-tag notification muting
- Tag colors

## Out of Scope

| Feature | Reason |
|---------|--------|
| Hierarchical / nested tags | Flat multi-tags cover the grouping need; hierarchy adds UI and query complexity for no current use case |
| Seed-mode on re-adding a removed artist | Events survive removal, so a re-add skips seed mode; accepted and documented in UI copy rather than fixed |
| Server-side batch import job | The 15s write timeout vs MusicBrainz 1 req/s rules out one blocking request, and a background job adds machinery v1.6 doesn't need — the SPA drives per-name search instead |
| Pre-search cleanup of pasted names ("Artist - extra") | Existing search ranking absorbs the noise; misses surface as "not found" on the review screen |
| Server-side paginated/sorted watchlist queries | 50–500 rows load in full today; client-side sort/filter is simpler and avoids sqlc dynamic ORDER BY pitfalls |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| TAG-01 | Phase 24 | Gaps Found |
| TAG-02 | Phase 24 | Gaps Found |
| TAG-03 | Phase 24 | Gaps Found |
| TAG-04 | Phase 24 | Gaps Found |
| TAG-05 | Phase 24 | Gaps Found |
| TAG-06 | Phase 24 | Gaps Found |
| TAG-07 | Phase 24 | Gaps Found |
| NOTE-01 | Phase 24 | Gaps Found |
| WLVW-01 | Phase 25 | Pending |
| WLVW-02 | Phase 25 | Pending |
| WLVW-03 | Phase 25 | Pending |
| WLVW-04 | Phase 25 | Pending |
| WLVW-05 | Phase 25 | Pending |
| WLVW-06 | Phase 25 | Pending |
| HIST-02 | Phase 25 | Pending |
| BULK-01 | Phase 26 | Pending |
| BULK-02 | Phase 26 | Pending |
| BULK-03 | Phase 26 | Pending |
| BULK-04 | Phase 26 | Pending |
| BULK-05 | Phase 26 | Pending |
| IMPT-01 | Phase 27 | Pending |
| IMPT-02 | Phase 27 | Pending |
| IMPT-03 | Phase 27 | Pending |
| IMPT-04 | Phase 27 | Pending |
| IMPT-05 | Phase 27 | Pending |
| NTFY-05 | Phase 28 | Pending |
| NTFY-06 | Phase 28 | Pending |
| NTFY-07 | Phase 28 | Pending |

**Coverage:**

- v1.6 requirements: 28 total
- Mapped to phases: 28
- Unmapped: 0 ✓

---
*Requirements defined: 2026-09-22*
*Last updated: 2026-09-22 after v1.6 roadmap creation (Phases 24-28)*
