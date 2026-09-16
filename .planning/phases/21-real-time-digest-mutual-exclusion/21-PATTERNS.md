# Phase 21: Real-Time ↔ Digest Mutual Exclusion - Pattern Map

**Mapped:** 2026-09-16
**Files analyzed:** 5
**Analogs found:** 5 / 5 (all modifications to existing files — no net-new files this phase)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/notifier/notifier.go` | service (self-modified) | CRUD + event-driven (outbox drain) | itself — extend existing `NotifyPending`/`Sender`/`dbOpTimeout` patterns | exact (in-file) |
| `cmd/server/main.go` (composition root, ~line 255-299) | config/wiring | request-response (DI wiring) | itself — `settingsStore` already constructed at line 255, `httpserver.WithSettings(settingsStore)` at ~line 279 shows the existing wiring idiom | exact (in-file) |
| `internal/notifier/notifier_test.go` | test | event-driven (fake-driven unit tests) | itself — existing `fakeSender`, `newTestLogger`, `spacingRecorder`, `notifier.New(...)` call sites | exact (in-file) |
| `web/app/components/system/DigestSettings.tsx` | component | request-response (instant-apply settings UI) | itself — existing `<dl>` row structure, `dt`/`dd` pairs | exact (in-file) |
| `web/app/components/system/DigestSettings.test.tsx` | test | request-response (RTL component test) | itself — existing test file for the same component | exact (in-file) |

All five files are modifications to files already read/understood in CONTEXT.md — this phase adds no new files. The "analog" for each is the file's own established internal conventions, since drop-tracker's pattern is one cohesive file per concern rather than scattered duplicates.

## Pattern Assignments

### `internal/notifier/notifier.go` (service, event-driven outbox drain)

**Analog:** itself (`internal/notifier/notifier.go`)

**Existing consumer-declared-seam pattern to copy for `SettingsReader`** (lines 39-45):
```go
// Sender is the narrow outbound-delivery seam NotifyPending depends on, declared
// in the consumer so a test can substitute a fake with no HTTP client.
type Sender interface {
	Send(ctx context.Context, embed discord.Embed) error
}

var _ Sender = (*discord.Client)(nil)
```
Model for the new interface (D-05: declared in `notifier`, not imported from `settings`, full `Settings` returned):
```go
type SettingsReader interface {
	Get(ctx context.Context) (settings.Settings, error)
}

var _ SettingsReader = (*settings.Service)(nil)
```

**`dbOpTimeout`-bounded helper pattern to copy for the new bounded settings read** (lines 131-147):
```go
// listUnnotified calls q.ListUnnotified under a dbOpTimeout derived from ctx, so
// a wedged connection surfaces as an error instead of parking forever; shutdown
// cancellation still propagates.
func listUnnotified(ctx context.Context, q sqlc.Querier) ([]sqlc.Event, error) {
	opCtx, cancel := context.WithTimeout(ctx, dbOpTimeout)
	defer cancel()
	return q.ListUnnotified(opCtx)
}

