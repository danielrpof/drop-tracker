# Phase 22: Scheduled Digest Send - Pattern Map

**Mapped:** 2026-09-16
**Files analyzed:** 9
**Analogs found:** 9 / 9

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/notifier/scheduler.go` (new, `DigestScheduler`) | service (lifecycle) | event-driven | `internal/poller/poller.go` (`Poller.Start`/`Stop`) | exact |
| `internal/notifier/digest.go` (new, `SendDigestIfDue`, digest-send sequence) | service | batch | `internal/notifier/notifier.go` (`NotifyPending`) | exact |
| `internal/notifier/digest_format.go` (new, digest message/embed builder) | transform/utility | transform | `internal/notifier/format.go` | exact |
| `internal/settings/settings.go` (modified: slot math, grace consts, clock, re-anchor) | service/model | CRUD | `internal/settings/settings.go` (existing `Cadence`/`Service`) | exact (self-extend) |
| `internal/db/migrations/000009_digest_last_slot_at.up.sql` (+ `.down.sql`) | migration | CRUD | `internal/db/migrations/000008_notification_settings.up.sql` | exact |
| `queries/notification_settings.sql` (modified: `UpdateNotificationSettings` CASE re-anchor, new ack statement) | model (sqlc query) | CRUD | `queries/notification_settings.sql` (existing), `queries/events.sql` (`AdvanceGroupTrackCountBaseline` CTE, `MarkNotified`) | exact |
| `internal/discord/client.go` | (unchanged — reused as-is) | request-response | n/a — `Client.Send(ctx, Embed)` reused verbatim | exact |
| `cmd/server/main.go` (modified: `time/tzdata` blank import, zone fail-fast, scheduler wiring + drain defer) | config/composition-root | event-driven | same file's existing poller wiring (lines ~299–328) | exact |
| `.github/workflows/full-pipeline.yml` (`build-scan` step) | config (CI) | batch | same file's `n1-boot` job (~line 380) | role-match |

## Pattern Assignments

### `internal/notifier/scheduler.go` (service, event-driven)

**Analog:** `internal/poller/poller.go`

**Imports pattern** (poller.go lines 17-32):
```go
import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)
```
`DigestScheduler` needs no `robfig/cron` import — D-18 explicitly rejects a third cron entry in favor of a `time.Ticker`. Drop the `cron` import; keep `context`/`log/slog`/`sync/atomic`/`time`.

**Lifecycle pattern to copy** (poller.go lines 264-295, `Start`/`Stop`):
```go
func (p *Poller) Start(ctx context.Context) {
	p.runCtx, p.runCancel = context.WithCancel(ctx)
	p.logger.Info("poller starting", slog.Duration("interval", p.interval))
	p.cron.Start()
}

func (p *Poller) Stop(ctx context.Context) error {
	p.logger.Info("poller stopping")
	stopCtx := p.cron.Stop()
	select {
	case <-stopCtx.Done():
		return nil
	case <-ctx.Done():
		if p.runCancel != nil {
			p.runCancel()
		}
		return ctx.Err()
	}
}
```
Copy the shape: retained child ctx set in `Start`, `Stop(drainCtx)` bounded-wait-then-cancel-on-timeout. Since `DigestScheduler` has no `cron.Cron` to `.Stop()`, replace `p.cron.Stop()`'s returned `stopCtx` with your own goroutine-done channel closed when the ticker loop exits (e.g. a `done chan struct{}` closed via `defer close(done)` inside the loop goroutine), and select on that instead of `stopCtx.Done()`.

**Ticker + immediate-first-tick pattern (D-10):** no existing analog fires immediately (poller's cron `@every` waits out the first interval; `authgate.Manager.sweepLoop` is explicitly excluded per CONTEXT.md). Implement fresh: run one `checkDue` call synchronously right after `Start` launches the goroutine, then loop on `time.NewTicker(dueCheckInterval).C` inside a `select` against `runCtx.Done()`.

**Injected clock (D-15/D-18):** mirror `spacingWait`'s test-seam idiom from notifier.go (line 39): `var spacingWait = time.After` — a package-level var, not a hardcoded call, so tests substitute it. `DigestScheduler` should take `now func() time.Time` (and ideally a tick source) as constructor args, not package vars, since fake-clock DST tests need to drive multiple independent instances.

---

### `internal/notifier/digest.go` (service, batch)

**Analog:** `internal/notifier/notifier.go` — reuse directly, don't duplicate

**Shared lock/read/list/suppress pattern to reuse verbatim** (notifier.go lines 41-56, 130-227):
```go
type Sender interface {
	Send(ctx context.Context, embed discord.Embed) error
}

