---
date: "2026-09-08 00:00"
promoted: false
---

Feature/module ideas for after v1.3 — brainstormed options for making the app more useful without increasing API polling. Captured as a note (an idea backlog), NOT a plan.

Context: core app is feature-complete through v1.3 (VPS deploy phase 17 on hold pending a real VPS). User asked "what's missing from the app itself, as a stakeholder/user." Options below, ranked by value-per-effort under the constraint "don't flood the API polling rates."

Current-state gaps identified:
1. Discord is the only notification sink — internal/notifier returns discord.Embed directly, no channel abstraction. Silent for non-Discord users.
2. App only reports what already dropped — no upcoming/announced releases, no calendar.
3. Watchlist management is thin — no tags/groups, notes, sorting, or bulk import. Painful past ~50 artists.
4. No operator visibility — no /ready, no metrics, no poll-cycle status panel. Odd for a DevOps-portfolio project.
5. No "catch me up" on add — adding an artist seeds silently, no recent-discography view.

OPTIONS:

Option A — Multi-channel notifications (RSS + generic webhook + optional email). Refactor notifier behind a Sink interface, keep Discord as one impl. Add: RSS/Atom feed (GET /feed.xml, feed token — zero push infra, no new external calls), generic outgoing webhook (one JSON POST per event), optional SMTP email digest. Value: high (removes biggest adoption limiter). API-polling impact: none. Effort: medium (interface refactor is the real work).

Option B — Digest mode + notification batching. Opt-in daily/weekly digest instead of one message per event. Uses only the events table. Value: medium-high for 20+ artist watchlists. API impact: none. Effort: low-medium. Pairs with Option A.

Option C — Operator status panel + /ready. poll_runs table (cycle start/end, artists checked, events found, errors), /ready readiness probe (DB + migrations, distinct from /health), small "System" panel in SPA. Value: medium for users, high for portfolio purpose; /ready is already a flagged dependency for the Phase 17 deploy gate. API impact: none. Effort: low-medium.

Option D — Upcoming releases / release calendar. Stop discarding future-dated release entries the poller already sees; add an "upcoming" event type + calendar view with countdowns; re-check pending items near their date, convert to new_release on drop. Value: high. API impact: low but non-zero. Effort: medium-high (touches detection, schema, UI; MusicBrainz future-date data quality is uneven).

Option E — Watchlist organization. Pure DB + UI: per-artist tags/groups, notes field, sort/filter watchlist, paste-a-list bulk add. Value: medium, grows with watchlist size. API impact: none. Effort: low-medium.

RECOMMENDATION: Option A then B as one small milestone (fixes the two things most likely to stop someone using the app). Option C is the alternative if leaning into the portfolio framing — small, on-theme, and /ready is needed for Phase 17 anyway.

Related existing quick-task: 260825-g6i-improve-history-tab-show-which-watchlist (overlaps Option C/E territory).
