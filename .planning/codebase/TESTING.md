# Testing Patterns

**Analysis Date:** 2026-08-12

## Test Framework

**Go:**
- Runner: stdlib `testing` package (no external test framework)
- No assertion library; use direct conditionals with `t.Errorf()` and `t.Fatalf()`
- Configuration: None (testing is built into Go runtime)

**TypeScript/React:**
- No test framework configured in `web/package.json`
- React Router does not require testing setup for the current build
- Future phases may add Jest, Vitest, or Playwright

**Run Commands:**
```bash
make test-short       # Quick tests without database: go test ./... -short -race -count=1
make test-integration # Full tests with database: go test ./... -race -count=1 -p 1
make test             # Alias for test-integration
```

**Test Flags:**
- `-race`: Enable race detector (catch concurrent access bugs)
- `-count=1`: Run each test exactly once (disable test caching)
- `-p 1`: Run packages sequentially (not in parallel)
- `-short`: Skip tests marked with `testing.Short()`

## Test File Organization

**Location:**
- Go: Co-located with implementation files in same directory
- File pattern: `{name}_test.go` (e.g., `config.go` → `config_test.go`)
- Package naming: `{package}_test` (separate package from implementation)
  - Example: `package config_test` (not `package config`)
  - Allows testing exported API without access to unexported implementation

**Directory Structure:**
```
internal/
├── config/
│   ├── config.go       (implementation)
│   └── config_test.go  (tests, package config_test)
├── httpserver/
│   ├── server.go
│   ├── health.go
│   ├── search.go
│   ├── server_test.go
│   ├── health_test.go
│   └── search_test.go
└── testutil/
    └── postgres.go     (test utilities, package testutil)
```

## Test Structure

**Basic Test Function:**
```go
func TestFunctionName_Behavior(t *testing.T) {
    // Arrange: set up test data and mocks
    data := setupTestData(t)
    
    // Act: call the function being tested
    result, err := functionUnderTest(data)
    
    // Assert: verify the result
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result != expected {
        t.Errorf("result = %v, want %v", result, expected)
    }
}
```

**Naming Convention:**
- `Test{FunctionName}_{Behavior}` or `Test{FunctionName}_{Condition}`
- Example: `TestHealth_Up`, `TestHealth_Down`, `TestHealth_DownOnTimeout`
- Each variant tests a specific scenario or condition

## Helper Functions

**Pattern:**
- Lowercase, descriptive names (not exported)
- First line: `t.Helper()` to mark as test helper
- Detailed documentation comments
- Return simple types or register cleanup with `t.Cleanup()`
- Example:
  ```go
  // testMBID derives a short, unique-per-test artist mbid from t.Name(),
  // the same helper convention as internal/watchlist/service_test.go's testMBID.
  func testMBID(t *testing.T) string {
    t.Helper()
    sum := sha256.Sum256([]byte(t.Name()))
    return "test-" + hex.EncodeToString(sum[:])[:12]
  }
  ```

**Common Helpers:**
- `testMBID(t *testing.T)`: Generate unique test artist MBIDs from test name
- `testLogger()`: Return a no-op slog.Logger (writes to io.Discard)
- `insertTestArtist(t, pool, mbid, name)`: Insert test row with automatic cleanup
- `newTestClient(t, ts, limiter)`: Wire a client to httptest.Server

**Test Utilities Package:**
- Location: `internal/testutil/postgres.go`
- Provides: `NewTestPool(t *testing.T) *pgxpool.Pool`
- Handles: DSN lookup (TEST_DATABASE_URL or DATABASE_URL env var), migration running, pool creation, cleanup registration

## Mocking & Stubs

**Strategy:**
- Define narrow interfaces in consuming package (seam pattern)
- Create file-local stub implementations in test file
- Stub does not need to be exported (tests in same package)

**Stub Pattern:**
```go
// stubPinger is a file-local double for httpserver.Pinger
type stubPinger struct {
    pingFunc func(context.Context) error
}

func (s stubPinger) Ping(ctx context.Context) error {
    return s.pingFunc(ctx)
}

var _ httpserver.Pinger = stubPinger{}  // Verify it implements the interface
```

