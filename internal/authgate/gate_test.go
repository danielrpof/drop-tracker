package authgate_test

// This file proves the gate end to end against a real httptest server wired
// through httpserver.New + httpserver.WithAuthGate: an unauthenticated data
// route is 401, POST /session with the correct passphrase mints a cookie
// that unlocks the route, DELETE /session clears it, and /health stays
// public. It also pins the exemption boundary (GATE-01 adjacency/empty/
// ordering/concurrency) and the full session-cookie contract (GATE-03,
// D-06/D-17).
//
// Cookies are extracted from resp.Cookies() and attached to later requests
// via req.AddCookie -- never an http.Client cookie jar: Go's jar refuses to
// replay a Secure cookie over a plain-http httptest server and would make a
// correct implementation look broken (Pitfall 8).

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/authgate"
	"github.com/danielrpof/drop-tracker/internal/events"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

const testPassphrase = "correct horse battery staple 42"

// --- minimal stubs so this external test package can call httpserver.New
// without a live Postgres or real external clients ---

type stubPinger struct{}

func (stubPinger) Ping(context.Context) error { return nil }

type stubStore struct{}

func (stubStore) Add(context.Context, watchlist.AddParams) (watchlist.Entry, error) {
	return watchlist.Entry{}, nil
}
func (stubStore) List(context.Context) ([]watchlist.Entry, error) { return []watchlist.Entry{}, nil }
func (stubStore) UpdatePreferences(context.Context, int64, watchlist.PreferencesParams) (watchlist.Entry, error) {
	return watchlist.Entry{}, nil
}
func (stubStore) Remove(context.Context, int64) error { return nil }

type stubEventsStore struct{}

func (stubEventsStore) List(context.Context, events.ListParams) (events.Page, error) {
	return events.Page{Events: []events.Event{}}, nil
}

type fakeAlerter struct{}

func (fakeAlerter) Alert(context.Context, string) error { return nil }

// syncBuffer is a mutex-guarded writer: httplog writes from the goroutine
// serving each request, so the concurrency test below would race a plain
// bytes.Buffer.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newGatedServer builds an httptest server with the gate enabled for
// passphrase. Returns the server plus the derived key so a test can mint its
// own tokens directly.
func newGatedServer(t *testing.T, logger *slog.Logger) (*httptest.Server, [32]byte) {
	t.Helper()
	srv := httpserver.New(
		stubPinger{}, stubStore{}, stubEventsStore{}, nil, logger,
		httpserver.WithAuthGate(testPassphrase, false, fakeAlerter{}),
	)
	t.Cleanup(srv.Close)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return ts, authgate.DeriveKey(testPassphrase)
}

// sessionCookie returns the dt_session cookie from resp, or fails.
func sessionCookie(t *testing.T, resp *http.Response) *http.Cookie {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == "dt_session" {
			return c
		}
	}
	t.Fatalf("no dt_session cookie in response Set-Cookie headers: %v", resp.Header.Values("Set-Cookie"))
	return nil
}

func login(t *testing.T, ts *httptest.Server, passphrase string) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"passphrase": passphrase})
	resp, err := http.Post(ts.URL+"/session", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /session: %v", err)
	}
	return resp
}

// --- Task 2: the tracer end-to-end sequence ---

