# Phase 14: Instance Passphrase Gate - Pattern Map

**Mapped:** 2026-08-27
**Files analyzed:** 17 (7 new Go, 4 modified Go, 4 new frontend, 2 modified frontend; plus `.env.example`)
**Analogs found:** 15 / 17 (2 have partial analogs only — the HMAC token codec and the per-IP limiter map are genuinely new code)

Downstream planner: every "copy from" below is a real file + line range read this session. Prefer these over RESEARCH.md's illustrative snippets where they overlap — RESEARCH.md's code is sketch, these are the load-bearing conventions the reviewer will check against.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/authgate/session.go` (NEW) | utility (crypto codec) | transform (sign/verify, pure, no net/http) | `internal/db/migrate.go` (pure helpers + heavy doc-comments) / stdlib `crypto/hmac` | partial — no existing HMAC-token codec in repo |
| `internal/authgate/gate.go` (NEW) | middleware | request-response | `internal/httpserver/server.go` `echoRequestID` + chi `middleware` idioms | role-match |
| `internal/authgate/login.go` (NEW) | handler + service | request-response / event-driven (throttle, global counter) | `internal/httpserver/health.go` (handler shape) + `cmd/server/main.go:120,146` (`rate.NewLimiter` construction) | role-match |
| `internal/authgate/alerter.go` (NEW) | service (seam) | request-response (webhook POST) | `internal/notifier/notifier.go:80-107,147-159` (`Sender`/`Sink`/`NoOp`/`Select`) | exact |
| `internal/authgate/weak.go` (NEW) | utility | transform (string → (reason,bool)) | `internal/config/config.go` `Load()` manual validation block (lines 62-90) | role-match |
| `internal/authgate/*_test.go` (NEW) | test | — | `internal/httpserver/server_test.go` (syncBuffer log capture, `newCapturingServer`), `health_test.go` (stub + httptest) | exact |
| `internal/httpserver/server.go` (MOD) | route/wiring | request-response | itself — `New(...)` at lines 62-92; functional-option shape from `internal/poller/poller.go:103-123,178` | exact |
| `internal/httpserver/server_test.go` (MOD) | test | — | itself — add one gated-construction helper beside `newCapturingServer` (lines 67-84) | exact |
| `internal/httpserver/health.go` (MOD or untouched) | handler | request-response | itself — stays registered outside the group; payload already minimal (lines 23-26) | exact |
| `internal/config/config.go` (MOD) | config | — | itself — Phase-grouped fields (lines 26-45), optional never `notEmpty` | exact |
| `cmd/server/main.go` (MOD) | wiring | — | itself — `notifier.Select(...)` call at line 195, `httpserver.New(...)` at 184-187, boot order at 88-98 | exact |
| `internal/discord/client.go` (reused, not modified) | service client | request-response | n/a — consumed via `discord.NewClient(url, nil)` (line 98) + `discord.Embed` | exact |
| `web/app/lib/api.ts` (MOD) | utility (fetch funnel) | request-response | itself — `apiFetch` at lines 111-133; wrappers 158-220 | exact |
| `web/app/lib/api.test.ts` (MOD) | test | — | itself — `vi.stubGlobal("fetch", …)` pattern at lines 16-26 | exact |
| `web/app/lib/authStore.ts` (NEW) | store (module pub/sub) | event-driven | no existing non-React store — closest is `web/app/lib/sources.ts` (plain module) | partial |
| `web/app/lib/authStore.test.ts` (NEW) | test | — | `web/app/lib/api.test.ts` (plain module unit test, no RTL) | role-match |
| `web/app/components/auth/PassphraseScreen.tsx` (NEW) | component | request-response (form → createSession) | `web/app/components/watchlist/SearchBox.tsx` (controlled input + async submit) | role-match |
| `web/app/components/auth/PassphraseScreen.test.tsx` (NEW) | test | — | `web/app/root.test.tsx` + `web/app/components/watchlist/SearchBox.test.tsx` | exact |
| `web/app/root.tsx` (MOD) | provider/layout | — | itself — `App()` at lines 53-69; add auth-state branch + logout control in `<nav>` | exact |
| `web/app/root.test.tsx` (MOD) | test | — | itself — `createRoutesStub` + `renderAppAt` helper (lines 61-97) | exact |
| `web/app/routes.ts` (MOD — likely untouched) | route config | — | itself (lines 8-11) — passphrase screen is an `<App>`-level branch, not a route; likely NO change needed | exact |
| `.env.example` (MOD) | config doc | — | itself (could not read — permission denied; see note) | — |

---

## Pattern Assignments

### `internal/authgate/alerter.go` (service seam — the strongest analog in this phase)

**Analog:** `internal/notifier/notifier.go` lines 80-107 and 147-159 — copy this structure almost verbatim.

**Seam interface + compile-time assertions** (`notifier.go:80-98`):
```go
// Sender is the narrow seam NotifyPending depends on for outbound delivery,
// declared here in the consumer ... so a test can substitute a fake with no
// real HTTP client.
type Sender interface {
	Send(ctx context.Context, embed discord.Embed) error
}

var _ Sender = (*discord.Client)(nil)

type Sink interface {
	NotifyPending(ctx context.Context, logger *slog.Logger) error
}

var _ Sink = (*Notifier)(nil)
var _ Sink = NoOp{}
```

**NoOp disabled-case type** (`notifier.go:100-107`):
```go
// NoOp is D-10's inert Sink: returned by Select when DISCORD_WEBHOOK_URL is
// unset, so poller.go's Notifier seam is always non-nil and no cycle method
// ever nil-checks it -- exactly as EventRecorder has no disabled-state
// concept either.
type NoOp struct{}

func (NoOp) NotifyPending(ctx context.Context, logger *slog.Logger) error { return nil }
```

**Select gate — the disabled-case idiom to mirror** (`notifier.go:147-159`):
```go
// Select returns the Sink cmd/server/main.go should wire into poller.New:
// D-10's gate lives here, behind an exported function, rather than inline
// in main.go, so it is unit-testable without booting the process. An empty
// webhookURL logs one Info line ... and returns NoOp{}; otherwise it returns
// a real Notifier over discord.NewClient(webhookURL, httpClient) ...
func Select(webhookURL string, q sqlc.Querier, httpClient *http.Client, logger *slog.Logger, opts ...Option) Sink {
	if webhookURL == "" {
		logger.Info("discord notifications disabled: DISCORD_WEBHOOK_URL not set")
		return NoOp{}
	}
	return New(q, discord.NewClient(webhookURL, httpClient), defaultSpacing, opts...)
}
```

For authgate this becomes: `Alerter` interface (`Alert(ctx, message) error`), `noopAlerter{}`, `discordAlerter{c *discord.Client}`, and `SelectAlerter(webhookURL string, logger *slog.Logger) Alerter`. Construct the real client with `discord.NewClient(webhookURL, nil)` (see `internal/discord/client.go:98` — nil httpClient self-defaults to `&http.Client{Timeout: defaultTimeout}`). Post via `discord.Embed` (same struct `notifier` uses).

---

### `internal/discord` reuse — the no-raw-error-wrap rule (Shared Pattern, but load-bearing here)

**Source:** `internal/discord/client.go:135-144`:
```go
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Deliberately NOT wrapping the raw *url.Error here ... Go's
		// *url.Error.Error() embeds the full request URL, and a Discord
		// webhook URL's path (/webhooks/{id}/{token}) IS the secret token.
		// Wrapping it would write a live credential into structured logs ...
		return fmt.Errorf("discord: send webhook: request failed")
	}
```
`authgate.discordAlerter.Alert` must **not** wrap or log the error returned by `discord.Client.Send` with `%w`/`%v` in a way that could re-expose the webhook URL. Log the outcome only (`logger.Warn("brute-force alert send failed")`), never the raw error text.

---

### `internal/authgate/session.go` (HMAC token codec — mostly new)

**No direct analog.** Use stdlib `crypto/hmac`, `crypto/sha256`, `crypto/subtle`, `crypto/rand`, `encoding/base64` (`RawURLEncoding`). RESEARCH.md §Pattern 2 (14-RESEARCH.md lines 282-317) is the spec — copy its `DeriveKey` / `Token` / `Sign` / `Verify(key, raw, now) (Token, needsRenew, ok)` shapes and the hardcoded constants (`sessionWindow=30d`, `renewAfter=15d`, `absoluteCap=90d`, D-07).

**Doc-comment density to match:** `internal/db/migrate.go:124-193` (the `redactError`/`kvPasswordPattern` block) and `internal/notifier/notifier.go:44-78` — this codebase writes multi-paragraph "why, not what" comments on every security-sensitive helper, citing the decision ID (`D-01`, `D-06`) and the pitfall/debug doc. The planner should budget comment lines accordingly.

**Constant-time compare (GATE-03, Pitfall 3):** SHA-256 both sides first so length is not observable, then `subtle.ConstantTimeCompare` / `hmac.Equal` on the 32-byte digests. Never `==`/`bytes.Equal` on the passphrase.

---

### `internal/authgate/gate.go` (middleware)

**Analog:** `internal/httpserver/server.go:98-105` — `echoRequestID` is the repo's one hand-rolled middleware, showing the exact closure shape to copy:
```go
func echoRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := middleware.GetReqID(r.Context()); id != "" {
			w.Header().Set(middleware.RequestIDHeader, id)
		}
		next.ServeHTTP(w, r)
	})
}
```
`Authenticate` and `RequireCSRFHeader` are methods on `*Manager` returning `func(http.Handler) http.Handler` of this shape. On renewal, write `Set-Cookie` on `w` before `next.ServeHTTP` (see RESEARCH.md line 317).

**Error-body convention:** fixed operator-authored JSON, never raw error text. Health handler shows the pattern (`health.go:41-43`): `w.Header().Set("Content-Type","application/json"); w.WriteHeader(status); json.NewEncoder(w).Encode(resp)`. 401 body: `{"error":"unauthenticated"}` — matches what `ApiError` in `web/app/lib/api.ts:118-129` parses.

---

### `internal/authgate/login.go` (login/logout handlers + throttle + global counter)

**Handler shape analog:** `internal/httpserver/health.go:28-44` — `context.WithTimeout` on `r.Context()`, build a typed response struct, set header, `WriteHeader`, `json.NewEncoder(w).Encode()`.

**`rate.Limiter` construction analog:** `cmd/server/main.go:120` (`rate.NewLimiter(rate.Limit(x), 1)`) and `:146`. The per-IP map + sweeper is new code — follow RESEARCH.md §Pattern 4 (lines 338-357).

**`var`-not-`const` for test-shrinkable timings:** `internal/notifier/notifier.go:67` (`dbOpTimeout`) and `:78` (`spacingWait = time.After`) establish the idiom — the fixed 250ms–1s login delay and the global-counter window should be package `var`s so `login_test.go` can shrink them. `notifier` exposes setters via `export_test.go`; mirror that.

**Audit logging (D-13):** structured `slog` lines, outcome + source IP only, passphrase never an attribute. Redaction mindset from `internal/db/migrate.go:190-193`:
```go
func redactError(err error) string {
	s := userInfoPattern.ReplaceAllString(err.Error(), "")
	return kvPasswordPattern.ReplaceAllString(s, "password=<redacted>")
}
```
The authgate equivalent is simpler — just never pass the value to `slog` — but the same "add a test that greps the captured log buffer for the secret" discipline applies (see `server_test.go:189-219` `TestNoDSNInLogs` — copy that test structure exactly).

**Client IP:** `chi/middleware.RealIP` (bundled, already imported in `server.go:11`).

---

### `internal/authgate/weak.go`

**Analog:** `internal/config/config.go:62-90` — the `Load()` manual-validation block. Same posture: a plain function returning a reason string, no side effects. Difference: weak.go **warns**, never errors (D-11). RESEARCH.md lines 511-521 has the heuristic sketch (`knownDefaults` slice, `< 16` runes, case-fold compare). Never log/return the value.

---

### `internal/httpserver/server.go` (MOD — the central wiring change)

**Current `New` signature** (`server.go:62`):
```go
func New(db Pinger, store watchlist.Store, eventsStore events.Store, sources []SearchSource, logger *slog.Logger) *Server {
```

**Functional-option analog:** `internal/poller/poller.go:103-123` + `:178`:
```go
type Option func(*Poller)

func WithMusicBrainzWorkers(n int) Option {
	return func(p *Poller) { p.mbWorkers = n }
}

func New(store watchlist.Store, mb ReleaseGroupSource, ..., logger *slog.Logger, opts ...Option) (*Poller, error) {
	...
	p := &Poller{ store: store, ... }
	for _, opt := range opts { opt(p) }
```
Also `internal/notifier/notifier.go:123-145` (`type Option func(*Notifier)` + `for _, opt := range opts { opt(n) }`).

**Apply to server.go:** add trailing `opts ...Option`; `WithAuthGate(passphrase string, alerter authgate.Alerter) Option`. When passphrase is empty the option is inert (no `Group`, no `middleware.RealIP`, no `/session` routes — RESEARCH.md §Pattern 1, lines 210-278, and Pitfall 7 at lines 450-453). Existing middleware chain at `server.go:66-79` stays byte-for-byte identical; the gate is `pr.Use(...)` inside `r.Group(...)`, not a 5th `r.Use`. `/health` (`server.go:81`) stays registered on `r`, outside the group.

**Route list to move into the gated group** (currently `server.go:82-87`): `/search`, `POST/GET/PATCH/DELETE /watchlist`, `/events`. `/health` and `r.NotFound(webassets.Handler())` (`server.go:88`) stay outside.

---

### `internal/httpserver/server_test.go` (MOD)

**Analog:** the file itself — `newCapturingServer(t, buf)` at lines 67-84, and `TestNoDSNInLogs` at 189-219 (secret-not-in-log-buffer test via `syncBuffer` at lines 26-41).

Add exactly one helper, e.g. `newGatedServer(t, passphrase)`, that calls `httpserver.New(stub, stubStore{}, stubEventsStore{}, nil, logger, httpserver.WithAuthGate(passphrase, fakeAlerter{}))`. **All ~40 existing 5-arg `httpserver.New(...)` call sites must stay unchanged** (verified: `health_test.go:57,86,126,151`, `server_test.go:83,203` all pass 5 args). Add an `Inert`/`GATE-07` test proving the 5-arg path 200s on every route with no passphrase.

---

### `internal/config/config.go` (MOD)

**Analog:** lines 26-45 (the Phase 3-5 / Phase 10 / Phase 11 grouped blocks). Add:
```go
	// Phase 14 — instance passphrase gate (GATE-01..07). Optional: empty =
	// gate fully disabled, every route behaves exactly as v1.2 (GATE-07).
	// Never notEmpty/required; no Load() validation beyond the boot-time
	// weak-heuristic WARN, which lives in cmd/server/main.go (WARN, never
	// fail — D-11).
	InstancePassphrase string `env:"INSTANCE_PASSPHRASE"`
```
Do **not** add a check to `Load()` (lines 62-90) — those are all non-negative-int guards; the passphrase has no such constraint.

---

### `cmd/server/main.go` (MOD)

**Boot-order analog:** lines 88-98 — `config.Load()` → `logging.New(cfg)` → `db.RunMigrations`. The weak-passphrase `WARN` goes immediately after `logger := logging.New(cfg)` (line 94), before migrations.

**Wiring analog:** line 195 (`notif := notifier.Select(cfg.DiscordWebhookURL, ..., logger, ...)`) and lines 184-187 (`httpserver.New(...)` call). Change the `httpserver.New` call to append `httpserver.WithAuthGate(cfg.InstancePassphrase, authgate.SelectAlerter(cfg.DiscordWebhookURL, logger))`. One added line for the WARN, one modified line for the option — that is the whole diff (matches CONTEXT.md "one wiring line").

---

### `web/app/lib/api.ts` (MOD)

**Analog:** the file itself. The single funnel is `apiFetch` at lines 111-133:
```ts
async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init)
  if (res.status === 204) { return undefined as T }
  if (!res.ok) {
    let message = res.statusText
    try {
      const body = (await res.json()) as { error?: string }
      if (body.error) { message = body.error }
    } catch { /* ... */ }
    throw new ApiError(res.status, message)
  }
  return (await res.json()) as T
}
```
**Change:** add, right after `const res = await fetch(path, init)`:
```ts
  if (res.status === 401) {
    authStore.markUnauthenticated()
    throw new ApiError(401, "unauthenticated")
  }