**Func-Field Pattern:**
- Stubs use function fields for behavior injection
- Zero value returns sensible default (nil, empty slice, no error)
- Example:
  ```go
  type fakeRecordingSource struct {
    recordings []musicbrainz.Recording
    err        error
  }
  
  func (f fakeRecordingSource) RecordingsByArtist(ctx context.Context, mbid string) ([]musicbrainz.Recording, error) {
    return f.recordings, f.err
  }
  ```

**Stateful Stubs:**
- Use `sync.Mutex` for call counting or state tracking
- Example:
  ```go
  type fakeReleaseDetailSource struct {
    mu        sync.Mutex
    callCount int
    releases  map[string][]musicbrainz.Release
  }
  
  func (f *fakeReleaseDetailSource) calls() int {
    f.mu.Lock()
    defer f.mu.Unlock()
    return f.callCount
  }
  ```

**HTTP Testing:**
- Use `httptest.Server` to mock external APIs
- Inspect request details in handler: `r.URL.Path`, `r.URL.Query()`
- Write fixture responses via handler
- Example:
  ```go
  ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(drakeSearchFixture))
  }))
  defer ts.Close()
  
  c := newTestClient(t, ts, unlimitedLimiter())
  ```

## Fixtures & Test Data

**Fixtures:**
- API response fixtures transcribed verbatim from live-verified responses
- Stored as string constants in test files (not separate JSON files)
- Comments reference documentation: e.g., "Fixture matches Deezer responses in 03-RESEARCH.md"
- Example:
  ```go
  const drakeSearchFixture = `{
    "data": [
      {"id": 246791, "name": "Drake", ...}
    ]
  }`
  ```

**Test Data Builders:**
- Small helper functions for complex test objects
- Example:
  ```go
  func mkRelease(mbid, title, date string, trackCounts ...int) musicbrainz.Release {
    media := make([]musicbrainz.Medium, len(trackCounts))
    for i, tc := range trackCounts {
      media[i] = musicbrainz.Medium{Format: "CD", Position: i + 1, TrackCount: tc}
    }
    return musicbrainz.Release{MBID: mbid, Title: title, Status: "Official", Date: date, Media: media}
  }
  ```

**Database Fixtures:**
- Tests share a single Docker Postgres instance (started via `make db-up`)
- Each test creates unique data via `insertTestArtist()` helper
- Unique IDs: derive from test name via SHA256 hash
- Cleanup: `t.Cleanup()` deletes created rows after test ends

## Assertions

**Pattern:**
- Direct conditional checks, no assertion library
- Use `t.Errorf()` for non-fatal failures (test continues)
- Use `t.Fatalf()` for fatal failures (test stops)

**Assertion Style:**
```go
// Non-fatal check (test continues if fails)
if status != http.StatusOK {
    t.Errorf("status = %d, want %d", status, http.StatusOK)
}

// Fatal check (test stops if fails)
if err != nil {
    t.Fatalf("unexpected error: %v", err)
}

// Struct field checking
if body.Status != "ok" || body.DB != "up" {
    t.Fatalf("body = %+v, want {Status:ok DB:up}", body)
}
```

**JSON Parsing:**
- Use `json.NewDecoder(resp.Body).Decode(&dst)` for response bodies
- Use `json.Unmarshal(data, &dst)` if need to check raw bytes first
- Define local structs to match response shape (not hardcode string checks)
- Example:
  ```go
  var body healthBody  // matches {"status": "...", "db": "..."}
  if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
    t.Fatalf("decode response body: %v", err)
  }
  if body.Status != "ok" || body.DB != "up" {
    t.Fatalf("body = %+v, want {Status:ok DB:up}", body)
  }
  ```

## Test Types

**Unit Tests:**
- Scope: Single function or method
- Mocks: External dependencies (API clients, database)
- Example: `TestDetectMusicBrainz_NewRelease` tests detection logic with stub `fakeRecordingSource` and `fakeReleaseDetailSource`
- Run with: `make test-short` (no database)

**Integration Tests:**
- Scope: Full request path (handler → service → database)
- Real database: `testutil.NewTestPool(t)` connects to Docker Postgres
- Example: `TestSearch_Success` wires real `httpserver.Server` to `httptest.Server`
- Run with: `make test-integration` (database required, sequential execution)

**E2E Tests:**
- Framework: Not used currently
- Future consideration for testing React UI against running server

## Concurrent Testing

