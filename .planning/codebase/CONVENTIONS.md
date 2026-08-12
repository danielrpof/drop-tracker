# Coding Conventions

**Analysis Date:** 2026-08-12

## Naming Patterns

**Files:**
- Go files: lowercase with underscores for logical grouping (`config.go`, `server.go`)
- Test files: `{name}_test.go` (e.g., `config_test.go`, `server_test.go`)
- Go packages: lowercase, no underscores (`config`, `httpserver`, `detection`)
- TypeScript/React files: PascalCase for components (`SearchBox.tsx`, `EventCard.tsx`), camelCase for utilities

**Functions/Methods:**
- Go: camelCase (e.g., `insertEvent`, `isSeedMode`, `seedNotifiedAt`, `parseWatchlistID`, `NewPool`)
- Constructors: `New` or `NewTypeName` (e.g., `New()`, `NewClient()`, `NewTestPool()`)
- Test helpers: lowercase starting with test prefix (e.g., `testMBID()`, `testLogger()`, `insertTestArtist()`)
- TypeScript: camelCase for functions (e.g., `runSearch`, `handleChange`, `guestFeatureHref`)

**Variables:**
- Go: camelCase (e.g., `mbClient`, `dzClient`, `notif`, `detector`, `dbURL`)
- TypeScript: camelCase (e.g., `value`, `loading`, `debounceRef`, `abortRef`)

**Types/Interfaces:**
- Go: PascalCase (e.g., `Detector`, `RecordingSource`, `ReleaseDetailSource`, `Server`, `Pinger`, `SearchSource`)
- TypeScript: PascalCase (e.g., `SearchBoxProps`, `EventCardProps`, `EventItem`, `SearchResponse`)
- Interfaces defined in consuming package (seam pattern), not provider package

**Constants:**
- Go: SCREAMING_SNAKE_CASE or descriptive camelCase names with extensive comments
  - Large constants: `shutdownTimeout`, `readHeaderTimeout`, `maxAddWatchlistBodyBytes`
  - Record constants: `SEARCH_DEBOUNCE_MS`, `maxMBIDRunes`, `maxNameRunes`
  - Pattern constants: `EVENT_BADGE` (exported map), `defaultHTTPPort` (unexported)
- TypeScript: SCREAMING_SNAKE_CASE (e.g., `SEARCH_DEBOUNCE_MS`)

**Exported vs. Unexported:**
- Go: Capitalize first letter for exported, lowercase for unexported
- Each exported type/function has a documentation comment
- Example: `type Detector struct { q sqlc.Querier ... }` (fields are unexported)

## Code Style

**Formatting:**
- Go: Use `gofmt` (implicit, verified by `golangci-lint`)
- TypeScript: Prettier v3.8.3 with Tailwind CSS plugin
  - 2-space indentation
  - Double quotes (not single)
  - No semicolons at line ends
  - Trailing comma: ES5 mode
  - Print width: 80 characters
  - Run via: `pnpm format` (in `web/` directory)

**Linting:**
- Go: golangci-lint v2.12.2 (v2 config schema, not v1)
  - Location: `.golangci.yml` at repo root
  - Standard linters enabled + gosec
  - gosec excludes test files (rules G101, G304)
  - Timeout: 5m
  - Verify locally before commit via pre-commit hook
- TypeScript: No explicit linter configured; rely on TypeScript compiler with `strict: true`

## Import Organization

**Go:**
1. Standard library imports (`fmt`, `context`, `testing`, etc.)
2. External dependencies (`github.com/...`, `golang.org/...`)
3. Internal packages (`github.com/danielrpof/drop-tracker/internal/...`)
- Blank line between each group
- Within each group: alphabetical order
- Example:
  ```go
  import (
    "context"
    "fmt"
    "testing"

    "golang.org/x/time/rate"

    "github.com/danielrpof/drop-tracker/internal/config"
    "github.com/danielrpof/drop-tracker/internal/db"
  )
  ```

