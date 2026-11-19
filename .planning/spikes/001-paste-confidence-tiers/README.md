---
spike: 001
idea: paste-a-list-bulk-add
name: paste-confidence-tiers
type: standard
validates: "Given a realistic 50-name hip-hop/reggaeton/R&B paste, when each line goes through the same MusicBrainz artist search /search uses (limit 10), then at least ~60% of rows are High under D-02"
verdict: VALIDATED
related: []
tags: [phase-27, musicbrainz, confidence, rate-limit]
---

# Spike 001: Paste confidence tiers (G-27-3)

## What This Validates

Given a realistic 50-name paste, when each line is searched exactly the way `GET /search` searches MusicBrainz, then at least ~60% of rows come out **High** under Phase 27 D-02 (`27-CONTEXT.md`, G-27-3 gate).

## How to Run

From the repo root (live MusicBrainz calls, about 1–2 minutes per run):

```
go run ./.planning/spikes/001-paste-confidence-tiers -rps 1 -out rps1.0
go run ./.planning/spikes/001-paste-confidence-tiers -rps 0.8 -out rps0.8
go run ./.planning/spikes/001-paste-confidence-tiers -rps 0.5 -out rps0.5
python .planning/spikes/001-paste-confidence-tiers/analyze.py   # writes merged.md
```

`main.go` uses the real `internal/musicbrainz.Client`, so the query (`artist:` Lucene scope and escaping) and `limit=10` match `/search`. `normalize` is a copy of `artistart.normalizeArtistName`. It sends a generic User-Agent, not the app's.

`names.txt` holds 40 canonical spellings plus casual ones a user would type: `tyler the creator`, `ye`, `Rosalia`, `HER`, `jhene aiko`, `Kanye West`.

## Investigation Trail

1. **First run at 1 req/s (the production `MUSICBRAINZ_RATE_LIMIT_PER_SEC` default):** 26 of 50 rows were HTTP 503, alternating with successes. The High rate could not be read from this run.
2. **Hypothesis: jitter at the ceiling.** Reran at 0.8 req/s and got 22 errors. Jitter alone doesn't explain it.
3. **Reran at 0.5 req/s** and still got 8 errors. Part of the throttling comes from how MusicBrainz treats this unregistered User-Agent or the IP's shared pool, not from our rate alone. The production UA carries a contact URL and was not tested (it identifies the project).
4. **Merged the three runs,** taking the first successful result per line, to measure tiers on 48 resolved rows. Kendrick Lamar and `tyler the creator` failed in all three runs.
5. **Added score-based rules** after reading the D-02 Medium rows. Every one of them had the right artist at rank 1 with score 100, and was demoted only by a low-score namesake further down (Drake 10:71, Nas 3:70, Eminem 2:68, Logic 3:82, JID 3:82, Young Thug 4:82, ROSALÍA 2:84, Ye 9:66). Requiring a score of 100 instead breaks J. Cole (2:98) and Karol G (2:99), where a different artist holds the 100 at rank 1.
6. **Production User-Agent check** (with the user's OK): `-rps 1 -ua "<MUSICBRAINZ_USER_AGENT default>" -out rps1.0-produa` gave 19 of 50 503s (38%). The registered UA does better than the generic one (52%), but the 503s are real. Filed as GitHub issue #17.

## Results

Merged run, 48 of 50 rows resolved (2 Error):

| Rule | Meaning | High | Medium | Error |
|---|---|---|---|---|
| **D02** (current) | unique exact name among the 10 results | 38 (76%; 79% of resolved) | 10 | 2 |
| S100 | unique exact among results scoring 100 | 44 (88%) | 4 | 2 |
| **S90** | unique exact among results scoring ≥ 90 | **46 (92%)** | 2 | 2 |
| TOPANY | top result is exact | 44 (88%) | 4 | 2 |

- **Verdict: VALIDATED.** D-02 clears the 60% gate (79% of resolved rows), so G-27-3 does not force a change.
- **No wrong High under any rule.** Every High pick under D02 and S90 is the intended artist (full table in `merged.md`).
- **S90 removes 8 of D02's 10 Medium rows** with no wrong picks. The two rows still Medium are right to be: `HER` (the artist is "H.E.R.", and two other score-90+ artists are literally named "HER") and `Kanye West` (MusicBrainz now names him "Ye", and the top result is the band "WEST."). S90 needs `score` added to `SearchArtist`, which is an added response field.
- **Casual spellings that fold correctly:** `Rosalia` → ROSALÍA, `jhene aiko` → Jhené Aiko, `Beyoncé`. Spellings that don't, as expected given Out of Scope no-cleaning: `tyler the creator` (MB uses "Tyler, The Creator") and `HER`. Without the comma `tyler the creator` isn't the same name, so it lands Medium at best.

### Surprise: 503s at the production rate

MusicBrainz returned 503 for 52% of requests at exactly 1 req/s, 44% at 0.8, and 16% at 0.5, with this spike's User-Agent. `internal/httpclient` does not retry. If production sees anything similar, the Phase 27 Error tier (D-04) will be common rather than rare, and the poller is losing MB fetches too. This is untested with the production User-Agent.

| Run (req/s) | User-Agent | 503 errors of 50 |
|---|---|---|
| 1.0 | generic | 26 |
| 0.8 | generic | 22 |
| 0.5 | generic | 8 |
| 1.0 | production default | 19 |

## Decisions taken (2026-11-18)

- **S90 adopted** as the High rule (27-CONTEXT G-27-3 outcome). `/search` gains `score`.
- **One automatic retry with backoff** for Error rows in Phase 27 (G-27-10).
- **Server-side 503 handling** (retry in `internal/httpclient`, rate headroom for the poller) is tracked in GitHub issue #17.
