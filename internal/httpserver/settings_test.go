package httpserver_test

// This file proves GET/PUT /settings/notifications end-to-end (DGST-01,
// DGST-02, DGST-03, DGST-04): a fresh migrated schema seeds the D-05
// singleton row with its defaults, PUT updates it in Postgres, and a
// second GET reads the change back -- not from an in-process cache.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/settings"
	"github.com/danielrpof/drop-tracker/internal/testutil"
)

// settingsBody mirrors the fields of settingsResponse this test asserts on,
// decoded by field name rather than raw string comparison.
type settingsBody struct {
	DigestEnabled    bool    `json:"digest_enabled"`
	DigestCadence    string  `json:"digest_cadence"`
	DigestLastSentAt *string `json:"digest_last_sent_at"`
	UpdatedAt        string  `json:"updated_at"`
}

func TestSettings_RoundTrip(t *testing.T) {
	// A process-wide singleton row: an isolated schema is mandatory here, not
	// stylistic -- a shared-fixture assertion on its defaults would be
	// corrupted by a sibling package or the live dev app (mirrors
	// internal/notifier's own reason for the same helper).
	pool := testutil.NewIsolatedTestPool(t, "settings_http_test")

	store := settings.NewService(sqlc.New(pool))
	srv := httpserver.New(pool, stubStore{}, stubEventsStore{}, nil, discardLogger(), httpserver.WithSettings(store))
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// Fresh schema: GET reports D-05's seeded defaults.
	resp, err := http.Get(ts.URL + "/settings/notifications")
	if err != nil {
		t.Fatalf("GET /settings/notifications: %v", err)
	}
	var got settingsBody
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	_ = resp.Body.Close()

	if got.DigestEnabled != false {
		t.Fatalf("digest_enabled = %v, want false", got.DigestEnabled)
	}
	if got.DigestCadence != "daily" {
		t.Fatalf("digest_cadence = %q, want %q", got.DigestCadence, "daily")
	}
	if got.DigestLastSentAt != nil {
		t.Fatalf("digest_last_sent_at = %v, want nil (JSON null)", *got.DigestLastSentAt)
	}

	// PUT updates the row.
	body, err := json.Marshal(map[string]any{
		"digest_enabled": true,
		"digest_cadence": "weekly",
	})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, ts.URL+"/settings/notifications", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /settings/notifications: %v", err)
	}
	var updated settingsBody
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", putResp.StatusCode, http.StatusOK)
	}
	if err := json.NewDecoder(putResp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode PUT response body: %v", err)
	}
	_ = putResp.Body.Close()

	if updated.DigestEnabled != true {
		t.Fatalf("digest_enabled = %v, want true", updated.DigestEnabled)
	}
	if updated.DigestCadence != "weekly" {
		t.Fatalf("digest_cadence = %q, want %q", updated.DigestCadence, "weekly")
	}

	// A second GET reads the change back from Postgres, not from any
	// in-process cache -- Service holds no memoised copy of the row.
	resp2, err := http.Get(ts.URL + "/settings/notifications")
	if err != nil {
		t.Fatalf("second GET /settings/notifications: %v", err)
	}
	var got2 settingsBody
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp2.StatusCode, http.StatusOK)
	}
	if err := json.NewDecoder(resp2.Body).Decode(&got2); err != nil {
		t.Fatalf("decode second response body: %v", err)
	}
	_ = resp2.Body.Close()

	if got2.DigestEnabled != true {
		t.Fatalf("second GET digest_enabled = %v, want true", got2.DigestEnabled)
	}
	if got2.DigestCadence != "weekly" {
		t.Fatalf("second GET digest_cadence = %q, want %q", got2.DigestCadence, "weekly")
	}
}

// --- Task 1: pin every PUT rejection path against a real Postgres row ---

// newSettingsRejectionServer wires a real-Postgres-backed settings server,
// following TestSettings_RoundTrip's exact construction, so a "nothing
// persisted" assertion reads back the real row rather than a fake's memory.
func newSettingsRejectionServer(t *testing.T) *httptest.Server {
	t.Helper()
	pool := testutil.NewIsolatedTestPool(t, "settings_http_test")
	store := settings.NewService(sqlc.New(pool))
	srv := httpserver.New(pool, stubStore{}, stubEventsStore{}, nil, discardLogger(), httpserver.WithSettings(store))
	t.Cleanup(srv.Close)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return ts
}

