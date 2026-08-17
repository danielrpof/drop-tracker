---
created: 2026-08-17T05:43:02.500Z
title: Fix white-on-white font in History filter dropdowns
area: ui
severity: cosmetic
files:
  - web/app/routes/history.tsx
---

## Problem

On the History page, the "Event types" and "Artists" filter dropdown boxes render white font on a white dropdown background — the option text is unreadable. Found during phase 10 UAT.

## Solution

TBD — likely a missing text-color class/token on the `<select>`/dropdown component used for these two filters. Check whether other selects in the app already use a correct text-color token and reuse it here.
