package httpserver_test

// This file proves GET/PUT /settings/notifications end-to-end (DGST-01,
// DGST-02, DGST-03, DGST-04): a fresh migrated schema seeds the D-05
// singleton row with its defaults, PUT updates it in Postgres, and a
// second GET reads the change back -- not from an in-process cache.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
