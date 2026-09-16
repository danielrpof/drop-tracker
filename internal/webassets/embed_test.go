package webassets_test

// Black-box, exported surface only, so this file exercises the package the
// same way any caller does -- mirrors internal/httpserver's own convention.
// internal/httpserver/spa_test.go already covers the router wiring end to
// end, but every one of its requests falls back to index.html; nothing
// today requests a real embedded asset path, so the handler's static-file
// branch has never run under test. That is this file's D-09 priority
// target.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/webassets"
)

// firstJSAsset returns the request path (e.g. "/assets/root-abc123.js") of
// the first JavaScript file found in the embedded build's asset directory,
// read from disk relative to this test file's own package directory. It
// fails the test with a clear message if the directory is empty or
// missing, rather than silently skipping the static-file branch this test
// exists to cover.
func firstJSAsset(t *testing.T) string {
	t.Helper()

	const assetsDir = "build/client/assets"
	entries, err := os.ReadDir(assetsDir)
	if err != nil {
		t.Fatalf("read %s: %v (the embedded SPA build must be present -- run `make web`)", assetsDir, err)
	}
	if len(entries) == 0 {
		t.Fatalf("%s is empty -- the embedded SPA build must contain at least one asset", assetsDir)
	}

	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".js") {
			return "/" + filepath.ToSlash(filepath.Join("assets", e.Name()))
		}
	}
	t.Fatalf("no .js file found under %s", assetsDir)
	return ""
}

// TestHandler_ServesRealEmbeddedAssetDirectly exercises the branch nothing
// else covers: a request for a path that exists in the embedded filesystem
// must be served directly by the file server, not by the index.html
// fallback. Content-Type is what actually distinguishes this branch from
// the fallback -- a status-only assertion would pass even if the fallback
// had served the entry document instead.
func TestHandler_ServesRealEmbeddedAssetDirectly(t *testing.T) {
	assetPath := firstJSAsset(t)

	ts := httptest.NewServer(webassets.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + assetPath)
	if err != nil {
		t.Fatalf("GET %s: %v", assetPath, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body := make([]byte, 1)
	n, _ := resp.Body.Read(body)
	if n == 0 {
		t.Fatal("response body is empty, want a non-empty asset body")
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "javascript") {
		t.Fatalf("Content-Type = %q, want it to indicate javascript (this is what proves the static-file branch ran, not the SPA fallback)", ct)
	}
}

// TestHandler_UnknownPathFallsBackToIndexHTML proves the fallback branch: a
// path that cannot exist in the embedded tree must still resolve to a 200
// with the entry document, since that is the behavior the SPA router
// depends on.
func TestHandler_UnknownPathFallsBackToIndexHTML(t *testing.T) {
	ts := httptest.NewServer(webassets.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/this-path-cannot-possibly-exist-in-the-build")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want it to contain %q", ct, "text/html")
	}
}

// TestHandler_RootPathServesIndexHTML proves the root path resolves to the
// entry document.
func TestHandler_RootPathServesIndexHTML(t *testing.T) {
	ts := httptest.NewServer(webassets.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want it to contain %q", ct, "text/html")
	}
}

// TestHandler_MissingAssetPathFallsBackInsteadOf404 proves a path that
// looks like it belongs under the asset directory, but does not actually
// exist, also falls back to the entry document rather than returning a
// 404 -- the SPA router depends on this behavior for any client-side route
// nested under a path that resembles an asset directory.
func TestHandler_MissingAssetPathFallsBackInsteadOf404(t *testing.T) {
	ts := httptest.NewServer(webassets.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/assets/this-file-does-not-exist.js")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (a missing asset path must fall back to index.html, not 404)", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want it to contain %q", ct, "text/html")
	}
}
