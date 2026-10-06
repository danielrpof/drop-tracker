# Phase 24 — External API Coverage Declaration

**Decided:** 2026-09-23 (planner, `/gsd-plan-phase 24`)

No external API integration: the phase adds drop-tracker's own tag and note routes and a Postgres migration; no third-party API, SDK, or webhook surface is touched.

## Reasoning

The detector fired on "API" and "wire" in the plans, which describe the app's own HTTP routes
(`/tags`, `/watchlist/{id}/tags`, `/watchlist/{id}/note`) and the SPA's `web/app/lib/api.ts` client for them.
None of that is a new external integration:

- MusicBrainz, Deezer, and the Discord webhook are untouched. Plan 24-07's final gate asserts no change under
  `internal/notifier`, `internal/discord`, `internal/musicbrainz`, `internal/deezer`, or `internal/detection`.
  Tags reach Discord only in Phase 28.
- No new Go module or npm package is added. `@base-ui/react` and `golang.org/x/text` are already dependencies;
  the shadcn components are vendored source from the first-party registry, not a runtime API.
- The only new outbound traffic is the one-time shadcn registry fetch during vendoring (a dev-time step
  covered by UI-SPEC's Registry Safety table), not an application integration.

A coverage matrix would list zero external capabilities.