type SettingsReader interface {
	Get(ctx context.Context) (settings.Settings, error)
}
```
`SendDigestIfDue` is a new method on the existing `*Notifier` struct (D-18), so it shares `n.q`, `n.sender`, `n.settingsReader`, `n.notifying`, and the existing `suppresses`/`listUnnotified`/`readSettings`/`logSettingsReadFailure` helpers unchanged. Add `SendDigestIfDue` to the `Sink` interface (notifier.go lines 61-73):
```go
type Sink interface {
	NotifyPending(ctx context.Context, logger *slog.Logger) error
}
```
becomes:
```go
type Sink interface {
	NotifyPending(ctx context.Context, logger *slog.Logger) error
	SendDigestIfDue(ctx context.Context, logger *slog.Logger, now time.Time) error
}
```
`NoOp.SendDigestIfDue` returns nil, mirroring `NoOp.NotifyPending` (notifier.go line 73).

**CAS lock pattern to copy** (notifier.go lines 236-240):
```go
if !n.notifying.CompareAndSwap(false, true) {
	logger.Info("skipping notify pass: already in progress")
	return nil
}
defer n.notifying.Store(false)
```
Same guard, different log line ("skipping digest send: already in progress" / D-17 step 1, D-24 Info level).

**Fail-closed settings read** (notifier.go lines 246-250, `readSettings`/`logSettingsReadFailure` at lines 190-207): reuse unchanged for D-17 step 2.

**dbOpTimeout / context.WithoutCancel pattern** (notifier.go lines 30-34, 168-175, 180-184): D-16's ack needs `context.WithoutCancel(ctx)` bounded by `dbOpTimeout` — extend the existing `listUnnotified`/`markNotified` helper shape:
```go
func listUnnotified(ctx context.Context, q sqlc.Querier) ([]sqlc.Event, error) {
	opCtx, cancel := context.WithTimeout(ctx, dbOpTimeout)
	defer cancel()
	return q.ListUnnotified(opCtx)
}
```
New `ackDigest` helper follows the same shape but wraps `context.WithoutCancel(ctx)` first, per D-16 ("so a shutdown landing after Discord's 2xx still acks instead of re-sending a whole batch"):
```go
func ackDigestBatch(ctx context.Context, q sqlc.Querier, ids []int64, slot time.Time, sentAt *time.Time) error {
	opCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dbOpTimeout)
	defer cancel()
	_, err := q.AckDigestBatch(opCtx, sqlc.AckDigestBatchParams{ /* ... */ })
	return err
}
```

**Sequence to implement (D-17, no direct analog — new orchestration):**
1. CAS `notifying` (copy above).
2. `readSettings` fail-closed (reuse).
3. `listUnnotified` + partition via `n.suppresses(ev)` (reuse `suppresses`, notifier.go lines 148-155).
4. Nothing sendable → ack suppressed ids + record slot only, log empty skip (D-24 Info).
5. Build message (`digest_format.go`) — re-read settings immediately before POST (same re-check idiom as notifier.go lines 284-293).
6. `n.sender.Send(ctx, embed)` (reuse `Sender` interface, `discord.Client.Send` untouched).
7. 2xx → `ackDigestBatch`; failure → return, write nothing (mirrors notifier.go's per-send error handling at lines 296-313, but batched instead of per-row).

---

### `internal/notifier/digest_format.go` (transform)

**Analog:** `internal/notifier/format.go`

**Reused helpers verbatim:**
- `truncateRunes` (format.go lines 199-209) — reuse for the ~100-rune title cap (D-21).
- `tracksFieldValue` (format.go lines 125-140) — reuse for D-26's `(12 → 15 tracks)` suffix, empty string omits the suffix entirely (matches D-26 "never `()`").
- URL helpers `newReleaseURL`, `musicBrainzRecordingURL`, `musicBrainzReleaseURL` (format.go lines 155-184) — reuse for D-07's linked lines; per D-07 extract per-event-type URL selection out of `formatEmbed`'s switch (format.go lines 58-73) into a small helper the digest builder also calls, rather than duplicating the switch.

**Event-type constants to reuse** (format.go lines 18-22):
```go
const (
	eventTypeNewRelease   = "new_release"
	eventTypeGuestFeature = "guest_feature"
	eventTypeDeluxeChange = "deluxe_change"
)
```
Group by these three for D-04's fixed headings.

**New pattern (no analog): markdown escaping (D-21).** No existing sanitizer in the codebase — `formatEmbed`'s fields go into Discord's structured `Fields`/`Title`, never markdown Description text, so no escaping was previously needed. Write a new `escapeMarkdown(s string) string` backslash-escaping `` \ * _ ~ ` | > # [ ] ( ) `` before values are interpolated into `[Artist — Title](url)` link text. Table-test against `[Deluxe]`, `(Remix)`, `*`, `_`, `\` per D-21.

