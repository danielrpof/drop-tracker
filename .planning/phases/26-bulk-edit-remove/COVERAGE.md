# Phase 26 — External API Coverage Declaration

**Decided:** 2026-10-15 (planner, `/gsd-plan-phase 26`)

No external API integration: the phase adds four bulk routes to drop-tracker's own gated HTTP API (`/watchlist/bulk/*`) and multi-select UI in the embedded SPA. It touches no third-party API, SDK, or webhook.

## Reasoning

The seal-time detector fired on "wraps … api". The plans say that the vendored shadcn popover *wraps* `@base-ui/react/popover`, and that the SPA calls the app's own routes through `~/lib/api`. Neither is an external service.

- MusicBrainz, Deezer, and the Discord webhook are untouched. Plan 26-12's blast-radius gate asserts that nothing under
  `internal/musicbrainz`, `internal/deezer`, `internal/discord`, `internal/notifier`, `internal/detection`, `internal/poller`,
  or `internal/events` changes, nor `queries/events.sql`.
- No new Go module or npm package is added (26-RESEARCH.md Package Legitimacy Audit: none). Plan 26-12 asserts that `go.mod`,
  `go.sum`, `web/package.json`, and `web/pnpm-lock.yaml` are unchanged since `f793ffc`.
- The only new routes are the app's own `POST /watchlist/bulk/delete`, `POST /watchlist/bulk/tags/add`,
  `POST /watchlist/bulk/tags/remove`, and `PATCH /watchlist/bulk/preferences`.

A coverage matrix would list zero external capabilities.
