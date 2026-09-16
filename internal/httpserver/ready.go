package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/httplog/v3"
)

// SchemaVersioner is the minimal surface handleReady needs to read the
// applied migration version. internal/db.SchemaVersionReader satisfies it.
// Mirrors Pinger: a consumer-declared seam so the readiness branches are
// testable with a fake and no live database.
type SchemaVersioner interface {
	SchemaVersion(ctx context.Context) (version uint, dirty bool, err error)
}

// readyCheckTimeout bounds the ping and the schema read GET /ready issues. It
// deliberately equals healthPingTimeout but stays a separate const so
// retuning readiness never silently retunes liveness (RDY-03).
const readyCheckTimeout = 3 * time.Second

// readyResponse is the D-03 locked shape. schema_applied is a pointer so it
// encodes as null when the database is unreachable; reason is omitted on 200.
type readyResponse struct {
	Status         string `json:"status"`
	SchemaApplied  *uint  `json:"schema_applied"`
	SchemaExpected uint   `json:"schema_expected"`
	Reason         string `json:"reason,omitempty"`
}

// reasonDBUnreachable is the single 503 reason shared by the ping-failure and
// the schema-read-failure branches (D-02): a caller cannot tell, and need not
// tell, which of the two database interactions failed.
const reasonDBUnreachable = "db_unreachable"

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), readyCheckTimeout)
	defer cancel()

	resp := readyResponse{Status: "ready", SchemaExpected: s.expectedSchema}
	code := http.StatusOK

	fail := func(reason string, applied *uint) {
		resp.Status, resp.Reason, resp.SchemaApplied = "not_ready", reason, applied
		code = http.StatusServiceUnavailable
	}

	if s.schema == nil {
		// WithReadiness was not supplied. Kept so the route table is identical
		// with and without the option; a cmd/server binary wires it always.
		fail("not_configured", nil)
	} else if err := s.db.Ping(ctx); err != nil {
		httplog.SetAttrs(r.Context(), slog.String("ready_db_error", err.Error()))
		fail(reasonDBUnreachable, nil)
	} else if applied, dirty, serr := s.schema.SchemaVersion(ctx); serr != nil {
		httplog.SetAttrs(r.Context(), slog.String("ready_schema_error", serr.Error()))
		fail(reasonDBUnreachable, nil)
	} else {
		switch {
		case dirty:
			// Checked before the version comparison: a dirty-and-behind
			// database reports schema_dirty.
			fail("schema_dirty", &applied)
		case applied < s.expectedSchema:
			// D-01: ready is applied-at-or-above-expected, never equality.
			// Phase 16's ahead-of-source guard (internal/db/migrate.go) lets a
			// rolled-back binary serve a newer additive schema; == would flap
			// the deferred Phase 17 deploy gate.
			fail("schema_behind", &applied)
		default:
			resp.SchemaApplied = &applied
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(resp)
}
