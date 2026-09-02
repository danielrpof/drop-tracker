# Roadmap: drop-tracker

## Overview

drop-tracker starts from an empty repo and builds outward from the data layer: a Postgres schema, config, and health-checked service skeleton first, then a fully tested watchlist CRUD API, then rate-limited MusicBrainz/Deezer clients with live search, then the detection engine that diffs poll results into new-release/guest-feature/deluxe events, then Discord notifications for those events, then the React UI that ties watchlist management and release history together, and finally the single-image containerization and full GitHub Actions CI/CD pipeline (lint, test, security scan, SBOM, semantic versioning, ghcr.io publish) that is the actual point of the project. Each phase produces something a user (or operator) can directly observe working before the next phase builds on it.

v1.1 picked up from a shipped, working v1.0 and closed four peer-reviewed gaps without changing what the app does for its user: the React frontend gained the component test suite it never had, the Full Pipeline started enforcing coverage floors on both languages instead of merely running tests, the events table gained a retention window that hides stale history from display while leaving every detection-critical row in place, and the poller stopped walking the watchlist one artist at a time. v1.2 then closed the two items left in the backlog plus a round of History-tab display bugs found in everyday use — search popularity ranking, artist-art backfill, and missing release dates/album art on History cards — without adding new capability.

**v1.3 Continuous Deployment** takes the last step the pipeline was always pointed at: the published image stops being the end of the line and starts being deployed. Because a deployed instance is a *public* instance, the milestone is sequenced defensively rather than by excitement — the passphrase gate that protects the watchlist and the Discord webhook lands first, so the instance is never briefly public-and-open (with a VPS firewall allowlist during provisioning as belt-and-suspenders — the app gate is the load-bearing control, not the only one); the PR coverage-diff comment (a self-contained CI reporting gap) lands second; the N-1 schema-compatibility check that makes image rollback actually safe lands third, *before* anything can auto-roll-back; and only then does the SSH deploy job with health-gated auto-rollback go live on a provisioned, HTTPS-fronted VPS.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-7 (shipped 2026-08-12)
- ✅ **v1.1 Hardening & Scale Readiness** — Phases 8-11.1 (shipped 2026-08-17)
- ✅ **v1.2 Cleanup & Display Fixes** — Phases 12-13 (shipped 2026-08-24)
- 🚧 **v1.3 Continuous Deployment** — Phases 14-17 (in progress)

Full phase-by-phase detail for the three shipped milestones is archived at `.planning/milestones/v1.0-ROADMAP.md`, `.planning/milestones/v1.1-ROADMAP.md`, and `.planning/milestones/v1.2-ROADMAP.md`. Accomplishment summaries: `.planning/MILESTONES.md`.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

<details>
<summary>✅ v1.0 MVP (Phases 1-7) — SHIPPED 2026-08-12</summary>

- [x] **Phase 1: Foundation — Data Layer, Config & Health** - Postgres schema/migrations, sqlc, env-based config, structured logging, and a `/health` endpoint the rest of the app is built on (completed 2026-08-05)
- [x] **Phase 2: Watchlist Core** - Users can add, remove, list, and configure per-artist alert preferences through a tested watchlist API (completed 2026-08-06)
- [x] **Phase 3: External Clients & Search** - Rate-limited MusicBrainz/Deezer clients power a live search-proxy and scheduled polling (completed 2026-08-07)
- [x] **Phase 4: Detection Engine** - Poll results are diffed against a "seen" store to reliably detect new releases, guest features, and deluxe/tracklist changes without duplicates or overlapping runs (completed 2026-08-08)
- [x] **Phase 5: Discord Notifications** - Detected events are posted to Discord with distinct formatting per event type, honoring mute preferences (completed 2026-08-08)
- [x] **Phase 6: Frontend & Release History** - Users manage their watchlist and browse detected release history entirely through a web UI (completed 2026-08-11)
- [x] **Phase 7: Containerization & CI/CD Pipeline** - The app ships as a single scanned, versioned, non-root Docker image via an automated GitHub Actions pipeline, with docker-compose for local dev (completed 2026-08-12)