func putSettingsRaw(t *testing.T, ts *httptest.Server, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, ts.URL+"/settings/notifications", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /settings/notifications: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read PUT response body: %v", err)
	}
	return resp.StatusCode, string(raw)
}

func getSettingsRaw(t *testing.T, ts *httptest.Server) (int, string) {
	t.Helper()
	resp, err := http.Get(ts.URL + "/settings/notifications")
	if err != nil {
		t.Fatalf("GET /settings/notifications: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read GET response body: %v", err)
	}
	return resp.StatusCode, string(raw)
}

// assertSettingsDefaults is the "nothing persisted" half of every rejection
// case: a fresh schema's defaults (digest off, cadence daily) must still be
// exactly what a follow-up GET reports.
func assertSettingsDefaults(t *testing.T, ts *httptest.Server) {
	t.Helper()
	code, raw := getSettingsRaw(t, ts)
	if code != http.StatusOK {
		t.Fatalf("follow-up GET status = %d, want 200 (body %s)", code, raw)
	}
	var got settingsBody
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("decode follow-up GET body %q: %v", raw, err)
	}
	if got.DigestEnabled != false || got.DigestCadence != "daily" {
		t.Fatalf("follow-up GET = %+v, want digest_enabled=false digest_cadence=daily -- nothing should have persisted", got)
	}
}

func TestSettings_PutRejectsUnknownCadence(t *testing.T) {
	ts := newSettingsRejectionServer(t)

	code, raw := putSettingsRaw(t, ts, `{"digest_enabled":true,"digest_cadence":"monthly"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body %s)", code, http.StatusBadRequest, raw)
	}
	var eb errorBody
	if err := json.Unmarshal([]byte(raw), &eb); err != nil {
		t.Fatalf("decode error body %q: %v", raw, err)
	}
	if eb.Error != "invalid digest cadence" {
		t.Fatalf("error = %q, want %q", eb.Error, "invalid digest cadence")
	}

	assertSettingsDefaults(t, ts)
}

// TestSettings_PutRejectsOmittedCadence pins that this route is full-object
// PUT semantics: an absent digest_cadence key decodes to the zero value "",
// which ParseCadence rejects exactly like a bogus one -- there is no
// partial-update path a client could rely on.
func TestSettings_PutRejectsOmittedCadence(t *testing.T) {
	ts := newSettingsRejectionServer(t)

	code, raw := putSettingsRaw(t, ts, `{"digest_enabled":true}`)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body %s)", code, http.StatusBadRequest, raw)
	}
	var eb errorBody
	if err := json.Unmarshal([]byte(raw), &eb); err != nil {
		t.Fatalf("decode error body %q: %v", raw, err)
	}
	if eb.Error != "invalid digest cadence" {
		t.Fatalf("error = %q, want %q -- an omitted cadence must be a client error, not a partial write", eb.Error, "invalid digest cadence")
	}

	assertSettingsDefaults(t, ts)
}

// TestSettings_PutRejectsUnknownFields proves digest_last_sent_at -- the
// server-owned watermark -- is unsettable through this route:
// DisallowUnknownFields rejects it before the store is ever reached.
func TestSettings_PutRejectsUnknownFields(t *testing.T) {
	ts := newSettingsRejectionServer(t)

	const body = `{"digest_enabled":true,"digest_cadence":"weekly","digest_last_sent_at":"2026-01-01T00:00:00Z"}`
	code, raw := putSettingsRaw(t, ts, body)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body %s)", code, http.StatusBadRequest, raw)
	}
	var eb errorBody
	if err := json.Unmarshal([]byte(raw), &eb); err != nil {
		t.Fatalf("decode error body %q: %v", raw, err)
	}
	if eb.Error != "invalid request body" {
		t.Fatalf("error = %q, want %q -- digest_last_sent_at is server-owned and must be unsettable through this route", eb.Error, "invalid request body")
	}

	assertSettingsDefaults(t, ts)
}

func TestSettings_PutRejectsWrongType(t *testing.T) {
	ts := newSettingsRejectionServer(t)

	code, raw := putSettingsRaw(t, ts, `{"digest_enabled":"yes","digest_cadence":"daily"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body %s)", code, http.StatusBadRequest, raw)
	}
	var eb errorBody
	if err := json.Unmarshal([]byte(raw), &eb); err != nil {
		t.Fatalf("decode error body %q: %v", raw, err)
	}
	if eb.Error != "invalid request body" {
		t.Fatalf("error = %q, want %q", eb.Error, "invalid request body")
	}

	assertSettingsDefaults(t, ts)
}

