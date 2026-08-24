package artistart

import (
	"context"
	"errors"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/deezer"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
)

// stubSearcher is a canned ArtistSearcher: Artists is returned verbatim
// (ignoring the query/limit arguments), or Err is returned if non-nil.
type stubSearcher struct {
	Artists []deezer.Artist
	Err     error
}

func (s stubSearcher) SearchArtists(_ context.Context, _ string, _ int) ([]deezer.Artist, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	return s.Artists, nil
}

// stubAlbumLister's zero value returns an empty slice for any artist id,
// satisfying AlbumLister with no configuration needed.
type stubAlbumLister struct {
	Albums map[string][]deezer.Album
	Err    error
	Called bool
}

func (s *stubAlbumLister) ArtistAlbums(_ context.Context, artistID string, _ int) ([]deezer.Album, error) {
	s.Called = true
	if s.Err != nil {
		return nil, s.Err
	}
	return s.Albums[artistID], nil
}

// stubGroupLister's zero value returns an empty slice, satisfying
// ReleaseGroupLister with no configuration needed.
type stubGroupLister struct {
	Groups []musicbrainz.ReleaseGroup
	Err    error
	Called bool
}

func (s *stubGroupLister) ReleaseGroupsByArtist(_ context.Context, _ string) ([]musicbrainz.ReleaseGroup, error) {
	s.Called = true
	if s.Err != nil {
		return nil, s.Err
	}
	return s.Groups, nil
}

func TestNormalizeArtistName(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{name: "whitespace and case", a: " Bad  Bunny ", b: "bad bunny", want: true},
		{name: "typographic apostrophe", a: "D’Angelo", b: "D'Angelo", want: true},
		{name: "acute accent", a: "Rosalía", b: "Rosalia", want: true},
		{name: "tilde", a: "Ñengo Flow", b: "Nengo Flow", want: true},
		{name: "not a substring match", a: "Drake", b: "Drake Bell", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeArtistName(tt.a) == normalizeArtistName(tt.b)
			if got != tt.want {
				t.Fatalf("normalizeArtistName(%q) == normalizeArtistName(%q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestMatch_SingleCloseNameCandidate(t *testing.T) {
	searcher := stubSearcher{Artists: []deezer.Artist{
		{ID: 123, Name: "Bad Bunny", Picture: "https://example.test/bb.jpg"},
	}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{})

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !got.Matched {
		t.Fatalf("Matched = false, want true")
	}
	if got.DeezerID == nil || *got.DeezerID != "123" {
		t.Fatalf("DeezerID = %v, want \"123\"", got.DeezerID)
	}
	if got.ImageURL == nil || *got.ImageURL != "https://example.test/bb.jpg" {
		t.Fatalf("ImageURL = %v, want the candidate's Picture", got.ImageURL)
	}
}

func TestMatch_NoCloseNameCandidateFailsClosed(t *testing.T) {
	searcher := stubSearcher{Artists: []deezer.Artist{
		{ID: 999, Name: "Someone Else"},
	}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{})

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if got.Matched {
		t.Fatalf("Matched = true, want false")
	}
	if got.DeezerID != nil || got.ImageURL != nil {
		t.Fatalf("DeezerID=%v ImageURL=%v, want both nil", got.DeezerID, got.ImageURL)
	}
}

func TestMatch_ZeroResultsFailsClosed(t *testing.T) {
	searcher := stubSearcher{}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{})

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if got.Matched {
		t.Fatalf("Matched = true, want false")
	}
}

func TestMatch_TwoSameNamedCandidatesWithNoTieBreakDataFailsClosed(t *testing.T) {
	searcher := stubSearcher{Artists: []deezer.Artist{
		{ID: 1, Name: "Bad Bunny"},
		{ID: 2, Name: "Bad Bunny"},
	}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{})

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if got.Matched {
		t.Fatalf("Matched = true, want false")
	}
	if got.DeezerID != nil || got.ImageURL != nil {
		t.Fatalf("DeezerID=%v ImageURL=%v, want both nil", got.DeezerID, got.ImageURL)
	}
}

func TestMatch_MatchedCandidateWithEmptyPictureYieldsNilImageURL(t *testing.T) {
	searcher := stubSearcher{Artists: []deezer.Artist{
		{ID: 456, Name: "Bad Bunny", Picture: ""},
	}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{})

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !got.Matched {
		t.Fatalf("Matched = false, want true")
	}
	if got.DeezerID == nil {
		t.Fatalf("DeezerID = nil, want non-nil")
	}
	if got.ImageURL != nil {
		t.Fatalf("ImageURL = %v, want nil (empty Picture)", got.ImageURL)
	}
}

func TestMatch_SearchErrorSurfaces(t *testing.T) {
	wantErr := errors.New("boom")
	searcher := stubSearcher{Err: wantErr}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{})

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want errors.Is(err, wantErr)", err)
	}
	if got.Matched || got.DeezerID != nil || got.ImageURL != nil {
		t.Fatalf("got = %+v, want the zero Result alongside the error", got)
	}
}

func TestMatch_EmptyNameFailsClosedWithoutOutboundCall(t *testing.T) {
	called := false
	searcher := stubSearcherFunc(func(_ context.Context, _ string, _ int) ([]deezer.Artist, error) {
		called = true
		return nil, nil
	})
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{})

	got, err := m.Match(context.Background(), "mbid-1", "   ")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if got.Matched {
		t.Fatalf("Matched = true, want false")
	}
	if called {
		t.Fatalf("SearchArtists was called for an empty/whitespace-only name, want no outbound call")
	}
}

// stubSearcherFunc adapts a plain func into ArtistSearcher, used only by the
// empty-name test above to assert no outbound call happens.
type stubSearcherFunc func(ctx context.Context, query string, limit int) ([]deezer.Artist, error)

func (f stubSearcherFunc) SearchArtists(ctx context.Context, query string, limit int) ([]deezer.Artist, error) {
	return f(ctx, query, limit)
}
