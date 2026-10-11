---
created: 2026-10-11T00:00:00.000Z
title: Withdraw pending events on remove
area: notifier
severity: minor
files:
  - queries/events.sql:38
  - queries/watchlist.sql:153
  - internal/watchlist/service.go:492
  - internal/notifier/notifier.go
---

## Problem

Removing an artist from the watchlist does not withdraw its pending events. The outbox query, `ListUnnotified` in `queries/events.sql`, selects every event with `notified_at IS NULL` and has no watchlist join, so alerts already detected for a removed artist are still delivered by the next real-time flush or digest. Phase 26 keeps this behaviour for single and bulk remove and discloses it in the remove confirmation (grill Q4 in `.planning/quick/261011-gq7-amend-phase-26-docs-with-grill-decisions/261011-gq7-DECISIONS.md`; 26-CONTEXT D-17 and Deferred Ideas).

## Solution

Withdraw a removed artist's pending events for both single remove and Phase 26 bulk remove. Either delete or mark the artist's unnotified events in the same statement as the watchlist delete, or make the outbox query skip events whose artist is no longer watched. Check the choice against the outbox and digest-ack rules in `docs/adr/0002-one-outbox-one-sender-lock.md` and `docs/adr/0003-digest-ack-splits-event-ack-from-completion.md`. Then drop the pending-alert clause from the remove confirmation copy in 26-UI-SPEC and the shipped UI.