func TestGate_EndToEnd_401_Login_200_Logout(t *testing.T) {
	ts, _ := newGatedServer(t, discardLogger())

	// 1. unauthenticated GET /watchlist -> 401 {"error":"unauthenticated"}
	resp, err := http.Get(ts.URL + "/watchlist")
	if err != nil {
		t.Fatalf("GET /watchlist: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth GET /watchlist status = %d, want 401", resp.StatusCode)
	}
	var body struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	_ = resp.Body.Close()
	if body.Error != "unauthenticated" {
		t.Fatalf("401 body error = %q, want %q", body.Error, "unauthenticated")
	}

	// 2. unauthenticated GET /health -> 200
	hResp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	if hResp.StatusCode != http.StatusOK {
		t.Fatalf("unauth GET /health status = %d, want 200", hResp.StatusCode)
	}
	_ = hResp.Body.Close()

	// 3. POST /session with the correct passphrase -> 204 + Set-Cookie
	lResp := login(t, ts, testPassphrase)
	if lResp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /session (correct) status = %d, want 204", lResp.StatusCode)
	}
	cookie := sessionCookie(t, lResp)
	_ = lResp.Body.Close()

	// 4. replay the cookie on GET /watchlist -> 200
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/watchlist", nil)
	req.AddCookie(cookie)
	aResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authenticated GET /watchlist: %v", err)
	}
	if aResp.StatusCode != http.StatusOK {
		t.Fatalf("authenticated GET /watchlist status = %d, want 200", aResp.StatusCode)
	}
	_ = aResp.Body.Close()

	// 5. DELETE /session -> 204 + Set-Cookie Max-Age=0
	dReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/session", nil)
	dResp, err := http.DefaultClient.Do(dReq)
	if err != nil {
		t.Fatalf("DELETE /session: %v", err)
	}
	if dResp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE /session status = %d, want 204", dResp.StatusCode)
	}
	cleared := sessionCookie(t, dResp)
	_ = dResp.Body.Close()
	if cleared.MaxAge > 0 {
		t.Fatalf("logout cookie MaxAge = %d, want <= 0 (Max-Age=0)", cleared.MaxAge)
	}
	if !strings.Contains(strings.ToLower(strings.Join(dResp.Header.Values("Set-Cookie"), " ")), "max-age=0") {
		t.Fatalf("logout Set-Cookie does not render Max-Age=0: %v", dResp.Header.Values("Set-Cookie"))
	}
}

func TestGate_WrongPassphrase_401_NoCookie(t *testing.T) {
	ts, _ := newGatedServer(t, discardLogger())

	resp := login(t, ts, "not the passphrase")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST /session (wrong) status = %d, want 401", resp.StatusCode)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "dt_session" {
			t.Fatalf("wrong passphrase set a dt_session cookie: %+v", c)
		}
	}
}

// TestGate_NoOptionsIsUngated proves the inert path: a server built with no
// options answers GET /watchlist 200 with no cookie at all (GATE-07).
func TestGate_NoOptionsIsUngated(t *testing.T) {
	srv := httpserver.New(stubPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/watchlist")
	if err != nil {
		t.Fatalf("GET /watchlist: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ungated GET /watchlist status = %d, want 200", resp.StatusCode)
	}
}

// --- Task 3: exemption boundary, empty inputs, ordering, concurrency ---

// healthPayloadFields decodes body and reports whether it looks like the
// /health JSON (has both "status" and "db" string fields).
func looksLikeHealth(b []byte) bool {
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return false
	}
	_, hasStatus := m["status"].(string)
	_, hasDB := m["db"].(string)
	return hasStatus && hasDB
}

func TestGate_ExemptionBoundary_HealthIsExactPathOnly(t *testing.T) {
	ts, _ := newGatedServer(t, discardLogger())

	// /health returns the payload unauthenticated
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	data, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !looksLikeHealth(data) {
		t.Fatalf("GET /health status=%d body=%q, want 200 + health payload", resp.StatusCode, data)
	}

	// adjacency: /healthz and /health/details must NOT return the health payload
	for _, path := range []string{"/healthz", "/health/details"} {
		r, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if looksLikeHealth(b) {
			t.Fatalf("GET %s returned the health payload %q -- /health leaked as a prefix", path, b)
		}
	}
}

// TestGate_PublicSPAShell pins D-04: the static shell and hashed assets stay
// reachable unauthenticated on a gated server, or the passphrase form could
// never render (Pitfall 23).
func TestGate_PublicSPAShell(t *testing.T) {
	ts, _ := newGatedServer(t, discardLogger())

	for _, path := range []string{"/", "/assets/does-not-exist-hashed.js"} {
		r, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = r.Body.Close()
		if r.StatusCode == http.StatusUnauthorized {
			t.Fatalf("GET %s returned 401 on a gated server -- the SPA shell must serve publicly (D-04)", path)
		}
	}
}

// TestGate_EmptyCookieInputs covers the three empty shapes: no Cookie
// header, an empty Cookie header, and dt_session present but empty. Each
// must be 401 and must not panic (Recoverer would turn a panic into a 500).
func TestGate_EmptyCookieInputs(t *testing.T) {
	ts, _ := newGatedServer(t, discardLogger())

	cases := []struct {
		name   string
		cookie string
		set    bool
	}{
		{"no cookie header", "", false},
		{"empty cookie header", "", true},
		{"dt_session present but empty", "dt_session=", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, ts.URL+"/events", nil)
			if tc.set {
				req.Header.Set("Cookie", tc.cookie)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("GET /events: %v", err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (a 500 means the gate panicked)", resp.StatusCode)
			}
		})
	}
}