// markNotified calls q.MarkNotified under the same bound. A timeout here lands on
// the WR-03 path: Discord already accepted the send, so the row stays pending
// and the next pass re-sends -- a visible duplicate, the preferred outcome.
func markNotified(ctx context.Context, q sqlc.Querier, id int64) (int64, error) {
	opCtx, cancel := context.WithTimeout(ctx, dbOpTimeout)
	defer cancel()
	return q.MarkNotified(opCtx, id)
}
```
Write a new `readDigestMode(ctx context.Context, r SettingsReader) (bool, error)` (or similarly named) helper following this exact shape: wraps ctx in `dbOpTimeout`, calls `r.Get(opCtx)`, returns the bool + error. D-03 requires this at both the top-of-pass gate and the per-send re-read, so factor it once and call it twice.

**`notifying atomic.Bool` CAS guard — do not rework, only build inside it** (lines 68-74, 155-161):
```go
type Notifier struct {
	q          sqlc.Querier
	sender     Sender
	spacing    time.Duration
	maxAgeDays int
	notifying  atomic.Bool
}
...
func (n *Notifier) NotifyPending(ctx context.Context, logger *slog.Logger) error {
	if !n.notifying.CompareAndSwap(false, true) {
		logger.Info("skipping notify pass: already in progress")
		return nil
	}
	defer n.notifying.Store(false)
```
Per D-04/ADR-0002, this CAS guard becomes the shared sender lock Phase 22's digest send also acquires. Phase 21 must not touch its shape — only add the digest-mode read immediately after the CAS succeeds and before `listUnnotified`.

**Required-constructor-argument pattern (not a functional `Option`) — `New`/`Select` signatures to extend** (lines 76-105):
```go
type Option func(*Notifier)

func WithMaxReleaseAgeDays(days int) Option {
	return func(n *Notifier) { n.maxAgeDays = days }
}

func New(q sqlc.Querier, sender Sender, spacing time.Duration, opts ...Option) *Notifier {
	n := &Notifier{q: q, sender: sender, spacing: spacing, maxAgeDays: defaultMaxReleaseAgeDays}
	for _, opt := range opts {
		opt(n)
	}
	return n
}

func Select(webhookURL string, q sqlc.Querier, httpClient *http.Client, logger *slog.Logger, opts ...Option) Sink {
	if webhookURL == "" {
		logger.Info("discord notifications disabled: DISCORD_WEBHOOK_URL not set")
		return NoOp{}
	}
	return New(q, discord.NewClient(webhookURL, httpClient), defaultSpacing, opts...)
}
```
D-05: add `settingsReader SettingsReader` as a new required positional parameter on both `New` and `Select` (after `sender`/before `spacing`, or wherever keeps call sites readable) — NOT an `Option`, so every existing call site fails to compile until updated (forces every test and `main.go` to be touched deliberately).

**`suppresses` cutoff — D-02 change, `ev.CreatedAt` replaces `time.Now()`** (lines 107-118):
```go
func (n *Notifier) suppresses(ev sqlc.Event) bool {
	cutoff := time.Now().UTC().AddDate(0, 0, -n.maxAgeDays).Format(time.DateOnly)
	return staleReleaseDate(ev.ReleaseDate, cutoff)
}
```
Change to anchor on `ev.CreatedAt` minus `maxAgeDays` minus 1 day (D-02's slack), leaving `staleReleaseDate` (lines 124-129) and its table test untouched — only the cutoff input changes.

**Structured `slog` logging style to copy for D-01's transition line and D-03's fail-closed Warn** (lines 180-221, especially the WR-03 Warn at 190-194 and the suppressed-summary Info at 216-220):
```go
logger.Warn("mark notified failed after a successful send: next pass will re-send this event",
	slog.Int64("event_id", ev.ID),
	slog.String("event_type", ev.EventType),
	slog.String("error", err.Error()),
)
...
logger.Info("notify pass suppressed stale events",
	slog.Int("suppressed_count", suppressed),
	slog.Int("pending_count", len(events)),
	slog.Int("max_release_age_days", n.maxAgeDays),
)
```
Model both D-01 (Info, only on mode change, on-to-off includes pending count) and D-03 (Warn, distinct wording, skipped on ctx-cancel) log lines on this terse structured-field style — no `fmt.Sprintf`, one field per fact, summary not per-row.

**Main loop structure to extend with per-send re-read (D-04 pass order)** (lines 162-209): the top-of-pass gate goes right after the CAS succeeds (before `listUnnotified` at line 162); the per-send re-read goes inside the `for` loop (line 168 onward) — after the `suppresses` check/ack (no re-read there per the spec), immediately before `n.sender.Send` at line 179.

---

### `cmd/server/main.go` (composition root)

**Analog:** itself, the existing `settingsStore` construction + `notifier.Select` call

**Wiring pattern** (already-read excerpt):
```go
settingsStore := settings.NewService(sqlc.New(pool))
...
notif := notifier.Select(cfg.DiscordWebhookURL, sqlc.New(pool), nil, logger, notifier.WithMaxReleaseAgeDays(cfg.NotifyMaxReleaseAgeDays))
```
`settingsStore` is constructed at line 255 (well before `notif` at line 299) and already implements `settings.Store.Get(ctx) (Settings, error)`, which structurally satisfies the new `notifier.SettingsReader` interface — no adapter type needed. Change the `notifier.Select(...)` call to pass `settingsStore` as the new required argument. Compare to the existing `httpserver.WithSettings(settingsStore)` call (shows the project's established idiom of passing the same `settingsStore` instance into multiple consumers).

---

### `internal/notifier/notifier_test.go` (test)

**Analog:** itself — existing fakes and call-site conventions

**Fake-double pattern to mirror for a fake `SettingsReader`** (lines 53-70):
```go
type fakeSender struct {
	fn func(ctx context.Context, embed discord.Embed) error
	calls int32
}

func (f *fakeSender) Send(ctx context.Context, embed discord.Embed) error {
	atomic.AddInt32(&f.calls, 1)
	if f.fn != nil {
		return f.fn(ctx, embed)
	}
	return nil
}

var _ notifier.Sender = (*fakeSender)(nil)
```
D-07 requires a fake `SettingsReader` that flips mode (or errors) on the k-th call — build it the same way: a `calls int32` counter (atomic), a `fn` hook, `var _ notifier.SettingsReader = (*fakeSettingsReader)(nil)` compile-time assertion.

**Test logger pattern for asserting D-01/D-03 log lines** (lines 44-51):
```go
func newTestLogger() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(handler), buf
}
```
Reuse as-is; assert on JSON-decoded log lines in `buf` for transition logging and fail-closed Warn tests.

**Every `notifier.New(q, sender, spacing)` call site must gain the new `SettingsReader` argument** — 13+ call sites found (lines 201, 206, 258, 280, 342, 366, 380, 410, 475, 529, 576, 624, 667, 743, 811) plus the `notifier.Select("", nil, nil, logger)` call at line 437 (D-10 NoOp path — passing `nil` for the new arg should still compile since NoOp never reads it, but Select's real branch needs the argument threaded to `New`).

**Isolated-pool testing note (D-07):** per CONTEXT.md, real-settings tests must build the reader from `testutil.NewIsolatedTestPool` (already imported at line 41), never the shared pool — matches this file's own header comment on why `NewIsolatedTestPool` is used for `ListUnnotified`'s unfiltered query.

---

### `web/app/components/system/DigestSettings.tsx` (component)

**Analog:** itself — existing `<dl>` row structure

**Row pattern to copy for D-06's helper text, placed under the Digest mode row** (lines 127-145):
```tsx
<dl className="grid grid-cols-[auto_1fr] items-center gap-x-4 gap-y-2">
  <dt id="digest-mode-label" className="text-label text-muted-foreground">
    Digest mode
  </dt>
  <dd className="flex items-center gap-1">
    <Switch
      checked={displayed.digest_enabled}
      onCheckedChange={handleDigestModeChange}
      disabled={saving}
      aria-labelledby="digest-mode-label"
    />
    <span className="text-body text-foreground">
      {displayed.digest_enabled ? "On" : "Off"}
    </span>
  </dd>
