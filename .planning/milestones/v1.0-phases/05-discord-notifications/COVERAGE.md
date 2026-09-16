# Phase 05 — Discord Webhook API Coverage Matrix

**Produced:** 2026-08-08 (plan time)
**External API:** Discord Webhook API (`POST https://discord.com/api/webhooks/{id}/{token}`)
**Baseline:** full coverage — every capability starts as `INTEGRATE`; this table is the subtraction record.

> Note on detection: `api-coverage.cjs --json` returned `detected: false` over the
> pre-plan phase scope (ROADMAP section + CONTEXT.md), because the phase prose
> keeps its API vocabulary inside fenced code blocks that the detector strips.
> The phase nonetheless genuinely integrates an external API, so this matrix is
> produced deliberately rather than skipped.

| capability | decision | reason |
|---|---|---|
| `execute-webhook` (POST, send a message) | INTEGRATE | The entire delivery path (NTFY-01/02/03). |
| `embeds[]` object (title, url, color, fields, thumbnail, timestamp) | INTEGRATE | D-01 locks embeds as the message format; color + emoji are what make types distinguishable. |
| `204 No Content` success handling | INTEGRATE | The only signal that flips `notified_at` (RESEARCH Pitfall 1 — a 200-only check would re-send forever). |
| `429` + `retry_after` body handling | INTEGRATE | D-08 requires honoring `Retry-After` before any retry within one send attempt. |
| Non-2xx error handling (4xx/5xx) | INTEGRATE | D-09's failure contract — leave `notified_at` NULL, log, let the next cycle re-pick. |
| Embed field character limits (title 256 / field value 1024) | INTEGRATE | ASVS V5 — community-editable MusicBrainz/Deezer content is truncated before marshaling. |
| `?wait=true` query param (returns the created message body) | OPT-OUT | No downstream consumer needs the created message id; opting in would swap the success code from 204 to 200 for zero benefit. |
| Edit webhook message (`PATCH /webhooks/{id}/{token}/messages/{msg}`) | OPT-OUT | Not needed — no message id is retained, and no phase behavior mutates an already-sent notification. |
| Delete webhook message (`DELETE .../messages/{msg}`) | OPT-OUT | Not needed — nothing in this phase un-sends a notification; no message id is retained. |
| Get webhook / get webhook-with-token (`GET /webhooks/...`) | OPT-OUT | Not needed — the webhook URL is operator-supplied config (OPS-03); its metadata is never read back. |
| File / attachment upload (`multipart/form-data`) | OPT-OUT | Not needed — cover art is referenced by URL via the embed thumbnail, never re-uploaded. |
| `content` field (plain message text) | OPT-OUT | Explicitly out of scope — D-01 locks embeds, and `content` is the only field that parses `@mentions`, so avoiding it removes the ping surface entirely. |
| `allowed_mentions` object | OPT-OUT | Not needed — a consequence of the `content` opt-out above: embeds do not parse mentions, so there is nothing to restrict. |
| `username` / `avatar_url` per-message override | OPT-OUT | Not needed — the webhook's channel-configured identity is already correct for all three event types. |
| `tts` (text-to-speech) | OPT-OUT | Not needed — a release alert has no accessibility or urgency case for TTS. |
| `thread_id` / `thread_name` (post into a thread) | OPT-OUT | Explicitly out of scope — D-01 rejects per-type channels/threads; one webhook, one channel, distinguished by color + emoji. |
| `components[]` (buttons / select menus) | OPT-OUT | Not needed yet — interactive components require a bot application and an interaction endpoint, which PROJECT.md's single-binary webhook-only scope excludes. |
| `poll` object | OPT-OUT | Not needed — no polling/voting behavior in a release-alert product. |
| Multi-embed message (up to 10 embeds per POST) | OPT-OUT | Explicitly out of scope — D-07 locks one event per message to preserve the 1:1 color/emoji mapping D-01 depends on. |
| Proactive `X-RateLimit-*` header bucket tracking | OPT-OUT | Not needed yet — D-07's fixed inter-send spacing plus D-08's reactive `Retry-After` handling is the locked strategy at one-webhook, one-burst-per-cycle volume. |
