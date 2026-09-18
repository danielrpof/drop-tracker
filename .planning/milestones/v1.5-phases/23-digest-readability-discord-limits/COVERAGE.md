# Phase 23 — External API Coverage Declaration

**Decided:** 2026-09-17 (planner, `/gsd-plan-phase 23`)

No external API integration: reuses `discord.Client.Send` unchanged, called N times instead of once.

## Reasoning

The "Full API Coverage by Default" capability fires when a phase integrates a new
external API, SDK, or service surface. Phase 23 introduces none:

- `discord.Client.Send(ctx context.Context, embed Embed) error` keeps its exact
  existing signature. 23-CONTEXT.md D-05 explicitly rejects widening it to
  `[]Embed`, on the arithmetic that Discord's ~6000-character budget is a total
  across all embeds in one message (packing 10 embeds buys 6000 characters, not
  40 960 — a 1.46x gain for a public-signature change and every caller).
- No new Discord endpoint, verb, query parameter, or request/response field is
  introduced. The phase calls the one existing execute-webhook POST more than
  once per digest instead of exactly once.
- `sendAttempt`'s request path, its single-429-retry policy (D-08), its
  no-body-echo rule, and its no-URL-wrapping rule are all unchanged. Plan 23-02
  adds a sentinel *error value* discriminating a case `sendAttempt` already
  reaches today (a 429 arriving on the retry attempt itself) — it changes the
  error's identity, not the HTTP surface.
- MusicBrainz and Deezer are untouched by this phase.

A coverage matrix would therefore enumerate one already-covered endpoint and add
no information.