// TestSettings_PutRejectsTrailingJSONValue pins WR-02/G-02-1 for this route:
// decodeJSONBody's end-of-stream assertion catches a second JSON value
// concatenated after a well-formed body.
func TestSettings_PutRejectsTrailingJSONValue(t *testing.T) {
	ts := newSettingsRejectionServer(t)

	const body = `{"digest_enabled":true,"digest_cadence":"weekly"}{"digest_enabled":false,"digest_cadence":"daily"}`
	code, raw := putSettingsRaw(t, ts, body)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body %s)", code, http.StatusBadRequest, raw)
	}
	var eb errorBody
	if err := json.Unmarshal([]byte(raw), &eb); err != nil {
		t.Fatalf("decode error body %q: %v", raw, err)
	}
	if eb.Error != "invalid request body" {
		t.Fatalf("error = %q, want %q -- a trailing JSON value must be indistinguishable from any other malformed body", eb.Error, "invalid request body")
	}

	assertSettingsDefaults(t, ts)
}

// TestSettings_PutRejectsOversizeBody pads digest_cadence past the shared
// 64 KiB ceiling (mirrors TestWatchlist_Add_RejectsOversizeBody) and asserts
// only that the status lands in the 4xx range -- the exact code depends on
// where http.MaxBytesReader trips, and pinning it would over-specify.
func TestSettings_PutRejectsOversizeBody(t *testing.T) {
	ts := newSettingsRejectionServer(t)

	body := `{"digest_enabled":true,"digest_cadence":"` + strings.Repeat("a", 70000) + `"}`
	code, raw := putSettingsRaw(t, ts, body)
	if code < 400 || code >= 500 {
		t.Fatalf("status = %d, want a 4xx (body %s)", code, raw)
	}

	assertSettingsDefaults(t, ts)
}

// --- Task 2: pin the gate, the CSRF requirement, the unconfigured 503, and the no-leak guarantee ---

// settingsTestPassphrase is the same literal server_test.go's newGatedServer
// and status_test.go's TestStatus_Gated401 already use for a gated server.
const settingsTestPassphrase = "a-real-passphrase"

// fakeSettingsStore is a file-local double for httpserver.SettingsStore,
// mirroring fakeStatusStore/fakeWatchlistCounter's func-field shape. calls,
// when non-nil, counts Update invocations so the "store never called" /
// "store called exactly once" assertions are direct observations rather than
// inferences.
type fakeSettingsStore struct {
	getFunc    func(context.Context) (settings.Settings, error)
	updateFunc func(context.Context, settings.UpdateParams) (settings.Settings, error)
	calls      *int32
}

func (f fakeSettingsStore) Get(ctx context.Context) (settings.Settings, error) {
	if f.getFunc != nil {
		return f.getFunc(ctx)
	}
	return settings.Settings{DigestCadence: settings.CadenceDaily}, nil
}

func (f fakeSettingsStore) Update(ctx context.Context, p settings.UpdateParams) (settings.Settings, error) {
	if f.calls != nil {
		atomic.AddInt32(f.calls, 1)
	}
	if f.updateFunc != nil {
		return f.updateFunc(ctx, p)
	}
	return settings.Settings{DigestEnabled: p.DigestEnabled, DigestCadence: p.DigestCadence}, nil
}

var _ httpserver.SettingsStore = fakeSettingsStore{}

// settingsServerOpts assembles the handler-/router-level server variants
// this task needs: gated or inert, a supplied fake store, or WithSettings
// omitted entirely -- mirroring statusOpts's shape in status_test.go.
type settingsServerOpts struct {
	passphrase   string
	store        httpserver.SettingsStore
	omitSettings bool
}