**TypeScript:**
1. External libraries (`react`, `lucide-react`, third-party)
2. Path alias imports (`~/components`, `~/lib`) — aliases defined in `tsconfig.json`: `~/*` → `./app/*`
- Blank line between groups
- Example:
  ```tsx
  import { Loader2 } from "lucide-react"
  import { useState } from "react"
  
  import { Input } from "~/components/ui/input"
  import { searchArtists } from "~/lib/api"
  ```

## Error Handling

**Pattern:**
- Go: Wrap errors with context using `fmt.Errorf` and the `%w` verb to preserve the error chain
- Always add context that explains what operation failed
- Example:
  ```go
  if err != nil {
    return fmt.Errorf("detection: insert event: %w", err)
  }
  ```

**Security:**
- Never echo raw error details or internal error text to client responses
- Use fixed, operator-authored error messages in HTTP responses
- Redact secrets (DSN, webhook URLs, API keys) from error output
- Example:
  ```go
  func writeError(w http.ResponseWriter, status int, msg string) {
    // msg is always a fixed string, never raw error text
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(errorResponse{Error: msg})
  }
  ```

**HTTP Handlers:**
- Decode/validate inputs first, fail fast with 400/422
- Call services with validated inputs
- Translate domain errors to HTTP status codes
- Return sanitized error messages

**Database Errors:**
- Catch in handler, translate to HTTP response
- Log full error for debugging, never expose to client
- Example: `INSERT ... ON CONFLICT DO NOTHING` for idempotency

## Logging

**Framework:**
- Go: `log/slog` (stdlib as of Go 1.21+)
- No third-party logging library; slog is the standard

**Configuration:**
- `slog.NewJSONHandler(w io.Writer, opts)` for production (JSON output for machine parsing)
- `slog.NewTextHandler(w io.Writer, opts)` for local development (human-readable)
- Choice gated on config: `cfg.LogFormat` with values `"json"` or `"text"`
- Logger always includes service name: `.With(slog.String("service", "drop-tracker"))`

**Usage Patterns:**
- Logger passed through dependency injection (constructor arg or middleware)
- Structured key-value pairs: `logger.Info("message", "key1", value1, "key2", value2)`
- Log level config: `cfg.LogLevel` with values `"debug"`, `"info"`, `"warn"`, `"error"`
- Example:
  ```go
  logger.Info("starting server", "addr", addr)
  logger.Error("poller drain failed", "poller_error", err.Error())
  ```

**Levels:**
- `Debug`: Detailed operational info (rare, only when debugging)
- `Info`: High-level operational events (startup, shutdown, config load)
- `Warn`: Degraded conditions, retries
- `Error`: Failures that don't stop the process (failed poll cycle, upstream unavailable)

**Test Output:**
- Tests use `io.Discard` writer to suppress output: `slog.NewTextHandler(io.Discard, nil)`
- Assertions check database state, not log output

## Comments

**When to Comment:**
- Every exported type, interface, and function has a documentation comment
- Unexported functions: comment if not obvious from name or context
- Complex logic: explain the "why" not the "what"
- Non-obvious requirements or constraints: reference design docs (e.g., D-02, T-02-04)

**Style:**
- Package-level comments start with `// Package {name}` describing the whole package's role
- Function/method comments describe what the receiver does, use third-person imperative
- Example:
  ```go
  // Package detection implements the diff-based event detection Phase 4 owns...
  // detectDeluxeChanges marks any... (starts with function name)
  ```

**JSDoc/TSDoc:**
- TypeScript components: no JSDoc comments needed if props interface is clear
- Exported functions: inline comments above the function explaining behavior
- Example comment pattern in TypeScript:
  ```tsx
  // SEARCH_DEBOUNCE_MS is the wait after the last keystroke before
  // searchArtists() fires — long enough that GET /search is not issued on
  // every keystroke, short enough to still read as search-as-you-type.
  const SEARCH_DEBOUNCE_MS = 300
  ```

**Reference Patterns:**
- Comments often reference design document identifiers (D-01, T-02-03, WLST-01, etc.)
- These are cross-references to phase/design documentation
- Example: `// WR-02: ...` refers to a specific code-review finding or design decision

## Function Design

