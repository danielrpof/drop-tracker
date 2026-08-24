# Phase 13: Fix History Dates, Guest-Feature Art & Artist Art - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-24
**Phase:** 13-fix-history-dates-guest-feature-art-artist-art
**Areas discussed:** Guest-feature date & art sourcing, Deluxe-change date rendering, Artist-art backfill scope, Artist match confidence & tie-breaking

---

## Guest-feature date & art sourcing

| Option | Description | Selected |
|--------|-------------|----------|
| Extra per-event MB lookup | Per new-insert `GET /ws/2/recording/{mbid}?inc=releases+release-groups` for a release-group MBID + date; bounded cost, shares existing rate limiter | ✓ |
| Fall back to artist's own photo | Use artist `image_url` as a generic thumbnail instead of per-track cover art | |
| Skip art/date for guest features | Leave guest-feature cards without art/date entirely, defer to a future phase | |

**User's choice:** Extra per-event MB lookup

| Option | Description | Selected |
|--------|-------------|----------|
| Earliest release | Use the release with the earliest date among the recording's releases | ✓ |
| First release returned by MusicBrainz | Take whichever release the API lists first (no ordering guarantee) | |
| You decide | Let implementation pick during planning/research | |

**User's choice:** Earliest release

| Option | Description | Selected |
|--------|-------------|----------|
| Same as today | Placeholder cover art + "Release date unknown" fallback when no releases found | ✓ |
| Hide the date/art row entirely | Don't reserve space for a missing date on that card | |

**User's choice:** Same as today
**Notes:** Recording-to-release lookup is per genuinely-new insert only, not per browse result — keeps the extra MusicBrainz call count bounded and distinguishes this from Phase 12's rejected "extra call per search result" pattern.

---

## Deluxe-change date rendering

| Option | Description | Selected |
|--------|-------------|----------|
| Same line, before the tracks | "{date} · {prev} → {current} tracks", matches NewReleaseBody's separator style | ✓ |
| Separate line below | Date on its own line under the track-count delta | |

**User's choice:** Same line, before the tracks

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, same fallback text | "Release date unknown" when release_date is null, consistent with NewReleaseBody | ✓ |
| Omit the date line entirely when null | Don't show any date text for deluxe changes missing a date | |

**User's choice:** Yes, same fallback text

---

## Artist-art backfill scope

| Option | Description | Selected |
|--------|-------------|----------|
| Both — new adds + backfill existing | Match at add-time going forward AND one-time backfill over existing image_url IS NULL rows | ✓ |
| New adds only | Fix going forward only, no backfill job | |
| On-demand backfill (button/endpoint) | Manual per-artist or bulk "find art" action instead of automatic pass | |

**User's choice:** Both — new adds + backfill existing

| Option | Description | Selected |
|--------|-------------|----------|
| One-time migration/startup pass | Sweeps every artist with image_url IS NULL once, on startup after this phase ships | ✓ |
| Folded into the existing poll cycle | Each artist's regular poll also checks/fills missing art | |
| N/A — no backfill (new adds only) | Only relevant if "New adds only" was chosen above | |

**User's choice:** One-time migration/startup pass

---

## Artist match confidence & tie-breaking

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, name-equality-first with album tie-break | Confirms backlog 999.2 design: exact/close name match required, album-title overlap only breaks ties | ✓ |
| Looser fuzzy name matching | Allow near-matches without an exact name hit | |
| You decide | Let implementation pick during planning/research | |

**User's choice:** Yes, name-equality-first with album tie-break

| Option | Description | Selected |
|--------|-------------|----------|
| Fail closed — leave image_url NULL | No art shown rather than risk attaching the wrong artist's photo | ✓ |
| Show the best-guess match anyway | Attach the top-ranked Deezer result even without high confidence | |

**User's choice:** Fail closed — leave image_url NULL

---

## Claude's Discretion

None — every gray area reached an explicit user decision this session. The exact MusicBrainz client method name/shape for the new per-recording releases lookup (D-01) is left to planning/research.

## Deferred Ideas

None — discussion stayed within phase scope.