func newSettingsServer(t *testing.T, o settingsServerOpts) *httptest.Server {
	t.Helper()
	opts := []httpserver.Option{}
	if o.passphrase != "" {
		opts = append(opts, httpserver.WithAuthGate(o.passphrase, false, nil))
	}
	if !o.omitSettings {
		store := o.store
		if store == nil {
			store = fakeSettingsStore{}
		}
		opts = append(opts, httpserver.WithSettings(store))
	}
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger(), opts...)
	t.Cleanup(srv.Close)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return ts
}

// loginForSettings mints a real session the way a browser does -- POST
// /session with the SPA's X-Requested-With header and the passphrase in the
// JSON body -- then lifts the dt_session cookie off the response, mirroring
// internal/authgate/gate_test.go's sessionCookie/login helpers. No cookie is
// hand-forged or signed here.
func loginForSettings(t *testing.T, ts *httptest.Server) *http.Cookie {
	t.Helper()
	body, err := json.Marshal(map[string]string{"passphrase": settingsTestPassphrase})
	if err != nil {
		t.Fatalf("marshal login body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/session", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build POST /session: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "drop-tracker")
	// http.DefaultClient carries a nil Jar, so no case here can accidentally
	// inherit a session it did not explicitly attach via req.AddCookie.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /session: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /session status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "dt_session" {
			return c
		}
	}
	t.Fatalf("no dt_session cookie in login response: %v", resp.Header.Values("Set-Cookie"))
	return nil
}

func TestSettings_GetGated401NoCookie(t *testing.T) {
	ts := newSettingsServer(t, settingsServerOpts{passphrase: settingsTestPassphrase})

	resp, err := http.Get(ts.URL + "/settings/notifications")
	if err != nil {
		t.Fatalf("GET /settings/notifications: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body %s)", resp.StatusCode, raw)
	}
	for _, leak := range []string{"digest_enabled", "digest_cadence", "digest_last_sent_at"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("401 body leaked %q: %s", leak, raw)
		}
	}
}

// TestSettings_PutGated401NoCookie proves the write verb also answers 401,
// not 403, without a session -- gate.Authenticate runs before
// gate.RequireCSRFHeader (server.go's gated-group construction).
func TestSettings_PutGated401NoCookie(t *testing.T) {
	ts := newSettingsServer(t, settingsServerOpts{passphrase: settingsTestPassphrase})

	code, raw := putSettingsRaw(t, ts, `{"digest_enabled":true,"digest_cadence":"weekly"}`)
	if code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, not 403 -- Authenticate must run before RequireCSRFHeader (body %s)", code, raw)
	}
}

