package httpserver_test

// This file proves the go:embed + chi SPA fallback wiring (06-RESEARCH.md
// Pattern 3, T-06-05): the root path and any unregistered non-asset path
// both serve the embedded SPA's index.html, while every pre-existing API
// route continues to reach its own handler instead of falling through to
// the SPA fallback.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/events"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
)

func TestSPA_RootPathReturns200WithHTML(t *testing.T) {
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want it to contain %q", ct, "text/html")
	}
}

func TestSPA_UnregisteredPathFallsBackToIndexHTML(t *testing.T) {
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/history/whatever/not-a-real-file")
	if err != nil {
		t.Fatalf("GET /history/whatever/not-a-real-file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (an unregistered client-side route must serve index.html, not 404)", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want it to contain %q", ct, "text/html")
	}
}

// TestSPA_APIRoutesStillReachTheirOwnHandlers proves T-06-05: /health and
// /events must never fall through to the SPA handler, even though both are
// registered before r.NotFound.
func TestSPA_APIRoutesStillReachTheirOwnHandlers(t *testing.T) {
	called := false
	stub := stubEventsStore{listFunc: func(context.Context, events.ListParams) (events.Page, error) {
		called = true
		return events.Page{Events: []events.Event{}}, nil
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, stub, nil, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	healthResp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer healthResp.Body.Close()
	if ct := healthResp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("/health Content-Type = %q, want application/json (must reach the health handler, not the SPA fallback)", ct)
	}

	eventsResp, err := http.Get(ts.URL + "/events")
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer eventsResp.Body.Close()
	if ct := eventsResp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("/events Content-Type = %q, want application/json (must reach handleListEvents, not the SPA fallback)", ct)
	}
	if !called {
		t.Fatal("stubEventsStore.List was not called -- /events fell through to the SPA fallback instead of handleListEvents")
	}
}