**New pattern (no analog): collation sort (D-22).** `golang.org/x/text/collate` is not yet imported anywhere in the repo (`go.mod` currently has it indirect only — promote to direct, no new module, per D-22). No existing sort-by-display-name code to copy from; use `collate.New(language.Und, collate.IgnoreCase)` and its `.CompareString` (or `.Buffer`/`Compare`) as the `sort.Slice` comparator, tie-broken by title then event id.

**Description assembly (D-05):** build a single `strings.Builder` producing `**New Releases**\n- [..](..)\n**Guest Features**\n...`, omitting a heading whose group is empty — no existing multi-section Description builder in the codebase; new code, but the `discord.Embed{Description: ...}` field is already defined and unused elsewhere (`internal/discord/client.go` line 40) so no `discord` package change is needed.

---

### `internal/settings/settings.go` (service/model, CRUD) — modified in place

**Analog:** itself — extend the existing `Service`/`Cadence` (settings.go lines 18-129)

**Existing Cadence type to build slot math against** (lines 18-46):
```go
type Cadence string
const (
	CadenceDaily  Cadence = "daily"
	CadenceWeekly Cadence = "weekly"
)
```
Add (D-15): `MostRecentSlot(now time.Time, cadence Cadence, loc *time.Location) time.Time`, grace-window constants:
```go
const (
	digestGraceDaily  = 12 * time.Hour  // D-12: a brief outage still delivers promptly
	digestGraceWeekly = 48 * time.Hour  // D-12: a day-long outage doesn't surprise-fire mid-afternoon
)
```
and the fixed zone name constant (D-01): `const digestZoneName = "America/New_York"`.

**Injectable clock (D-15):** `Service` currently has no clock field (settings.go lines 74-81, `Service{q sqlc.Querier}`). Add `now func() time.Time` defaulted to `time.Now` in `NewService`, mirroring how `notifier.Notifier` takes constructor args rather than reading `time.Now()` inline — `Service.Update` needs it to compute `$slot` for D-14's re-anchor.

**Update method to extend** (lines 98-112) — `Service.Update` currently:
```go
func (s *Service) Update(ctx context.Context, p UpdateParams) (Settings, error) {
	cadence, err := ParseCadence(string(p.DigestCadence))
	if err != nil { return Settings{}, err }
	row, err := s.q.UpdateNotificationSettings(ctx, sqlc.UpdateNotificationSettingsParams{
		DigestEnabled: p.DigestEnabled,
		DigestCadence: string(cadence),
	})
	...
}
```
D-14 requires `Service.Update` to compute the re-anchor slot itself (`MostRecentSlot(s.now(), cadence, loc)`) and pass it as a new query param — `UpdateParams`/HTTP contract stay unchanged per D-14, only the internal sqlc call gains a param.

