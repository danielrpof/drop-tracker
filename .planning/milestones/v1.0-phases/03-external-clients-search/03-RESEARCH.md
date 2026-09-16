# Phase 3: External Clients & Search - Research

**Researched:** 2026-08-07
**Domain:** Hand-rolled HTTP API clients (MusicBrainz, Deezer), rate limiting, cron-based polling
**Confidence:** MEDIUM-HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Search-proxy contract**
- **D-01:** Single combined endpoint, `GET /search?q=...`, queries MusicBrainz and Deezer server-side and returns one response. No separate `/search/musicbrainz` and `/search/deezer` resources — matches Phase 2's flat, unprefixed route convention (D-14 from Phase 2).
- **D-02:** No cross-source dedup/merge. Response contains MusicBrainz results and Deezer results as separate, source-tagged lists — no fuzzy-matching by name. Consistent with REQUIREMENTS.md explicitly listing dual-source reconciliation as out of scope; would be inconsistent to build name-matching just for search when it's rejected for detection. — **Reversibility:** reversible — merging later is additive, doesn't break the existing per-source shape.
- **D-03:** Partial results on source failure. If one upstream (e.g. Deezer) errors or times out, the endpoint returns whatever succeeded plus a per-source status/error flag — never fails the whole request because one source is degraded.

**Scheduler scope (Phase 3 vs Phase 4)**
- **D-04:** `robfig/cron` is wired up in Phase 3, not deferred to Phase 4. The poll job runs on `Config.PollInterval`, calls the MusicBrainz/Deezer clients per watchlisted artist, and the handler logs a structured line per artist/source (artist, source, item count) rather than doing anything with the data. Phase 4 replaces the log statement with real diff logic against the "seen" store — the scheduler plumbing, rate-limiting-under-real-schedule behavior, and overlap guard (D-07) are de-risked now instead of being built alongside the diff engine.
- **D-05:** The poll cycle reads the current watchlist via the existing `watchlist.Store` interface (`internal/watchlist/service.go`) — no new dedicated poll-list query. Reuses Phase 2's list query and DB seam as-is.
- **D-06:** An artist with a null `deezer_id` (added via MusicBrainz-only search, Phase 2 D-02) is skipped for the Deezer half of the poll cycle — no name-search fallback to opportunistically backfill `deezer_id`.

**Rate-limiting & concurrency shape**
- **D-07:** One `rate.Limiter` per client (MusicBrainz, Deezer), each sized from `Config.MusicBrainzRateLimitPerSec` / `Config.DeezerRateLimitPer5s` (already stubbed in `internal/config/config.go`). The poll cycle iterates watchlisted artists **sequentially**, calling `limiter.Wait(ctx)` before each request — no worker pool. The limiter itself enforces pacing; correctness doesn't depend on manual concurrency bookkeeping.
- **D-08:** MusicBrainz and Deezer run as **independent poll cycles** (separate cron jobs or separate goroutines), each gated only by its own limiter. MusicBrainz's slow ~1/sec pace never throttles Deezer's faster ~50/5s pace, matching CLNT-01/CLNT-02 as two distinct requirements with independent success criteria.
- **D-09:** Each per-source poll cycle has an in-process overlap guard (atomic/mutex "cycle in progress" flag) — if a cron tick fires while the previous cycle for that source is still running, the tick is skipped and logged as a warning. Built now at the scheduler level so Phase 4's DTCT-05 ("no overlapping detection runs") inherits this property instead of retrofitting it. — **Reversibility:** reversible — a future distributed-lock replacement (noted as a Phase-7+ consideration in the tech stack doc) swaps this flag out without changing calling code.

**MusicBrainz / Deezer entity scope**
- **D-10:** The MusicBrainz poll client fetches **release-groups only** (browse-by-artist) in Phase 3. Enough to satisfy CLNT-01 and gives Phase 4 what it needs for new-release/deluxe detection (DTCT-01/DTCT-02) immediately. Recordings-by-artist-credit (needed for guest-feature detection, DTCT-03) is a different query shape with no consumer until Phase 4 — that fetch path is built in Phase 4 alongside the diff logic that uses it.
- **D-11:** The MusicBrainz search-proxy queries the `/ws/2/artist?query=` artist-search endpoint only — returns matching artists (MBID, name, disambiguation, type). No combined artist+release-group search; no requirement (WLST-01/UI-01) asks for searching by release/song title.
- **D-12:** The Deezer poll client fetches **artist albums/releases only** (by `deezer_id`), mirroring the release-groups-only scope decided for MusicBrainz. No track-level data — Deezer is documented in REQUIREMENTS.md as a secondary/faster signal, not the primary guest-feature detection source, so track-level fetch has no Phase 4 consumer.

### Claude's Discretion
- Exact `GET /search` query-param shape (result limit, pagination if any), and per-request timeout values for the MusicBrainz/Deezer HTTP clients — not discussed, left to planning/research.
- Whether the overlap guard (D-09) is a `sync.Mutex`, `atomic.Bool`, or similar — implementation detail.
- Exact structured-log field names for the log-only poll handler (D-04) — follow existing `request_id`/slog conventions from Phase 1/2.
- Go project layout for the two new client packages (`internal/musicbrainz`, `internal/deezer`) — matches CLAUDE.md's stated package naming, no further decisions needed.

