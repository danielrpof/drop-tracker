# Phase 27: Paste-a-List Bulk Add - Context

**Gathered:** 2026-11-18
**Status:** Ready for planning (G-27-3 spike done: `.planning/spikes/001-paste-confidence-tiers/`)
**Amended:** 2026-11-18 by a post-discuss grilling (G-27-1..10). Where an amendment conflicts with the original text, the amendment wins.

<domain>
## Phase Boundary

The user pastes up to 100 artist names. The SPA resolves each name with a rate-limited `GET /search` call, sorts the rows into confidence tiers for review, and adds the confirmed set to the watchlist in one request (IMPT-01..05). Nothing is written until the user confirms. Out of scope: server-side resolve, cleaning pasted names before search, applying tags at confirm, and fixing the seed-mode gap on re-add (REQUIREMENTS.md Out of Scope / Future).

</domain>

<decisions>
## Implementation Decisions

### Confidence rules
- **D-01:** Resolution uses **only the MusicBrainz results**, because only MB results can be added (`isAddableSource`, `web/app/lib/sources.ts`). The Deezer column that `/search` returns is ignored for confidence, candidates, and confirm. `/search` is not changed. A `?sources=` parameter was considered and rejected.
- **D-02:** **High** means exactly one MB result has a normalized name equal to the normalized pasted line. Normalization folds case, trims and collapses whitespace, and folds diacritics. If two or more results match exactly (two artists named "Future", say), the row is **Medium**, so the user picks.
  - **Amended (G-27-3): provisional.** Before planning, run a spike that sends a real 50-name list from the project's genres through today's `/search` and measures how many rows come out High. If fewer than about 60% do, add the already-fetched MB `score` to `SearchArtist` (an added field; D-01 rejected only `?sources=`) and revisit this rule using it.
  - **Amended (G-27-3 outcome, spike 001): High uses score.** D-02 as written gave 76% High, which clears the gate. The user still chose the better rule: **High = exactly one MB result whose name is the same name as the line *and* whose `score` is ≥ 90.** The spike measured 92% High with no wrong picks. Every D-02 Medium had the right artist at rank 1 with score 100, and was demoted only by a low-score namesake (Drake, Nas, Eminem, Logic, ROSALÍA). Requiring exactly 100 was rejected: J. Cole (98) and Karol G (99) sit at rank 2 behind a different artist scoring 100. **Medium** (D-03) becomes "MB returned at least one artist, but no unique same-name result scores ≥ 90". Two such results scoring ≥ 90 is still Medium (`HER`).
  - **Required change:** `/search` adds `score` (int, MB's 0–100 relevance) to every `SearchArtist`. It is an added field. Deezer has no score, so it sends `null`, and D-01 ignores Deezer anyway. Fix `searchResultLimit` at 10 and pin it in a test, because the tier rule depends on it.
  - **Amended (G-27-5):** normalization means **Same name** (CONTEXT.md). The TS normalizer follows Go's `normalizeArtistName` (`internal/artistart/match.go`) exactly, including the U+2019 apostrophe fold and the same diacritic table, not `NFD`-and-strip. Both `go test` and `vitest` check against one fixture of input → normalized pairs, `internal/artistart/testdata/normalize_cases.json`.
- **D-03:** **Medium** means there is no unique exact match but MB returned at least one artist. The top result is pre-filled and left unselected. **Not found** means MB returned zero artists. There is no fuzzy-closeness threshold.
- **D-04:** A failed search (MB `status: "error"`, HTTP error, or network failure) gets a **separate Error state**. It is never counted as Not found and never starts selected. Each Error row has a Retry button, and a bulk "Retry failed" action re-runs them all. Not found must mean MB really had nothing.
  - **Amended (G-27-10, spike 001 / issue #17): one automatic retry first.** At the production 1 req/s, MB answered 38% of searches with 503 (production User-Agent, residential IP), so Error will be common, not rare. A failed search (anything D-04 counts as Error, never an abort) is re-queued **once**, automatically, after a backoff of about 2s. It goes through the same 2-wide queue and limiter, behind the searches already waiting. The row shows Error, with Retry, only if that retry also fails. Progress counts a line as done once it settles for the last time, and the Error tally counts only rows that failed both tries. The manual Retry and "Retry failed" stay as the fallback, and manual retries get no extra automatic retry. Retrying on the server (`internal/httpclient`) and rate headroom for the poller are tracked in GitHub issue #17, not here.
- **D-05:** Only High rows start selected (SC3).

### Art & Deezer link at confirm
- **D-06:** Confirm **inserts every row without running the artist-art match**, so the request stays well under the 15s `writeTimeout`. Without the match a row has no `image_url` or `deezer_id`, and the poller skips Deezer polling for any artist with no `deezer_id` (`internal/poller/poller.go:588`). After the insert commits, a **detached background job** runs `artistart.Matcher.Match` for each newly added artist through the same `ActivityGate`/limiter path that `Service.Add` and `artistart.Backfill` use, so art and the Deezer link show up within minutes with no restart. — **Reversibility:** reversible
  - **Amended (G-27-1): there is no new per-request job.** `cmd/server/main.go` owns one single-flight **backfill runner** around the existing `artistart.Backfill`, and the startup sweep becomes its first run. Confirm calls `Kick()` after its commit. A kick during a run sets a "run again" flag, so the runner never runs two sweeps at once. `ListArtistsMissingImage` already picks up the new artists (no image, never tried), so no matching code is added. The runner keeps the existing `ActivityGate` yielding, attempt recording and Stats log. Matching stays one artist at a time, so it is only ever one waiter on the MB limiter. `httpserver` starts no goroutines.
  - **Amended (G-27-9):** confirm calls `Kick()` only when `added > 0`. A re-added artist whose last match failed under 24h ago stays inside the existing cooldown and waits for it to end, the same as in the startup sweep. There is no cooldown bypass.
- **D-07:** The client sends **no Deezer data** at confirm (no id, no image hint). The MB→Deezer link belongs to `artistart.Matcher`. Linking by name across sources is the identity guess the project avoids (SearchResultsColumns D-09).
- **D-08:** Until the background match finishes, new cards use the existing no-art placeholder. There is **no "resolving" indicator and no client re-polling**. Art appears on the next natural refresh.
- **D-09:** The background match is **best-effort**. It honours the shutdown context the same way the backfill drain does (`backfillDrainTimeout`, `cmd/server/main.go`). Any artist it misses still has no image, so `ListArtistsMissingImage` picks it up in the next startup backfill sweep. No queue and no new table.
  - **Amended (G-27-1):** "the same way" means literally the same drain. The runner's goroutine replaces the startup backfill goroutine in `main.go`, and its existing `backfillDrainTimeout` defer keeps the drain-before-`pool.Close()` order. A run still in progress at shutdown is cancelled. Any artist it missed is picked up by the next run.

### Paste entry & limits
- **D-10:** An **"Add from list" button** next to `SearchBox` on the Watchlist page opens a **dialog**. It is hosted at route level in `web/app/routes/watchlist.tsx` like `ManageTagsDialog` and `ConfirmDialog`. The dialog steps through textarea → resolving (progress) → review → result, and the list behind it refreshes in place. The steps are labelled in the header: 1 Paste · 2 Find matches · 3 Review · 4 Done.
  - **Paste step:** a labelled textarea. Below it, a live `aria-live="polite"` helper line reads "N names · X duplicates and Y blank lines will be skipped", and turns into the inline over-cap error (D-11). **Edit list** in review comes back here with the text intact.
  - **Resolving step (sketch 002 winner, variant B):** a **dedicated progress screen** with no rows. It shows "12 of 40" in tabular figures, a `role="progressbar"` bar, running tallies (High / Medium / Not found / Error, each with an icon and text), and the names being searched right now. Review opens when every search has finished. The action is **"Stop and review"** (D-13). Reviewing live while searches ran (variant A) was rejected because rows and tab counts kept moving under the cursor. See `.planning/sketches/002-dialog-flow/`.
- **D-11:** The maximum is **100 distinct names** after blank lines and duplicates are dropped. Duplicates are found with the D-02 normalization. If the paste is over the cap, it is refused with a clear message before any search starts. The confirm body is bounded to match: at most 100 items, plus a request body size limit.
- **D-12:** If the user closes the dialog after any search has run, it asks **"Discard this list?"** first. Yes aborts in-flight searches with the same `AbortController` that Cancel uses and drops all state. Nothing is added either way.
  - **Amended (G-27-7):** the dialog's unmount cleanup also aborts that `AbortController`. `apiFetch`'s global 401 handler (`web/app/lib/api.ts:244`) unmounts the Watchlist when a session expires, and remaining searches must not keep firing 401s. A session that expires mid-resolve loses the pasted list. That is accepted, and there is no saved draft.
- **D-13:** **Cancel** during resolve stops in-flight and pending searches and opens **review with the rows resolved so far**. Lines never searched show as "Not searched": unselected, and they can be retried. The user can still confirm the partial set.

### Review screen behavior
UI decisions follow the `ui-ux-pro-max` skill (user directive). `/gsd-sketch` and `/gsd-ui-phase 27` must apply it too.
- **D-14:** Each row is a **stacked row card** at every width, never a wide table. Contents: a checkbox, the pasted line (muted), the matched artist with its disambiguation and country, a confidence badge, and the alternate picker underneath. Every confidence badge has **an icon and a text label** (High / Medium / Not found / Error / Not searched), never color alone.
- **D-15:** The alternate picker is a **base-ui/shadcn `Select`** over the up-to-10 MB results already fetched for that row (name · disambiguation · country). Picking a candidate selects the row. No re-search field. A row the user doesn't want is just left unchecked, which skips it.
- **D-16 (amended by sketch 001):** When two rows pick the **same MB artist**, the **selected row wins**. Any other row with that artist gets a "Same artist as line N, which is selected" note and a disabled checkbox. If neither row is selected, neither is blocked. Selecting a row or changing a pick re-evaluates every row. Confirm never sends the same mbid twice. This replaces "the first line in paste order wins", which let an unchecked Medium "ye" disable a High "Kanye West".
- **D-17:** **Already-watched** rows are matched by `mbid` against the loaded watchlist. They show an "On watchlist" badge (icon and text) and a disabled checkbox. Picking a different candidate re-enables the row. On the server, an artist that is already watched at confirm time (a race) is a non-fatal `skipped` and never fails the batch.
- **D-18:** Rows are **grouped by status** in a fixed order. **Needs review** (Medium, Error, Not found, Not searched) comes first, then **Ready** (High), then **On watchlist / duplicates**. The groups are shown as a **segmented control** (sketch 001 winner, variant C): "Needs review (n) · Ready (n) · Skipped (n)", with one group visible at a time and Needs review open first. The segmented control stays pinned under the dialog header. Group membership is fixed when review starts, so rows never jump tabs while the user edits them. Only their badges and notes update. See `.planning/sketches/001-review-screen/`.
- **D-19:** One atomic `role="status"` region announces the selection and progress as full phrases ("12 of 40 searched", "37 selected"). Per-row updates never each get their own live region.
- **D-20:** After Confirm, the dialog switches to a **result view**: "Added N · Skipped N · Failed N", with names listed for skipped and failed rows, and a Done button. The Watchlist calls `refresh()` underneath. Failed names can be copied back into a new paste. This replaces the toast pattern Phase 26 used.

### Claude's Discretion
- ~~How many searches run at once.~~ **Decided (G-27-2): 2**, one in flight and one queued. The MB limiter is 1 req/s with a burst of 1 and is shared with 3 poller workers, so more concurrency adds waiters but no throughput. It would also nearly fill the browser's 6-connection limit over HTTP/1.1 in local dev. Mid-cycle the paste gets about 2/5 of the budget, so 100 names take about 100s idle and about 250s during a poll cycle. The resolving screen makes no promise about the rate: either no time estimate, or one computed from the rate it actually sees. UI-SPEC's "About 1 search a second" must change. UAT must run against a genuinely rate-limited client, not `rate.Inf` (ROADMAP note).
- The confirm endpoint's path and verb (e.g. `POST /watchlist/bulk/add` inside `registerDataRoutes`), and request shape (mbid, name, disambiguation per item) are still the planner's call.
  - **Decided (G-27-4):** confirm checks every item first (mbid is a UUID, name length matches single add's rules), and invalid items are reported as `failed`. All valid items go in **one transaction**: the artist rows are upserted, then the watchlist insert uses `ON CONFLICT DO NOTHING`, so an already-watched artist comes back as `skipped`. Any database error fails the whole request, with no partial commits. The inline match is never run (D-06).
  - **Decided (G-27-6):** the response is `{added, skipped:[{mbid, reason}], failed:[{mbid, reason}]}`, extending the Phase 26 D-04 shape (see **Bulk outcome**, CONTEXT.md). `skipped` means attempted with nothing to do (`already_watched`). `failed` means refused as invalid. Only failed names get the D-20 "copy back into a new paste" action.
- New entries use the D-08 defaults from Phase 3 (all release types, nothing muted, no note).
- The exact copy, the copy for the seed-mode re-add gap on previously removed artists (ROADMAP carry-over, Phase 26 D-17 wording), and the visual tokens. These are set in `/gsd-sketch` → `/gsd-ui-phase 27` using ui-ux-pro-max.
  - **Decided (G-27-8):** the re-add gap gets **one fixed line on the review screen**, covering both effects: old tags return (TAG-07) and recent releases may notify. There is no per-row "Previously watched" badge and no new server lookup, because the client only knows about artists currently on the watchlist.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Scope & requirements
- `.planning/ROADMAP.md` §Phase 27 — goal, success criteria, planner notes (client orchestration, matchTimeout trap, sketch first)
- `.planning/REQUIREMENTS.md` — IMPT-01..05 and the Out of Scope rows (server-side batch import, cleaning names before search)
- `.planning/research/PITFALLS.md` — #3 (15s write timeout vs MB 1 req/s), #6 (seed mode on re-add)
- `.planning/research/SUMMARY.md` — v1.6 research summary, including the bounded-concurrency suggestion

### Prior phase decisions
- `.planning/phases/26-bulk-edit-remove/26-CONTEXT.md` — bulk endpoint conventions (D-04 outcome shape, D-20 store/option wiring, bounded id lists, refresh after a bulk action, D-17 seed-mode wording)
- `.planning/phases/24-artist-tags-notes/24-CONTEXT.md` — ConfirmDialog (D-16), plain-JSX rendering of user text

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `web/app/lib/api.ts` `searchArtists(query, signal)` — already accepts an `AbortSignal`, which handles cancel per request
- `web/app/lib/sources.ts` — `isAddableSource`, `identityField` (mbid for the already-watched check)
- `web/app/components/common/ConfirmDialog.tsx` — the "Discard this list?" prompt
- `web/app/components/ui/checkbox.tsx`, plus a base-ui `Select` (vendor it via the shadcn CLI if not present yet)
- `web/app/routes/watchlist.tsx` — route-level dialog hosting and `refresh()`
- `internal/artistart` — `Backfill` (wrapped by the G-27-1 runner), `ActivityGate`, and `normalizeArtistName` (the reference for the G-27-5 fixture)
- `internal/watchlist/service.go` — `Add` (validation, `UpsertArtist` COALESCE, `ErrDuplicate` on `watchlist_artist_id_key`). The bulk variant must skip the inline match.

### Established Patterns
- `GET /search` returns `sources.musicbrainz.status` = `ok`/`error` with a fixed error string, which drives D-04
- MB results always have `image_url: null`, so every bulk add would hit the 8s `matchTimeout` if the match stayed inline (`internal/watchlist/service.go:34`)
- Postgres errors are mapped by constraint name, never by checking first. Every SPA call goes through `apiFetch`. Names render as plain JSX text.

### Integration Points
- The new confirm route goes inside `registerDataRoutes` (`internal/httpserver/server.go`), which gives it session auth and CSRF. Pin the 401 and 403 tests like `bulk_test.go`.
- Bulk add belongs on the Phase 26 `WatchlistBulkStore` (`internal/httpserver/bulk.go`) or a sibling of it, not on `watchlist.Store`.
- The backfill runner (G-27-1) replaces the startup backfill goroutine in `cmd/server/main.go` and keeps its gate and drain defer. The confirm handler gets its `Kick` through a narrow interface injected into `httpserver`.
- Seed mode is tracked per source (`internal/detection/detector.go:87-93`), so a `deezer_id` that arrives later from the runner seeds on Deezer separately. No back-catalogue flood.

</code_context>

<specifics>
## Specific Ideas

- Approved row card layout (ASCII preview):
  ```
  [x] "future"                    (v) High
      Future · US rapper, Atlanta
      [ Future · US rapper      v ]
  [ ] "ye"                        (!) Medium
      Ye · (no disambiguation)
      [ Kanye West · US         v ]
  [ ] "xqzv"                      (?) Not found
      No MusicBrainz match
  [ ] "sza"                       (x) Error
      Search failed    [Retry]
  ```
- Run `/gsd-sketch` on the review screen before `/gsd-ui-phase 27` (ROADMAP research flag), with ui-ux-pro-max applied.
- **Done (G-27-3):** spike 001 (`.planning/spikes/001-paste-confidence-tiers/`, see `merged.md` for each name). D-02 gave 76% High, and the score ≥ 90 rule gives 92%, adopted above. For planner fixtures, use the spike's real cases: `Drake` (namesake at rank 10, score 71 → High), `J. Cole` (right artist at rank 2, score 98 → High), `HER` (two exact matches scoring ≥ 90 → Medium), `Kanye West` (no exact match → Medium), `Rosalia` (folds to ROSALÍA → High).

### Accepted risks (grilling)
- The Deezer half of each `/search` is fetched and thrown away (about 100 calls per paste, within the 50 per 5s budget). D-01 stands.
- After a large import, the first MB poll cycle (100 artists seeding) plus the runner's matches can take longer than the 15m interval and skip one tick (`ErrCycleInProgress`). This is accepted because the G-27-1 runner keeps matching to one waiter.

</specifics>

<deferred>
## Deferred Ideas

- Re-searching an edited name for a Not found row (fixing typos inside the review screen). It was considered for D-15, and a fresh paste covers it for now.
- Applying tags at confirm time (REQUIREMENTS.md Future).

### Reviewed Todos (not folded)
- The D-15 prev-tag query resolution, unifying the sqlscan quote state machines, and the dead `resuming` assignment in the digest chunker. These were keyword false positives, belong to tooling and the notifier, and are out of this phase's scope.

</deferred>

---

*Phase: 27-paste-a-list-bulk-add*
*Context gathered: 2026-11-18*
