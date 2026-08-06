---
phase: 02-watchlist-core
verified: 2026-08-06T19:30:00Z
status: human_needed
score: 43/43 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 32/32
  gaps_closed:
    - "G-02-2a (WR-01): UpsertArtist silently dropped disambiguation/image_url on re-add — closed by 02-05 (widened ON CONFLICT SET list, COALESCE on all three nullable metadata columns)"
    - "G-02-2b (WR-02): UpdatePreferences had an unhandled not-found race and a lost-update race under concurrent PATCH — closed by 02-06 (single data-modifying CTE, one round trip, pgx.ErrNoRows translated to ErrNotFound)"
  gaps_remaining: []
  regressions: []
human_verification:
  - test: "Decide the disposition of the two NEW Warning-level findings in the current 02-REVIEW.md (dated after 02-05/02-06 landed, commit 25c285c): new-WR-01 (Service.UpdatePreferences has no independent no-op guard — a direct, non-HTTP caller can send PreferencesParams{} and get a silent no-op 200 that only bumps updated_at) and new-WR-02 (handleAddWatchlist/handleUpdateWatchlist's json.Decoder.Decode never checks the stream is exhausted, so trailing garbage after a valid JSON body is silently ignored rather than rejected)."
    expected: "A recorded decision (accept as documented risk for v1, or open a follow-up plan) for each finding, the same pattern already used for the prior WR-01/WR-02 pair."
    why_human: "Both re-confirmed present by direct source inspection during this verification (internal/watchlist/service.go:216-263 has no equivalent of the handler's `if req.ReleaseTypes == nil && req.MutedEventTypes == nil` guard; internal/httpserver/watchlist.go:90-96 and :223-229 both `dec.Decode(&req)` once with no follow-up `dec.Decode(&struct{}{})` != io.EOF check). Neither breaks a declared must-have truth for this phase (no truth asserts domain-layer-only no-op rejection or trailing-data rejection), so this is not a phase must-have gap under the plan's own contract — but both are real, reproducible robustness gaps in Phase 2's own files that a human should knowingly accept or schedule, exactly as WR-01/WR-02 were before this cycle."
  - test: "Out-of-scope but noteworthy: decide whether to fast-follow CR-01 (Critical) from the same current 02-REVIEW.md — internal/db/migrate.go's redactError only strips the URL-form DSN pattern (scheme://user:pass@host), not a libpq keyword/value-form password=... fragment, so a raw Postgres password could still reach a log line or a returned error if a DSN-parse failure (not just a dial-refused failure) ever embeds the connection string verbatim."
    expected: "A recorded accept-or-fix decision, tracked separately from Phase 2 (this file was never touched by any 02-01..02-06 plan — `git log -- internal/db/migrate.go` shows only Phase 1 commits) since it does not affect Phase 2's watchlist goal, but flagged because it is Critical severity and directly implicates CLAUDE.md's 'all secrets via environment variables only... nothing real ever committed' constraint."
    why_human: "Confirmed present by direct source read of internal/db/migrate.go:107-109 (`redactError` still only applies `userInfoPattern`, no keyword/value password pattern). This is a Phase 1 file with zero Phase 2 commits against it and no Phase 2 must-have references it, so it does not block this phase's status determination — but an adversarial verification pass should not silently drop a Critical, unresolved, security-relevant finding just because it sits one file outside the phase boundary."
---

# Phase 2: Watchlist Core Verification Report

**Phase Goal:** "Users can fully manage their watchlist — add, remove, list, and configure per-artist alert preferences — through a tested API service layer."
**Verified:** 2026-08-06T19:30:00Z
**Status:** human_needed
**Re-verification:** Yes — after gap closure (plans 02-05, 02-06 landed since the prior VERIFICATION.md)

**Note on ROADMAP `Mode: mvp`:** This phase (and all seven phases in ROADMAP.md) is labeled `Mode: mvp`, but the goal is not in strict user-story form (`As a ..., I want to ..., so that ...`) — confirmed via `user-story.validate`, which returns `valid: false`. Phase 1's own VERIFICATION.md was previously produced in standard (non-MVP-flow) form under the same condition, so this re-verification follows that established precedent rather than the MVP-mode "User Flow Coverage" framing, which would otherwise require refusing to verify. Flagging this for awareness; it is a project-wide metadata/process discrepancy, not a Phase 2 code defect.

