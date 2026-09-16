# Phase 20: Digest Settings & Operator Control - Context

**Gathered:** 2026-09-11
**Status:** Ready for planning

<domain>
## Phase Boundary

An operator can turn digest mode on and pick daily or weekly cadence from inside the app, and that choice sticks across restarts — while notification behavior stays exactly what v1.4 shipped (nothing routes through digest mode yet; that's Phase 21+).

Requirements: DGST-01, DGST-02, DGST-03, DGST-04, DGST-16 (`.planning/REQUIREMENTS.md`).

**In scope:**
- One additive migration: a singleton `notification_settings` table (`digest_enabled`, `digest_cadence`, `digest_last_sent_at`, `updated_at`)
- `internal/settings.Store` — narrow Get/Update seam, mirrors `internal/pollruns.Store`'s shape
- Gated `GET`/`PUT` settings routes inside `registerDataRoutes`
- A digest control section added to the existing SPA `/system` view: toggle, daily/weekly dropdown, last-sent display (renders "never sent yet" — nothing writes `digest_last_sent_at` until Phase 22)

**Not in this phase:**
- Any change to real-time notification behavior — Phase 21 (mutual exclusion) and Phase 22 (scheduled send) own that
- The digest scheduler itself
- Grouping, window header, Discord chunking — Phase 23
</domain>

<decisions>
## Implementation Decisions

### UI Placement

- **D-01: Digest control lives inside the existing `/system` view, not a new page/route.** One place for all operator-facing state (per-source health, recent-runs table, About block, and now digest settings) rather than a 4th nav tab for one toggle + one dropdown. Reuses the view's existing gated fetch-on-mount plumbing.

### Interaction Model

- **D-02: Instant-apply, not a Save button.** Toggling digest on/off or changing the cadence dropdown fires the `PUT` immediately — matches the rest of the SPA's act-immediately feel (watchlist add/remove has no form/Save step either). No batched multi-field submit, no new interaction pattern to introduce.

### Feedback & Errors

- **D-03: Inline status, keep-stale-on-failure.** A brief inline confirmation near the control on a successful save. On a failed `PUT`, the toggle/dropdown reverts to its last-known-good value and an inline error line appears near the control — mirrors the System view's existing D-11/D-12 keep-stale-on-refresh pattern (`19-CONTEXT.md`) rather than introducing a toast primitive that doesn't exist anywhere in the SPA today.

### Cadence Field Visibility

- **D-04: Cadence dropdown stays visible and editable when digest mode is off, just disabled.** An operator can see and pre-set a cadence before turning digest on; greying it out (not hiding it) keeps the on/off ↔ cadence relationship visible rather than having the UI jump/reflow when the toggle flips.

### Schema

- **D-05: Singleton enforcement is a `CHECK (id = 1)` constraint plus a migration-time seed `INSERT`, not upsert-on-read.** Ratified during a post-roadmap grilling session (2026-09-11) that stress-tested the v1.5 plan, over research's own schema sketch in `.planning/research/ARCHITECTURE.md`. This needs no app-level "ensure a row exists" race handling — `GetNotificationSettings`/`UpdateNotificationSettings` are both trivial single-row PK lookups — and matches the project's existing inline-CHECK convention (`events_source_valid`, `events_event_type_valid`). No longer open for the planner to re-decide.

### Claude's Discretion

- **Exact migration column types/constraints beyond what's named** (e.g. whether `digest_cadence` is a Postgres `TEXT CHECK` or an `ENUM` type) — follows the project's existing inline-CHECK convention per `CONVENTIONS.md` / ROADMAP notes, not re-litigated here.
- **Component decomposition** within the `/system` view (a new sub-component vs. inline JSX in `system.tsx`) — planner's call, following the existing per-source-panel/About-block component split.
- **Exact inline confirmation/error copy strings** — small, not worth locking in discussion; planner writes them consistent with the System view's existing tone (e.g. "Couldn't refresh — showing data as of <time>").

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & roadmap
- `.planning/REQUIREMENTS.md` — DGST-01, DGST-02, DGST-03, DGST-04, DGST-16 are this phase's contract; the "Out of Scope" table binds (no per-user prefs, no operator-configurable fire time/timezone picker).
- `.planning/ROADMAP.md` → "Phase 20: Digest Settings & Operator Control" → "Notes for the phase planner" — the primary brief: migration shape (`000008`, next free number since `poll_runs` was rejected — see ADR-0001), `internal/settings.Store` pattern, HTTP registration inside `registerDataRoutes` (inherits `gate.Authenticate` + `RequireCSRFHeader`), SPA integration point, Definition of Done commands.
- `.planning/ROADMAP.md` → "Ordering rationale" note (above Phase Details) — why Phase 21 (the gate) lands before Phase 22 (the sender); this phase's `digest_last_sent_at` column exists now so Phase 20's panel can render "never sent yet" before Phase 22 makes it live.

### Research
- `.planning/research/SUMMARY.md` — synthesized findings across stack/features/architecture/pitfalls for the whole v1.5 milestone.
- `.planning/research/ARCHITECTURE.md` — the `internal/settings` package shape, no-cache single-row-read rationale (what makes "no restart needed" true by construction).
- `.planning/research/STACK.md` — Discord embed-limit constants (not this phase's concern, but referenced for later phases); settings-table + cache pattern discussion.

### Migration safety (mandatory reading before writing the migration)
- `internal/db/migrations/README.md` — backward-incompatible vs. unsafe-forward migration rules, N-1 rollback invariant, the `allow-destructive` annotation syntax. `cmd/migration-check` CI-enforces this; a bare `CREATE TABLE` with inline CHECKs + a paired `.down.sql` produces zero findings.
- `docs/adr/0001-in-process-ring-buffer-for-poll-run-history.md` — why `poll_runs` was rejected in favor of the ring buffer, freeing migration number `000008` for this phase's `notification_settings` table.

### Prior phase context (established patterns this phase follows)
- `.planning/milestones/v1.4-phases/18-backend-readiness-poll-run-history-status-api/18-CONTEXT.md` — `internal/pollruns.Store`'s narrow Get/Update seam shape (this phase's `internal/settings.Store` mirrors it), consumer-declared-seam convention, structural route exemptions (gate vs. inert branch, never path-string matched), functional-option pattern for `httpserver.New`, sqlc/`make sqlc-check` local-only drift-gate convention.
- `.planning/milestones/v1.4-phases/19-frontend-system-view/19-CONTEXT.md` — the `/system` view's existing structure this phase extends into: five-state render machine (D-04), keep-stale-on-refresh (D-11/D-12 — this phase's D-03 above follows the same posture for a settings save instead of a data refresh), `apiFetch`/`ApiError`/401-interceptor reuse, `web/app/lib/format.ts` timestamp formatters (for rendering `digest_last_sent_at` / "never sent yet"), dark-theme-only styling, no-polling-anywhere convention.

