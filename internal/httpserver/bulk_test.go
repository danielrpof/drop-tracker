package httpserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

var _ httpserver.WatchlistBulkStore = (*watchlist.Service)(nil)

// bulkDeleteWire is the JSON shape of a bulk-delete outcome.
type bulkDeleteWire struct {
	Removed     []int64 `json:"removed"`
	AlreadyGone []int64 `json:"already_gone"`
}

// Tracer: three selected entries deleted over HTTP in one statement against
// a real Postgres, including an id that is already gone.
func TestBulk_DeleteEndToEnd(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "bulk_http_test")
	ctx := t.Context()

	svc := watchlist.NewService(sqlc.New(pool))
	srv := httpserver.New(pool, svc, stubEventsStore{}, nil, discardLogger(), httpserver.WithWatchlistBulk(svc))
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	var ids []int64
	for _, suffix := range []string{"-a", "-b", "-c"} {
		e, err := svc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + suffix, Name: "Bulk Artist" + suffix})
		if err != nil {
			t.Fatalf("seed entry %s: %v", suffix, err)
		}
		ids = append(ids, e.ID)
	}
	a, b, c := ids[0], ids[1], ids[2]

	body, err := json.Marshal(map[string]any{"ids": []int64{a, b, 999999999}})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp, err := http.Post(ts.URL+"/watchlist/bulk/delete", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("POST /watchlist/bulk/delete: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got bulkDeleteWire
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if want := []int64{a, b}; !slices.Equal(got.Removed, want) {
		t.Fatalf("removed = %v, want %v", got.Removed, want)
	}
	if want := []int64{999999999}; !slices.Equal(got.AlreadyGone, want) {
		t.Fatalf("already_gone = %v, want %v", got.AlreadyGone, want)
	}

	listResp, err := http.Get(ts.URL + "/watchlist")
	if err != nil {
		t.Fatalf("GET /watchlist: %v", err)
	}
	defer func() { _ = listResp.Body.Close() }()
	var entries []struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != c {
		t.Fatalf("remaining entries = %+v, want only id %d", entries, c)
	}
}
