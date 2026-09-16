# Phase 10: Event Retention Window - Pattern Map

**Mapped:** 2026-08-13
**Files analyzed:** 8 (all modifications, no new files)
**Analogs found:** 8 / 8 (all self-analogs — this phase edits the exact files whose existing code is the pattern to extend)

## File Classification

| Modified File | Role | Data Flow | Closest Analog | Match Quality |
|----------------|------|-----------|-----------------|----------------|
| `queries/events.sql` (`ListEvents` stmt) | model (sqlc query) | CRUD (read, always-applied filter) | same file, `ListEvents`'s existing `sqlc.narg` filters + `HasAnyEvent`'s `EXISTS` idiom | exact (extend existing statement) |
| `internal/config/config.go` | config | request-response (boot-time parse) | same file, `DatabaseURL notEmpty` fail-fast field | exact |
| `internal/events/service.go` | service | CRUD | same file, `Service.List`'s existing `PageSize` clamp | exact |
| `internal/httpserver/events.go` | controller | request-response | same file, `eventsResponse` envelope + `handleListEvents` | exact |
| `cmd/server/main.go` (`events.NewService` call site) | config/wiring | request-response | same file, existing `events.NewService(sqlc.New(pool))` call | exact |
| `internal/httpserver/boot_e2e_test.go` (`events.NewService` call site) | test | request-response | same file, existing call site | exact |
| `web/app/routes/history.tsx` | component | request-response | same file, existing `isFiltered` ternary + two `EmptyState` calls | exact |
| `web/app/lib/api.ts` (`EventsPage` type) | utility (typed fetch wrapper) | request-response | same file, `EventsPage` interface | exact |

No genuinely new role/data-flow combination is introduced by this phase — every file is a targeted extension of an existing, already-idiomatic pattern in the same file. `internal/config/config_test.go` and `internal/httpserver/events_test.go` are the test analogs (see below) but are not themselves required edits per CONTEXT.md; planner should still extend them per RESEARCH.md's Wave 0 Gaps.

## Pattern Assignments

### `queries/events.sql` (controller-adjacent: sqlc query, `ListEvents` statement)

**Analog:** same file — `ListEvents`'s existing `sqlc.narg` filter idiom (lines 92-100) and `HasAnyEvent`'s `EXISTS` idiom (lines 27-32)

**Existing optional-filter pattern** (lines 96-98):
```sql
WHERE (sqlc.narg('artist_id')::bigint IS NULL OR artist_id = sqlc.narg('artist_id')::bigint)
  AND (sqlc.narg('event_type')::text IS NULL OR event_type = sqlc.narg('event_type')::text)
  AND (sqlc.narg('cursor')::bigint IS NULL OR id < sqlc.narg('cursor')::bigint)
ORDER BY id DESC
LIMIT sqlc.arg('page_size');
```

**Pattern to apply:** the new cutoff predicate is NEVER optional, so it must use `sqlc.arg` (required), not `sqlc.narg` — do not copy the `IS NULL OR` idiom for this one predicate:
```sql
  AND created_at >= sqlc.arg('cutoff')::timestamptz
```
Append this line after the existing three `AND` clauses, before `ORDER BY`.

