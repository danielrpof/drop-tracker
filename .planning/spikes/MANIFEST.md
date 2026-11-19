# Spike Manifest

## Ideas

### paste-a-list-bulk-add
Phase 27: paste artist names, resolve each through the existing MusicBrainz search, sort rows into High / Medium / Not found / Error tiers for review, and add the confirmed set in one request.

**Requirements:**

- Confidence uses only MusicBrainz results and the shared **Same name** normalization (27-CONTEXT D-01, G-27-5)
- Searches go through the existing `/search` and the shared MB limiter. There is no server-side resolve (27-CONTEXT, ROADMAP)
- High = exactly one same-name MB result with score ≥ 90. `/search` exposes `score` (spike 001, G-27-3)
- A failed search gets one automatic retry with backoff before it shows as Error (spike 001, G-27-10)

## Spikes

| # | Idea | Name | Type | Validates | Verdict | Tags |
|---|------|------|------|-----------|---------|------|
| 001 | paste-a-list-bulk-add | paste-confidence-tiers | standard | Given a realistic 50-name paste, when searched like `/search`, then ≥ ~60% of rows are High under D-02 | VALIDATED (D-02 76% High, S90 92%; 52% HTTP 503 at 1 req/s) | phase-27, musicbrainz, confidence, rate-limit |
