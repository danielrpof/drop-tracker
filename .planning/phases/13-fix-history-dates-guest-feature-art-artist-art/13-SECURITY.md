---
phase: 13
slug: fix-history-dates-guest-feature-art-artist-art
status: verified
threats_open: 0
asvs_level: 1
created: 2026-08-25
---

# Phase 13 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| drop-tracker → musicbrainz.org | Outbound request whose path segment embeds a recording MBID originating from a prior MusicBrainz browse response | Semi-trusted, community-editable recording MBID |
| musicbrainz.org → drop-tracker | Inbound JSON decoded into `RecordingRelease`; dates, release-group ids, titles flow into stored event rows and rendered React text nodes | Semi-trusted release/date/id data |
| stored event row → History card | `release_date`/`cover_art_url` rendered client-side | Semi-trusted string values, plain JSX text nodes only |
| drop-tracker → api.deezer.com | Outbound artist search keyed by an artist name originating from MusicBrainz | Semi-trusted artist name |
| drop-tracker → musicbrainz.org (tie-break) | Outbound release-group browse keyed by a stored artist MBID | Stored artist MBID |
| api.deezer.com → drop-tracker | Inbound artist/album JSON whose `Picture` URL is written to `artists.image_url` and rendered as an image source | Semi-trusted image URL |
| artistart → artists table | Match results written via `UpsertArtist`'s COALESCE merge; attempt bookkeeping via `RecordArtMatchAttempt` (D-12) | DeezerID, ImageURL, attempt timestamp |
| HTTP client → POST /watchlist | Untrusted request whose handling now triggers outbound third-party calls (Deezer/MusicBrainz) before responding | Caller-supplied artist add payload |
| backfill goroutine → Postgres pool | Background sweep holding pool connections concurrently with the HTTP server and poll cycles | DB connections, artist rows |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-13-01 | Tampering | `ReleasesForRecording` URL construction | medium | mitigate | `url.PathEscape(trimmed)` + `url.Values` for query — verified present at `recording_lookup.go:84` | closed |
| T-13-02 | Information Disclosure | `ReleasesForRecording` non-OK status branch | medium | mitigate | Status-code-only error, no body echo — verified at `recording_lookup.go:107` | closed |
| T-13-03 | Denial of Service | Per-insert lookup inside `detectGuestFeatures`'s loop | medium | mitigate | Fires only on genuinely-new recordings, shares process-wide rate limiter, no pagination loop — verified by code read | closed |
| T-13-04 | Denial of Service | Truncated linked-release list (25-item cap) | low | accept | `MaxRecordingReleaseLinks` cap + `release_link_ceiling_count` log attribute — verified present at `musicbrainz.go:249,300` | closed |
| T-13-05 | Spoofing | Semi-trusted upstream values rendered in History cards | low | accept | Plain JSX text nodes, no `dangerouslySetInnerHTML` anywhere in `web/app/` — verified (0 matches) | closed |
| T-13-19 | Denial of Service | One artist's seed cycle monopolizing the shared MusicBrainz rate budget | medium | mitigate | `maxNewGuestFeatureLookupsPerCycle = 20` — verified present and enforced at `musicbrainz.go:35,226` | closed |
| T-13-06 | Spoofing | `Matcher.Match` identity resolution (wrong-artist photo) | high | mitigate | D-08 strict normalized-name equality, D-09 fail-closed, `NbFan` excluded from executable code — verified (0 occurrences outside comments) at `match.go` | closed |
| T-13-07 | Tampering | Artist name (semi-trusted) used as a Deezer query | low | mitigate | Existing `deezer.Client` methods already URL-encode/escape — no raw interpolation introduced (unchanged, verified by code read) | closed |
| T-13-08 | Information Disclosure | Error paths inside `Match` | low | mitigate | No response-body echo in `match.go` (0 matches for `resp.Body`/raw error text) — delegates entirely to existing status-code-only client errors | closed |
| T-13-09 | Denial of Service | Tie-break fetch fan-out | medium | mitigate | `maxTieBreakCandidates = 5` — verified present and enforced at `match.go:201,250` | closed |
| T-13-10 | Denial of Service | Backfill sweep query breadth | low | mitigate | `ListArtistsMissingImage` joins `watchlist`, bounded by watchlist size + D-12 cooldown — verified in `artists.sql` | closed |
| T-13-11 | Information Disclosure | Deezer `Picture` URL stored and later rendered | low | accept | Rendered as image `src`, not markup — same posture as existing Phase 6 image URLs | closed |
| T-13-20 | Denial of Service | Backfill and add-time match contending for shared rate-limited clients | medium | mitigate | `ActivityGate` yielding — verified wired in `backfill.go` (`waitForActivityGate`, called in the sweep loop) | closed |
| T-13-12 | Denial of Service | Add-time match inside POST /watchlist request path | high | mitigate | `matchTimeout = 8s` on a derived sub-context, bounded by `maxTieBreakCandidates`, registers on `ActivityGate` — verified at `service.go:30,214` | closed |
| T-13-13 | Denial of Service | Backfill sweep contending for pool connections | medium | mitigate | Single sequential goroutine, `ctx.Err()` checked per iteration, drained before `pool.Close()` (LIFO defer order, verified at `main.go:109` vs `203+`) | closed |
| T-13-14 | Denial of Service | Backfill blocking process startup | medium | mitigate | Started asynchronously after `pollr.Start(ctx)` — verified at `main.go:202` | closed |
| T-13-15 | Spoofing | A wrong-artist photo persisted by either call site | high | mitigate | Both call sites delegate to the single `Matcher.Match`; `backfill.go` contains no match-rule logic of its own (0 occurrences of `titlesMatch`/`foldDiacritics`/`tieBreak(`) — verified | closed |
| T-13-16 | Tampering | Backfill overwriting unrelated artist metadata | medium | mitigate | `Disambiguation: nil` passed explicitly, `UpsertArtist`'s COALESCE preserves existing values — verified at `backfill.go:193` | closed |
| T-13-17 | Repudiation | Silent art-resolution failures | low | mitigate | Add-time logs at Warn, sweep logs at Error per-artist + one Info summary with `Stats` counters and match rate — verified by code read | closed |
| T-13-18 | Information Disclosure | Error strings logged from upstream calls | low | accept | Existing client methods already return status-code-only messages, no body echoed | closed |
| T-13-21 | Denial of Service | Restart-heavy deployment re-attempting fail-closed artists every startup | medium | mitigate | `RecordArtMatchAttempt` + `ListArtistsMissingImage`'s 24h cooldown predicate — verified in `artists.sql:22,46` | closed |
| T-13-SC-01 | Supply Chain | `web/package.json` gained `@testing-library/dom ^10.4.1` (devDependency) during 13-01's execution — not anticipated by the plan's threat model, which claimed "zero npm packages" | low | accept | Official `@testing-library` org package, the documented peer dependency of the already-approved `@testing-library/react`; added as a Rule-3 blocking-issue auto-fix (documented in 13-01-SUMMARY.md), not a new capability. `devDependencies` only — zero production/runtime bundle exposure. Package identity confirmed against the npm registry's `@testing-library` scope naming convention already established elsewhere in this file (no typosquat pattern). | closed |