func TestSettings_PutGatedForbiddenWithoutCSRFHeader(t *testing.T) {
	var calls int32
	ts := newSettingsServer(t, settingsServerOpts{passphrase: settingsTestPassphrase, store: fakeSettingsStore{calls: &calls}})
	cookie := loginForSettings(t, ts)

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/settings/notifications", strings.NewReader(`{"digest_enabled":true,"digest_cadence":"weekly"}`))
	if err != nil {
		t.Fatalf("build PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	// Deliberately no X-Requested-With header.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /settings/notifications: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("store.Update called %d times, want 0", got)
	}
}

func TestSettings_PutGatedSucceedsWithCookieAndHeader(t *testing.T) {
	var calls int32
	ts := newSettingsServer(t, settingsServerOpts{passphrase: settingsTestPassphrase, store: fakeSettingsStore{calls: &calls}})
	cookie := loginForSettings(t, ts)

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/settings/notifications", strings.NewReader(`{"digest_enabled":true,"digest_cadence":"weekly"}`))
	if err != nil {
		t.Fatalf("build PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "drop-tracker")
	req.AddCookie(cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /settings/notifications: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("store.Update called %d times, want 1", got)
	}
}

// TestSettings_UngatedNeverAnswers401 preserves the v1.2 posture for an
// unconfigured (no passphrase) instance: neither verb is ever 401.
func TestSettings_UngatedNeverAnswers401(t *testing.T) {
	ts := newSettingsServer(t, settingsServerOpts{})

	getResp, err := http.Get(ts.URL + "/settings/notifications")
	if err != nil {
		t.Fatalf("GET /settings/notifications: %v", err)
	}
	_ = getResp.Body.Close()
	if getResp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("GET status = 401 on an inert (ungated) server, want non-401")
	}

	putCode, putRaw := putSettingsRaw(t, ts, `{"digest_enabled":true,"digest_cadence":"weekly"}`)
	if putCode == http.StatusUnauthorized {
		t.Fatalf("PUT status = 401 on an inert (ungated) server, want non-401 (body %s)", putRaw)
	}
}

// TestSettings_NotConfiguredAnswers503 mirrors TestStatus_NotConfigured: a
// server built with no WithSettings option answers 503 with the shared fixed
// body on both verbs, so the route table is identical with and without it.
func TestSettings_NotConfiguredAnswers503(t *testing.T) {
	ts := newSettingsServer(t, settingsServerOpts{omitSettings: true})

	getCode, getRaw := getSettingsRaw(t, ts)
	if getCode != http.StatusServiceUnavailable {
		t.Fatalf("GET status = %d, want 503 (body %s)", getCode, getRaw)
	}
	var getErr errorBody
	if err := json.Unmarshal([]byte(getRaw), &getErr); err != nil {
		t.Fatalf("decode GET error body %q: %v", getRaw, err)
	}
	if getErr.Error != "settings not available" {
		t.Fatalf("GET error = %q, want %q", getErr.Error, "settings not available")
	}

	putCode, putRaw := putSettingsRaw(t, ts, `{"digest_enabled":true,"digest_cadence":"weekly"}`)
	if putCode != http.StatusServiceUnavailable {
		t.Fatalf("PUT status = %d, want 503 (body %s)", putCode, putRaw)
	}
	var putErr errorBody
	if err := json.Unmarshal([]byte(putRaw), &putErr); err != nil {
		t.Fatalf("decode PUT error body %q: %v", putRaw, err)
	}
	if putErr.Error != "settings not available" {
		t.Fatalf("PUT error = %q, want %q", putErr.Error, "settings not available")
	}
}

// TestSettings_NoLeak mirrors TestStatus_NoLeak: a store error whose text
// embeds a DSN password and a Discord webhook token must never reach the
// raw response body on either verb, even though the decoded error field is
// the fixed "internal error" string.
func TestSettings_NoLeak(t *testing.T) {
	const dsn = "postgres://tracker:Sup3rSecret@db.internal:5432/drop_tracker?sslmode=disable"
	const webhook = "https://discord.com/api/webhooks/123456789/abcdefSECRETtoken"
	getErr := fmt.Errorf("get notification settings against %s (webhook %s)", dsn, webhook)
	updateErr := fmt.Errorf("update notification settings against %s (webhook %s)", dsn, webhook)

	cases := []struct {
		name string
		ts   *httptest.Server
		put  bool
	}{
		{"get-error", newSettingsServer(t, settingsServerOpts{store: fakeSettingsStore{
			getFunc: func(context.Context) (settings.Settings, error) { return settings.Settings{}, getErr },
		}}), false},
		{"update-error", newSettingsServer(t, settingsServerOpts{store: fakeSettingsStore{
			updateFunc: func(context.Context, settings.UpdateParams) (settings.Settings, error) { return settings.Settings{}, updateErr },
		}}), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var code int
			var raw string
			if tc.put {
				code, raw = putSettingsRaw(t, tc.ts, `{"digest_enabled":true,"digest_cadence":"weekly"}`)
			} else {
				code, raw = getSettingsRaw(t, tc.ts)
			}
			if code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500 (body %s)", code, raw)
			}
			var eb errorBody
			if err := json.Unmarshal([]byte(raw), &eb); err != nil {
				t.Fatalf("decode error body %q: %v", raw, err)
			}
			if eb.Error != "internal error" {
				t.Fatalf("error = %q, want %q", eb.Error, "internal error")
			}
			for _, leak := range []string{dsn, webhook, "Sup3rSecret", "SECRETtoken", "://", "password", getErr.Error(), updateErr.Error()} {
				if strings.Contains(raw, leak) {
					t.Fatalf("response body leaked %q: %s", leak, raw)
				}
			}
		})
	}
}