// TestGate_RejectedRequestIsLoggedWithRequestID proves D-05 ordering: the
// gate runs inside the Group, after the four parent middlewares, so a
// rejected 401 is still logged carrying a request_id attribute.
func TestGate_RejectedRequestIsLoggedWithRequestID(t *testing.T) {
	buf := &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(buf, nil))
	ts, _ := newGatedServer(t, logger)

	resp, err := http.Get(ts.URL + "/watchlist")
	if err != nil {
		t.Fatalf("GET /watchlist: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}

	sc := bufio.NewScanner(strings.NewReader(buf.String()))
	for sc.Scan() {
		var fields map[string]any
		if json.Unmarshal(sc.Bytes(), &fields) != nil {
			continue
		}
		if id, ok := fields["request_id"].(string); ok && id != "" {
			return
		}
	}
	t.Fatalf("no log line carried a non-empty request_id attribute:\n%s", buf.String())
}

// TestGate_Concurrency is the backstop-tier check for GATE-01 concurrency:
// CI runs it under -race; this dev machine cannot. N parallel unauthenticated
// requests each 401, N parallel authenticated requests each 200.
func TestGate_Concurrency(t *testing.T) {
	ts, key := newGatedServer(t, discardLogger())

	now := time.Now()
	valid := authgate.Sign(key, authgate.Token{IssuedAt: now, Expiry: now.Add(30 * 24 * time.Hour)})

	const n = 12
	run := func(withCookie bool) []int {
		codes := make([]int, n)
		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func(idx int) {
				defer wg.Done()
				req, _ := http.NewRequest(http.MethodGet, ts.URL+"/events", nil)
				if withCookie {
					req.AddCookie(&http.Cookie{Name: "dt_session", Value: valid})
				}
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					codes[idx] = -1
					return
				}
				_ = resp.Body.Close()
				codes[idx] = resp.StatusCode
			}(i)
		}
		wg.Wait()
		return codes
	}

	for i, c := range run(false) {
		if c != http.StatusUnauthorized {
			t.Fatalf("unauth request %d: status %d, want 401", i, c)
		}
	}
	for i, c := range run(true) {
		if c != http.StatusOK {
			t.Fatalf("authed request %d: status %d, want 200", i, c)
		}
	}
}

// --- Task 4: session cookie contract ---

func TestGate_LoginCookieAttributes(t *testing.T) {
	ts, _ := newGatedServer(t, discardLogger())

	resp := login(t, ts, testPassphrase)
	defer func() { _ = resp.Body.Close() }()
	c := sessionCookie(t, resp)

	if !c.HttpOnly {
		t.Error("cookie HttpOnly = false, want true")
	}
	if !c.Secure {
		t.Error("cookie Secure = false, want true")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite = %v, want Lax", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("cookie Path = %q, want /", c.Path)
	}
	wantMaxAge := int((30 * 24 * time.Hour).Seconds())
	if c.MaxAge != wantMaxAge {
		t.Errorf("cookie MaxAge = %d, want %d (2592000)", c.MaxAge, wantMaxAge)
	}

	// Assert on the raw header text too, so a future refactor to a hand-built
	// header string cannot silently drop an attribute.
	raw := strings.ToLower(strings.Join(resp.Header.Values("Set-Cookie"), " "))
	for _, want := range []string{"httponly", "secure", "samesite=lax", "path=/", "max-age=2592000"} {
		if !strings.Contains(raw, want) {
			t.Errorf("raw Set-Cookie %q missing %q", raw, want)
		}
	}
}

func TestGate_TwoLoginsRotateCookieValue(t *testing.T) {
	ts, _ := newGatedServer(t, discardLogger())

	r1 := login(t, ts, testPassphrase)
	c1 := sessionCookie(t, r1)
	_ = r1.Body.Close()

	r2 := login(t, ts, testPassphrase)
	c2 := sessionCookie(t, r2)
	_ = r2.Body.Close()

	if c1.Value == c2.Value {
		t.Fatal("two consecutive logins produced the same cookie value (D-17 rotation)")
	}
}

