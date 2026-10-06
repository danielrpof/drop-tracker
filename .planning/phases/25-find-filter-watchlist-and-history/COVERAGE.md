# Phase 25 — External API Coverage Declaration

**Decided:** 2026-10-05 (planner, `/gsd-plan-phase 25`)

No external API integration: the phase extends drop-tracker's own `GET /watchlist` query and `GET /events` filter, and adds client-side find/sort/filter in the embedded SPA. It touches no third-party API, SDK, or webhook.

## Reasoning

The detector fired on "API" in 25-02's objective ("Let the History API narrow the feed..."). That sentence refers to the app's own
`GET /events` route, not to an external service.

- MusicBrainz, Deezer, and the Discord webhook are untouched. Plan 25-10's blast-radius gate asserts that nothing under
  `internal/musicbrainz`, `internal/deezer`, `internal/discord`, `internal/notifier`, `internal/detection`, or `internal/poller`
  changes.
- No new Go module or npm package is added (25-RESEARCH.md Package Legitimacy Audit: none). Plan 25-10 asserts that `go.mod`, `go.sum`,
  `web/package.json`, and `web/pnpm-lock.yaml` are unchanged.
- The only new request parameter is `tag_id` on the app's own `GET /events`. The only new response fields are `latest_release_date`
  and `next_release_date` on the app's own `GET /watchlist`.

A coverage matrix would list zero external capabilities.