## Goal Achievement

### Observable Truths

All 32 must-have truths from plans 02-01 through 02-04 were re-verified against the current codebase (full build, `go vet`, a full `go test ./... -count=1` run against real Postgres — 81/81 tests pass, 0 fail), plus 11 new must-have truths from the two gap-closure plans (02-05, 02-06) that landed since the prior VERIFICATION.md.

| # | Plan | Truth | Status | Evidence |
|---|------|-------|--------|----------|
| 1-32 | 02-01..04 | All 32 previously-verified truths (add/duplicate/validation/list/ordering/delete/concurrency/patch/CHECK-backstop) | ✓ VERIFIED | Full suite re-run green (81/81 tests pass); no regression in any pre-existing `TestService_*`/`TestWatchlist_*` test — see full list in this phase's prior VERIFICATION.md, whose evidence lines were independently re-confirmed against current source rather than merely copied forward |
| 33 | 02-05 | Re-add with changed disambiguation/image_url updates the stored row; API response reflects it (G-02-2a) | ✓ VERIFIED | `TestService_Add_RefreshesArtistMetadataOnReAdd` PASS; `queries/artists.sql:14-19` widens the `ON CONFLICT` SET list |
| 34 | 02-05 | Re-add omitting disambiguation/image_url/deezer_id leaves stored value intact | ✓ VERIFIED | `TestService_Add_OmittedMetadataSurvivesReAdd` PASS |
| 35 | 02-05 | Artists master row reused, not duplicated, on re-add (D-03) | ✓ VERIFIED | Same test's `SELECT count(*) FROM artists WHERE mbid = $1` == 1 assertion |
| 36 | 02-05 | Committed sqlc output regenerated from edited source, no drift | ✓ VERIFIED | `sqlc generate` + `git diff --exit-code -- internal/db/sqlc/` clean (re-run directly in this verification) |
| 37 | 02-05 | No Go module added/removed/upgraded | ✓ VERIFIED | `git diff --exit-code -- go.mod go.sum` clean |
| 38 | 02-06 | PATCH for a row deleted mid-write returns 404, never 500 (G-02-2b) | ✓ VERIFIED | `TestService_UpdatePreferences_RowDeletedMidWriteReturnsErrNotFound` PASS (deterministic held-lock); `TestWatchlist_Patch_ConcurrentWithDeleteNeverReturns500` PASS (25-iteration end-to-end) |
| 39 | 02-06 | Two concurrent PATCH calls on different axes both take effect, neither reverts the other | ✓ VERIFIED | `TestService_UpdatePreferences_ConcurrentAxisWriteIsNotLost` PASS (deterministic held-lock); `TestWatchlist_Patch_ConcurrentDifferentAxesBothSurvive` PASS (25-iteration end-to-end) |
| 40 | 02-06 | Untouched axis resolved inside the same statement, one round trip, no separate unlocked read | ✓ VERIFIED | `grep -v '^\s*//' service.go \| grep -c 's.q.ListWatchlist(ctx)'` == 1 (only in `List`, not `UpdatePreferences`); `queries/watchlist.sql:38-56`'s single `WITH updated AS (UPDATE ...) SELECT ...` CTE |
| 41 | 02-06 | Every pre-existing preference behavior unchanged (partial update, empty-vs-omitted, dedup/canonical order, 400 on invalid value, 404 on unknown id) | ✓ VERIFIED | All pre-existing `TestService_UpdatePreferences_*` and `TestWatchlist_Patch_*` tests pass unedited in the full suite run |
| 42 | 02-06 | Committed sqlc output regenerated, no drift | ✓ VERIFIED | `sqlc generate` + `git diff --exit-code -- internal/db/sqlc/` clean |
| 43 | 02-06 | No Go module added/removed/upgraded | ✓ VERIFIED | `git diff --exit-code -- go.mod go.sum` clean |

