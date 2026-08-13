---
created: 2026-08-11T18:30:00.000Z
title: EventCard crashes History route on unrecognized event_type
area: ui
severity: major
files:
  - web/app/components/history/EventCard.tsx:39
---

## Problem

Surfaced by Phase 6 code review (WR-03, `.planning/phases/06-frontend-release-history/06-REVIEW.md`).

`EVENT_BADGE[event.event_type]` is keyed by the three known `event_type` literals, but `event_type` comes from the database via `GET /events` with no runtime validation against that union. A future event type, a manual DB edit, or a bug elsewhere in detection could produce a value not in `EVENT_BADGE`. `badge` would be `undefined`, and `badge.color`/`badge.emoji`/`badge.label` throw a `TypeError`, crashing the whole History route (caught only by the top-level `ErrorBoundary`, showing the generic "Oops!" page for the entire tab instead of just isolating the one bad card).

## Solution

Fall back to a default badge instead of indexing unconditionally:

```ts
const badge = EVENT_BADGE[event.event_type] ?? { label: event.event_type, emoji: "❔", color: "var(--color-secondary)" }
```