```
Insert the always-visible helper text ("While on, new events wait for the next digest; switching back off delivers them individually.") as a `<p>` styled like the existing status-message paragraph at lines 193-209 (`text-label text-muted-foreground`), but unconditional — not gated on `status !== "idle"`. Simplest placement: inside the `<dd>` for Digest mode (spanning under the Switch row) or immediately after the `</dl>` closing tag before the `{status !== "idle" && ...}` block — planner's call, but keep it inside `<CardContent>`.

---

## Shared Patterns

### Bounded DB/settings reads
**Source:** `internal/notifier/notifier.go` lines 131-147 (`listUnnotified`, `markNotified`)
**Apply to:** the new digest-mode-read helper in `notifier.go` — every read must be wrapped in `context.WithTimeout(ctx, dbOpTimeout)`.

### Structured slog logging
**Source:** `internal/notifier/notifier.go` lines 180-221
**Apply to:** D-01 transition Info line, D-03 fail-closed Warn line — `slog.Int`, `slog.String("error", ...)` field style, summary-per-pass not per-row.

### Consumer-declared narrow interface seams
**Source:** `internal/notifier/notifier.go` lines 39-52 (`Sender`, `Sink`)
**Apply to:** the new `SettingsReader` interface — declare in `notifier`, not imported from `settings`; add a `var _ SettingsReader = (*settings.Service)(nil)` compile-time assertion mirroring `var _ Sender = (*discord.Client)(nil)`.

### Required constructor argument vs. functional Option
**Source:** `internal/notifier/notifier.go` lines 76-105 (`Option`, `WithMaxReleaseAgeDays`, `New`, `Select`)
**Apply to:** `SettingsReader` must be a required positional parameter (D-05), never an `Option` — this is the one place this phase deliberately breaks from the existing `Option` pattern, by design (a forgotten option would silently ship an ungated notifier).

### Instant-apply settings UI conventions
**Source:** `web/app/components/system/DigestSettings.tsx` (whole file, especially lines 41-118)
**Apply to:** no functional changes needed for D-06 (pure static helper text, no new state) — just match existing Tailwind utility classes (`text-label text-muted-foreground`, `text-body text-foreground`) already used in this file's status-message paragraph.

## No Analog Found

None — all five files in scope are modifications to files already fully read and understood via CONTEXT.md's required reading; this phase introduces no new files or new architectural roles.

## Metadata

**Analog search scope:** `internal/notifier/`, `internal/settings/`, `cmd/server/main.go`, `web/app/components/system/`
**Files scanned:** `notifier.go`, `notifier_test.go`, `settings.go`, `main.go` (lines 230-320), `DigestSettings.tsx`
**Pattern extraction date:** 2026-09-16
