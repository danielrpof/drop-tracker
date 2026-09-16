# Requirements: drop-tracker

**Defined:** 2026-09-11
**Core Value:** A single Go binary that reliably detects and notifies on new releases for watched artists, built and shipped through a CI/CD pipeline rigorous enough to demonstrate real DevOps practice.

## v1.5 Requirements

Requirements for the "Digest Notifications" milestone. Each maps to roadmap phases.

### Digest Configuration

- [x] **DGST-01**: Operator can toggle digest mode on/off from the SPA, persisted in Postgres, taking effect without a redeploy or restart
- [x] **DGST-02**: Operator can choose a digest cadence — daily or weekly — from the SPA
- [x] **DGST-03**: Digest on/off + cadence settings survive process restart (Postgres-backed, not in-memory)
- [x] **DGST-04**: Default state is digest off — real-time per-event notifications remain unchanged unless the operator opts in

### Digest Scheduling

- [ ] **DGST-05**: Digest job fires at a stable, predictable time per the chosen cadence, resilient to the process being down at the exact fire time (a missed fire is caught on the next check while still within a bounded grace window after its scheduled time; past that window it is not sent late, and its pending events go out at the next scheduled fire — never silently dropped)
- [ ] **DGST-06**: Digest scheduling handles daylight-saving-time transitions without skipping or double-firing a digest for the same period
- [ ] **DGST-07**: Digest scheduling works correctly in the shipped container image (Alpine base) despite its minimal timezone database

### Digest Delivery

- [ ] **DGST-08**: When digest mode is on, all three event types (new release, guest feature, deluxe/tracklist change) accumulated since the last digest are batched into one scheduled Discord message
- [ ] **DGST-09**: A digest with zero accumulated events is not sent (silent skip, no empty message)
- [ ] **DGST-10**: Events within a digest are grouped for readability (by event type and/or artist) rather than an unstructured flat list
- [ ] **DGST-11**: Each digest message shows the window it covers (e.g. "since [timestamp]")
- [ ] **DGST-12**: A digest that would exceed Discord's per-message embed/character limits splits into multiple messages instead of silently truncating content

### Real-Time / Digest Interop

- [x] **DGST-13**: When digest mode is on, real-time per-event Discord notifications stop firing for the same events (no duplicate delivery)
- [x] **DGST-14**: Toggling from digest back to real-time flushes any events accumulated during the digest window through the normal real-time path, rather than losing or re-batching them
- [ ] **DGST-15**: A persisted record of the last handled scheduled fire (separate from the last-successful-digest-send timestamp shown to the operator) determines whether a digest is due, while outbox state determines which events it carries — so a late, skipped, or duplicate scheduler tick self-corrects instead of dropping or re-sending events

### Operator Visibility

- [x] **DGST-16**: The existing SPA "System" view (or the new digest settings panel) shows the last digest send time and current digest mode/cadence

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

Mapped during roadmap creation (2026-09-11). Phase numbering continues from v1.4's Phase 19.

| Requirement | Phase | Status |
|-------------|-------|--------|
| DGST-01 | Phase 20 | Complete |
| DGST-02 | Phase 20 | Complete |
| DGST-03 | Phase 20 | Complete |
| DGST-04 | Phase 20 | Complete |
| DGST-05 | Phase 22 | Pending |
| DGST-06 | Phase 22 | Pending |
| DGST-07 | Phase 22 | Pending |
| DGST-08 | Phase 22 | Pending |
| DGST-09 | Phase 22 | Pending |
| DGST-10 | Phase 22 | Pending |
| DGST-11 | Phase 23 | Pending |
| DGST-12 | Phase 23 | Pending |
| DGST-13 | Phase 21 | Complete |
| DGST-14 | Phase 21 | Complete |
| DGST-15 | Phase 22 | Pending |
| DGST-16 | Phase 20 | Complete |

**Per-phase coverage:**

| Phase | Name | Requirements |
|-------|------|--------------|
| 20 | Digest Settings & Operator Control | DGST-01, DGST-02, DGST-03, DGST-04, DGST-16 |
| 21 | Real-Time ↔ Digest Mutual Exclusion | DGST-13, DGST-14 |
| 22 | Scheduled Digest Send | DGST-05, DGST-06, DGST-07, DGST-08, DGST-09, DGST-10, DGST-15 |
| 23 | Digest Readability & Discord Limits | DGST-11, DGST-12 |

**Coverage:**

- v1.5 requirements: 16 total
- Mapped to phases: 16 ✓
- Unmapped: 0 ✓
- Duplicated across phases: 0 ✓

---
*Requirements defined: 2026-09-11*
*Last updated: 2026-09-11 — traceability populated by roadmap (Phases 20-23)*
*Re-mapped 2026-09-11 — DGST-10 (grouping) moved from Phase 23 to Phase 22 following a grilling-session challenge to the milestone plan: building a flat, ungrouped digest in Phase 22 only to replace it with the grouped format in Phase 23 was planned throwaway work. See ROADMAP.md Phase 22/23 notes.*
*Reworded 2026-09-16 — DGST-05 (bounded grace-window catch-up) and DGST-15 (slot record decides "due", last-sent stays the displayed timestamp, outbox decides content) following a grilling-session challenge to the Phase 22 plan. See `.planning/phases/22-scheduled-digest-send/22-CONTEXT.md` D-11–D-13.*