**Size:**
- Keep functions small and focused on one thing
- If a function has many branches or substeps, consider extracting helpers
- No hard line, but aim for ~50 lines of logic

**Parameters:**
- Functions accept concrete types or narrow interfaces
- Interfaces defined in consuming package (e.g., `detector` defines `RecordingSource`, not provider)
- Constructor injection preferred over global state
- Example:
  ```go
  func New(q sqlc.Querier, recordings RecordingSource, releases ReleaseDetailSource) *Detector
  ```

**Return Values:**
- Always return errors as the last return value
- Named return values rare unless clarifying (e.g., `(int, bool, error)`)
- Use `(T, error)` pattern, never `(error, T)`
- Example:
  ```go
  func (d *Detector) isSeedMode(ctx context.Context, artistID int64, source string) (bool, error)
  ```

**Receivers:**
- Methods on pointers for types that mutate state
- Value receivers for small, immutable types
- Consistent within a type (all methods pointer or all value)

## Module Design

**Exports:**
- Each package exports a narrow interface, not the whole implementation
- Dependencies are passed in at construction time, not retrieved from globals
- Example: `httpserver.Server` wraps `watchlist.Store` (interface), `events.Store` (interface), etc.

**Barrel Files:**
- No barrel/index pattern (`/index.go`)
- Each Go package is in its own directory with its own name

**Internal Packages:**
- All non-public packages in `internal/` directory
- Prevents external imports of internal APIs
- Matches Go ecosystem convention

**Initialization:**
- Single entrypoint: `cmd/server/main.go`
- Builds dependency graph in order: config → logger → migrations → pool → services → HTTP server
- Returns errors early, never reaches a "running-but-broken" state

## Type Conversions & Nullable Values

**Pointers:**
- Use `*string`, `*int32` for nullable/optional fields
- Convert empty strings to nil: `func nullableString(s string) *string`
- Check for nil before dereferencing

**Type Assertions:**
- Go: rarely needed; use interface{}only in JSON unmarshaling
- TypeScript: use `typeof` or `instanceof` checks, never assume types
- Example:
  ```go
  if s == "" { return nil }
  return &s
  ```

## Concurrency

**Goroutines:**
- Avoid global state; pass channels and values
- Use `context.Context` for cancellation and timeouts
- Example: HTTP servers use `signal.NotifyContext` for graceful shutdown

**Synchronization:**
- Mutex fields are unexported and immediately guarded
- Example:
  ```go
  type Cache struct {
    mu    sync.Mutex
    items map[string]Item
  }
  ```

**Testing:**
- Use `sync.WaitGroup` to coordinate goroutines in tests
- Use context cancellation for timeout testing

## Validation

**Input Validation:**
- Validate inputs early, before any expensive operations
- Trim strings: `strings.TrimSpace()`
- Check length by rune count, not bytes: `utf8.RuneCountInString()`
- Examples:
  - Rune limits: `maxNameRunes = 512`
  - Byte limits: `maxAddWatchlistBodyBytes = 65536`

**Database Constraints:**
- Assume database enforces schema constraints
- Field lengths validated at HTTP handler level before insert
- Foreign keys enforced by the database

## TypeScript/React Specifics

**Component Props:**
- Define as `export interface ComponentNameProps { ... }`
- Destructure in function signature: `export function Component({ prop1, prop2 }: ComponentNameProps)`

**State Management:**
- Use `useState` for local component state
- Use `useRef` for mutable values that don't trigger re-renders
- Pass handlers up as props (callback pattern)
- Example:
  ```tsx
  export function SearchBox({ onResults }: SearchBoxProps) {
    const [loading, setLoading] = useState(false)
    const abortRef = useRef<AbortController | null>(null)
  }
  ```

**Type Imports:**
- Use `import type { Type }` for type-only imports
- Example: `import type { SearchResponse } from "~/lib/api"`

**TypeScript Config:**
- Strict mode: `strict: true`
- Target: ES2022
- Module resolution: bundler
- Path aliases: `~/*` → `./app/*`

---

*Convention analysis: 2026-08-12*
