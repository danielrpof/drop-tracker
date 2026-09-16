---
phase: 04
slug: detection-engine
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-08
---

# Phase 04 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| MusicBrainz API -> `internal/detection` | Community-editable, semi-trusted JSON (release-group/recording/release titles, dates, MBIDs, artist-credit arrays) crosses into decode, then into persisted rows | Release-group titles/dates, recording titles, credited artist names, MBIDs |
| Deezer API -> `internal/detection` | Semi-trusted JSON (album titles, cover URLs, record types, numeric ids) crosses into decode and then into persisted rows | Album titles, cover URLs, numeric album ids |
| `internal/detection` -> Postgres | External strings (MusicBrainz/Deezer-sourced) cross into stored data that Phase 5 (Discord embeds) and Phase 6 (web UI) later render | Titles, artist names, dates, cover-art URLs, track counts |
| cron tick -> `RunMusicBrainzCycle` / `RunDeezerCycle` | Concurrent entry into a cycle that performs database writes, not just logging | Trigger only — no external data |
| MusicBrainz `/ws/2/recording`, `/ws/2/release` -> `internal/musicbrainz` | Never-before-called (this phase) endpoints whose response shapes were `[ASSUMED]` by analogy to the live-verified release-group envelope (MusicBrainz unreachable during research/execution, PROJECT.md Broken Windows Ledger #3) | Recording/release JSON payloads, nested artist-credit and media arrays |
| `internal/musicbrainz` -> MusicBrainz | Outbound request volume against a free, volunteer-run community API that throttles by User-Agent | HTTP requests only |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-04-01 | Denial of Service | `internal/detection/musicbrainz.go` new_release loop | medium | mitigate | `range`-only iteration over the fetched release-group slice (`for _, g := range groups`), never a fixed-index read | closed |
| T-04-02 | Information Disclosure | `internal/detection` error wrapping and log lines | low | mitigate | Every error wrapped `"detection: <op>: %w"` (confirmed present in `detector.go`, `musicbrainz.go`, `deezer.go`); log lines carry artist MBID/name/counts only, never a DSN or raw driver error string | closed |
| T-04-03 | Tampering | `events` writes carrying external title/date strings | low | mitigate | All writes go through sqlc-generated parameterized queries; no string-concatenated SQL found anywhere in `internal/detection` or `queries/events.sql` | closed |
| T-04-04 | Tampering | stored `title`/`artist_name` later rendered by Phase 5/6 | low | accept | MusicBrainz titles are community-wiki content; escaping is explicitly deferred to the Phase 5 (Discord embed) / Phase 6 (web UI) render layer, documented in 04-01-PLAN.md's threat model | closed (accepted risk) |
| T-04-05 | Denial of Service | `RunMusicBrainzCycle` with detection wired in | medium | mitigate | Per-artist detection error is logged and `continue`d past; `mbRunning` guard release is on `defer` (`poller.go:205`), proven by `TestPoller_RunMusicBrainzCycle_GuardReleasedAfterDetectionError` and (Deezer side) `TestPoller_RunDeezerCycle_GuardReleasedAfterDetectionError` | closed |
| T-04-06 | Elevation of Privilege | migration `000003` applied at boot | low | mitigate | Runs through the existing `db.RunMigrations` path with its retry policy and DSN redaction; no new privileged path introduced | closed |
| T-04-07 | Denial of Service | seed mode on a newly added prolific artist | medium | mitigate | Seed rows written with `notified_at` pre-set (D-13), so `ListUnnotified` can never return them — proven by `TestDetector_SeedMode_FirstCyclePreNotifies` | closed |
| T-04-08 | Spoofing | Deezer album id crossing into the dedup key | low | mitigate | `external_id` formatted from the decoded `int64` via `strconv.FormatInt`, never raw response text; `source` column keeps namespaces separate | closed |
| T-04-09 | Denial of Service | preference predicates reading `Entry.ReleaseTypes`/`Entry.MutedEventTypes` | low | mitigate | Both read via `range`-based membership tests (`filter.go:44,57,72`), never fixed-index access; both columns CHECK-constrained at the database | closed |
| T-04-10 | Information Disclosure | Deezer detection log lines | low | mitigate | Log artist MBID, name and counts only — never the upstream response body or a driver error string | closed |
| T-04-11 | Tampering | Deezer cover URL persisted verbatim into `cover_art_url` | low | accept | Deezer supplies the URL directly; rendering/escaping belongs to Phase 5's Discord embed and Phase 6's UI render paths, documented in 04-02-PLAN.md's threat model | closed (accepted risk) |
| T-04-12 | Denial of Service | `isGuestFeature` indexing the artist-credit array | high | mitigate | `len(rec.ArtistCredit) == 0` guard before reading position zero (`musicbrainz.go:398,413`), proven by `TestIsGuestFeature_EmptyCredit` against nil and empty slices | closed |
| T-04-13 | Denial of Service | `RecordingsByArtist` pagination driven by upstream `recording-count` | high | mitigate | Bounded loop at `maxRecordingPages = 10` (`recordings.go:26,104`), terminating additionally on an empty page, proven by `TestRecordingsByArtist_StopsAtPageCeiling` | closed |
| T-04-14 | Spoofing | a recording page request reaching the network outside `c.doRequest` | high | mitigate | Every page request routes through `c.doRequest` (limiter + User-Agent); confirmed no direct `httpClient` reference in `recordings.go` | closed |
| T-04-15 | Information Disclosure | non-200 error text from the recording endpoint | medium | mitigate | Error messages carry the status code only, never response-body text | closed |
| T-04-16 | Denial of Service | a prolific artist's recording volume stretching the poll cycle | medium | accept | Bounded by D-05's page ceiling; residual cost (longer cycle) absorbed by the existing overlap guard, made visible via `page_ceiling_reached`, documented in 04-03-PLAN.md's threat model | closed (accepted risk) |
| T-04-17 | Tampering | credited artist names persisted into `artist_name` | low | accept | Community-wiki content; escaping belongs to Phase 5/6 render paths, documented in 04-03-PLAN.md's threat model | closed (accepted risk) |
| T-04-18 | Denial of Service | `Release.TrackCount()` over the `media` array | medium | mitigate | Sums via `range` (`releases.go:70-76`), no fixed-index access — absent/empty/partial `media` yields 0 rather than panicking | closed |
| T-04-19 | Denial of Service | `ReleasesByReleaseGroup` pagination driven by upstream `release-count` | high | mitigate | Bounded loop at `maxReleasePages = 10` (`releases.go:23,112`), terminating additionally on an empty page, proven by `TestReleasesByReleaseGroup_StopsAtPageCeiling` | closed |
| T-04-20 | Spoofing | a release-detail page request reaching the network outside `c.doRequest` | high | mitigate | Every page request routes through `c.doRequest`; confirmed no direct `httpClient` reference in `releases.go` | closed |
| T-04-21 | Denial of Service | request volume scaling with an artist's already-seen release-group count | medium | accept | Locked by D-01/D-04; the per-source overlap guard absorbs a long cycle by skipping the next tick, made observable via `detail_fetch_count`, documented in 04-04-PLAN.md's threat model | closed (accepted risk) |
| T-04-22 | Tampering | baseline write-back on `track_count` | medium | mitigate | Only the baseline column is ever mutated; the D-12 display snapshot stays write-once through `ON CONFLICT DO NOTHING`, proven by `TestInsertEvent_SnapshotIsWriteOnce`. A lower fresh count never lowers the baseline | closed |
| T-04-23 | Information Disclosure | non-200 error text from the release endpoint | medium | mitigate | Error messages carry the status code only, never response-body text | closed |
| T-04-SC | Tampering | Go module dependency set | high | mitigate | Zero package installs across all 4 plans (04-RESEARCH.md Package Legitimacy Audit: not applicable); `git diff --stat` on `go.mod`/`go.sum` across the full phase range is empty | closed |

*Status: open · closed · open — below {block_on} threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-04-01 | T-04-04 | MusicBrainz `title`/`artist_name` are community-wiki content; escaping is the render layer's responsibility (Phase 5 Discord embed, Phase 6 web UI), not the storage layer's | Plan 04-01 threat model | 2026-08-07 |
| AR-04-02 | T-04-11 | Deezer-supplied cover URL persisted verbatim; same render-layer-owns-escaping rationale as T-04-04 | Plan 04-02 threat model | 2026-08-07 |
| AR-04-03 | T-04-16 | Recording browse cost for a prolific artist is bounded by the existing page ceiling and overlap guard; residual cost is a longer cycle, not unbounded request growth, and is made observable via `page_ceiling_reached` | Plan 04-03 threat model | 2026-08-07 |
| AR-04-04 | T-04-17 | Credited artist names are community-wiki content; same render-layer-owns-escaping rationale as T-04-04 | Plan 04-03 threat model | 2026-08-07 |
| AR-04-05 | T-04-21 | Per-cycle MusicBrainz request volume scaling with an artist's already-seen release-group count is a locked design consequence of D-01/D-04; the overlap guard absorbs a long cycle by skipping the next tick rather than stacking cycles | Plan 04-04 threat model | 2026-08-07 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-08 | 24 | 24 | 0 | Claude (gsd-secure-phase, grep-level L1 verification — register authored at plan time across all 4 plans, ASVS level 1 short-circuit per workflow rules) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-08
