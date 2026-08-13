---
created: 2026-08-11T18:30:00.000Z
title: guestFeatureHref missing encodeURIComponent on external_id
area: ui
severity: cosmetic
files:
  - web/app/components/history/EventCard.tsx:108-116
---

## Problem

Surfaced by Phase 6 code review (IN-01, `.planning/phases/06-frontend-release-history/06-REVIEW.md`).

```ts
function guestFeatureHref(event: EventItem): string | null {
  if (event.source === "musicbrainz") {
    return `https://musicbrainz.org/recording/${event.external_id}`
  }
  if (event.source === "deezer") {
    return `https://www.deezer.com/track/${event.external_id}`
  }
  return null
}
```

`external_id` is interpolated directly into the URL path without `encodeURIComponent`. In practice MusicBrainz/Deezer ids are UUIDs/numeric strings so this is low-risk today, but it's not guaranteed by any type or validation, and a value containing `?`, `#`, or `/` would produce a malformed or unintended link.

## Solution

```ts
return `https://musicbrainz.org/recording/${encodeURIComponent(event.external_id)}`
```