**Existing `EXISTS` idiom for D-06's "has older events" signal** (lines 30-32):
```sql
SELECT EXISTS(
    SELECT 1 FROM events WHERE artist_id = $1 AND source = $2
) AS has_any;
```
Follow this exact shape for a new `HasOlderEvents`-style query (name/shape at Claude's discretion per CONTEXT.md D-06), scoped by the same `artist_id`/`event_type` optional filters as `ListEvents` but with `created_at < cutoff`.

**Commenting convention to preserve** (lines 21-24, 27-29, 34-40, 42-51 all explain why each query is/isn't filtered): `ListEvents`'s own comment block (lines 76-91) must be extended to explain why IT is filtered by retention while `ListExternalIDs`/`HasAnyEvent`/`GroupTrackCountBaseline`/`ListUnnotified` are explicitly not — this is Pitfall 4 from RESEARCH.md, not optional.

---

### `internal/config/config.go` (config)

**Analog:** same file — `DatabaseURL`'s `notEmpty` fail-fast field (line 19) and `Load()`'s aggregate-error passthrough (lines 40-46)

**Existing field-grouping-by-phase convention** (lines 17-35):
```go
type Config struct {
	// Phase 1 — required, must never boot half-configured.
	DatabaseURL string `env:"DATABASE_URL,notEmpty"`
	HTTPPort    int    `env:"HTTP_PORT" envDefault:"8080"`
	...
	// Phase 3-5 — optional, sane defaults, never `notEmpty`/`required`.
	DiscordWebhookURL string        `env:"DISCORD_WEBHOOK_URL"`
	PollInterval      time.Duration `env:"POLL_INTERVAL" envDefault:"15m"`
	...
}
```

**Pattern to apply:** add a new `// Phase 10 —` grouping comment block with `EventRetentionDays int \`env:"EVENT_RETENTION_DAYS" envDefault:"90"\``, matching the existing per-phase-comment convention exactly (D-02 — plain `int`, not `time.Duration`, unlike `PollInterval`).

**Existing `Load()` shape** (lines 40-46):
```go
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
```

**Validation pattern to add** (no existing numeric-minimum check in this file — `caarlos0/env/v11` has no built-in tag for it, confirmed by RESEARCH.md): add a manual post-`env.Parse` check before the final `return cfg, nil`:
```go
if cfg.EventRetentionDays <= 0 {
	return nil, fmt.Errorf("EVENT_RETENTION_DAYS must be > 0, got %d", cfg.EventRetentionDays)
}
```
Requires adding `"fmt"` to the import block (currently only `"time"` and `caarlos0/env`).

**Test analog:** `internal/config/config_test.go` — `TestLoad_Defaults` (lines 39-71), `TestLoad_EmptyRequired`/`TestLoad_MissingRequired` (lines 115-137) are the templates for new `TestLoad_*` cases covering default=90, override, and `<=0` rejection. `TestEnvExampleCompleteness` (lines 255-268) will fail until `.env.example` gets an `EVENT_RETENTION_DAYS=90` line — do this in the same commit (Pitfall 1).

---

### `internal/events/service.go` (service)

**Analog:** same file — `Service.List`'s existing `PageSize` clamp (lines 95-102) and `NewService` constructor (lines 81-84)

**Existing constructor + struct shape** (lines 76-84):
```go
// Service is the sqlc-backed implementation of Store.
type Service struct {
	q sqlc.Querier
}

// NewService builds a Service backed by q.
func NewService(q sqlc.Querier) *Service {
	return &Service{q: q}
}
```

**Pattern to apply:** widen to carry `retentionDays int`:
```go
type Service struct {
	q             sqlc.Querier
	retentionDays int
}

func NewService(q sqlc.Querier, retentionDays int) *Service {
	return &Service{q: q, retentionDays: retentionDays}
}
```

**Existing clamp-in-service-layer pattern** (lines 95-109) — this is the direct precedent for where cutoff computation belongs (Go service layer, not HTTP boundary, not SQL `now()`):
```go
func (s *Service) List(ctx context.Context, p ListParams) (Page, error) {
	pageSize := p.PageSize
	switch {
	case pageSize <= 0:
		pageSize = DefaultPageSize
	case pageSize > MaxPageSize:
		pageSize = MaxPageSize
	}

	rows, err := s.q.ListEvents(ctx, sqlc.ListEventsParams{
		ArtistID:  p.ArtistID,
		EventType: p.EventType,
		Cursor:    p.Cursor,
		PageSize:  pageSize,
	})
	...
}
```

**Pattern to apply:** add cutoff computation alongside the pageSize clamp, using the codebase's own `pgtype.Timestamptz` construction precedent from `internal/detection/detector.go:97-102` (`seedNotifiedAt`):
```go
func seedNotifiedAt(seedMode bool) pgtype.Timestamptz {
	if !seedMode {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
}
```
Applied shape for the cutoff (always `Valid: true` — Pitfall 5 warns a zero-value `pgtype.Timestamptz{}` silently becomes SQL `NULL`, which makes `created_at >= NULL` never true and empties the whole feed):
```go
cutoff := pgtype.Timestamptz{
	Time:  time.Now().Add(-time.Duration(s.retentionDays) * 24 * time.Hour),
	Valid: true,
}
```
Then thread `Cutoff: cutoff` into the `sqlc.ListEventsParams{...}` call alongside the existing three fields, and add `Page.HasOlderEvents bool` (or chosen name) to the `Page` struct (lines 63-66), computed via a second query call following the `EXISTS`-based query added to `queries/events.sql` above.

**`toEvent` / row-conversion pattern for reference** (lines 132-154) — unaffected by this phase, included only because `CreatedAt: row.CreatedAt.Time` (line 147) confirms the codebase's existing `pgtype.Timestamptz` → `time.Time` unwrap convention, the same type family the new cutoff parameter uses.

---

### `internal/httpserver/events.go` (controller)

**Analog:** same file — `eventsResponse` envelope (lines 17-25) and `handleListEvents` (lines 72-120)

**Existing envelope shape** (lines 22-25):
```go
type eventsResponse struct {
	Events     []events.Event `json:"events"`
	NextCursor *int64         `json:"next_cursor"`
}
```

**Pattern to apply (D-06):** add `HasOlderEvents bool \`json:"has_older_events"\`` (snake_case matches every existing field in this envelope).

**Existing handler wiring** (lines 100-119):
```go
page, err := s.events.List(r.Context(), events.ListParams{
	ArtistID:  artistID,
	EventType: eventType,
	Cursor:    cursor,
	PageSize:  pageSize,
})
if err != nil {
	httplog.SetAttrs(r.Context(), slog.String("events_error", err.Error()))
	writeError(w, http.StatusInternalServerError, "internal error")
	return
}

evs := page.Events
if evs == nil {
	evs = []events.Event{}
}

w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusOK)
_ = json.NewEncoder(w).Encode(eventsResponse{Events: evs, NextCursor: page.NextCursor})
```
Pattern to apply: no new error-handling branch needed — `page.HasOlderEvents` flows straight through from `Service.List`'s existing return value into the final `eventsResponse{...}` literal, same as `NextCursor` does today.

**Error handling pattern (unchanged, apply as-is to any new code path):** every store error in this handler follows the same log-then-fixed-500-message shape (line 106-109) — no new error class is introduced by this phase.

---

### `cmd/server/main.go` / `internal/httpserver/boot_e2e_test.go` (wiring / test)

**Analog:** RESEARCH.md Pattern 3 — exactly two call sites construct a real `*events.Service`, both currently `events.NewService(sqlc.New(pool))`. Both need the new `retentionDays` argument: `events.NewService(sqlc.New(pool), cfg.EventRetentionDays)` at `cmd/server/main.go:104`, and an equivalent literal int at `internal/httpserver/boot_e2e_test.go:54`. Every other `httpserver.New(...)` call site (11 files) passes `stubEventsStore{}` directly and is untouched — do not widen `httpserver.New`'s signature.

---

### `web/app/routes/history.tsx` (component)

**Analog:** same file — existing `isFiltered` ternary and two `EmptyState` calls (lines 122, 150-159)

**Existing empty-state branch** (lines 150-159):
```tsx
{!error && !initialLoading && events.length === 0 && (
  <EmptyState
    heading={isFiltered ? "No matching events" : "No release activity yet"}
    body={
      isFiltered
        ? "Try a different artist or event type."
        : "Add an artist to your watchlist to start tracking new releases, features, and deluxe editions."
    }
  />
)}
```

**Pattern to apply (D-05/D-06):** extend the same `EmptyState` component (no new component needed — its own header comment at `web/app/components/common/EmptyState.tsx:6-8` already anticipates "filtered-to-zero History" as a shared use case) with a third branch keyed off a new `hasOlderEvents` state value threaded from `page.has_older_events` (mirrors how `nextCursor` is threaded from `page.next_cursor` at line 78). Priority order: `isFiltered` check first (existing), then `hasOlderEvents` (new), then the plain "No release activity yet" fallback — since `isFiltered` and retention-emptiness are mutually informative but `isFiltered` should still take visual precedence when both are true (a user-applied filter is the more actionable explanation).

**State-threading precedent** (lines 53-54, 77-78):
```tsx
const [events, setEvents] = useState<EventItem[]>([])
const [nextCursor, setNextCursor] = useState<number | null>(null)
...
setEvents(page.events)
setNextCursor(page.next_cursor)
```
Add `const [hasOlderEvents, setHasOlderEvents] = useState(false)` and `setHasOlderEvents(page.has_older_events)` alongside these, in both the initial-fetch effect (line 77-79) and `handleLoadMore` (line 99-108) — though only the initial-fetch value is relevant to the empty-state branch itself.

**Test analog:** `web/app/routes/history.test.tsx` (referenced in RESEARCH.md, not yet read this session — existing file, extend with the new empty-state case per Wave 0 Gaps).

---

### `web/app/lib/api.ts` (`EventsPage` type)

**Analog:** same file — `EventsPage` interface (lines 32-35)

**Existing shape:**
```ts
export interface EventsPage {
  events: EventItem[]
  next_cursor: number | null
}
```

**Pattern to apply:** add `has_older_events: boolean` matching the new Go `eventsResponse.HasOlderEvents` field — this file's own header comment (lines 1-7) states every wire shape here is typed directly against real Go response bodies, not guessed, so this edit must mirror `internal/httpserver/events.go`'s field exactly (same name, same nullability — non-nullable bool, no `| null`).

---

## Shared Patterns

### Fail-fast config validation
**Source:** `internal/config/config.go` line 19 (`DatabaseURL string \`env:"DATABASE_URL,notEmpty"\``) + `Load()`'s aggregate-error passthrough
**Apply to:** `EVENT_RETENTION_DAYS` — same posture (D-03): invalid config aborts boot with a clear, field-naming error, never silently reinterpreted. Since `caarlos0/env/v11` has no built-in numeric-minimum tag, this is a manual `if cfg.EventRetentionDays <= 0 { return nil, fmt.Errorf(...) }` check in `Load()`, not a struct tag.

### `pgtype.Timestamptz` construction
**Source:** `internal/detection/detector.go:97-102` (`seedNotifiedAt`)
**Apply to:** `internal/events/service.go`'s new cutoff parameter — always construct with `Valid: true` explicitly; the zero-value `pgtype.Timestamptz{}` silently means SQL `NULL`, which combined with `created_at >= NULL` (never true in standard SQL) would empty the entire History feed with no loud error (Pitfall 5).

### `sqlc.arg` (required) vs `sqlc.narg` (optional) filter idiom
**Source:** `queries/events.sql`'s existing `ListEvents` statement (lines 96-98, optional) vs `LIMIT sqlc.arg('page_size')` (line 100, required)
**Apply to:** the new `created_at >= sqlc.arg('cutoff')::timestamptz` predicate — must use `sqlc.arg`, never `sqlc.narg`, since the retention filter is never optional (using `narg` would allow a caller to pass a nil cutoff and silently bypass the filter, undermining DATA-02).

### Domain-service-layer validation/clamping, never HTTP-boundary or SQL-embedded
**Source:** `internal/events/service.go`'s existing `PageSize` clamp (lines 96-102), explicitly called out in this file's own doc comment (line 90: "PageSize is clamped here — not at the HTTP boundary")
**Apply to:** cutoff computation belongs in `events.Service.List`, not in `internal/httpserver/events.go` and not as SQL `now() - interval`. Keeps the codebase's single validation/clamping layer convention intact and keeps the cutoff boundary unit-testable in Go without a live Postgres clock dependency.

### `EXISTS(...)` short-circuit idiom for existence checks
**Source:** `queries/events.sql`'s `HasAnyEvent` (lines 30-32)
**Apply to:** the new "are there older events" query (D-06) — `SELECT EXISTS(SELECT 1 FROM events WHERE <same filters> AND created_at < cutoff LIMIT 1)`, avoiding a full-scan `COUNT`.

### Comment-driven "why this query is/isn't filtered" convention
**Source:** every existing query in `queries/events.sql` (`ListExternalIDs`, `HasAnyEvent`, `ListUnnotified`, `GroupTrackCountBaseline` — lines 21-65) each carries an explanatory comment about its scope/safety invariant
**Apply to:** `ListEvents`'s comment block must be extended to explain why it alone gets the retention filter, and the four untouched queries' existing comments should not be modified — this is the guardrail against Pitfall 4 (accidentally "fixing" the other four queries to also filter by retention).

## No Analog Found

None. Every file this phase touches already has a directly-applicable, in-file precedent to extend (see Pattern Assignments above) — this is a small, mechanically well-precedented phase per RESEARCH.md's own framing ("not a research-an-unfamiliar-framework phase").

## Metadata

**Analog search scope:** `queries/events.sql`, `internal/config/config.go` (+ `config_test.go`), `internal/events/service.go`, `internal/httpserver/events.go`, `internal/detection/detector.go` (for `pgtype.Timestamptz` precedent), `cmd/server/main.go`, `internal/httpserver/boot_e2e_test.go`, `web/app/routes/history.tsx`, `web/app/components/common/EmptyState.tsx`, `web/app/lib/api.ts`
**Files scanned:** 10 (all read directly this session; no repo-wide search needed since RESEARCH.md already pinned every file/line reference precisely)
**Pattern extraction date:** 2026-08-13
