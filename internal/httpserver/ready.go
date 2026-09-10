package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
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

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), readyCheckTimeout)
	defer cancel()

	resp := readyResponse{Status: "ready", SchemaExpected: s.expectedSchema}
	code := http.StatusOK

	setNotReady := func() {
		resp.Status = "not_ready"
		code = http.StatusServiceUnavailable
	}

	if s.schema == nil {
		setNotReady()
	} else if err := s.db.Ping(ctx); err != nil {
		setNotReady()
	} else if applied, dirty, verr := s.schema.SchemaVersion(ctx); verr != nil {
		setNotReady()
	} else {
		a := applied
		resp.SchemaApplied = &a
		// D-01: ready is applied-at-or-above-expected and not dirty, never
		// equality. Phase 16's ahead-of-source guard (internal/db/migrate.go)
		// lets a rolled-back binary serve a newer additive schema; == would
		// flap the deferred Phase 17 deploy gate.
		if ready := !dirty && applied >= s.expectedSchema; !ready {
			setNotReady()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(resp)
}