</details>

<details>
<summary>✅ v1.1 Hardening & Scale Readiness (Phases 8-11.1) — SHIPPED 2026-08-17</summary>

- [x] **Phase 8: Frontend Test Suite** - The watchlist, search, and history React surfaces get a Vitest + React Testing Library suite that mocks the app's own API boundary (completed 2026-08-12)
- [x] **Phase 9: CI Coverage Gates** - The Full Pipeline blocks the build when Go coverage drops below 80% or frontend coverage drops below 70% (completed 2026-08-13)
- [x] **Phase 10: Event Retention Window** - History and API hide events older than a configurable window (default 90 days) while every row and all detection state stay intact (completed 2026-08-13)
- [x] **Phase 11: Bounded Concurrent Polling** - Each source polls several artists at a time through a bounded worker pool, without breaking rate limits, overlap guards, or baseline correctness (completed 2026-08-17)
- [x] **Phase 11.1: Address tech debt: v1.1 cleanup (INSERTED)** - Closed the milestone audit's non-blocking tech debt: frontend coverage gaps, a real History filter accessibility bug, a Prettier CI gate, notification-loss observability, and Nyquist validation reconciliation (completed 2026-08-17)

</details>

<details>
<summary>✅ v1.2 Cleanup & Display Fixes (Phases 12-13) — SHIPPED 2026-08-24</summary>

- [x] **Phase 12: Cleanup: CoverArt Reset & Search Popularity Ranking** - Fixed the shared `CoverArt` component's stale-placeholder bug and added Deezer-fan-count popularity ranking plus a MusicBrainz country-code disambiguation fallback to artist search (completed 2026-08-19)
- [x] **Phase 13: Fix History Dates, Guest-Feature Art & Artist Art** - History cards now show release dates and guest-feature album art, and MusicBrainz artists get real artist art via a fail-closed MusicBrainz→Deezer matcher wired into add-time and a startup backfill sweep (completed 2026-08-24)

</details>

**v1.3 Continuous Deployment**

- [x] **Phase 14: Instance Passphrase Gate** - A configured passphrase puts the watchlist API and Discord-backed actions behind a signed session cookie, with the SPA prompting for it — and stays completely inert when unconfigured (completed 2026-09-01)
- [ ] **Phase 15: PR Coverage-Diff Comment** - Every same-repo PR carries one always-current, never-blocking comment showing backend and frontend coverage and their delta versus the main baseline
- [ ] **Phase 16: Rollback-Safe Migrations** - CI proves the previously-released image still boots healthy against the current branch's schema, and the expand/contract rule becomes a documented standing constraint
- [ ] **Phase 17: Automated VPS Deploy with Health-Gated Rollback** - Merging to main ships the released image to a provisioned HTTPS VPS unattended, and a bad release restores the previous one by itself

## Phase Details

### Phase 14: Instance Passphrase Gate

**Goal**: A drop-tracker instance on a public URL only exposes its watchlist data, search proxy, and event history to someone who knows the instance passphrase — while local dev, docker-compose, and the existing test suites keep working with no passphrase configured at all. This must land before anything in this milestone makes the app publicly reachable, so the instance is never briefly public-and-open.
**Depends on**: Phase 13 (v1.2 close). Independent of Phases 15 and 16.
**Requirements**: GATE-01, GATE-02, GATE-03, GATE-04, GATE-05, GATE-06, GATE-07
**Success Criteria** (what must be TRUE):

1. With a passphrase configured, an unauthenticated request to any data route (`/search`, `/watchlist`, `/events`) comes back `401`, while `GET /health` (exact path only), the session-login endpoint, and the SPA shell + static bundle still load unauthenticated.
2. Opening the instance in a browser shows a passphrase form — not a blank screen, spinner, or generic error — and submitting the correct passphrase once restores the full watchlist and history UI; a wrong passphrase does not.
3. That browser is still authenticated after the container is restarted or redeployed, and choosing log out immediately returns it to the passphrase form.
4. The session cookie carries `HttpOnly`, `Secure`, `SameSite=Lax` and a bounded lifetime when inspected in devtools, and repeated wrong-passphrase attempts from one client are throttled (rejected with `429`) rather than answered at full speed indefinitely.
5. With no passphrase configured, every route behaves exactly as it did in v1.2 — `make test-integration`, `pnpm test`, and `docker compose up` all pass untouched with no passphrase anywhere.