### Deferred Ideas (OUT OF SCOPE)
- **Distributed/multi-instance scheduler locking** — the in-process overlap guard (D-09) is explicitly single-instance only; a Postgres-advisory-lock-based guard for running multiple app instances is noted in the tech stack doc as a later consideration, not this phase.
- **Deezer name-search fallback to backfill `deezer_id`** — considered and rejected for the poll cycle (D-06); could resurface as a small enhancement in a later phase if users frequently add MusicBrainz-only artists.
- **Combined artist+release-group search** — considered and rejected for the search-proxy (D-11); no current requirement needs searching by release/song title, would be its own phase if ever wanted.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| WLST-01 | User can search MusicBrainz and Deezer catalogs to find an artist to add to the watchlist | Live-verified `GET /ws/2/artist?query=` and `GET /search/artist?q=` request/response shapes (see Code Examples); Pattern 1 (narrow-interface `ArtistSearcher` seam) and the `/search` handler structure in the Architecture Patterns diagram |
| CLNT-01 | System polls MusicBrainz for each watchlisted artist on a configurable schedule, respecting MusicBrainz's rate limit | Live-verified `GET /ws/2/release-group?artist=` shape (D-10); MusicBrainz rate-limit figures and required `User-Agent` header (Pitfall 1, directly reproduced this session); Pattern 2 (`limiter.Wait(ctx)` request helper) and Pattern 3 (independent cron job wiring) |
| CLNT-02 | System polls Deezer for each watchlisted artist on a configurable schedule, respecting Deezer's rate limit | Live-verified `GET /artist/{id}/albums` shape (D-12); Deezer rate-limit and quota-error behavior (Pitfall 2); D-06 nil-`deezer_id` skip behavior (Pitfall 3, grounded in `[VERIFIED: internal/watchlist/service.go:54]`) |
| CLNT-03 | System exposes a live search-proxy endpoint that queries MusicBrainz and Deezer catalogs in real time | D-01/D-02/D-03 search-proxy contract (verbatim above); Architecture Patterns diagram's `GET /search` flow; Open Question 1 (query-param/limit shape) |
</phase_requirements>

## Summary

This phase builds two hand-rolled `net/http` clients (`internal/musicbrainz`, `internal/deezer`), a combined `GET /search` proxy handler, and a `robfig/cron`-driven poll loop — all already scoped tightly by CONTEXT.md's D-01 through D-12. The research risk here is not architectural (that's locked) but **API-mechanics accuracy**: exact endpoint URLs, query parameter names, JSON field names, pagination shape, and rate-limit/error behavior for two external services whose official docs are partially gated (Deezer) or scattered across a wiki (MusicBrainz).

To close that risk, every endpoint this phase needs was **exercised live** this session — actual `GET` requests against `musicbrainz.org` and `api.deezer.com` — and the exact JSON shapes below are transcribed from those real responses, not from documentation prose. This is the strongest verification available for external HTTP APIs and is called out explicitly per-claim below. The MusicBrainz rate-limiting pitfall (missing/generic `User-Agent` triggering `503`s) was also **directly reproduced** during this research session, which independently confirms CLAUDE.md's existing warning.

`robfig/cron/v3` (pinned v3.0.1, matching CLAUDE.md) and `golang.org/x/time/rate` (latest v0.15.0) are both new `go.mod` dependencies this phase must add — neither is in `go.mod` today. Both APIs are small, stable, and verified against `pkg.go.dev` plus the Go module proxy directly.

