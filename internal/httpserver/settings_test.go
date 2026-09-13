package httpserver_test

// This file proves GET/PUT /settings/notifications end-to-end (DGST-01,
// DGST-02, DGST-03, DGST-04): a fresh migrated schema seeds the D-05
// singleton row with its defaults, PUT updates it in Postgres, and a
// second GET reads the change back -- not from an in-process cache.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
