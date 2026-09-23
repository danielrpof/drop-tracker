package httpserver_test

// This is the tracer: one entry point (attach) wired through every backend
// layer against a real Postgres, proving the whole chain -- normalize,
// get-or-create, link, and the single-query GET /watchlist enrichment.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/tags"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

// tagWire is the JSON shape of a single tag on the wire.
type tagWire struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// watchlistTagsEntryBody is the subset of GET /watchlist's per-entry body
// this file asserts on.
type watchlistTagsEntryBody struct {
	ID   int64     `json:"id"`
	MBID string    `json:"mbid"`
	Tags []tagWire `json:"tags"`
	Note *string   `json:"note"`
}

func TestTags_AttachEndToEnd(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_http_test")
	ctx := context.Background()

	watchlistSvc := watchlist.NewService(sqlc.New(pool))
	tagsSvc := tags.NewService(pool)
	srv := httpserver.New(pool, watchlistSvc, stubEventsStore{}, nil, discardLogger(), httpserver.WithTags(tagsSvc))
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	tagged, err := watchlistSvc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + "-tagged", Name: "Tagged Artist"})
	if err != nil {
		t.Fatalf("seed tagged entry: %v", err)
	}
	untagged, err := watchlistSvc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + "-untagged", Name: "Untagged Artist"})
	if err != nil {
		t.Fatalf("seed untagged entry: %v", err)
	}

	body := `{"name":"Reggaeton "}`
	resp, err := http.Post(ts.URL+"/watchlist/"+strconv.FormatInt(tagged.ID, 10)+"/tags", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /watchlist/{id}/tags: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var attached tagWire
	if err := json.NewDecoder(resp.Body).Decode(&attached); err != nil {
		t.Fatalf("decode attach response: %v", err)
	}
	if attached.Name != "Reggaeton" {
		t.Fatalf("attached tag name = %q, want %q (trimmed)", attached.Name, "Reggaeton")
	}
	if attached.ID == 0 {
		t.Fatal("attached tag id is zero")
	}

	listResp, err := http.Get(ts.URL + "/watchlist")
	if err != nil {
		t.Fatalf("GET /watchlist: %v", err)
	}
	defer func() { _ = listResp.Body.Close() }()

	var entries []watchlistTagsEntryBody
	if err := json.NewDecoder(listResp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode watchlist list: %v", err)
	}

	var taggedEntry, untaggedEntry *watchlistTagsEntryBody
	for i := range entries {
		switch entries[i].ID {
		case tagged.ID:
			taggedEntry = &entries[i]
		case untagged.ID:
			untaggedEntry = &entries[i]
		}
	}
	if taggedEntry == nil {
		t.Fatal("tagged entry missing from GET /watchlist")
	}
	if untaggedEntry == nil {
		t.Fatal("untagged entry missing from GET /watchlist")
	}

	if len(taggedEntry.Tags) != 1 || taggedEntry.Tags[0].ID != attached.ID || taggedEntry.Tags[0].Name != "Reggaeton" {
		t.Fatalf("tagged entry tags = %+v, want [{%d Reggaeton}]", taggedEntry.Tags, attached.ID)
	}
	if taggedEntry.Note != nil {
		t.Fatalf("tagged entry note = %v, want nil", *taggedEntry.Note)
	}

	if untaggedEntry.Tags == nil {
		t.Fatal("untagged entry tags is null, want []")
	}
	if len(untaggedEntry.Tags) != 0 {
		t.Fatalf("untagged entry tags = %+v, want []", untaggedEntry.Tags)
	}
}
