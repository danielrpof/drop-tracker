---
status: diagnosed
trigger: "UAT gap G-02-1 (02-UAT.md Test 1): user decision on WR-01/WR-02 (Warning-severity, 02-REVIEW.md) is 'fix' rather than 'accept as documented risk'. Investigate to confirm both findings still hold against current code before a gap-closure plan is written."
created: 2026-08-06T18:17:35Z
updated: 2026-08-06T18:17:35Z
---

## Current Focus

hypothesis: CONFIRMED (both). WR-01: Service.UpdatePreferences has no independent no-op guard, relying solely on the HTTP handler's check. WR-02: both JSON decoders in internal/httpserver/watchlist.go accept trailing data after a valid JSON value because dec.Decode is never followed by an EOF-exhaustion check.
test: Read internal/watchlist/service.go and internal/httpserver/watchlist.go at current line numbers, compare against 02-REVIEW.md's WR-01/WR-02 citations.
expecting: Line ranges and mechanism unchanged since review (2026-08-06, same day) — confirmed exactly.
next_action: none — goal is find_root_cause_only; return ROOT CAUSE FOUND to orchestrator. A downstream gap-closure plan should implement the fixes described in Resolution.fix.

## Symptoms

expected: A recorded decision (accept as documented risk for v1, or open a follow-up plan) for each of WR-01 and WR-02 — the same disposition pattern already applied to the prior (now-fixed) WR-01/WR-02 pair from an earlier review pass.
actual: User's UAT response was "fix" for both — i.e. the decision is to close both gaps with a fast-follow code fix rather than accept them as documented risk.
errors: None — these are static code-review findings (Warning severity), not runtime failures or test failures. No stack trace, no failing test to reproduce.
reproduction: |
  WR-01 (service-layer no-op guard): call watchlist.Service.UpdatePreferences(ctx, id, watchlist.PreferencesParams{}) directly (bypassing the HTTP layer) against an existing watchlist row -- e.g. from a unit test constructing *Service directly, or a hypothetical future non-HTTP caller (admin tool, Phase 3+ scheduler). This issues UpdateWatchlistPreferences with SetReleaseTypes=false, SetMutedEventTypes=false, silently bumps updated_at, and returns success (Entry, nil) with a duplicate/no-op change, rather than a rejection.
  WR-02 (JSON trailing-data): POST /watchlist or PATCH /watchlist/{id} with a body like {"mbid":"x","name":"y"}{"anything":"else"} (two concatenated top-level JSON values). json.Decoder.Decode(&req) reads only the first value and returns nil error; the second value is silently discarded and the request is processed as a normal, valid request instead of being rejected with 400.
started: Found during the phase 02 fresh full code review (02-REVIEW.md, reviewed 2026-08-06T00:00:00Z) covering the complete Phase 02 diff including gap-closure plans 02-05/02-06.

## Eliminated

(none — investigation confirmed the pre-existing diagnosis directly; no alternative hypotheses were needed)

## Evidence

- timestamp: 2026-08-06T18:17:35Z
  checked: internal/watchlist/service.go, full file (338 lines), current working tree (git status clean, HEAD 18f51e1)
  found: |
    Service.UpdatePreferences spans lines 216-263. Lines 216-224 build sqlc.UpdateWatchlistPreferencesParams with SetReleaseTypes/SetMutedEventTypes both defaulted false. Lines 225-232 set SetReleaseTypes=true only if p.ReleaseTypes != nil; lines 233-240 do the same for MutedEventTypes. There is no check anywhere in the method (nor in Add's sibling validation block, nor in normalizeSet) that rejects the case where BOTH p.ReleaseTypes == nil AND p.MutedEventTypes == nil. If both are nil, params keeps SetReleaseTypes=false, SetMutedEventTypes=false and control falls straight through to s.q.UpdateWatchlistPreferences(ctx, params) at line 242 -- a real DB round trip that (per 02-REVIEW's confirmed CTE rewrite from gap G-02-2b) still executes an UPDATE ... SET updated_at = now() WHERE id = $id and returns success.
  implication: WR-01 is confirmed exactly as described in 02-REVIEW.md. Line numbers match the review's citation (216-241 falls inside the actual 216-263 method body, covering the validation block precisely). Contract enforcement ("reject a body/call supplying neither key") exists only one layer up, in the HTTP handler.

- timestamp: 2026-08-06T18:17:35Z
  checked: internal/httpserver/watchlist.go, full file (285 lines), current working tree
  found: |
    handleUpdateWatchlist (lines 212-258) does perform the guard: lines 231-234 check `if req.ReleaseTypes == nil && req.MutedEventTypes == nil { writeError(..., "no preferences supplied"); return }` immediately after decode, before calling s.watchlist.UpdatePreferences. This confirms the guard exists ONLY at the HTTP boundary, not in the Store/Service interface that Store's own doc comment (service.go:79-82) says other phases are meant to build on ("the reusable API surface").
  implication: Confirms the review's framing -- a non-HTTP caller of watchlist.Store/Service (unit test constructing *Service directly, future admin tool, Phase 3+ scheduler) has no protection against the no-op-update contract violation. This is a genuine domain-boundary gap, not merely redundant defense-in-depth.