### Definition of Done
- `C:\CodeProjects\drop-tracker\.claude\CLAUDE.md` "Definition of Done" — `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"` before staging, then `corepack pnpm --dir web test`. `go vet`, `golangci-lint run`, `make test`, `make coverage-gate`, `make sqlc-check` (local-only, no CI counterpart) for the Go side.
- `.planning/codebase/CONVENTIONS.md` — narrow consumer-declared seams, functional options, `fmt.Errorf` + `%w` error wrapping, sanitized HTTP error responses, comment discipline (1-3 line summaries, one design-doc ID reference, no restating signatures).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/pollruns.Store` — the shape `internal/settings.Store` mirrors: narrow `Get`/`Update`, mutex-guarded, no cache invalidation logic needed (single PK-indexed row read is cheap).
- `web/app/routes/system.tsx` — the existing five-state (loading/error/session-expired/first-run/loaded) System view this phase adds a digest section to; already fetches a gated JSON endpoint on mount.
- `web/app/lib/api.ts` — `apiFetch<T>`, `ApiError` (carries `.status`), the global-401 interceptor, `X-Instance-Gated` latch — the digest settings `GET`/`PUT` calls route through this for free.
- `web/app/lib/format.ts` — existing timestamp formatters (relative/absolute) to reuse for rendering `digest_last_sent_at`.
- `internal/httpserver/server.go` `registerDataRoutes` — where the new routes register to inherit `gate.Authenticate`, the `X-Instance-Gated` marker, and `authgate.RequireCSRFHeader` (the SPA's `apiFetch` already sends `X-Requested-With` on non-GET, so no client change needed there).

### Established Patterns
- Consumer-declared narrow seams (`watchlist.Store`, `poller.EventRecorder`, `httpserver.Pinger`) — `settings.Store`/`SettingsReader` follows suit.
- Structural route exemptions in `server.go` (gated vs. inert branch), never a path-string allowlist.
- Functional options keep `httpserver.New` additive.
- sqlc codegen committed; `make sqlc-check` is the only drift gate (local, no CI counterpart — regenerate and commit manually).
- Keep-stale UI pattern (System view's manual-refresh D-11/D-12) — this phase's D-03 (inline status, revert-on-failure) is the same posture applied to a settings write instead of a data refresh.

### Integration Points
- `internal/db/migrations/` — new `000008_notification_settings.up.sql` / `.down.sql`.
- `queries/` — new `notification_settings.sql` sqlc query file.
- `internal/settings/` — new package (store).
- `internal/httpserver/server.go` — new routes in `registerDataRoutes`.
- `web/app/routes/system.tsx` — new digest section (+ likely a new sub-component under `web/app/components/system/`, following the existing per-source-panel/About-block split — planner's call).
- `web/app/lib/api.ts` — new `getDigestSettings()`/`updateDigestSettings()` wrappers + wire types.
- `cmd/server/main.go` — composition-root wiring: construct `settings.Store`, pass into `httpserver.New`.

</code_context>

<specifics>
## Specific Ideas

- Digest control renders inside the existing `/system` view, near (not replacing) the About block — exact ordering/layout is a UI-SPEC decision (`/gsd-ui-phase 20` runs before planning).
- "Never sent yet" is the explicit copy for a null `digest_last_sent_at`, matching the System view's existing "explicit copy, never blank" convention (D-06/`null` → "—" precedents from Phase 19).
- Toggle + dropdown both instant-apply; a failed `PUT` reverts the control to its last-known-good value with an inline error line, not a page-level error state (the rest of `/system` keeps working).

</specifics>

<deferred>
## Deferred Ideas

- Toast notification component — not introduced here; the SPA has no toast primitive today and D-03 chose inline status instead. Revisit only if a future phase needs transient notifications badly enough to justify the new primitive.
- A separate dedicated settings page — considered (D-01), rejected in favor of extending `/system`.
- Digest scheduling, mutual exclusion with real-time, grouping/chunking — all later phases (21-23), not this one.

### Reviewed Todos (not folded)
- *Move `shadcn` out of `web/package.json` dependencies* — tooling cleanup, unrelated to digest settings; weak keyword match only.
- *Resolve D-15 prev-release query files from `--prev-tag`* — backend `cmd/migration-check` tooling, unrelated.
- *Unify `internal/sqlscan`'s two quote state machines* — backend, unrelated.

</deferred>

---

*Phase: 20-digest-settings-operator-control*
*Context gathered: 2026-09-11*
*Amended: 2026-09-11 — D-05 (singleton enforcement) added and moved out of Claude's Discretion following a post-roadmap grilling session; see `20-DISCUSSION-LOG.md` for the audit trail.*