**Score:** 43/43 declared must-have truths verified. 0 truths left behaviorally unverified.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `queries/artists.sql` | `UpsertArtist` with exhaustive `ON CONFLICT` SET list | ✓ VERIFIED | 3 real `COALESCE(EXCLUDED.<col>, artists.<col>)` clauses (deezer_id, disambiguation, image_url) + 1 in leading comment; `name` bare-assigned (NOT NULL) |
| `internal/db/sqlc/artists.sql.go`, `querier.go` | Regenerated codegen matching the widened query | ✓ VERIFIED | `sqlc generate` produces zero working-tree diff |
| `internal/watchlist/service_test.go` | Both halves of the COALESCE contract covered | ✓ VERIFIED | `TestService_Add_RefreshesArtistMetadataOnReAdd`, `TestService_Add_OmittedMetadataSurvivesReAdd` both present and passing |
| `queries/watchlist.sql` | `UpdateWatchlistPreferences` as a single data-modifying CTE | ✓ VERIFIED | Lines 38-56: `WITH updated AS (UPDATE ... CASE/ELSE ... RETURNING ...) SELECT ... JOIN artists` |
| `internal/db/sqlc/watchlist.sql.go`, `querier.go` | Regenerated with `SetReleaseTypes`/`SetMutedEventTypes bool` params, artist-joined row | ✓ VERIFIED | `sqlc generate` produces zero working-tree diff; `UpdateWatchlistPreferencesParams`/`Row` exports confirmed via passing tests that construct/consume them |
| `internal/watchlist/service.go` | One-round-trip `UpdatePreferences`, `pgx.ErrNoRows` → `ErrNotFound` | ✓ VERIFIED | Lines 216-263: single `s.q.UpdateWatchlistPreferences` call, `errors.Is(err, pgx.ErrNoRows)` branch present |
| `internal/httpserver/watchlist_test.go` | End-to-end concurrent-PATCH coverage | ✓ VERIFIED | `TestWatchlist_Patch_ConcurrentDifferentAxesBothSurvive`, `TestWatchlist_Patch_ConcurrentWithDeleteNeverReturns500` present, both passing |
| `internal/httpserver/watchlist.go` | Unchanged (404 comes from the service handing over the right sentinel) | ✓ VERIFIED | `git diff --exit-code -- internal/httpserver/watchlist.go` against pre-02-06 state was gated by the plan's own verify step and re-confirmed unmodified by 02-06 per its SUMMARY's key-files list |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| `queries/artists.sql` | `internal/db/sqlc/artists.sql.go` | `sqlc generate`, gated by clean diff | ✓ WIRED | Regenerated, zero drift |
| `internal/db/sqlc/artists.sql.go` | `internal/watchlist/service.go` | `Service.Add` calls `UpsertArtist`, builds `Entry` from its returned row | ✓ WIRED | `service.go:134-140`, `toEntry` |
| `queries/watchlist.sql` | `internal/watchlist/service.go` | `UpdatePreferences` calls `s.q.UpdateWatchlistPreferences` once | ✓ WIRED | `service.go:242` |
| `internal/watchlist/service.go` | `internal/httpserver/watchlist.go` | `pgx.ErrNoRows` → `ErrNotFound` → handler's existing `errors.Is` branch → 404 | ✓ WIRED | No handler edit needed or made; confirmed by `TestWatchlist_Patch_ConcurrentWithDeleteNeverReturns500`'s 404-body assertion passing |