```
**Custom header (D-15):** the existing non-GET wrappers set `headers: { "Content-Type": "application/json" }` (lines 174, 194) — `removeWatchlist` (lines 204-206) sets none. Add `"X-Requested-With": "drop-tracker"` to every non-GET call. Simplest per RESEARCH.md line 389: inject it centrally in `apiFetch` when `init?.method && init.method !== "GET"`.

**New wrappers to add** (mirror `removeWatchlist` at 204-206 for the 204 shape): `createSession(passphrase: string): Promise<void>` → `POST /session` with the header + JSON body; `deleteSession(): Promise<void>` → `DELETE /session`.

---

### `web/app/lib/authStore.ts` (NEW)

**Partial analog:** no existing non-React store. Closest structural precedent is a plain `~/lib/*` module. Use the `useSyncExternalStore` sketch in RESEARCH.md lines 363-376 (module-level `authed` bool + `Set<() => void>` listeners + `subscribe`/`markAuthenticated`/`markUnauthenticated` + a `useAuthed()` hook). Keep it framework-free so `api.ts` (which is mocked wholesale in most tests) can import it without pulling React.

---

### `web/app/components/auth/PassphraseScreen.tsx` (NEW)

**Analog:** `web/app/components/watchlist/SearchBox.tsx` — controlled `<input>` + async submit + error surface, in the dark-only theme (`root.tsx:16-21` — `className="dark"` is unconditional; no theme provider). Single password field, submit calls `createSession(value)` then `authStore.markAuthenticated()` on success; wrong passphrase (`ApiError` 401/rejected) shows an inline error and does **not** flip state. Full-screen, no `<nav>` tabs (rendered by `<App>` before the tab bar — see next).

---

### `web/app/root.tsx` (MOD)

**Analog:** `App()` at lines 53-69. Add at the top of `App()`:
```tsx
  const authed = useAuthed()
  if (!authed) return <PassphraseScreen />
```
before the `<div><nav>…</nav><main><Outlet/></main></div>` return. Add a logout control inside `<nav>` (lines 56-63) beside the two `<NavLink>`s — calls `deleteSession()` then `authStore.markUnauthenticated()`. The `<Outlet/>` remount on the auth-state flip is what re-fetches route data for free (each route fetches in a mount `useEffect` — verified `web/app/routes/watchlist.tsx:40-49`).

`web/app/routes.ts` (lines 8-11) most likely needs **no change** — the passphrase screen is an `<App>`-level conditional, not a route.

---

### Test files (Go + frontend)

**Go — log-capture + secret-leak assertion:** `internal/httpserver/server_test.go:26-41` (`syncBuffer`) and `:189-219` (`TestNoDSNInLogs`). Copy verbatim for D-13's "passphrase never in logs" test.

**Go — stub + httptest handler test:** `internal/httpserver/health_test.go:39-47` (`stubPinger`), `:55-81` (httptest server + typed body decode + field-name assertions, not raw string match).

**Go — table-driven with `t.Run` subcases:** RESEARCH.md lines 561-583 (session_test.go sketch). Repo lean is individually-named `TestXxx` funcs; table + `t.Run` is acceptable for the codec.

**Frontend — plain-module unit test (no RTL):** `web/app/lib/api.test.ts:16-26` — `let fetchMock = vi.fn(); vi.stubGlobal("fetch", fetchMock)` in `beforeEach`, `vi.unstubAllGlobals()` in `afterEach`. Use this exact shape for `authStore.test.ts` and the api.ts 401-interceptor test. **Do not `vi.mock("~/lib/api")` in api.test.ts** (comment at lines 10-15 explains why).

**Frontend — RTL with router context:** `web/app/root.test.tsx:61-97` — `createRoutesStub([{ path:"/", Component: App, children:[…] }])` then `render(<Stub initialEntries={[path]} />)`. Or `renderRoute` from `web/app/lib/test/routeStub.tsx` for a single component. Use for `PassphraseScreen.test.tsx` and the `<App>` gate-branch extension to `root.test.tsx`.

---

## Shared Patterns

### Disabled-case seam (never a nil check in the request path)
**Source:** `internal/notifier/notifier.go:100-107,147-159`
**Apply to:** `internal/authgate/alerter.go` (`noopAlerter`), and the gate wiring in `internal/httpserver/server.go` (when passphrase empty, the group/middleware simply does not exist — RESEARCH.md §Pattern 1).
The runtime seam is always non-nil; "disabled" is a concrete no-op type wired at construction.

### Functional-option constructor
**Source:** `internal/poller/poller.go:103-123,178`; `internal/notifier/notifier.go:123-145`
**Apply to:** `httpserver.New(..., opts ...Option)` + `httpserver.WithAuthGate(...)`.
Keeps every existing call site a pure additive change — the ~40 5-arg `httpserver.New` test call sites must not need editing (GATE-07 / success criterion 5).

### Secret never reaches a log line
**Source:** `internal/db/migrate.go:175-193` (`redactError`); `internal/discord/client.go:136-144` (no raw `*url.Error` wrap); `internal/httpserver/server_test.go:189-219` (`TestNoDSNInLogs` — the enforcing test)
**Apply to:** `internal/authgate/login.go` audit lines, `internal/authgate/weak.go`, `internal/authgate/alerter.go`. Passphrase is never an `slog` attribute, never in an error string, never in a URL. Add a buffer-grep test.
**Verified safe already:** `httplog` in `server.go:68-78` logs no request/response bodies and not the `Cookie` header (RESEARCH.md Pitfall 6) — do not widen `LogRequestHeaders`.

### Fixed JSON error body, never raw error text
**Source:** `internal/httpserver/health.go:22-26,41-43`; `web/app/lib/api.ts:92-103` (`ApiError` parses `{"error": "..."}`)
**Apply to:** all authgate handler responses (`401`, `403`, `429`, login-failure).

### Test-shrinkable timing as package `var`
**Source:** `internal/notifier/notifier.go:67,78` + `export_test.go` setter idiom
**Apply to:** authgate's fixed login delay, per-IP rate/burst, global-counter window/threshold.

### Single fetch funnel (frontend)
**Source:** `web/app/lib/api.ts:107-133`
**Apply to:** the 401 interceptor and the `X-Requested-With` header — one edit inside `apiFetch` covers every endpoint; do not add per-wrapper logic.

### Router-context test rendering
**Source:** `web/app/lib/test/routeStub.tsx`; `web/app/root.test.tsx:61-74`
**Apply to:** `PassphraseScreen.test.tsx`, `root.test.tsx` gate-branch extension. Never hand-roll a memory-router wrapper.

---

## No Analog Found

| File | Role | Data Flow | Reason / what to use instead |
|------|------|-----------|------------------------------|
| `internal/authgate/session.go` (Sign/Verify) | crypto codec | transform | No HMAC-token codec exists in the repo. Build from stdlib per RESEARCH.md §Pattern 2 (lines 282-317). Borrow only the doc-comment density from `internal/db/migrate.go` and the constant-time-compare discipline from RESEARCH.md §Don't Hand-Roll. |
| per-IP `rate.Limiter` map + sweeper (in `login.go`) | throttle infra | event-driven | Repo only has singleton `rate.Limiter`s (`main.go:120,146`), never a keyed map. Use the canonical Go map+mutex+lastSeen+sweeper pattern from RESEARCH.md §Pattern 4 (lines 338-357). |
| `web/app/lib/authStore.ts` | non-React pub/sub store | event-driven | No non-React store in `web/app`. Use the `useSyncExternalStore` sketch in RESEARCH.md lines 363-376. |

---

## Metadata

**Analog search scope:** `internal/httpserver/`, `internal/notifier/`, `internal/config/`, `internal/discord/`, `internal/db/`, `internal/poller/`, `cmd/server/`, `web/app/lib/`, `web/app/components/`, `web/app/` root, `web/app/**/*.test.{ts,tsx}`
**Files read this session:** `internal/httpserver/server.go`, `server_test.go`, `health.go`, `health_test.go`; `internal/notifier/notifier.go`; `internal/config/config.go`; `cmd/server/main.go`; `internal/discord/client.go` (lines 85-159); `internal/db/migrate.go` (redact block); `internal/poller/poller.go` (option block); `web/app/lib/api.ts`, `api.test.ts`; `web/app/root.tsx`, `root.test.tsx`; `web/app/routes.ts`; `web/app/routes/watchlist.tsx` (lines 1-60); `web/app/lib/test/routeStub.tsx`
**Could not read:** `.env.example` (permission denied by sandbox) — planner should add `INSTANCE_PASSPHRASE` with a comment recommending a 24+ char random value, and must NOT put a usable default value there (the `.env.example` value is on the weak-passphrase denylist per RESEARCH.md line 715). Follow the commenting style of the other vars already in that file.
**Pattern extraction date:** 2026-08-27