- timestamp: 2026-08-06T18:17:35Z
  checked: internal/httpserver/watchlist.go lines 84-172 (handleAddWatchlist) and 212-258 (handleUpdateWatchlist), specifically the decode blocks at 90-96 and 223-229
  found: |
    handleAddWatchlist, lines 90-96:
      var req addWatchlistRequest
      dec := json.NewDecoder(r.Body)
      dec.DisallowUnknownFields()
      if err := dec.Decode(&req); err != nil {
          writeError(w, http.StatusBadRequest, "invalid request body")
          return
      }
    handleUpdateWatchlist, lines 223-229 (identical pattern):
      var req updateWatchlistRequest
      dec := json.NewDecoder(r.Body)
      dec.DisallowUnknownFields()
      if err := dec.Decode(&req); err != nil {
          writeError(w, http.StatusBadRequest, "invalid request body")
          return
      }
    Both blocks call dec.Decode exactly once and never call it again to check for a second value / EOF. json.Decoder.Decode's documented behavior is to decode one JSON value per call and leave the stream positioned after it -- it does not verify end-of-stream. DisallowUnknownFields only rejects unknown *keys within* the decoded object; it has no effect on trailing top-level values after the object closes.
  implication: WR-02 is confirmed exactly as described in 02-REVIEW.md, in both cited locations, with line numbers matching precisely (90-96 and 223-229). A request body with a second concatenated JSON value after the first well-formed object will decode successfully and be processed normally; the trailing bytes are silently dropped. This applies to both POST /watchlist and PATCH /watchlist/{id}, i.e. every JSON-body-accepting handler in this file (handleListWatchlist and handleRemoveWatchlist take no body).

- timestamp: 2026-08-06T18:17:35Z
  checked: 02-REVIEW.md frontmatter/findings summary and .planning/STATE.md decision log
  found: |
    02-REVIEW.md records this as a fresh full re-review (not incremental) that independently reconfirmed the PRIOR WR-01/WR-02 pair (UpsertArtist COALESCE gap; UpdatePreferences race conditions) as fixed via gap-closure plans 02-05/02-06 (see STATE.md decision log entries for [Phase 02-05] and [Phase 02-06]). The CURRENT WR-01/WR-02 pair under investigation here is a distinct, newer pair of Warning findings from that same fresh review pass -- not a re-opening of the already-closed prior pair. 02-UAT.md Test 1's "expected" field explicitly references "the same pattern already used for the prior WR-01/WR-02 pair" (i.e. gap-closure-plan-or-accept), confirming this is understood project-wide as a new, separate pair needing its own disposition.
  implication: No naming collision confusion -- this is a clean net-new pair of findings requiring a net-new gap-closure plan, following the established 02-05/02-06 precedent (small, targeted plans, each closing exactly one WR with a fix + regression test).

## Resolution

root_cause: |
  WR-01 (code, domain-boundary gap): internal/watchlist/service.go's Service.UpdatePreferences (lines 216-263) validates and applies each preference axis independently but never checks the case where both PreferencesParams.ReleaseTypes and PreferencesParams.MutedEventTypes are nil. The WLST-05/06 contract "reject a call that supplies neither axis" is enforced exclusively by internal/httpserver/watchlist.go's handleUpdateWatchlist (lines 231-234), one layer above the domain boundary that watchlist.Store/Service is documented (service.go:79-82) to be the reusable surface for. Any caller that reaches Service.UpdatePreferences directly bypasses this check entirely.

  WR-02 (code, missing-validation gap): internal/httpserver/watchlist.go's handleAddWatchlist (decode block at lines 90-96) and handleUpdateWatchlist (decode block at lines 223-229) both call json.Decoder.Decode exactly once and treat a nil error as "the entire body is valid." json.Decoder.Decode only consumes a single JSON value per call and does not assert the stream is exhausted afterward -- this is documented, well-known encoding/json behavior, not a version-specific bug. DisallowUnknownFields guards against unexpected *keys*, not trailing *values* after the first one. A body containing two concatenated JSON values is accepted with the second silently discarded.

  Both are confirmed unchanged from 02-REVIEW.md's citations -- exact line-range match, same mechanism, same files. No fix has been applied yet (this session is diagnose-only per orchestrator instruction).
fix: |
  NOT APPLIED (goal: find_root_cause_only -- this session is diagnosis/confirmation only; a downstream gap-closure plan implements the fix). Directions suggested by 02-REVIEW.md and independently confirmed sound during this investigation:

  WR-01: Add a guard at the top of Service.UpdatePreferences, before any DB call:
    if p.ReleaseTypes == nil && p.MutedEventTypes == nil {
        return Entry{}, ErrNoPreferencesSupplied // new sentinel, consistent with ErrDuplicate/ErrNotFound/ErrInvalidReleaseType
    }
  Prefer a dedicated sentinel error (errors.New, package-level var) over a bare errors.New(...) inline, so callers -- including handleUpdateWatchlist, which could then delete its own duplicate inline check and instead errors.Is-match this sentinel to a 400, the same pattern already used for ErrInvalidReleaseType/ErrInvalidEventType -- get a single source of truth, and the HTTP layer's check becomes provably redundant/removable rather than a second independent implementation of the same rule.

  WR-02: After a successful dec.Decode(&req) in both handleAddWatchlist and handleUpdateWatchlist, assert stream exhaustion:
    if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
  This is the standard Go idiom for rejecting trailing JSON data. Apply identically to both decode sites for consistency (matches this file's existing pattern of near-identical decode blocks in both handlers).
verification: N/A -- no fix applied in this session (diagnose-only). Verification belongs to the downstream fix-and-verify pass: for WR-01, a test constructing *Service directly (bypassing the HTTP layer) and calling UpdatePreferences with PreferencesParams{} should assert the new sentinel error; for WR-02, an httptest-based request with a concatenated-JSON body should assert 400 on both POST /watchlist and PATCH /watchlist/{id}.
files_changed: []
