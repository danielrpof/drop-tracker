// Package webassets embeds the built React SPA (web/build/client, copied
// here by the Makefile's `web` target) and serves it as the chi router's
// NotFound fallback (06-RESEARCH.md Pattern 3). The embed directive cannot
// reference a parent directory, so the built output has to live under this
// package's own directory tree -- see build/client/.
package webassets

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// webFS embeds the SPA's built client bundle. all: (not plain build/client)
// so any Vite-emitted asset directory beginning with "." or "_" is included
// too -- a defensive default even though typical Vite output (assets/)
// doesn't need it.
//
//go:embed all:build/client
var webFS embed.FS

// Handler serves the embedded SPA build: a request matching an actual
// static file (e.g. /assets/index-abc123.js) is served directly from the
// embedded filesystem; any other path is served index.html instead, so
// React Router's client-side router can resolve the route in the browser
// (06-RESEARCH.md Pattern 1/3). Files are served exclusively from fs.Sub of
// an embed.FS -- an embedded FS has no parent directory to traverse into,
// and the OS filesystem is never consulted (T-06-03).
func Handler() http.Handler {
	sub, err := fs.Sub(webFS, "build/client")
	if err != nil {
		// The embed directive above guarantees build/client exists at
		// compile time -- fs.Sub can only fail here if the embedded tree is
		// malformed, which would already have failed the build.
		panic("webassets: build/client not found in embedded FS: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if f, err := sub.Open(path); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// No matching static file: fall back to index.html at the root path
		// so React Router's client-side router resolves the requested route.
		// r.Clone deep-copies the URL (among other fields), so mutating the
		// clone's Path never affects the original request.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}