### Behavioral Spot-Checks / Direct Execution

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full build | `go build ./...` | exit 0 | ✓ PASS |
| Static analysis | `go vet ./...` | exit 0, no output | ✓ PASS |
| Full test suite (real Postgres) | `TEST_DATABASE_URL=... go test ./... -count=1` | all packages `ok`, 81/81 tests pass, 0 fail | ✓ PASS |
| New gap-closure tests, named | `go test ./internal/watchlist/... -run 'TestService_Add_RefreshesArtistMetadataOnReAdd\|TestService_Add_OmittedMetadataSurvivesReAdd\|TestService_UpdatePreferences_ConcurrentAxisWriteIsNotLost\|TestService_UpdatePreferences_RowDeletedMidWriteReturnsErrNotFound' -v` | 4/4 PASS | ✓ PASS |
| New end-to-end concurrency tests, named | `go test ./internal/httpserver/... -run 'TestWatchlist_Patch_ConcurrentDifferentAxesBothSurvive\|TestWatchlist_Patch_ConcurrentWithDeleteNeverReturns500' -v` | 2/2 PASS | ✓ PASS |
| sqlc drift check | `sqlc generate && git diff --exit-code -- internal/db/sqlc/` | exit 0, no diff (only CRLF line-ending warnings) | ✓ PASS |
| `go mod verify` | `go mod verify` | "all modules verified" | ✓ PASS |
| No dependency drift | `git diff --exit-code -- go.mod go.sum` | exit 0, clean | ✓ PASS |
| No debt markers in phase files | `grep -nE 'TBD\|FIXME\|XXX\|TODO\|HACK\|PLACEHOLDER'` across `internal/watchlist/`, `internal/httpserver/` | no matches | ✓ PASS |
| Untouched-axis single-round-trip invariant | `grep -v '^\s*//' service.go \| grep -c 's.q.ListWatchlist(ctx)'` | 1 (in `List` only) | ✓ PASS |
| Working tree clean at HEAD | `git status --porcelain` | empty | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| WLST-02 | 02-01, 02-02, 02-05 | User can add an artist to the watchlist from search results | ✓ SATISFIED | `POST /watchlist` implemented, tested, D-08/D-09 both proven, re-add metadata refresh now correct |
| WLST-03 | 02-03 | User can remove an artist from the watchlist | ✓ SATISFIED | `DELETE /watchlist/{id}` hard delete, 404/400 branches, concurrency-safe |
| WLST-04 | 02-03 | User can list all artists currently on the watchlist | ✓ SATISFIED | `GET /watchlist` joined, ordered, `[]`-safe |
| WLST-05 | 02-02, 02-04, 02-06 | User can set per-artist release-type filters | ✓ SATISFIED | Set on add and via `PATCH`; PATCH now concurrency-safe and 404-honest |
| WLST-06 | 02-02, 02-04, 02-06 | User can set per-artist notification/mute preferences | ✓ SATISFIED | Same as WLST-05, mute axis |

`.planning/REQUIREMENTS.md` traceability table maps WLST-02 through WLST-06 to Phase 2, all `[x]`/Complete (lines 13-17, 109-113). No orphaned requirements — all five appear in at least one plan's `requirements:` frontmatter, including the two gap-closure plans (02-05: WLST-02; 02-06: WLST-05, WLST-06).

### Anti-Patterns Found

None in phase-modified files. Scanned `internal/watchlist/`, `internal/httpserver/`, `queries/artists.sql`, `queries/watchlist.sql` for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` and common stub phrases — zero matches.

### Code Review Findings Carried Forward (current 02-REVIEW.md, commit `25c285c`, post-gap-closure)

The code review artifact was regenerated after 02-05/02-06 landed (this is a *different, later* review than the one the prior VERIFICATION.md referenced — confirmed via `git log -- 02-REVIEW.md`, which shows two commits: `3516368` pre-gap-closure and `25c285c` post-gap-closure, currently at HEAD).

**Previously-open findings, now confirmed resolved by direct re-inspection:**
- **Old WR-01 (UpsertArtist drops disambiguation/image_url on re-add):** RESOLVED — `queries/artists.sql`'s SET list now covers all three nullable metadata columns.
- **Old WR-02 (UpdatePreferences not-found race + lost-update race):** RESOLVED — single-CTE rewrite, deterministic held-lock tests passing.

**New findings from the post-gap-closure review pass, not yet addressed:**

1. **CR-01 (Critical) — `internal/db/migrate.go`'s `redactError` only strips URL-form DSN userinfo, not libpq keyword/value-form `password=...`.** Confirmed by direct read of `migrate.go:107-109`: `redactError` applies only `userInfoPattern` (`scheme://user:pass@`), with no pattern for `password=\S+`. **This file was never touched by any Phase 2 plan** (`git log -- internal/db/migrate.go` shows only Phase 1 commits) and no Phase 2 must-have references it, so it is out of this phase's scope and does not affect the status determination below — but it is Critical severity, security-relevant (CLAUDE.md's "all secrets via environment variables only... nothing real ever committed"), and currently unresolved at HEAD. Recorded as a human-decision item for visibility, not as a Phase 2 gap.

