package httpserver_test

// This file proves every GET /ready branch (RDY-01, RDY-02): the healthy
// path, the ahead-of-source rollback case (>= not ==), the three configured
// failure reasons, the empty-schema_migrations edge, the not-configured
// route-parity reason, and that no 503 body carries error-derived text.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/httpserver"
)

// readyBody decodes GET /ready's D-03 response shape by field name;
// schema_applied is a pointer so a null is distinguishable from a zero.
type readyBody struct {
	Status         string `json:"status"`
	SchemaApplied  *uint  `json:"schema_applied"`
	SchemaExpected uint   `json:"schema_expected"`
	Reason         string `json:"reason"`
}

// fakeSchemaVersioner is a file-local double for httpserver.SchemaVersioner,
// shared with status_test.go (plan 18-04). It exists so the readiness
// branches can be driven with no database present.
type fakeSchemaVersioner struct {
	fn func(context.Context) (uint, bool, error)
}

func (f fakeSchemaVersioner) SchemaVersion(ctx context.Context) (uint, bool, error) {
	return f.fn(ctx)
}

var _ httpserver.SchemaVersioner = fakeSchemaVersioner{}

func schemaAt(version uint, dirty bool) fakeSchemaVersioner {
	return fakeSchemaVersioner{fn: func(context.Context) (uint, bool, error) {
		return version, dirty, nil
	}}
}

// getReady issues GET /ready against a fresh test server and returns the
// status code, the decoded body, and the raw body bytes.
func getReady(t *testing.T, srv *httpserver.Server) (int, readyBody, string) {
	t.Helper()
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var body readyBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode body %q: %v", raw, err)
	}
	return resp.StatusCode, body, string(raw)
}

func TestReady_Healthy(t *testing.T) {
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger(),
		httpserver.WithReadiness(schemaAt(7, false), 7))
	code, body, raw := getReady(t, srv)

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", code, raw)
	}
	if body.Status != "ready" {
		t.Fatalf("status field = %q, want ready", body.Status)
	}
	if body.SchemaApplied == nil || *body.SchemaApplied != 7 || body.SchemaExpected != 7 {
		t.Fatalf("schema_applied/expected = %v/%d, want 7/7", body.SchemaApplied, body.SchemaExpected)
	}
	if strings.Contains(raw, "reason") {
		t.Fatalf("200 body carries a reason key: %s", raw)
	}
}

func TestReady_AheadOfSource(t *testing.T) {
	// applied = expected + 1: a cleanly rolled-back binary serving a newer
	// additive schema is ready (D-01 -- >=, never ==).
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger(),
		httpserver.WithReadiness(schemaAt(8, false), 7))
	code, body, raw := getReady(t, srv)

	if code != http.StatusOK || body.Status != "ready" {
		t.Fatalf("status = %d / %q, want 200 / ready (body %s)", code, body.Status, raw)
	}
	if body.SchemaApplied == nil || *body.SchemaApplied != 8 {
		t.Fatalf("schema_applied = %v, want 8", body.SchemaApplied)
	}
}

func TestReady_DBUnreachable(t *testing.T) {
	pinger := stubPinger{pingFunc: func(context.Context) error {
		return errors.New("dial tcp: connection refused")
	}}
	schemaCalled := false
	sv := fakeSchemaVersioner{fn: func(context.Context) (uint, bool, error) {
		schemaCalled = true
		return 7, false, nil
	}}
	srv := httpserver.New(pinger, stubStore{}, stubEventsStore{}, nil, discardLogger(),
		httpserver.WithReadiness(sv, 7))
	code, body, raw := getReady(t, srv)

	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body %s)", code, raw)
	}
	if body.Status != "not_ready" || body.Reason != "db_unreachable" {
		t.Fatalf("status/reason = %q/%q, want not_ready/db_unreachable", body.Status, body.Reason)
	}
	if body.SchemaApplied != nil {
		t.Fatalf("schema_applied = %v, want null", *body.SchemaApplied)
	}
	if schemaCalled {
		t.Fatal("schema seam was called after the ping failed")
	}
}