**Plans**: 7/7 plans executed (4 phase plans + 3 gap-closure plans)

Plans:
**Wave 1**

- [x] 14-01-PLAN.md — Tracer: end-to-end 401 → login → 200 gate slice, inert path, and the full session-cookie contract (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 14-02-PLAN.md — Brute-force defense: per-IP throttle, fixed delay, global counter with Discord alert, and auth audit logging (wave 2)
- [x] 14-03-PLAN.md — SPA gate: auth store, `apiFetch` 401 interceptor, PassphraseScreen, and the Log out control (wave 2)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 14-04-PLAN.md — Hardening: CSRF header enforcement, no-referrer policy, weak-passphrase boot WARN, and `.env.example` (wave 3)

**Gap closure** *(UAT G-14-1 — gate never engaged: container booted with an empty `INSTANCE_PASSPHRASE`)*

- [x] 14-05-PLAN.md — Close G-14-1: compose host-shell pass-through for the gate env vars, a secret-free boot log line reporting gate active/inert, a UAT Test 1 configuration precondition, and the operator `.env` reconciliation checkpoint

**Gap closure** *(UAT G-14-2 — the Log out control disappears after a page reload while still logged in)*

- [x] 14-06-PLAN.md — Close G-14-2: back `gateActive` with `sessionStorage` so the Log out control survives a document reload for the browser session (D-18), with guarded access for private-mode and the SPA-mode Node prerender, plus a reload regression test and a UAT Test 5 precondition

**Gap closure** *(UAT G-14-3 — the Log out control never appears at all on a fresh session that already holds a valid `dt_session` cookie: the SPA has no gated-load signal except a 401 or a typed login)*

- [x] 14-07-PLAN.md — Close G-14-3: `gate.Authenticate` marks every response that passes the gate with a fixed header so a gated 2xx is self-identifying, `apiFetch` latches it into a new `authStore.markGateActive()` that never touches `authed` and never clears, plus the WR-01 storage-guard hardening and a UAT Test 5 re-run sequence that actually reproduces the defect

**UI hint**: yes

_Notes:_ Joint backend + frontend slice — a Go `internal/authgate` package plus chi middleware on a protected route `Group`, and the SPA's `401`-interception login form and logout. Splitting the server contract from the SPA handling would leave a half-phase that renders a broken app. All discuss/spec decisions are resolved in `14-CONTEXT.md` (D-01…D-18): derived signing key, hardcoded 30d/90d timings, public SPA shell, warn-only passphrase strength, `gateActive`-gated Log out (D-18). Adds **two** env vars: `INSTANCE_PASSPHRASE` (the gate) and `TRUST_PROXY_HEADERS` (default `false`; gates `middleware.RealIP` so a pre-proxy or misconfigured deploy can't be `X-Forwarded-For`-spoofed — D-14). Rotating `INSTANCE_PASSPHRASE` is the only revoke-all lever (D-10) — Phase 17's runbook documents it. Login hardening: undelayed 429, `maxConcurrentLogins` semaphore, alert-only global counter (D-12). Relevant pitfalls: PITFALLS.md 14-23. **Validation contract (`14-VALIDATION.md`) content is authored but the formal `/gsd-validate-phase 14` step is still pending — run it before execution.**

### Phase 15: PR Coverage-Diff Comment

**Goal**: Every pull request from a same-repo branch carries a single, always-current comment showing what it does to backend and frontend coverage relative to main — closing the last CI reporting gap without ever becoming a new merge blocker.
**Depends on**: Phase 13 (v1.2 close). Independent of Phase 14; scheduled before Phases 16 and 17 because all three edit `.github/workflows/full-pipeline.yml`.
**Requirements**: CICD-13, CICD-14
**Success Criteria** (what must be TRUE):