2. **New WR-01 (Warning) — `Service.UpdatePreferences` has no independent no-op guard.** Confirmed at `service.go:216-263`: the "at least one axis supplied" check exists only in `handleUpdateWatchlist`, not in the domain-layer `Service.UpdatePreferences` itself. A non-HTTP caller (a future admin tool, Phase 3+ scheduler, or test) invoking `UpdatePreferences(ctx, id, PreferencesParams{})` directly gets a silent success that only bumps `updated_at`. Not tied to any declared must-have truth for this phase.

3. **New WR-02 (Warning) — decoders accept trailing data after a valid JSON body.** Confirmed at `watchlist.go:90-96` (`handleAddWatchlist`) and `:223-229` (`handleUpdateWatchlist`): `dec.Decode(&req)` is never followed by a stream-exhaustion check, so `{"mbid":"x","name":"y"}{"garbage":true}` decodes and processes as a normal valid request. Not tied to any declared must-have truth for this phase.

4. **IN-01/IN-02 (Info):** redundant double panic recovery in the middleware stack; `handleUpdateWatchlist` lacks the same fail-fast allow-list pre-check `handleAddWatchlist` has (functionally harmless — `Service.UpdatePreferences`'s `normalizeSet` still validates before any DB call). Neither is a defect; both are consistency notes.

## Human Verification Required

### 1. Decide disposition of new-WR-01 and new-WR-02 (in-scope, Warning-severity, code review findings)

**Test:** Review the current `02-REVIEW.md`'s WR-01 (no domain-layer no-op guard on `UpdatePreferences`) and WR-02 (JSON decoders accept trailing data) and decide whether either needs a fast-follow fix before Phase 3/4 build additional callers of `internal/watchlist`, or is an accepted risk for v1.
**Expected:** A recorded decision, following the same pattern already used for the prior WR-01/WR-02 pair (which resulted in plans 02-05/02-06).
**Why human:** Both are real, reproducible gaps confirmed by direct source inspection, but neither breaks a declared must-have truth or a WLST-* requirement as literally written, so it is a scoping/priority call, not something a verifier can resolve unilaterally.

### 2. Out-of-scope flag: CR-01 (Critical, Phase 1 file, currently unresolved)

**Test:** Review CR-01 in the current `02-REVIEW.md` — `internal/db/migrate.go`'s `redactError` does not scrub libpq keyword/value-form passwords, only URL-form userinfo — and decide whether to open a fast-follow fix (likely scoped to Phase 1, or a small cross-cutting hardening plan) before it's forgotten.
**Expected:** A recorded accept-or-fix decision; if fixed, it should land as its own plan since it touches no Phase 2 file.
**Why human:** Confirmed present at HEAD by direct source read. Does not block Phase 2 (file untouched by any 02-01..02-06 plan, no Phase 2 must-have references it), but is Critical severity and directly implicates a hard project security constraint (CLAUDE.md), so it should not be silently dropped just because it falls one file outside this phase's boundary.

### Gaps Summary

No gaps against Phase 2's own declared must-haves. All 43 must-have truths (32 original + 5 from 02-05 + 6 from 02-06) are independently confirmed against the current codebase — a real, single full-suite test run against Postgres (81/81 tests pass), the four new gap-closure tests run by name, `go build`/`go vet`/`sqlc generate`-diff/`go mod verify`, and direct source inspection of every referenced code path. No stub, no orphaned requirement, no debt marker in phase-modified files, and no regression in any of the 32 previously-verified truths.

The two gaps flagged in the prior verification cycle (G-02-2a / old WR-01, G-02-2b / old WR-02) are both closed, with deterministic tests proving the fixes rather than merely asserting they were made.

The `human_needed` status reflects two fresh in-scope Warning-level findings from the post-gap-closure code review (not gaps against this phase's own contract, but real robustness gaps worth an explicit accept-or-fix call) plus one out-of-scope Critical finding surfaced for visibility rather than silently dropped.

---

_Verified: 2026-08-06T19:30:00Z_
_Verifier: Claude (gsd-verifier)_