**Primary recommendation:** Build `internal/musicbrainz` and `internal/deezer` as thin `net/http` wrappers returning small first-class Go structs (not raw `map[string]any`), each holding one `*rate.Limiter` (`Wait(ctx)` before every call, per D-07), each request setting the exact `User-Agent` string from `Config.MusicBrainzUserAgent` (MusicBrainz only — Deezer needs no auth/UA requirement). Wire two independent `robfig/cron` jobs from `cmd/server/main.go` (per D-08), each wrapped in a `sync.Mutex`-or-`atomic.Bool` overlap guard (per D-09) that logs-and-skips a tick instead of blocking it.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Live artist/catalog search (`GET /search`) | API / Backend | External Service (MusicBrainz, Deezer) | Backend proxies and shapes two upstream responses into one; no business logic lives client-side (no UI this phase) |
| Rate-limited outbound HTTP to MusicBrainz/Deezer | API / Backend | — | `golang.org/x/time/rate` limiters live inside `internal/musicbrainz`/`internal/deezer`, called only from backend code (search handler, poll jobs) |
| Scheduled polling (CLNT-01, CLNT-02) | API / Backend (in-process scheduler) | Database (watchlist read) | `robfig/cron` runs inside the same Go binary (PROJECT.md single-binary constraint); reads watchlist via `watchlist.Store`, writes nothing yet (D-04 log-only) |
| Watchlist read for poll-list (D-05) | Database / Storage | API / Backend | `watchlist.Store.List` already exists (Phase 2); poller is a second, non-HTTP caller of the same interface |
| Overlap-guard state (D-09) | API / Backend (in-process) | — | Single-instance in-memory flag; explicitly not Database-tier (no advisory lock) per D-09's reversibility note |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/robfig/cron/v3` | v3.0.1 | Cron-based background poller | Already locked in CLAUDE.md/PROJECT.md. `[VERIFIED: go module proxy — go list -m -versions github.com/robfig/cron/v3 → v3.0.0-rc1 v3.0.0 v3.0.1]` confirmed this session against `proxy.golang.org`. |
| `golang.org/x/time/rate` | v0.15.0 | Token-bucket rate limiting for MusicBrainz/Deezer clients (D-07) | Already locked in CLAUDE.md as "use `golang.org/x/time/rate`". `[VERIFIED: go module proxy — go list -m -versions golang.org/x/time → ... v0.15.0]` (latest) confirmed this session. |
| `net/http` (stdlib) | Go 1.26 (project's pinned toolchain) | HTTP client transport for both external API clients | CLAUDE.md mandates hand-rolled clients on plain `net/http` — no HTTP client library needed. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `encoding/json` (stdlib) | Go 1.26 | Decode MusicBrainz/Deezer JSON responses into typed structs | Both APIs return plain JSON; no codegen needed given the small, hand-picked field subset each client actually uses (D-10/D-11/D-12 scope). |
| `context` (stdlib) | Go 1.26 | Request cancellation/timeout propagation into `http.NewRequestWithContext` and `limiter.Wait(ctx)` | Every outbound call must accept a caller `ctx` — both for HTTP handler request-scoped cancellation (search proxy) and for `cron`'s job-provided `ctx` (poll cycle), and so `limiter.Wait` can be interrupted by shutdown. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled `internal/musicbrainz`/`internal/deezer` on `net/http` | A generated/wrapped community client (e.g. `github.com/michiwend/gomusicbrainz`, `github.com/stayradiated/deezer`) | Already rejected in CLAUDE.md — neither API publishes an OpenAPI spec, and this phase's surface (artist search + one browse/list endpoint per source) is small enough that a hand-rolled client is simpler to keep in exact D-10/D-11/D-12 scope and to test with `httptest.Server` per CLAUDE.md's testing constraint. Community clients also pull in fields/endpoints this phase explicitly excludes (recordings, tracks). |
| Two separate `robfig/cron` jobs (D-08) | A single job iterating both sources | Rejected by D-08 explicitly: MusicBrainz's slow ~1/sec pace must never throttle Deezer's faster ~50/5s pace. |

**Installation:**
```bash
go get github.com/robfig/cron/v3@v3.0.1
go get golang.org/x/time@v0.15.0
```

**Version verification:** Both versions confirmed this session via `go list -m -versions <module>` against the live Go module proxy (`GOPROXY=https://proxy.golang.org,direct`), the Go-ecosystem equivalent of `npm view <pkg> version` — this is an authoritative registry query, not a training-data guess.

## Package Legitimacy Audit

> The `gsd-tools package-legitimacy check` seam only supports `npm|pypi|crates` ecosystems; this phase's two new dependencies are Go modules, so the ecosystem-appropriate substitute — direct Go module proxy verification (`go list -m -versions`) — was used instead, per the package-legitimacy protocol's "ecosystem-specific registry verification" step.

| Package | Registry | Age | Downstream signal | Source Repo | Verdict | Disposition |
|---------|----------|-----|--------------------|--------------|---------|-------------|
| `github.com/robfig/cron/v3` | Go module proxy (proxy.golang.org) | v3.0.1 published 2020-01-24 (stable, no churn since — `[CITED: pkg.go.dev/github.com/robfig/cron/v3]`) | Already a locked CLAUDE.md decision; de facto standard Go cron library, widely imported | `github.com/robfig/cron` | OK | Approved |
| `golang.org/x/time` | Go module proxy (proxy.golang.org) | Maintained under the official `golang.org/x` umbrella (Go team-owned) | Official Go extended-standard-library module — highest possible trust tier for a Go dependency | `golang.org/x/time` (go.googlesource.com mirror) | OK | Approved |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

Both packages were already named as locked decisions in `.claude/CLAUDE.md`'s Supporting Libraries table before this research ran — this audit independently re-confirms both are live, correctly-named, non-typosquatted modules via the authoritative Go module proxy rather than accepting the CLAUDE.md table at face value.

## Architecture Patterns

### System Architecture Diagram

```
                     ┌─────────────────────────────────────────────┐
                     │              cmd/server (single binary)      │
                     │                                               │
  HTTP request       │  ┌──────────────┐                             │
  GET /search?q=  ───┼─▶│ chi router   │                             │
                     │  │ (httpserver) │                             │
                     │  └──────┬───────┘                             │
                     │         │ search.go handler                   │
                     │         ▼                                     │
                     │  ┌──────────────────┬──────────────────┐      │
                     │  │ internal/         │ internal/         │      │
                     │  │ musicbrainz        │ deezer            │      │
                     │  │ .SearchArtists()   │ .SearchArtists()  │      │
                     │  └────────┬───────────┴─────────┬─────────┘      │
                     │       rate.Limiter.Wait()    rate.Limiter.Wait() │
                     │           │                        │            │
                     └───────────┼────────────────────────┼────────────┘
                                 ▼                        ▼
                     musicbrainz.org/ws/2/artist   api.deezer.com/search/artist
                     (partial-results-on-failure per D-03 — one source erroring
                      never fails the combined response)

                     ┌─────────────────────────────────────────────┐
                     │              cmd/server (same binary)         │
                     │                                                │
                     │  ┌────────────────┐   ┌────────────────────┐  │
                     │  │ robfig/cron job │   │ robfig/cron job     │  │
                     │  │ "poll-musicbrainz"│  │ "poll-deezer"       │  │
                     │  │ (own overlap     │   │ (own overlap        │  │
                     │  │  guard, D-09)    │   │  guard, D-09)        │  │
                     │  └────────┬─────────┘   └──────────┬──────────┘  │
                     │           │ watchlist.Store.List() │             │
                     │           ▼                        ▼             │
                     │  ┌─────────────────────────────────────────┐    │
                     │  │  Postgres: watchlist JOIN artists         │    │
                     │  └─────────────────────────────────────────┘    │
                     │           │ per artist, sequential (D-07)         │
                     │           ▼                        ▼             │
                     │  rate.Limiter.Wait() ──▶ MB   rate.Limiter.Wait() ──▶ Deezer
                     │           │ release-groups         │ albums (D-12)  │
                     │           ▼                        ▼               │
                     │      slog.Info("poll result", artist, source,      │
                     │                  item_count)   -- D-04 log only,   │
                     │      no diff/store write this phase                │
                     └────────────────────────────────────────────────┘
```

### Recommended Project Structure
```
internal/
├── musicbrainz/
│   ├── client.go       # Client struct, NewClient(userAgent string, limiter *rate.Limiter, httpClient *http.Client)
│   ├── search.go        # SearchArtists(ctx, query string) ([]Artist, error) -- D-11
│   ├── releasegroups.go # ReleaseGroupsByArtist(ctx, mbid string) ([]ReleaseGroup, error) -- D-10
│   └── client_test.go   # httptest.Server-backed tests, no live calls (CLAUDE.md testing constraint)
├── deezer/
│   ├── client.go        # Client struct, NewClient(limiter *rate.Limiter, httpClient *http.Client) -- no UA/auth needed
│   ├── search.go         # SearchArtists(ctx, query string) ([]Artist, error) -- D-11-equivalent for Deezer
│   ├── albums.go          # ArtistAlbums(ctx, deezerID string) ([]Album, error) -- D-12
│   └── client_test.go
├── httpserver/
│   └── search.go          # GET /search handler: calls both clients, D-02 (no dedup), D-03 (partial results)
└── poller/
    ├── poller.go           # wires the two robfig/cron jobs + overlap guards (D-08, D-09), started from cmd/server/main.go
    └── poller_test.go
```

### Pattern 1: Narrow-interface client seam (mirrors `httpserver.Pinger` / `watchlist.Store`)
**What:** Each client exposes a small interface (e.g. `musicbrainz.ArtistSearcher`, `musicbrainz.ReleaseGroupLister`) that the search handler and poller depend on, not the concrete `*musicbrainz.Client`.
**When to use:** Every consumer of `internal/musicbrainz`/`internal/deezer` (search handler, poll job) — matches the project's established Phase 1/2 pattern (`Pinger`, `watchlist.Store`) called out explicitly in CONTEXT.md's code_context section.
**Example:**
```go
// internal/musicbrainz/client.go
package musicbrainz

import (
	"context"
	"net/http"

	"golang.org/x/time/rate"
)

// Client is a rate-limited, hand-rolled wrapper around MusicBrainz's ws/2
// JSON API (D-10, D-11 scope only: artist search + release-groups browse).
type Client struct {
	baseURL    string
	userAgent  string
	httpClient *http.Client
	limiter    *rate.Limiter
}

func NewClient(userAgent string, limiter *rate.Limiter, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{
		baseURL:    "https://musicbrainz.org/ws/2",
		userAgent:  userAgent,
		httpClient: httpClient,
		limiter:    limiter,
	}
}

// ArtistSearcher is the narrow seam internal/httpserver's search handler
// depends on -- mirrors httpserver.Pinger / watchlist.Store.
type ArtistSearcher interface {
	SearchArtists(ctx context.Context, query string) ([]Artist, error)
}

var _ ArtistSearcher = (*Client)(nil)
```
*Source: pattern derived from `internal/httpserver/server.go`'s `Pinger` interface and `internal/watchlist/service.go`'s `Store` interface — both read this session — extended to the new packages per CONTEXT.md's explicit code_context guidance.*

### Pattern 2: Rate-limited request helper (D-07)
**What:** A single unexported `doRequest` method on each client that calls `limiter.Wait(ctx)` immediately before every `http.Client.Do`, so no call site can forget the limiter.
**When to use:** Every outbound request from both clients — search proxy calls and poll-cycle calls alike, since both are D-07's "one `rate.Limiter` per client" scope.
**Example:**
```go
// Source: golang.org/x/time/rate godoc (pkg.go.dev/golang.org/x/time/rate),
// fetched and verified this session.
func (c *Client) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	return c.httpClient.Do(req)
}
```

### Pattern 3: Two independent cron jobs with per-job overlap guard (D-08, D-09)
**What:** Each source gets its own `AddFunc` registration and its own guard variable — never a shared mutex across sources, which would reintroduce the cross-source throttling D-08 explicitly rejects.
**When to use:** Wiring the poller in `cmd/server/main.go` or a new `internal/poller` package.
**Example:**
```go
// Source: pkg.go.dev/github.com/robfig/cron/v3 godoc, fetched and verified
// this session.
c := cron.New() // standard 5-field spec; PollInterval is a time.Duration
                 // (config.go), so translate to "@every <duration>" syntax.
var mbRunning atomic.Bool
_, err := c.AddFunc(fmt.Sprintf("@every %s", cfg.PollInterval), func() {
	if !mbRunning.CompareAndSwap(false, true) {
		logger.Warn("musicbrainz poll cycle skipped: previous cycle still running")
		return
	}
	defer mbRunning.Store(false)
	runMusicBrainzPollCycle(ctx, store, mbClient, logger)
})

var dzRunning atomic.Bool
_, err = c.AddFunc(fmt.Sprintf("@every %s", cfg.PollInterval), func() {
	if !dzRunning.CompareAndSwap(false, true) {
		logger.Warn("deezer poll cycle skipped: previous cycle still running")
		return
	}
	defer dzRunning.Store(false)
	runDeezerPollCycle(ctx, store, dzClient, logger)
})

c.Start()
// on shutdown (mirrors cmd/server/main.go's existing SIGTERM handling):
stopCtx := c.Stop()
<-stopCtx.Done()
```
Note: `@every <duration>` accepts Go's `time.Duration.String()` format (e.g. `"15m0s"`) directly — `cfg.PollInterval.String()` is fine to interpolate.

### Anti-Patterns to Avoid
- **Sharing one `rate.Limiter` across MusicBrainz and Deezer:** Directly contradicts D-07/D-08 — MusicBrainz's ~1/sec limiter would throttle Deezer's ~50/5s traffic to MusicBrainz's pace.
- **Using `limiter.Allow()` instead of `limiter.Wait(ctx)` in the poll cycle:** `Allow()` is non-blocking and drops the request if no token is available — wrong for a sequential background poller that should simply pace itself, not skip artists. `Wait` is the documented correct choice for "background workers that should slow down, not fail" `[CITED: pkg.go.dev/golang.org/x/time/rate]`.
- **Omitting or defaulting the MusicBrainz `User-Agent`:** Reproduced directly this session — a generic/anonymous UA triggers HTTP 503 within a handful of requests. `Config.MusicBrainzUserAgent` (already stubbed, non-empty default) must be set on literally every MusicBrainz request, not just the poll cycle.
- **Treating a Deezer "Quota limit exceeded" response as a network error:** Deezer returns HTTP 200 with an in-body `{"error":{...}}` object on rate-limit breach (community-confirmed, see Pitfall 2 below) — a client that only checks `resp.StatusCode != 200` will silently treat a quota error as a successful empty/malformed response.
- **Building a shared cross-source dedup layer for `/search`:** Explicitly rejected by D-02 — return MusicBrainz and Deezer results as separate, source-tagged lists.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Rate limiting outbound requests | A manual `time.Sleep`/ticker-based throttle | `golang.org/x/time/rate` (`rate.NewLimiter` + `Wait(ctx)`) | Token-bucket semantics (burst handling, context cancellation) are easy to get subtly wrong with hand-rolled sleep logic; the stdlib-adjacent package is already the locked CLAUDE.md decision. |
| Scheduled recurring jobs | A hand-rolled `time.Ticker` loop with manual overlap tracking | `robfig/cron/v3` | Cron spec parsing, `@every` support, and `Stop()`'s graceful-drain `context.Context` are non-trivial to reimplement correctly; already locked in CLAUDE.md. |
| JSON decoding of MusicBrainz/Deezer responses | Untyped `map[string]any` traversal in handler code | Small first-class Go structs per endpoint (`Artist`, `ReleaseGroup`, `Album`) with `encoding/json` struct tags | Typed structs catch field-name typos at compile time and match the existing project convention (`sqlc`-generated structs, `watchlist.Entry`) of typed domain objects rather than dynamic maps. |

**Key insight:** Both new dependencies (`robfig/cron`, `x/time/rate`) are already-locked, narrowly-scoped libraries with small APIs — the risk in this phase is not "should we build vs. buy" (already answered) but getting the *external API mechanics* right, which is why this research prioritized live-verifying every endpoint over re-litigating the stack choice.

## Common Pitfalls

### Pitfall 1: MusicBrainz throttles on User-Agent, not auth — and the failure looks like flaky infra
**What goes wrong:** Requests intermittently return `HTTP 503 Service Unavailable` with no clear error body, easily misdiagnosed as a MusicBrainz outage or network flakiness.
**Why it happens:** MusicBrainz keys its throttling off the `User-Agent` header rather than an API key. A missing, default (Go's `http.Client` sends none by default unless set), or "anonymous-looking" UA is rate-limited far more aggressively (~50 req/sec shared pool across *all* anonymous clients, vs. no specific documented ceiling for identified UAs beyond the global 300 req/sec).
**How to avoid:** Set `req.Header.Set("User-Agent", cfg.MusicBrainzUserAgent)` on every single outbound MusicBrainz request — not just the poll cycle, also the search-proxy path. `Config.MusicBrainzUserAgent` already defaults to a non-empty, correctly-formatted value (`internal/config/config.go:32`, `[VERIFIED: internal/config/config.go:32]` — `MusicBrainzUserAgent string `env:"MUSICBRAINZ_USER_AGENT" envDefault:"drop-tracker/0.1.0 (+https://github.com/danielrpof/drop-tracker)"``).
**Warning signs:** Sporadic 503s that correlate with request *volume* (bursts of search-proxy traffic or a poll cycle over many artists) rather than a fixed downstream outage window.
**Directly observed this session:** A batch of `WebFetch` calls to `musicbrainz.org` (which does not set a MusicBrainz-meaningful `User-Agent`) began returning `HTTP 503, Retry-After: 0` after a handful of rapid requests — independent, first-hand confirmation of this exact pitfall during this research session.

### Pitfall 2: Deezer signals rate-limit breach in the response body, not the HTTP status
**What goes wrong:** A client that checks only `resp.StatusCode == http.StatusOK` treats a rate-limited Deezer response as a success, then fails downstream when it tries to unmarshal an `{"error":{...}}` payload into an `Artist`/`Album` struct (or worse, silently proceeds with zero-value fields).
**Why it happens:** Deezer's public API returns errors as `{"error":{"type":"Exception","message":"Quota limit exceeded","code":4}}` with HTTP 200 in the common case (community-documented pattern — Deezer's official docs on this specific behavior are behind a login wall, so treat this as `[CITED: community-corroborated, MEDIUM confidence]`, not independently verified against Deezer's own text this session).
**How to avoid:** After a successful HTTP-level response, check for a top-level `"error"` key in the decoded JSON before treating `"data"` as valid — a minimal wrapper struct `{Error *struct{Code int; Message string} `json:"error"`}` checked before decoding into the real payload type handles this cleanly.
**Warning signs:** Empty/near-empty search or album results during burst traffic (e.g. a poll cycle over a large watchlist) despite no HTTP-level error being logged.

### Pitfall 3: `deezer_id` is nullable — D-06 requires an explicit skip, not a failed request
**What goes wrong:** Calling `internal/deezer.ArtistAlbums(ctx, artist.DeezerID)` with a nil/empty `deezer_id` (an artist added via MusicBrainz-only search, per Phase 2 D-02) either panics on a nil dereference or sends a request to a malformed URL (`/artist//albums`).
**Why it happens:** `watchlist.Entry.DeezerID` is `*string` (`[VERIFIED: internal/watchlist/service.go:54]` — `DeezerID *string \`json:"deezer_id"\`` — read this session) and is genuinely nil for MusicBrainz-only adds; D-06 explicitly says this case is *skipped*, not backfilled.
**How to avoid:** The Deezer poll cycle must check `if artist.DeezerID == nil { continue }` (with a debug/info log line noting the skip) before calling the Deezer client at all — this is a pure Go-side branch, no HTTP request should be attempted.
**Warning signs:** A poll cycle log showing MusicBrainz item counts for every watchlisted artist but Deezer errors (not skips) for artists known to have been added via MusicBrainz-only search.

### Pitfall 4: `robfig/cron`'s `Stop()` does not wait by itself — you must consume its returned context
**What goes wrong:** Calling `c.Stop()` and immediately proceeding with process shutdown (e.g. closing the DB pool) can race an in-flight poll-job goroutine that is still mid-request when the pool closes underneath it.
**Why it happens:** `Stop()`'s doc explicitly separates "stop scheduling new runs" from "wait for the current run to finish" — the latter requires the caller to `<-stopCtx.Done()` on the returned `context.Context` `[CITED: pkg.go.dev/github.com/robfig/cron/v3]`.
**How to avoid:** Mirror the existing graceful-shutdown pattern already in `cmd/server/main.go` (`[VERIFIED: cmd/server/main.go:107-114]` — the `case <-ctx.Done()` branch that calls `httpSrv.Shutdown(shutdownCtx)`): add a matching `stopCtx := cronScheduler.Stop(); <-stopCtx.Done()` step before `pool.Close()` runs (deferred call already exists at `cmd/server/main.go:80`).
**Warning signs:** Intermittent `pgx` "conn closed" errors during local `docker compose down` or CI test teardown that only reproduce when a poll cycle happens to be running at shutdown time.

## Code Examples

### MusicBrainz artist search — live-verified request/response (D-11)
```
GET https://musicbrainz.org/ws/2/artist?query=artist:Drake&fmt=json&limit=2
```
```json
{
  "created": "2026-08-07T20:30:23.945Z",
  "count": 303,
  "offset": 0,
  "artists": [
    {
      "id": "9fff2f8a-21e6-47de-a2b8-7f449929d43f",
      "type": "Person",
      "score": 100,
      "name": "Drake",
      "sort-name": "Drake",
      "country": "CA",
      "disambiguation": "Canadian rapper"
    }
  ]
}
```
*Source: `[VERIFIED: live MusicBrainz API response, fetched this session]` — the exact fields above (`id`, `type`, `score`, `name`, `sort-name`, `country`, `disambiguation`, top-level `count`/`offset`/`artists`) are quoted verbatim from the real response body observed via a direct `GET` this session, not from documentation prose. `query` uses Lucene syntax (`query=artist:Drake` scopes the search to the `artist` field specifically; a bare `query=Drake` searches across `alias`/`artist`/`sortname`). `limit` defaults to 25, max 100 `[CITED: musicbrainz.org/doc/MusicBrainz_API]`.

### MusicBrainz release-groups browse-by-artist — live-verified (D-10)
```
GET https://musicbrainz.org/ws/2/release-group?artist={mbid}&type=album|ep&fmt=json&limit=3
```
```json
{
  "release-group-count": 61,
  "release-group-offset": 0,
  "release-groups": [
    {
      "id": "0b6c2da8-ac60-4663-beae-bd3dc6dea94a",
      "title": "Nothing Was the Same",
      "primary-type": "Album",
      "secondary-types": [],
      "first-release-date": "2013-09-19",
      "disambiguation": ""
    }
  ]
}
```
*Source: `[VERIFIED: live MusicBrainz API response, fetched this session]` — fields quoted verbatim from the real response body. `type=album|ep` filters to those two primary types in one call (pipe-separated); omitting `type` returns every release-group type (album/single/EP/broadcast/other). Pagination: `offset` param, `release-group-count` gives the total for loop termination.

### Deezer artist search — live-verified (D-11 equivalent for Deezer)
```
GET https://api.deezer.com/search/artist?q=Drake&limit=2
```
```json
{
  "data": [
    {
      "id": 246791,
      "name": "Drake",
      "link": "https://www.deezer.com/artist/246791",
      "picture": "https://api.deezer.com/artist/246791/image",
      "nb_album": 78,
      "nb_fan": 24047501,
      "tracklist": "https://api.deezer.com/artist/246791/top?limit=50",
      "type": "artist"
    }
  ],
  "total": 100,
  "next": "https://api.deezer.com/search/artist?q=Drake&limit=2&index=2"
}
```
*Source: `[VERIFIED: live Deezer API response, fetched this session]` — fields quoted verbatim. No API key/auth required. Pagination is `index`/`limit`; `next` is a full ready-to-fetch URL for the following page, present only when more results exist.

### Deezer artist albums — live-verified (D-12)
```
GET https://api.deezer.com/artist/246791/albums?limit=3
```
```json
{
  "data": [
    {
      "id": 983217461,
      "title": "HABIBTI",
      "link": "https://www.deezer.com/album/983217461",
      "cover": "https://api.deezer.com/album/983217461/image",
      "release_date": "2026-05-15",
      "record_type": "album",
      "tracklist": "https://api.deezer.com/album/983217461/tracks",
      "explicit_lyrics": true,
      "type": "album"
    }
  ],
  "total": 78,
  "next": "https://api.deezer.com/artist/246791/albums?limit=3&index=3"
}
```
*Source: `[VERIFIED: live Deezer API response, fetched this session]` — fields quoted verbatim. `record_type` values observed/documented include `album`, `single`, `ep`, `compilation` (only `album` seen directly this session; `single`/`ep`/`compilation` are `[CITED: community client docs, not directly observed]` — flag as an assumption if D-12's Phase-4 consumer needs to filter on this field). Nonexistent artist id returns HTTP 200 `{"data":[],"total":0}` — `[VERIFIED: live Deezer API response, fetched this session]`, not an HTTP error, so a poll cycle for a stale/deleted Deezer artist ID degrades gracefully rather than erroring.

### Deezer quota-exceeded error shape (not independently reproduced — community-sourced)
```json
{"error":{"type":"Exception","message":"Quota limit exceeded","code":4}}
```
*Source: `[CITED: community-corroborated across multiple GitHub issues/forums, MEDIUM confidence]` — this session did not deliberately trigger Deezer's rate limit (would have required ~50+ rapid requests, risking a longer-lived throttle against the shared research IP), so this shape is not independently `[VERIFIED]`. Treat as the working assumption; the executor should add a unit test asserting this exact shape is handled once `internal/deezer`'s `httptest.Server`-backed tests are written (CLAUDE.md testing constraint means this can be tested with a fake server regardless).

## State of the Art

Not applicable in the traditional sense — MusicBrainz's `ws/2` API and Deezer's public API are both long-stable, versioned surfaces with no recent breaking changes found during this research. No "old approach → current approach" migration exists here; the risk this phase manages is *accuracy of a stable API*, not *staleness of a fast-moving one*.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Deezer's exact quota-exceeded error JSON shape (`{"error":{"type":"Exception","message":"Quota limit exceeded","code":4}}`) and that it returns HTTP 200 rather than a 4xx/5xx status | Common Pitfalls #2, Code Examples | If Deezer actually returns a non-200 status or a different error body, a client written to only check the in-body `"error"` key would still work (defensive coding covers both cases), but a client that assumes *only* the in-body check is needed (skipping HTTP status checks entirely) could miss a genuine 5xx. Low-severity: recommend checking both HTTP status AND in-body `"error"` key regardless. |
| A2 | Deezer `record_type` values beyond `album` (i.e. `single`, `ep`, `compilation`) — only `album` was directly observed in this session's live call | Code Examples (Deezer artist albums) | If Phase 4's diff logic (out of this phase's scope but a stated future consumer) needs to filter/classify by `record_type`, the exact string values for non-album types should be re-verified with a live call against an artist known to have singles/EPs before Phase 4 planning locks on them. |
| A3 | MusicBrainz's documented per-IP guidance of "~1 request/sec for well-behaved clients" (distinct from the 50/sec anonymous-UA and 300/sec global figures, which were directly read from the official Rate_Limiting page) | Common Pitfalls #1, Standard Stack | `Config.MusicBrainzRateLimitPerSec` already defaults to `1` (`internal/config/config.go:33`), which is consistent with this guidance and was set in an earlier phase — low risk, but if MusicBrainz's actual soft ceiling for identified UAs is materially higher, the default is merely conservative, not wrong. |

**Assumption A1/A2 risk is low-severity** (both degrade to "write slightly more defensive code than strictly necessary," not "the phase's plan is built on a false premise") — no blocking user-confirmation checkpoint is required before planning, but the plan should route Pitfall #2/#3's exact-shape assertions through `httptest.Server`-backed unit tests (already a CLAUDE.md testing requirement) rather than only integration-testing against the live API, so a future Deezer/MusicBrainz response-shape change is caught by CI rather than production 500s.

## Open Questions

1. **Exact `GET /search` query-param shape (result limit, pagination) — explicitly left to planning per CONTEXT.md's Claude's Discretion**
   - What we know: MusicBrainz supports `limit`/`offset` (default 25, max 100); Deezer supports `limit`/`index` (default appears to be 25 based on observed `total: 100` vs `limit=2` request — untested default without an explicit `limit`).
   - What's unclear: Whether the combined `/search` proxy should expose its own `limit` query param (forwarded to both upstreams) or hard-code a small fixed page size (e.g. 10 per source) given this is a live user-facing "type to search" endpoint, not a paged browse UI.
   - Recommendation: Hard-code a small fixed limit per source (e.g. 10) for v1 — CONTEXT.md's D-01 describes this as a single combined search-as-you-type endpoint, not a paginated browse view; a fixed cap avoids over-fetching on every keystroke-triggered search without adding pagination plumbing this phase doesn't need. Planner should treat this as a discretionary implementation detail, not a blocking decision.

2. **Per-request HTTP timeout values for the MusicBrainz/Deezer clients — explicitly left to planning per CONTEXT.md's Claude's Discretion**
   - What we know: `cmd/server/main.go` already sets conservative HTTP *server* timeouts (5s/15s/15s/60s) for inbound requests (WR-02); no existing precedent for outbound client timeouts.
   - What's unclear: The right balance between "long enough that a briefly-slow MusicBrainz/Deezer response doesn't spuriously fail a poll cycle" and "short enough that a hung upstream doesn't stall the sequential poll loop (D-07) for the whole watchlist."
   - Recommendation: 10-second per-request timeout on both clients' `http.Client` (a `context.WithTimeout` wrapping each individual call, or a flat `http.Client{Timeout: 10 * time.Second}`) is a reasonable default matching neither service's documented SLA (none published) but comfortably above normal observed latency for both APIs during this session's live testing (sub-second for every call made).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Building/testing `internal/musicbrainz`, `internal/deezer`, `internal/poller` | ✓ | go1.26.5 windows/amd64 | — |
| Docker (daemon running) | `docker-compose.yml` Postgres fixture for `httptest.Server`-independent integration tests (`testutil.RequirePostgresDSN`) | ✓ | Docker Client 29.6.2, daemon responsive | — |
| Outbound internet access to `musicbrainz.org` / `api.deezer.com` | Live manual verification during dev; NOT required for CI (CLAUDE.md mandates `httptest.Server` mocks, no live calls in CI) | ✓ (verified this session) | — | CI never needs this — tests must mock both APIs regardless (CICD-01) |
| `make` on PATH (bash shell) | Existing `Makefile` targets (`make build`, `make test`, etc.) | Partial — `make.exe` installed via winget (per Phase 01 STATE.md history) but not resolved on this bash session's `PATH` | GNU Make 4.4.1 (per prior phase's install) | Invoke `go build`/`go test`/`go vet` directly, or run `make` from a shell where winget's per-user install path is on `PATH` (already a known Phase 01 environment quirk, not new to this phase) |

**Missing dependencies with no fallback:** none.

**Missing dependencies with fallback:** `make` resolution in this specific bash session — already a documented pre-existing quirk from Phase 01 (see STATE.md's Phase 01-04 entry on GNU Make install), not a new blocker introduced by this phase; direct `go` command invocation is always available as a fallback.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (no third-party test framework — matches Phase 1/2 convention observed in `internal/watchlist/service_test.go`, `internal/httpserver/*_test.go`) |
| Config file | none — plain `go test ./...` |
| Quick run command | `go test -short ./...` (skips DB-backed tests per `testutil.RequirePostgresDSN`'s `testing.Short()` gate, `[VERIFIED: internal/testutil/postgres.go:29-31]`) |
| Full suite command | `TEST_DATABASE_URL=... go test ./...` (or `make test` once `make` is resolved on `PATH`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CLNT-03 | `GET /search?q=` returns combined MusicBrainz + Deezer results | unit (httptest.Server-backed fakes for both upstreams) | `go test ./internal/httpserver/... -run TestSearch -short` | ❌ Wave 0 — `internal/httpserver/search_test.go` |
| CLNT-03 (D-03) | One upstream failing/timing out still returns the other source's results + a per-source error flag | unit | `go test ./internal/httpserver/... -run TestSearch_PartialFailure -short` | ❌ Wave 0 |
| WLST-01 | MusicBrainz artist search returns MBID, name, disambiguation, type (D-11) | unit (httptest.Server fake) | `go test ./internal/musicbrainz/... -short` | ❌ Wave 0 — `internal/musicbrainz/search_test.go` |
| CLNT-01 | MusicBrainz release-groups browse-by-artist, self-rate-limited (D-10) | unit (httptest.Server fake + fake clock or short-interval limiter) | `go test ./internal/musicbrainz/... -run TestReleaseGroups -short` | ❌ Wave 0 — `internal/musicbrainz/releasegroups_test.go` |
| CLNT-02 | Deezer artist-albums fetch, self-rate-limited (D-12) | unit (httptest.Server fake) | `go test ./internal/deezer/... -short` | ❌ Wave 0 — `internal/deezer/albums_test.go` |
| CLNT-01, CLNT-02 | Two independent poll cycles never share a limiter/goroutine; per-source overlap guard skips an overlapping tick (D-08, D-09) | unit (fake clients + manual trigger, no real cron ticks) | `go test ./internal/poller/... -short` | ❌ Wave 0 — `internal/poller/poller_test.go` |
| D-06 | Deezer poll cycle skips artists with nil `deezer_id` without erroring | unit | `go test ./internal/poller/... -run TestDeezerPoll_SkipsNilDeezerID -short` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -short ./...`
- **Per wave merge:** `TEST_DATABASE_URL=... go test ./...` (full suite, DB-backed tests included)
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/musicbrainz/search_test.go`, `internal/musicbrainz/releasegroups_test.go` — httptest.Server fakes for `/ws/2/artist` and `/ws/2/release-group` responses (use the live-verified JSON shapes above as fixtures)
- [ ] `internal/deezer/search_test.go`, `internal/deezer/albums_test.go` — httptest.Server fakes for `/search/artist` and `/artist/{id}/albums`, including a quota-error fixture per Pitfall 2 (Assumption A1)
- [ ] `internal/httpserver/search_test.go` — combined-endpoint test doubles for both `ArtistSearcher` interfaces (D-02 no-dedup, D-03 partial-results)
- [ ] `internal/poller/poller_test.go` — overlap-guard and independent-cycle tests; no framework install needed, all stdlib `testing`

*(No test framework install needed — `go test` is already the project's established tool.)*

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | Neither MusicBrainz nor Deezer's read-only public endpoints used by this phase require API keys/auth (both live-verified this session with zero auth headers) |
| V3 Session Management | No | No session state introduced this phase |
| V4 Access Control | No | `/search` is an unauthenticated read-only proxy, consistent with the rest of this single-operator deployable (PROJECT.md) — no new access-control surface |
| V5 Input Validation | Yes | The `q` query param on `GET /search` must be length-capped and non-empty-validated before being forwarded to either upstream — mirror `internal/httpserver/watchlist.go`'s existing `maxNameRunes`/`utf8`-aware validation pattern (`[VERIFIED: internal/httpserver/watchlist.go:1-46]`, read this session) rather than introducing a new validation idiom |
| V6 Cryptography | No | No secrets/crypto introduced this phase beyond the existing `MusicBrainzUserAgent` (not a secret) and no new webhook/API-key config |
| V13 API and Web Service | Yes | Both external API calls must set explicit timeouts (Open Question 2) and must not leak upstream error bodies verbatim to the `/search` API consumer — return a generic per-source error flag (D-03) instead of forwarding raw MusicBrainz/Deezer error text, consistent with `internal/httpserver/watchlist.go`'s `writeError` pattern of never echoing raw downstream error text (`[VERIFIED: internal/httpserver/watchlist.go:52-60]`) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Unbounded `q` param forwarded to two upstream APIs (resource exhaustion via slow/large search queries) | Denial of Service | Length-cap `q` before forwarding (mirror `maxNameRunes`-style const), and apply the per-request client timeouts from Open Question 2 |
| SSRF via user-controlled input reaching an outbound HTTP client | Tampering / Elevation of Privilege | Not applicable here — both MusicBrainz and Deezer base URLs are hard-coded constants in the client packages (`https://musicbrainz.org/ws/2`, `https://api.deezer.com`), never derived from user input; the `q` param is a query-string *value*, never a URL/host |
| Rate-limit bypass via a client that ignores 503/quota-exceeded and retries unboundedly | Denial of Service (against the upstream, and self-inflicted against MusicBrainz's IP-level throttling) | `rate.Limiter.Wait(ctx)` already self-throttles outbound request *rate*; do not add automatic retry-on-503 logic this phase (out of scope — D-04's poll handler is log-only, no retry/backoff requirement stated) |

## Sources

### Primary (HIGH confidence — live-verified this session)
- `https://musicbrainz.org/ws/2/artist?query=artist:Drake&fmt=json&limit=2` — live response, artist search shape
- `https://musicbrainz.org/ws/2/release-group?artist={mbid}&type=album|ep&fmt=json&limit=3` — live response, release-group browse shape
- `https://api.deezer.com/search/artist?q=Drake&limit=2` — live response, Deezer artist search shape
- `https://api.deezer.com/artist/246791/albums?limit=3` — live response, Deezer artist albums shape
- `https://api.deezer.com/artist/999999999999999/albums` — live response, nonexistent-artist behavior
- `go list -m -versions github.com/robfig/cron/v3` / `go list -m -versions golang.org/x/time` — Go module proxy, version verification
- `[VERIFIED: internal/config/config.go:32-34]`, `[VERIFIED: internal/watchlist/service.go:54,87-92]`, `[VERIFIED: internal/httpserver/server.go:17-73]`, `[VERIFIED: cmd/server/main.go:80-115]` — in-repo files read this session

### Secondary (MEDIUM confidence — official docs / cross-checked)
- `https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting` — official rate-limit figures and User-Agent format, fetched this session, corroborated by a directly-reproduced 503
- `https://musicbrainz.org/doc/MusicBrainz_API` — endpoint/param documentation, fetched this session
- `https://pkg.go.dev/github.com/robfig/cron/v3` — official godoc, fetched this session
- `https://pkg.go.dev/golang.org/x/time/rate` — official godoc, fetched this session

### Tertiary (LOW confidence — community-sourced, not independently verified)
- Deezer quota-exceeded error JSON shape (`{"error":{"type":"Exception",...}}`) — corroborated across multiple GitHub issues/community forums, Deezer's own developer docs are login-gated so this was not confirmed against Deezer's own text
- `record_type` values beyond `album` (single/ep/compilation) — from a third-party Go client (`github.com/stayradiated/deezer`) and community docs, not directly observed in a live response this session (see Assumption A2)

## Metadata

**Confidence breakdown:**
- Standard Stack: HIGH — both new dependencies (`robfig/cron/v3`, `x/time/rate`) already locked in CLAUDE.md and re-verified against the authoritative Go module proxy this session
- Architecture: HIGH — pattern is a direct extension of Phase 1/2's already-established `Pinger`/`watchlist.Store` narrow-interface convention, read from the actual source files this session
- External API mechanics (MusicBrainz/Deezer endpoints, JSON shapes): HIGH for every field/shape quoted from a live response this session (search, release-groups, artist-albums); MEDIUM-LOW for the two items in the Assumptions Log (Deezer quota-error shape, non-album `record_type` values) not independently reproduced
- Pitfalls: HIGH for Pitfall 1 (directly reproduced this session) and Pitfall 3/4 (grounded in in-repo `[VERIFIED]` code reads); MEDIUM for Pitfall 2 (community-corroborated, not independently reproduced)

**Research date:** 2026-08-07
**Valid until:** 2026-09-06 (30 days — both external APIs are stable/versioned with no announced deprecations found; re-verify live response shapes if planning is materially delayed past this window, since neither service publishes a formal versioning/deprecation policy this research surfaced)
