# Requirements: drop-tracker

**Defined:** 2026-09-11
**Core Value:** A single Go binary that reliably detects and notifies on new releases for watched artists, built and shipped through a CI/CD pipeline rigorous enough to demonstrate real DevOps practice.

## v1.5 Requirements

Requirements for the "Digest Notifications" milestone. Each maps to roadmap phases.

### Digest Configuration

- [ ] **DGST-01**: Operator can toggle digest mode on/off from the SPA, persisted in Postgres, taking effect without a redeploy or restart
- [ ] **DGST-02**: Operator can choose a digest cadence — daily or weekly — from the SPA
- [ ] **DGST-03**: Digest on/off + cadence settings survive process restart (Postgres-backed, not in-memory)
- [ ] **DGST-04**: Default state is digest off — real-time per-event notifications remain unchanged unless the operator opts in

### Digest Scheduling

- [ ] **DGST-05**: Digest job fires at a stable, predictable time per the chosen cadence, resilient to the process being down at the exact fire time (a missed tick is caught on the next check, not silently skipped)
- [ ] **DGST-06**: Digest scheduling handles daylight-saving-time transitions without skipping or double-firing a digest for the same period
- [ ] **DGST-07**: Digest scheduling works correctly in the shipped container image (Alpine base) despite its minimal timezone database

### Digest Delivery

- [ ] **DGST-08**: When digest mode is on, all three event types (new release, guest feature, deluxe/tracklist change) accumulated since the last digest are batched into one scheduled Discord message
- [ ] **DGST-09**: A digest with zero accumulated events is not sent (silent skip, no empty message)
- [ ] **DGST-10**: Events within a digest are grouped for readability (by event type and/or artist) rather than an unstructured flat list
- [ ] **DGST-11**: Each digest message shows the window it covers (e.g. "since [timestamp]")
- [ ] **DGST-12**: A digest that would exceed Discord's per-message embed/character limits splits into multiple messages instead of silently truncating content

### Real-Time / Digest Interop

- [ ] **DGST-13**: When digest mode is on, real-time per-event Discord notifications stop firing for the same events (no duplicate delivery)
- [ ] **DGST-14**: Toggling from digest back to real-time flushes any events accumulated during the digest window through the normal real-time path, rather than losing or re-batching them
- [ ] **DGST-15**: A watermark (last-successful-digest-send timestamp) determines what's "new since last digest," so a late, skipped, or duplicate scheduler tick self-corrects instead of dropping or re-sending events

### Operator Visibility

- [ ] **DGST-16**: The existing SPA "System" view (or the new digest settings panel) shows the last digest send time and current digest mode/cadence

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Multi-channel notification sinks (RSS, generic webhook, email) | Parked as backlog Option A — a separate milestone; digest mode stays on the existing Discord webhook |
| Per-event-type digest overrides (e.g. releases real-time, features/deluxe digested) | Single-operator instance; adds preference-center complexity the research found no strong signal for |
| Upcoming-release calendar | Parked as backlog Option D — separate milestone |
| Watchlist tags/notes/sort/bulk-add | Parked as backlog Option E — separate milestone |
| Operator-configurable time-of-day / custom IANA timezone picker for digest fire time | Out of the "daily or weekly" cadence scope confirmed with the operator; a fixed, documented fire time is sufficient for v1.5 |
| Per-user digest preferences | App has no multi-user accounts (see PROJECT.md Out of Scope) — this is one instance-wide setting |

## Traceability

Populated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| DGST-01 | TBD | Pending |
| DGST-02 | TBD | Pending |
| DGST-03 | TBD | Pending |
| DGST-04 | TBD | Pending |
| DGST-05 | TBD | Pending |
| DGST-06 | TBD | Pending |
| DGST-07 | TBD | Pending |
| DGST-08 | TBD | Pending |
| DGST-09 | TBD | Pending |
| DGST-10 | TBD | Pending |
| DGST-11 | TBD | Pending |
| DGST-12 | TBD | Pending |
| DGST-13 | TBD | Pending |
| DGST-14 | TBD | Pending |
| DGST-15 | TBD | Pending |
| DGST-16 | TBD | Pending |

**Coverage:**
- v1.5 requirements: 16 total
- Mapped to phases: 0 (pending roadmap)
- Unmapped: 16 ⚠️ (resolved by roadmapper)

---
*Requirements defined: 2026-09-11*
*Last updated: 2026-09-11 after initial v1.5 definition*