// TestGate_SlidingRenewal drives a gated request with a token past its
// halfway mark and asserts the response carries a fresh cookie whose decoded
// IssuedAt is byte-identical and whose Expiry moved later (Pitfall 5).
func TestGate_SlidingRenewal(t *testing.T) {
	ts, key := newGatedServer(t, discardLogger())

	issued := time.Now().Add(-20 * 24 * time.Hour).Truncate(time.Second)
	old := authgate.Token{IssuedAt: issued, Expiry: issued.Add(30 * 24 * time.Hour)}
	raw := authgate.Sign(key, old)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/events", nil)
	req.AddCookie(&http.Cookie{Name: "dt_session", Value: raw})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	renewed := sessionCookie(t, resp)
	got, _, ok := authgate.Verify(key, renewed.Value, time.Now())
	if !ok {
		t.Fatal("renewed cookie does not verify")
	}
	if !got.IssuedAt.Equal(issued) {
		t.Fatalf("renewed IssuedAt = %v, want unchanged %v", got.IssuedAt, issued)
	}
	if !got.Expiry.After(old.Expiry) {
		t.Fatalf("renewed Expiry = %v, want later than %v", got.Expiry, old.Expiry)
	}
}

// TestGate_SlidingRenewal_ShrunkDurations exercises the same renewal
// guarantee as TestGate_SlidingRenewal but with the D-06 durations shrunk to
// milliseconds via the export_test setters, so the "past halfway" condition
// is reached without a 20-day-old token.
func TestGate_SlidingRenewal_ShrunkDurations(t *testing.T) {
	defer authgate.SetSessionWindowForTest(200 * time.Millisecond)()
	defer authgate.SetRenewAfterForTest(150 * time.Millisecond)()
	defer authgate.SetAbsoluteCapForTest(time.Hour)()

	ts, key := newGatedServer(t, discardLogger())

	issued := time.Now().Truncate(time.Millisecond)
	raw := authgate.Sign(key, authgate.Token{IssuedAt: issued, Expiry: issued.Add(200 * time.Millisecond)})

	time.Sleep(120 * time.Millisecond) // now past the 50ms-remaining halfway mark

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/events", nil)
	req.AddCookie(&http.Cookie{Name: "dt_session", Value: raw})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	renewed := sessionCookie(t, resp)
	if renewed.Value == raw {
		t.Fatal("renewed cookie value equals the original -- no renewal happened")
	}
	got, _, ok := authgate.Verify(key, renewed.Value, time.Now())
	if !ok {
		t.Fatal("renewed cookie does not verify")
	}
	if !got.IssuedAt.Equal(issued) {
		t.Fatalf("renewed IssuedAt = %v, want unchanged %v", got.IssuedAt, issued)
	}
}

// TestGate_AbsoluteCapRejectsUnexpiredToken: a token past the 90-day cap is
// 401 even though its Expiry is still in the future.
func TestGate_AbsoluteCapRejectsUnexpiredToken(t *testing.T) {
	ts, key := newGatedServer(t, discardLogger())

	issued := time.Now().Add(-91 * 24 * time.Hour)
	tok := authgate.Token{IssuedAt: issued, Expiry: time.Now().Add(24 * time.Hour)}
	raw := authgate.Sign(key, tok)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/events", nil)
	req.AddCookie(&http.Cookie{Name: "dt_session", Value: raw})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (absolute cap must fire despite an unexpired Expiry)", resp.StatusCode)
	}
}

func TestGate_LogoutClearsCookie(t *testing.T) {
	ts, _ := newGatedServer(t, discardLogger())

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/session", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /session: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	c := sessionCookie(t, resp)
	if c.MaxAge > 0 {
		t.Fatalf("MaxAge = %d, want 0", c.MaxAge)
	}
	raw := strings.ToLower(strings.Join(resp.Header.Values("Set-Cookie"), " "))
	if !strings.Contains(raw, "max-age=0") {
		t.Fatalf("Set-Cookie %q does not render Max-Age=0", raw)
	}
}

// TestGate_MintedCookieSurvivesNewManager proves GATE-02/D-08: a cookie
// minted by one Manager verifies against a second Manager built from the
// same passphrase -- a restart or redeploy does not log anyone out.
func TestGate_MintedCookieSurvivesNewManager(t *testing.T) {
	k1 := authgate.DeriveKey(testPassphrase)
	now := time.Now()
	raw := authgate.Sign(k1, authgate.Token{IssuedAt: now, Expiry: now.Add(30 * 24 * time.Hour)})

	k2 := authgate.DeriveKey(testPassphrase)
	if _, _, ok := authgate.Verify(k2, raw, now.Add(time.Hour)); !ok {
		t.Fatal("cookie minted under a first derived key did not verify under a second derived key from the same passphrase")
	}
}