1. Opening a PR from a same-repo branch produces exactly one comment reporting backend and frontend coverage totals alongside each one's delta versus the main-branch baseline.
2. Pushing further commits to that PR edits the same comment in place — three pushes leave one comment, not three.
3. When no baseline is available (first run after the feature ships, or an evicted cache entry), the comment still appears with absolute coverage numbers and says the delta is unavailable, instead of erroring or reporting a nonsense delta.
4. A PR whose coverage drops still shows that drop in the comment and stays mergeable — only the pre-existing 80%/70% coverage gates can block it.
5. A merge to main publishes that run's coverage as the baseline the next PR diffs against, and no PR run recomputes main's coverage.

**Plans**: 3/3 plans executed

Plans:
**Wave 1**

- [x] 15-01-PLAN.md — `cmd/coverage-report` Go tool: profile/summary/sidecar parsing, delta math, markdown renderer, `--mode=total|sidecar|comment`, plus the `gosec` carve-out (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 15-02-PLAN.md — Makefile wiring: `COVER_PKGS` exclusion, `make coverage-report`, and the D-17 `coverage-gate` cutover with a measured margin check (wave 2)
- [x] 15-03-PLAN.md — CI wiring: Vitest `json-summary` reporter, artifact hand-off, main-branch cache baseline, and the report-only `coverage-comment` job (wave 2)

_Notes:_ CI-only, report-only, low risk. Must not join any release-path `needs:` graph — `needs: [test, frontend-test]` and nothing else. `pull-requests: write` is scoped to this job only, never workflow-wide. Same-repo branches only; fork PRs degrade to the job summary. `pull_request_target` is prohibited outright. Open decision for discuss/spec: baseline storage (Actions cache with `restore-keys` vs. an orphan `coverage-baseline` branch). Relevant pitfalls: PITFALLS.md 24-27.

### Phase 16: Rollback-Safe Migrations

**Goal**: Rolling the app image back to its previous release is *provably* safe against the schema the newer release already applied — enforced by a CI check and a written rule, not by remembering. This lands before the deploy job so the discipline is in force from the first v1.3 migration rather than becoming a latent outage the moment auto-rollback exists.
**Depends on**: Phase 13 (v1.2 close). Independent of Phases 14 and 15; scheduled after 15 because both edit `.github/workflows/full-pipeline.yml`.
**Requirements**: MGRT-01, MGRT-02
**Success Criteria** (what must be TRUE):

1. A CI check boots the previously-released image against a database migrated to the current branch's schema and passes only if that older binary starts and stays healthy.
2. A branch carrying a destructive migration (`DROP COLUMN`, `RENAME`, type narrowing, or `ADD COLUMN ... NOT NULL` with no default) turns that check red, with a failure message that names the N-1 compatibility rule rather than just reporting an opaque boot error.
3. The expand/contract rule — additive-only per release, destructive changes split across releases, no blocking DDL in boot migrations — is documented as a standing constraint where someone writing a migration will actually encounter it.
4. The older binary's boot migration succeeds against an ahead-of-source schema (it no-ops rather than failing on a migration version it has never heard of), proven by an automated test rather than assumed — closing the research's one MEDIUM-confidence assumption about golang-migrate's `Up()` behavior.

**Plans**: TBD

_Notes:_ Split out of the deploy phase deliberately. Pitfall 8 is the milestone's highest-cost failure mode (a non-backward-compatible migration turns a routine rollback into data loss), and it is *cross-cutting* — the rule binds every migration from now on, not just ones written during the deploy phase. Building it last inside the heaviest phase would put the safety precondition after the thing it protects. CI-only, no VPS required. Relevant pitfalls: PITFALLS.md 8, 9, 10.

### Phase 17: Automated VPS Deploy with Health-Gated Rollback

**Goal**: Merging to main puts that exact released version live on a public HTTPS URL within minutes with no human action, and a bad release restores the previous one by itself — the deployment step the whole pipeline was built pointing at.
**Depends on**: Phase 14 (the gate must be merged before the instance is publicly reachable), Phase 16 (rollback must be provably safe before anything auto-rolls-back), and Phase 15 (workflow edits land first to keep the shared `full-pipeline.yml` manageable).
**Requirements**: DPLY-01, DPLY-02, DPLY-03, DPLY-04, DPLY-05, DPLY-06, DPLY-07, DPLY-08
**Success Criteria** (what must be TRUE):

1. Following the checked-in provisioning runbook/wizard from a fresh VPS produces a running instance reachable over HTTPS at a public URL that asks for the passphrase — with the production compose file and rollout script versioned in the repo, and the real `.env` plus the pinned image tag existing only on the box.
2. Merging a PR to main results, with no human action, in the newly released ghcr.io tag running on the VPS, confirmed by that version being live at the public URL.
3. Deliberately releasing a known-bad image (one whose `/health` never comes up) results in the previous version automatically running healthy again and the workflow run ending red — verified as a real drill against the live box, not by reading the rollback script.
4. The deploy job does not run on pull requests or on forks, and two merges landing minutes apart deploy strictly one after the other rather than overlapping or cancelling each other.
5. A deploy against a host presenting an unexpected SSH host key fails instead of connecting, and the workflow run's logs contain no passphrase, DSN, or Discord webhook URL.

**Plans**: TBD

_Notes:_ The heaviest phase of the milestone — 17 of PITFALLS.md's 27 catalogued pitfalls land here. Requires a small change to the existing `release` job to expose `outputs.version`. The deploy job needs `needs: [release]` + a `push`/`refs/heads/main` guard + its own `concurrency` group with `cancel-in-progress: false` + a `production` GitHub Environment holding the SSH secrets. Rollback must target the previous image **pinned locally on the VPS**, never re-pulled from ghcr.io. The `/health` poll runs inside the SSH session against `127.0.0.1`, not from the runner. **Needs a discuss/spec pass before planning** — decisions to lock: whether the accepted brief swap-gap downtime is documented-and-accepted or mitigated, and post-deploy image-prune policy.

**Provisioning runbook (DPLY-07) must now also cover** (added during the Phase 14 plan-hardening review):

- A **VPS firewall allowlist** in front of the instance during provisioning, so it is never both publicly reachable *and* pre-gate/pre-proxy — belt-and-suspenders alongside the passphrase gate.
- Set `TRUST_PROXY_HEADERS=true` on the box **only after** the reverse proxy is confirmed live and the container port is unpublished (D-14). It stays unset/false at every earlier step.
- The **passphrase-rotation = revoke-all** procedure (D-10): rotate `INSTANCE_PASSPHRASE`, redeploy — every session invalidated.
- A **Postgres backup + restore procedure** (folds in the basics of OPS-04, previously Future): it is the recovery path when an image rollback also needs the schema restored (PITFALLS #8, the milestone's highest-cost failure). Schedule this as part of Phase 16 or 17 work, not "someday".

**TLS reverse-proxy choice — recommended: Caddy on-VPS** over Cloudflare Tunnel. Caddy keeps the "self-hosted, minimal external dependency" ethos and puts no third party in the request path; automatic HTTPS via ACME; health-gated upstream cutover is available if DPLY-09 (near-zero-downtime) is ever pursued. Confirm at the Phase 17 discuss. This choice blocks the first deploy, so lock it early — before Phase 15 planning.

Relevant pitfalls: PITFALLS.md 1-13, 19, 22.

## Progress

**Execution Order:**
Phases execute in numeric order: 14 → 15 → 16 → 17

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 14. Instance Passphrase Gate | 7/7 | Complete    | 2026-09-01 |
| 15. PR Coverage-Diff Comment | 3/3 | In Progress|  |
| 16. Rollback-Safe Migrations | 0/? | Not started | - |
| 17. Automated VPS Deploy with Health-Gated Rollback | 0/? | Not started | - |

## Backlog

*(none currently)*