**Pattern:**
- Use `sync.WaitGroup` to coordinate goroutines
- Collect results in array indexed by goroutine ID
- Example:
  ```go
  const n = 20
  type result struct {
    status int
    body   healthBody
    err    error
  }
  results := make([]result, n)
  
  var wg sync.WaitGroup
  wg.Add(n)
  for i := 0; i < n; i++ {
    go func(idx int) {
      defer wg.Done()
      resp, err := http.Get(ts.URL + "/health")
      results[idx] = result{status: resp.StatusCode, body: body, err: err}
    }(i)
  }
  wg.Wait()
  
  for i, r := range results {
    if r.err != nil {
      t.Fatalf("request %d: %v", i, r.err)
    }
  }
  ```

## Error & Edge Case Testing

**Error Paths:**
- Stubs that return specific errors
- Verify error is handled gracefully (no panic)
- Verify error message is sanitized (no secrets leaked)
- Example:
  ```go
  func TestHealth_Down(t *testing.T) {
    pingErr := errors.New("connection refused: db-error-marker")
    stub := stubPinger{pingFunc: func(context.Context) error { return pingErr }}
    srv := httpserver.New(stub, ...)
    
    // Verify response doesn't leak the raw error
    if strings.Contains(string(data), pingErr.Error()) {
      t.Fatalf("response body leaked error: %s", data)
    }
  }
  ```

**Timeout Testing:**
- Use context cancellation to simulate timeouts
- Example:
  ```go
  stub := stubPinger{pingFunc: func(ctx context.Context) error {
    <-ctx.Done()  // Wait for context cancellation
    return ctx.Err()  // Return context error
  }}
  ```

**Input Validation:**
- Test each input boundary (empty, max length, invalid format)
- Verify 400 response with clear error message
- Example:
  ```go
  func TestSearch_MissingOrBlankQReturns400(t *testing.T) {
    paths := []string{"/search", "/search?q=", "/search?q=%20%20"}
    for _, p := range paths {
      t.Run(p, func(t *testing.T) {
        // Verify blank q is rejected before calling source
        resp, _ := http.Get(ts.URL + p)
        if resp.StatusCode != http.StatusBadRequest {
          t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
        }
      })
    }
  }
  ```

## Database Testing

**Setup:**
- Environment variable: `TEST_DATABASE_URL` (defaults to `DATABASE_URL`)
- Docker container: `postgres` service in `docker-compose.yml` on port 5433
- Migrations: Run automatically by `testutil.NewTestPool(t)`
- Skipping: Tests marked with `t.Skip()` if database unavailable

**Isolation:**
- Tests must run sequentially: `-p 1` flag (see Makefile)
- Reason: Shared single database; migrations run `DROP SCHEMA public CASCADE`
- Each test creates unique data derived from test name
- Cleanup: `t.Cleanup()` deletes test rows after each test

**Example Pattern:**
```go
func TestDetectMusicBrainz_NewRelease(t *testing.T) {
    pool := testutil.NewTestPool(t)  // Connects, runs migrations, registers cleanup
    ctx := context.Background()
    mbid := testMBID(t)  // Unique per test
    artistID := insertTestArtist(t, pool, mbid, "Test Artist")  // Creates row, registers cleanup
    
    // Test code here
    
    // After test: t.Cleanup() deletes the inserted artist row
}
```

## Coverage

**Requirements:**
- No explicit coverage target or requirement
- Coverage measurement: Not enforced by CI
- Approach: Aim for behavior coverage (all code paths, error cases), not line coverage

**Running Coverage (manual):**
```bash
go test ./... -cover
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Test Patterns Reference

**Table-Driven Tests:**
- Not commonly used in current codebase
- Prefer individual test functions with distinct names
- Each test function tests one specific scenario

**Subtests:**
- Used sparingly (`t.Run()` for parameterized variants)
- Example:
  ```go
  func TestSearch_MissingOrBlankQReturns400(t *testing.T) {
    paths := []string{"/search", "/search?q="}
    for _, p := range paths {
      t.Run(p, func(t *testing.T) {
        // test each path
      })
    }
  }
  ```

**Benchmarks:**
- Not used currently
- Could add `Benchmark*` functions if performance profiling needed

---

*Testing analysis: 2026-08-12*