**toSettings mapper pattern** (lines 117-129) — same nullable-timestamp unwrap idiom (`row.DigestLastSentAt.Valid` → `*time.Time`) should be copied for exposing `digest_last_slot_at` internally if `Settings` needs it (likely notifier-internal only, not API-facing per D-13's "slot record kept distinct" — check whether `Settings` struct needs a new field or whether `SendDigestIfDue` reads the sqlc row directly).

---

### `internal/db/migrations/000009_digest_last_slot_at.up.sql` (migration)

**Analog:** `internal/db/migrations/000008_notification_settings.up.sql`

**Pattern to copy** (000008 lines 1-13): a short header comment naming the phase/decision, then a plain `ALTER TABLE` for the additive nullable column (D-13: "Additive nullable column: clean under `cmd/migration-check` and N-1 boot"):
```sql
-- Phase 22 (D-13): the slot record -- "the most recent scheduled fire this
-- singleton row has handled" -- kept distinct from digest_last_sent_at (last
-- successful send). Nullable, no default: never handled until the first
-- due-check writes it.
ALTER TABLE notification_settings ADD COLUMN digest_last_slot_at timestamptz;
```
Read `internal/db/migrations/README.md` first per CONTEXT.md's canonical refs — it governs naming/up-down pairing conventions this repo enforces.

---

### `queries/notification_settings.sql` (model/sqlc)

**Analog:** itself (existing `UpdateNotificationSettings`) + `queries/events.sql`'s `AdvanceGroupTrackCountBaseline` for the CTE idiom, `MarkNotified` for the idempotent-ack `WHERE ... IS NULL` predicate.

**D-14's CASE re-anchor** — extend the existing plain positional UPDATE (notification_settings.sql lines 4-11):
```sql
UPDATE notification_settings
SET digest_enabled = $1,
    digest_cadence  = $2,
    updated_at      = now()
WHERE id = 1
RETURNING *;
```
add a new positional param for the computed slot and a `CASE` per D-14:
```sql
UPDATE notification_settings
SET digest_enabled     = $1,
    digest_cadence      = $2,
    digest_last_slot_at = CASE
        WHEN $1 AND (NOT digest_enabled OR digest_cadence <> $2) THEN $3
        ELSE digest_last_slot_at
    END,
    updated_at          = now()
WHERE id = 1
RETURNING *;
```
(Note the CASE reads the *old* `digest_enabled`/`digest_cadence` — the SET list evaluates RHS against pre-update row values in Postgres, matching D-14's parenthetical "the right-hand `digest_enabled`/`digest_cadence` are the old values".)

**D-16's single-statement batch ack** — model the CTE-plus-UPDATE idiom on `AdvanceGroupTrackCountBaseline` (events.sql, the `WITH existing AS (...) UPDATE ... FROM existing ... RETURNING` shape) and the idempotent predicate from `MarkNotified` (events.sql: `UPDATE events SET notified_at = now() WHERE id = $1 AND notified_at IS NULL`). New statement acks `events` (`WHERE id = ANY($ids) AND notified_at IS NULL`) and updates the singleton settings row's `digest_last_slot_at` (always) and `digest_last_sent_at` (only when sent, via `COALESCE`/nullable param per D-16) in one data-modifying CTE — e.g.:
```sql
-- name: AckDigestBatch :exec
WITH acked AS (
    UPDATE events SET notified_at = now()
    WHERE id = ANY(sqlc.arg('ids')::bigint[]) AND notified_at IS NULL
    RETURNING id
)
UPDATE notification_settings
SET digest_last_slot_at = sqlc.arg('slot')::timestamptz,
    digest_last_sent_at = COALESCE(sqlc.narg('sent_at')::timestamptz, digest_last_sent_at)
WHERE id = 1;
```
Runs through `sqlc.Querier` directly (D-16: "the watermark write therefore goes through `Querier` directly, not through `settings.Service`") — `internal/db/sqlc/db.go`'s note that `WithTx` exists only on `*Queries` is why this stays one statement rather than a Go transaction.

---

### `cmd/server/main.go` (composition root)

**Analog:** same file's existing poller wiring block (lines 251-328)

**Pattern to copy for scheduler wiring** (mirrors lines 302-328 exactly): construct the scheduler after `notif := notifier.Select(...)`, `Start(ctx)` it, and defer its `Stop(drainCtx)` **after** `defer pool.Close()` (LIFO — copy the comment style at lines 315-321 verbatim in intent):
```go
digestSched := notifier.NewDigestScheduler(notif, logger, time.Now)
digestSched.Start(ctx)
defer func() {
	drainCtx, cancel := context.WithTimeout(context.Background(), pollDrainTimeout)
	defer cancel()
	if err := digestSched.Stop(drainCtx); err != nil {
		logger.Error("digest scheduler drain failed", "scheduler_error", err.Error())
	}
}()
```

**New pattern (no analog): `time/tzdata` blank import + zone fail-fast (D-23).** No existing zone-loading code in this file. Add to the import block (main.go lines 6-36):
```go
import (
	_ "time/tzdata"
	...
)
```
and near startup (before the scheduler is built), a fail-fast load with no fallback:
```go
loc, err := time.LoadLocation("America/New_York")
if err != nil {
	return fmt.Errorf("load America/New_York zone: %w", err)
}
logger.Info("digest zone resolved", slog.String("zone", loc.String()), slog.String("offset", time.Now().In(loc).Format("-07:00")))
```

---

### `.github/workflows/full-pipeline.yml` `build-scan` step

**Analog:** the `n1-boot` job (~line 380) in the same file — reuse its Postgres bring-up pattern (service container + boot + log assertion) for D-23's new step, which additionally greps the just-built image's boot log for the zone-resolved Info line.

## Shared Patterns

### Narrow consumer-declared seams
**Source:** `internal/notifier/notifier.go` lines 41-56 (`Sender`, `SettingsReader`), `internal/poller/poller.go` lines 62-102 (`ReleaseGroupSource`, `AlbumSource`, `EventRecorder`, `Notifier`)
**Apply to:** `DigestScheduler`'s dependency on `Sink` (extend, don't type-assert — D-18), any new interface the digest path needs.
```go
type Sink interface {
	NotifyPending(ctx context.Context, logger *slog.Logger) error
	SendDigestIfDue(ctx context.Context, logger *slog.Logger, now time.Time) error
}
```

### CAS lock, not mutex, for overlap/serialization guards
**Source:** `internal/notifier/notifier.go` lines 79-95, 236-240; `internal/poller/poller.go` lines 193-194, 359-364
**Apply to:** `SendDigestIfDue` reuses the same `notifying atomic.Bool` field already on `*Notifier` — do not add a second lock.

### Bounded per-call DB timeout + `context.WithoutCancel` for post-2xx acks
**Source:** `internal/notifier/notifier.go` lines 30-34, 168-194
**Apply to:** `ackDigestBatch` (D-16) — wrap `context.WithoutCancel(ctx)` first, then the existing `dbOpTimeout` pattern.

### Fail-closed settings read with shared Warn literal
**Source:** `internal/notifier/notifier.go` lines 196-207
**Apply to:** `SendDigestIfDue`'s settings reads (D-17 steps 2 and 5) — call `readSettings`/`logSettingsReadFailure` unchanged.

### slog structured summary lines, not per-row
**Source:** `internal/notifier/notifier.go` lines 328-338, `internal/poller/poller.go` lines 499-502
**Apply to:** D-24's due/sent/empty-skip/grace-expired/lock-held log lines — one line per decision, fields not free text.

### Lifecycle Start/Stop with retained child ctx and bounded drain
**Source:** `internal/poller/poller.go` lines 264-295
**Apply to:** `DigestScheduler.Start`/`Stop` (D-18) — see Pattern Assignments above for the one required divergence (no `cron.Cron`, so `Stop` needs its own done-channel instead of `cron.Stop()`'s returned context).

### Functional options for construction, deliberately not used for fixed constants
**Source:** `internal/notifier/notifier.go` lines 97-105 (`Option`, `WithMaxReleaseAgeDays`); `internal/poller/poller.go` lines 126-157
**Apply to:** D-08 explicitly rejects an env-configurable due-check interval — hardcode `dueCheckInterval = 5 * time.Minute` as an unexported const, not an `Option`.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| Markdown-escaping helper (`escapeMarkdown`) | utility | transform | No existing sanitizer — `formatEmbed` never puts community text into markdown-parsed Description text, only structured Fields. New code, informed by D-21's character list. |
| `golang.org/x/text/collate` sort comparator | utility | transform | No existing locale-aware sort in the codebase; `x/text` is only an indirect dependency today. New code per D-22. |
| Slot-math calendar functions (`MostRecentSlot`) | utility (in `internal/settings`) | transform | No existing "compute the Nth fixed local-time instant" logic anywhere; closest neighbor is `internal/notifier/notifier.go`'s `staleReleaseDate`/`suppresses` cutoff math, but that is plain date-string comparison, not `time.Date`-based DST-aware slot computation. Write from scratch per D-11. |
| DST fake-clock scheduler tests | test | event-driven | No existing fake-clock test harness for a ticker-driven goroutine in this codebase (poller tests use real cron with short intervals). New test infrastructure needed — closest structural reference is still `poller_test.go`'s Start/Stop lifecycle tests, but the DST fake-clock stepping itself has no analog. |

## Metadata

**Analog search scope:** `internal/poller/`, `internal/notifier/`, `internal/settings/`, `internal/discord/`, `queries/`, `internal/db/migrations/`, `cmd/server/main.go`, `.github/workflows/full-pipeline.yml`
**Files scanned:** 9 read in full (poller.go, notifier.go, format.go, settings.go, discord/client.go, notification_settings.sql, events.sql, migration 000008, main.go relevant sections)
**Pattern extraction date:** 2026-09-16
