package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/go-chi/httplog/v3"

	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
)

// maxSearchQueryRunes caps the q query param the same way watchlist.go's
// maxNameRunes caps name -- rune-counted, not byte-counted, and rejected
// before any outbound request is built (T-03-02).
const maxSearchQueryRunes = 512

// searchResultLimit is the fixed per-source result cap for GET /search.
// This is a live, search-as-you-type endpoint, not a paged browse view, so
// a small fixed cap avoids over-fetching on every keystroke without adding
// pagination plumbing this phase doesn't need (03-RESEARCH.md Open
// Question 1).
const searchResultLimit = 10

// SearchArtist is the wire shape for one artist result in the GET /search
// response, common across every source -- source-tagged so a caller can
// tell which upstream produced it (D-02: no cross-source dedup/merge).
type SearchArtist struct {
	Source         string  `json:"source"`
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Disambiguation *string `json:"disambiguation"`
	Type           string  `json:"type"`
	ImageURL       *string `json:"image_url"`
}

// SearchSource is the consumer-side seam handleSearch depends on -- defined
// here, in the consumer, exactly as Pinger and watchlist.Store are, so a
// test can substitute a stub with no real external HTTP client (D-01).
type SearchSource interface {
	Name() string
	SearchArtists(ctx context.Context, q string, limit int) ([]SearchArtist, error)
}

// sourceResult is one source's entry in searchResponse.Sources: status is
// "ok" or "error"; error is the fixed operator-authored string
// "source unavailable", never raw upstream text (D-03, T-03-01).
type sourceResult struct {
	Status  string         `json:"status"`
	Error   string         `json:"error,omitempty"`
	Artists []SearchArtist `json:"artists"`
}

// searchResponse is the GET /search response envelope (D-01, D-02, D-03).
// Sources is a map keyed by source name so a second source (plan 03-02's
// Deezer) is added by adding a key, never by reshaping the response.
type searchResponse struct {
	Query   string                  `json:"query"`
	Sources map[string]sourceResult `json:"sources"`
}

// musicBrainzSource adapts musicbrainz.ArtistSearcher into SearchSource.
// Defined here, not in internal/musicbrainz, so that package never needs to
// import internal/httpserver -- the dependency direction stays one-way.
type musicBrainzSource struct {
	client musicbrainz.ArtistSearcher
}

// NewMusicBrainzSource wraps c as a SearchSource named "musicbrainz".
func NewMusicBrainzSource(c musicbrainz.ArtistSearcher) SearchSource {
	return musicBrainzSource{client: c}
}

func (s musicBrainzSource) Name() string { return "musicbrainz" }

func (s musicBrainzSource) SearchArtists(ctx context.Context, q string, limit int) ([]SearchArtist, error) {
	artists, err := s.client.SearchArtists(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]SearchArtist, 0, len(artists))
	for _, a := range artists {
		var disambiguation *string
		if a.Disambiguation != "" {
			d := a.Disambiguation
			disambiguation = &d
		}
		out = append(out, SearchArtist{
			Source:         "musicbrainz",
			ID:             a.MBID,
			Name:           a.Name,
			Disambiguation: disambiguation,
			Type:           a.Type,
			ImageURL:       nil,
		})
	}
	return out, nil
}

var _ SearchSource = musicBrainzSource{}

// handleSearch implements GET /search (WLST-01, CLNT-03, D-01/D-02/D-03):
// fans out to every configured source concurrently, using r.Context() so an
// inbound cancellation cancels every in-flight upstream call and limiter
// wait, and joins before writing any response -- WriteHeader only happens
// after the WaitGroup completes, so a cancelled request never emits a
// partial body. A source failing never fails the whole request (D-03): raw
// upstream error text never reaches the response body (T-03-01, V13) --
// it is logged via httplog.SetAttrs instead.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}
	if utf8.RuneCountInString(q) > maxSearchQueryRunes {
		writeError(w, http.StatusBadRequest, "q is too long")
		return
	}

	sources := make(map[string]sourceResult, len(s.sources))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, src := range s.sources {
		wg.Add(1)
		go func(src SearchSource) {
			defer wg.Done()

			artists, err := src.SearchArtists(r.Context(), q, searchResultLimit)

			var result sourceResult
			if err != nil {
				httplog.SetAttrs(r.Context(), slog.String(src.Name()+"_search_error", err.Error()))
				result = sourceResult{Status: "error", Error: "source unavailable", Artists: []SearchArtist{}}
			} else {
				if artists == nil {
					artists = []SearchArtist{}
				}
				result = sourceResult{Status: "ok", Artists: artists}
			}

			mu.Lock()
			sources[src.Name()] = result
			mu.Unlock()
		}(src)
	}
	wg.Wait()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(searchResponse{Query: q, Sources: sources})
}
