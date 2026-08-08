---
status: testing
phase: 05-discord-notifications
source: [05-VERIFICATION.md]
started: 2026-08-08T22:02:40Z
updated: 2026-08-08T22:02:40Z
---

## Current Test

number: 1
name: Live Discord rendering and mention-suppression check
expected: |
  Point DISCORD_WEBHOOK_URL at a real Discord channel, seed one of each event type
  (new_release, guest_feature, deluxe_change -- including an artist name containing
  @everyone if feasible in a throwaway test channel), and let a poll cycle or manual
  NotifyPending trigger deliver them.

  Three visually distinct messages render correctly:
  - new_release: green sidebar, new-release emoji title prefix, Artist/Release Date/Type
    fields, cover-art thumbnail, clickable release-group/album link
  - guest_feature: yellow sidebar, distinct emoji prefix, Artist field, clickable
    recording link
  - deluxe_change: fuchsia sidebar, distinct emoji prefix, Tracks field showing the
    count delta, clickable release link

  No mention token (@everyone, @here, role/user mention) pings anyone, even if present
  in a community-edited artist name.
awaiting: user response

## Tests

### 1. Live Discord rendering and mention-suppression check
expected: |
  Three visually distinct messages render correctly (color, emoji, fields, thumbnail,
  clickable link per type) and no mention token pings anyone. httptest.Server-backed
  tests prove the correct JSON payload is sent (including "allowed_mentions":{"parse":[]}),
  but only a real Discord client can confirm the message actually renders and behaves
  as intended.
result: [pending]

### 2. Backstop-tier truncation boundary and total-budget checks
expected: |
  Confirm a title of exactly 256 runes (including multi-byte characters) is sent
  unmodified, and that a worst-case fully-populated embed (max title + all optional
  fields at their limits) stays under Discord's ~6000-character total embed budget.
  Exact-256-rune title round-trips unchanged; total payload never approaches the
  ~6000-char ceiling. Both truths are marked `verification: backstop` in
  05-02-PLAN.md's frontmatter -- no automated test asserts either boundary case
  directly (only over-limit truncation, at 300 runes, is tested).
result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