*Status: open · closed · open — below {block_on} threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-13-01 | T-13-04 | 25-linked-release truncation is a MusicBrainz-imposed ceiling; worst case is a wrong-but-plausible display date, never a crash. Made observable via `release_link_ceiling_count`. | Phase 13 plan (13-01-PLAN.md) | 2026-08-24 |
| AR-13-02 | T-13-05 | Semi-trusted upstream text rendered as plain JSX text nodes only — existing Phase 6 XSS posture already covers this surface. | Phase 13 plan (13-01-PLAN.md) | 2026-08-24 |
| AR-13-03 | T-13-11 | Deezer image URL rendered as an `<img>` src, not markup — same posture as every other upstream-sourced image URL in this app. | Phase 13 plan (13-02-PLAN.md) | 2026-08-24 |
| AR-13-04 | T-13-18 | Upstream client error messages are already status-code-only by existing convention; no new body-echo surface introduced. | Phase 13 plan (13-03-PLAN.md) | 2026-08-24 |
| AR-13-05 | T-13-SC-01 | `@testing-library/dom` is an official first-party peer dependency of an already-approved package, devDependency-only, added as an unplanned but necessary auto-fix during execution. | Security audit (this document) | 2026-08-25 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-25 | 22 | 22 | 0 | Claude (orchestrator, ASVS L1 grep-depth self-verification — gsd-security-auditor subagent hit a session usage limit mid-run and was not retried; verification completed inline against the same threat register and constraints) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-25
