---
created: 2026-10-04T00:00:00.000Z
title: Unify History filters onto base-ui Combobox
area: web
severity: minor
files:
  - web/app/components/history/HistoryFilters.tsx
  - web/app/components/ui/combobox.tsx
---

## Problem

After Phase 25 the SPA has two combobox implementations. Phase 24 vendored base-ui's `Combobox`, which the Watchlist tag input and tag filter use. History's Artist, Event type, and Tag filters still use the hand-rolled `Combobox<T>` in `HistoryFilters.tsx` (Phase 11.1, `aria-activedescendant`). Each implementation has its own keyboard and accessibility behavior to maintain, and Phase 25 adds hardening to the hand-rolled one (capped listbox height, scroll-into-view, truncating trigger).

The decision to keep both for now came from the Phase 25 grill (G7 in `.planning/phases/25-find-filter-watchlist-and-history/25-CONTEXT.md`).

## Solution

Move History's three filters to base-ui `Combobox` in single-select mode, keeping their current labels, URL params, and `All …` first options. Then delete the hand-rolled `Combobox<T>` and its tests.