func TestReady_SchemaReadError(t *testing.T) {
	sv := fakeSchemaVersioner{fn: func(context.Context) (uint, bool, error) {
		return 0, false, errors.New("read schema_migrations: boom")
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger(),
		httpserver.WithReadiness(sv, 7))
	code, body, raw := getReady(t, srv)

	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body %s)", code, raw)
	}
	if body.Status != "not_ready" || body.Reason != "db_unreachable" {
		t.Fatalf("status/reason = %q/%q, want not_ready/db_unreachable", body.Status, body.Reason)
	}
	if body.SchemaApplied != nil {
		t.Fatalf("schema_applied = %v, want null", *body.SchemaApplied)
	}
}

func TestReady_SchemaBehind(t *testing.T) {
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger(),
		httpserver.WithReadiness(schemaAt(6, false), 7))
	code, body, raw := getReady(t, srv)

	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body %s)", code, raw)
	}
	if body.Status != "not_ready" || body.Reason != "schema_behind" {
		t.Fatalf("status/reason = %q/%q, want not_ready/schema_behind", body.Status, body.Reason)
	}
	if body.SchemaApplied == nil || *body.SchemaApplied != 6 {
		t.Fatalf("schema_applied = %v, want 6", body.SchemaApplied)
	}
}

func TestReady_SchemaDirty(t *testing.T) {
	// dirty AND behind: dirty wins because it is checked first.
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger(),
		httpserver.WithReadiness(schemaAt(6, true), 7))
	code, body, raw := getReady(t, srv)

	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body %s)", code, raw)
	}
	if body.Status != "not_ready" || body.Reason != "schema_dirty" {
		t.Fatalf("status/reason = %q/%q, want not_ready/schema_dirty", body.Status, body.Reason)
	}
	if body.SchemaApplied == nil || *body.SchemaApplied != 6 {
		t.Fatalf("schema_applied = %v, want 6", body.SchemaApplied)
	}
}

func TestReady_EmptySchemaMigrations(t *testing.T) {
	// db.SchemaVersion maps pgx.ErrNoRows to (0, false, nil); a non-zero
	// expected then reads as schema_behind with schema_applied 0, never a panic.
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger(),
		httpserver.WithReadiness(schemaAt(0, false), 7))
	code, body, raw := getReady(t, srv)

	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body %s)", code, raw)
	}
	if body.Status != "not_ready" || body.Reason != "schema_behind" {
		t.Fatalf("status/reason = %q/%q, want not_ready/schema_behind", body.Status, body.Reason)
	}
	if body.SchemaApplied == nil || *body.SchemaApplied != 0 {
		t.Fatalf("schema_applied = %v, want 0", body.SchemaApplied)
	}
}

func TestReady_NotConfigured(t *testing.T) {
	// No WithReadiness option -- the route still answers so the table is
	// byte-identical with and without the option. A cmd/server binary always
	// wires the option, so this reason is unreachable in production.
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger())
	code, body, raw := getReady(t, srv)

	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body %s)", code, raw)
	}
	if body.Status != "not_ready" || body.Reason != "not_configured" {
		t.Fatalf("status/reason = %q/%q, want not_ready/not_configured", body.Status, body.Reason)
	}
}

func TestReady_NoLeak(t *testing.T) {
	const dsn = "postgres://tracker:Sup3rSecret@db.internal:5432/drop_tracker?sslmode=disable"
	const webhook = "https://discord.com/api/webhooks/123456789/abcdefSECRETtoken"
	pingErr := fmt.Errorf("dial %s failed", dsn)
	schemaErr := fmt.Errorf("read %s (also %s)", dsn, webhook)

	pinger := stubPinger{pingFunc: func(context.Context) error { return pingErr }}
	sv := fakeSchemaVersioner{fn: func(context.Context) (uint, bool, error) { return 0, false, schemaErr }}

	cases := []struct {
		name string
		srv  *httpserver.Server
	}{
		{"ping", httpserver.New(pinger, stubStore{}, stubEventsStore{}, nil, discardLogger(),
			httpserver.WithReadiness(schemaAt(7, false), 7))},
		{"schema", httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger(),
			httpserver.WithReadiness(sv, 7))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, raw := getReady(t, tc.srv)
			if code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503 (body %s)", code, raw)
			}
			for _, leak := range []string{
				dsn, webhook, "Sup3rSecret", "SECRETtoken", "://", "password",
				pingErr.Error(), schemaErr.Error(),
			} {
				if strings.Contains(raw, leak) {
					t.Fatalf("response body leaked %q: %s", leak, raw)
				}
			}
		})
	}
}
