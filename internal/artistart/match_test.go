package artistart

import (
	"context"
	"errors"
	"fmt"
	"slices"
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

// The tests below cover Task 2's shared-album-title tie-break (D-08
// amended, grilling round Q6).

func TestTitlesMatch(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{name: "exact equality", a: "un verano sin ti", b: "un verano sin ti", want: true},
		{name: "containment above length guard", a: "un verano sin ti (deluxe)", b: "un verano sin ti", want: true},
		{name: "containment below length guard", a: "a team", b: "a", want: false},
		{name: "no overlap", a: "un verano sin ti", b: "debi tirar mas fotos", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := titlesMatch(tt.a, tt.b)
			if got != tt.want {
				t.Fatalf("titlesMatch(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestMatch_TieBreakResolvesOnSharedAlbumTitle(t *testing.T) {
	searcher := stubSearcher{Artists: []deezer.Artist{
		{ID: 1, Name: "Bad Bunny"},
		{ID: 2, Name: "Bad Bunny"},
	}}
	albums := &stubAlbumLister{Albums: map[string][]deezer.Album{
		"1": {{Title: "Un Verano Sin Ti"}},
		"2": {{Title: "Something Else Entirely"}},
	}}
	groups := &stubGroupLister{Groups: []musicbrainz.ReleaseGroup{
		{Title: "Un Verano Sin Ti"},
	}}
	m := NewMatcher(searcher, albums, groups)

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !got.Matched {
		t.Fatalf("Matched = false, want true")
	}
	if got.DeezerID == nil || *got.DeezerID != "1" {
		t.Fatalf("DeezerID = %v, want \"1\" (the candidate sharing an album title)", got.DeezerID)
	}
}

func TestMatch_TieBreakNoSharedTitleFailsClosed(t *testing.T) {
	searcher := stubSearcher{Artists: []deezer.Artist{
		{ID: 1, Name: "Bad Bunny"},
		{ID: 2, Name: "Bad Bunny"},
	}}
	albums := &stubAlbumLister{Albums: map[string][]deezer.Album{
		"1": {{Title: "Something A"}},
		"2": {{Title: "Something B"}},
	}}
	groups := &stubGroupLister{Groups: []musicbrainz.ReleaseGroup{
		{Title: "Un Verano Sin Ti"},
	}}
	m := NewMatcher(searcher, albums, groups)

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if got.Matched {
		t.Fatalf("Matched = true, want false")
	}
}

func TestMatch_TieBreakBothShareTitleFailsClosed(t *testing.T) {
	searcher := stubSearcher{Artists: []deezer.Artist{
		{ID: 1, Name: "Bad Bunny"},
		{ID: 2, Name: "Bad Bunny"},
	}}
	albums := &stubAlbumLister{Albums: map[string][]deezer.Album{
		"1": {{Title: "Un Verano Sin Ti"}},
		"2": {{Title: "Un Verano Sin Ti"}},
	}}
	groups := &stubGroupLister{Groups: []musicbrainz.ReleaseGroup{
		{Title: "Un Verano Sin Ti"},
	}}
	m := NewMatcher(searcher, albums, groups)

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if got.Matched {
		t.Fatalf("Matched = true, want false (an ambiguous tie-break resolves to no photo)")
	}
	if got.DeezerID != nil || got.ImageURL != nil {
		t.Fatalf("DeezerID=%v ImageURL=%v, want both nil", got.DeezerID, got.ImageURL)
	}
}

func TestMatch_TieBreakTitleComparisonIsNormalized(t *testing.T) {
	searcher := stubSearcher{Artists: []deezer.Artist{
		{ID: 1, Name: "Bad Bunny"},
		{ID: 2, Name: "Bad Bunny"},
	}}
	albums := &stubAlbumLister{Albums: map[string][]deezer.Album{
		"1": {{Title: "un verano sin ti"}},
		"2": {{Title: "Something Else"}},
	}}
	groups := &stubGroupLister{Groups: []musicbrainz.ReleaseGroup{
		{Title: "Un Verano Sin Ti"},
	}}
	m := NewMatcher(searcher, albums, groups)

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !got.Matched {
		t.Fatalf("Matched = false, want true (title comparison is normalized like names)")
	}
}

func TestMatch_TieBreakDeluxeEditionSuffixResolves(t *testing.T) {
	searcher := stubSearcher{Artists: []deezer.Artist{
		{ID: 1, Name: "Bad Bunny"},
		{ID: 2, Name: "Bad Bunny"},
	}}
	albums := &stubAlbumLister{Albums: map[string][]deezer.Album{
		"1": {{Title: "Un Verano Sin Ti (Deluxe)"}},
		"2": {{Title: "Something Else"}},
	}}
	groups := &stubGroupLister{Groups: []musicbrainz.ReleaseGroup{
		{Title: "Un Verano Sin Ti"},
	}}
	m := NewMatcher(searcher, albums, groups)

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !got.Matched {
		t.Fatalf("Matched = false, want true (containment resolves an edition-suffix mismatch)")
	}
	if got.DeezerID == nil || *got.DeezerID != "1" {
		t.Fatalf("DeezerID = %v, want \"1\"", got.DeezerID)
	}
}

func TestMatch_TieBreakShortSubstringDoesNotResolve(t *testing.T) {
	searcher := stubSearcher{Artists: []deezer.Artist{
		{ID: 1, Name: "Bad Bunny"},
		{ID: 2, Name: "Bad Bunny"},
	}}
	albums := &stubAlbumLister{Albums: map[string][]deezer.Album{
		"1": {{Title: "A"}},
		"2": {{Title: "Something Else"}},
	}}
	groups := &stubGroupLister{Groups: []musicbrainz.ReleaseGroup{
		{Title: "A Team"},
	}}
	m := NewMatcher(searcher, albums, groups)

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if got.Matched {
		t.Fatalf("Matched = true, want false (containment below minTieBreakTitleLength must not resolve)")
	}
}

func TestMatch_SingleCandidateIssuesNoTieBreakFetch(t *testing.T) {
	searcher := stubSearcher{Artists: []deezer.Artist{
		{ID: 1, Name: "Bad Bunny", Picture: "https://example.test/bb.jpg"},
	}}
	albums := &stubAlbumLister{}
	groups := &stubGroupLister{}
	m := NewMatcher(searcher, albums, groups)

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !got.Matched {
		t.Fatalf("Matched = false, want true")
	}
	if albums.Called {
		t.Fatalf("ArtistAlbums was called for a single close-name candidate, want no tie-break fetch")
	}
	if groups.Called {
		t.Fatalf("ReleaseGroupsByArtist was called for a single close-name candidate, want no tie-break fetch")
	}
}

func TestMatch_TieBreakReleaseGroupsErrorSurfaces(t *testing.T) {
	wantErr := errors.New("mb boom")
	searcher := stubSearcher{Artists: []deezer.Artist{
		{ID: 1, Name: "Bad Bunny"},
		{ID: 2, Name: "Bad Bunny"},
	}}
	groups := &stubGroupLister{Err: wantErr}
	m := NewMatcher(searcher, &stubAlbumLister{}, groups)

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want errors.Is(err, wantErr)", err)
	}
	if got.Matched {
		t.Fatalf("Matched = true, want false")
	}
}

func TestMatch_TieBreakArtistAlbumsErrorSurfaces(t *testing.T) {
	wantErr := errors.New("dz boom")
	searcher := stubSearcher{Artists: []deezer.Artist{
		{ID: 1, Name: "Bad Bunny"},
		{ID: 2, Name: "Bad Bunny"},
	}}
	albums := &stubAlbumLister{Err: wantErr}
	groups := &stubGroupLister{Groups: []musicbrainz.ReleaseGroup{{Title: "Un Verano Sin Ti"}}}
	m := NewMatcher(searcher, albums, groups)

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want errors.Is(err, wantErr)", err)
	}
	if got.Matched {
		t.Fatalf("Matched = true, want false")
	}
}

func TestMatch_TieBreakEmptyReleaseGroupSetFailsClosed(t *testing.T) {
	searcher := stubSearcher{Artists: []deezer.Artist{
		{ID: 1, Name: "Bad Bunny"},
		{ID: 2, Name: "Bad Bunny"},
	}}
	albums := &stubAlbumLister{Albums: map[string][]deezer.Album{
		"1": {{Title: "Un Verano Sin Ti"}},
	}}
	groups := &stubGroupLister{} // no release groups at all
	m := NewMatcher(searcher, albums, groups)

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if got.Matched {
		t.Fatalf("Matched = true, want false (nothing to break the tie with)")
	}
	if albums.Called {
		t.Fatalf("ArtistAlbums was called despite an empty release-group set, want no album fetch")
	}
}

func TestMatch_TieBreakExceedsMaxCandidatesFailsClosedWithNoFetches(t *testing.T) {
	artists := make([]deezer.Artist, 0, maxTieBreakCandidates+1)
	for i := 0; i < maxTieBreakCandidates+1; i++ {
		artists = append(artists, deezer.Artist{ID: int64(i + 1), Name: "Bad Bunny"})
	}
	searcher := stubSearcher{Artists: artists}
	albums := &stubAlbumLister{}
	groups := &stubGroupLister{}
	m := NewMatcher(searcher, albums, groups)

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if got.Matched {
		t.Fatalf("Matched = true, want false")
	}
	if albums.Called || groups.Called {
		t.Fatalf("a tie set exceeding maxTieBreakCandidates issued a fetch, want none")
	}
}

// The tests below cover the D-09r quick task: Tier 0 (curated
// MusicBrainz->Deezer url-rel) and Tier 1 (alias retry), plus the
// deezerArtistIDFromURL URL-parsing helper both tiers share.

// stubArtistDetailLookup is a canned ArtistDetailLookup: Detail is returned
// verbatim (ignoring the mbid argument), or Err is returned if non-nil.
type stubArtistDetailLookup struct {
	Detail musicbrainz.ArtistDetail
	Err    error
	Called bool
}

func (s *stubArtistDetailLookup) LookupArtist(_ context.Context, _ string) (musicbrainz.ArtistDetail, error) {
	s.Called = true
	if s.Err != nil {
		return musicbrainz.ArtistDetail{}, s.Err
	}
	return s.Detail, nil
}

// stubArtistFetcher is a canned ArtistFetcher: Artist is returned verbatim
// (ignoring the artistID argument), or Err is returned if non-nil.
type stubArtistFetcher struct {
	Artist deezer.Artist
	Err    error
	Called bool
}

func (s *stubArtistFetcher) ArtistByID(_ context.Context, _ string) (deezer.Artist, error) {
	s.Called = true
	if s.Err != nil {
		return deezer.Artist{}, s.Err
	}
	return s.Artist, nil
}

// stubSearcherByQuery routes SearchArtists by the exact (unnormalized) query
// string it receives, and records every query in call order -- unlike
// stubSearcher's fixed-result shape, this lets Tier 1's alias-retry tests
// assert both per-alias results/errors and the exact search order.
type stubSearcherByQuery struct {
	Results    map[string][]deezer.Artist
	ErrByQuery map[string]error
	Queries    []string
}

func (s *stubSearcherByQuery) SearchArtists(_ context.Context, query string, _ int) ([]deezer.Artist, error) {
	s.Queries = append(s.Queries, query)
	if err, ok := s.ErrByQuery[query]; ok {
		return nil, err
	}
	return s.Results[query], nil
}

func TestDeezerArtistIDFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "www deezer artist", url: "https://www.deezer.com/artist/6144", want: "6144"},
		{name: "bare deezer artist", url: "https://deezer.com/artist/6144", want: "6144"},
		{name: "locale-prefixed deezer artist", url: "https://www.deezer.com/en/artist/6144", want: "6144"},
		{name: "trailing slash", url: "https://www.deezer.com/artist/6144/", want: "6144"},
		{name: "apple music streaming relation", url: "https://music.apple.com/gb/artist/657515", want: ""},
		{name: "spotify", url: "https://open.spotify.com/artist/abc", want: ""},
		{name: "deezer album, not artist", url: "https://www.deezer.com/album/6144", want: ""},
		{name: "lookalike host", url: "https://deezer.com.evil.test/artist/6144", want: ""},
		{name: "non-numeric id", url: "https://www.deezer.com/artist/notanumber", want: ""},
		{name: "missing id", url: "https://www.deezer.com/artist/", want: ""},
		{name: "syntactically invalid url", url: "://not a url", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deezerArtistIDFromURL(tt.url)
			if got != tt.want {
				t.Fatalf("deezerArtistIDFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestMatch_Tier0_FreeStreamingRelationToDeezerResolvesWithoutNameSearch(t *testing.T) {
	links := &stubArtistDetailLookup{Detail: musicbrainz.ArtistDetail{
		Relations: []musicbrainz.ArtistRelation{
			{Type: "free streaming", URL: musicbrainz.ArtistRelationURL{Resource: "https://www.deezer.com/artist/6144"}},
		},
	}}
	fetcher := &stubArtistFetcher{Artist: deezer.Artist{ID: 6144, Picture: "https://example.test/pic.jpg"}}
	searcher := stubSearcherFunc(func(_ context.Context, _ string, _ int) ([]deezer.Artist, error) {
		t.Fatal("SearchArtists called, want Tier 0 to short-circuit before any name search")
		return nil, nil
	})
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{}, WithArtistLinks(links, fetcher))

	got, err := m.Match(context.Background(), "mbid-1", "some artist")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !got.Matched {
		t.Fatalf("Matched = false, want true")
	}
	if got.DeezerID == nil || *got.DeezerID != "6144" {
		t.Fatalf("DeezerID = %v, want \"6144\"", got.DeezerID)
	}
	if got.ImageURL == nil || *got.ImageURL != "https://example.test/pic.jpg" {
		t.Fatalf("ImageURL = %v, want the confirmed artist's Picture", got.ImageURL)
	}
}

func TestMatch_Tier0_StreamingRelationTypeAlsoResolves(t *testing.T) {
	links := &stubArtistDetailLookup{Detail: musicbrainz.ArtistDetail{
		Relations: []musicbrainz.ArtistRelation{
			{Type: "streaming", URL: musicbrainz.ArtistRelationURL{Resource: "https://www.deezer.com/artist/6144"}},
		},
	}}
	fetcher := &stubArtistFetcher{Artist: deezer.Artist{ID: 6144, Picture: "https://example.test/pic.jpg"}}
	searcher := stubSearcherFunc(func(_ context.Context, _ string, _ int) ([]deezer.Artist, error) {
		t.Fatal("SearchArtists called, want Tier 0 to short-circuit before any name search")
		return nil, nil
	})
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{}, WithArtistLinks(links, fetcher))

	got, err := m.Match(context.Background(), "mbid-1", "some artist")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !got.Matched || got.DeezerID == nil || *got.DeezerID != "6144" {
		t.Fatalf("got = %+v, want a matched result with DeezerID 6144", got)
	}
}

func TestMatch_Tier0_StreamingToAppleMusicOnlyFallsThroughToNameSearch(t *testing.T) {
	links := &stubArtistDetailLookup{Detail: musicbrainz.ArtistDetail{
		Relations: []musicbrainz.ArtistRelation{
			{Type: "streaming", URL: musicbrainz.ArtistRelationURL{Resource: "https://music.apple.com/gb/artist/657515"}},
		},
	}}
	fetcher := &stubArtistFetcher{}
	searcher := stubSearcher{Artists: []deezer.Artist{{ID: 123, Name: "Bad Bunny", Picture: "https://example.test/bb.jpg"}}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{}, WithArtistLinks(links, fetcher))

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !got.Matched {
		t.Fatalf("Matched = false, want true (name path should have decided)")
	}
	if fetcher.Called {
		t.Fatalf("ArtistByID was called, want Tier 0 to decline (non-Deezer host)")
	}
}

func TestMatch_Tier0_TwoDifferentDeezerIDsIsAmbiguousFallsThroughToNameSearch(t *testing.T) {
	links := &stubArtistDetailLookup{Detail: musicbrainz.ArtistDetail{
		Relations: []musicbrainz.ArtistRelation{
			{Type: "free streaming", URL: musicbrainz.ArtistRelationURL{Resource: "https://www.deezer.com/artist/1111"}},
			{Type: "streaming", URL: musicbrainz.ArtistRelationURL{Resource: "https://www.deezer.com/artist/2222"}},
		},
	}}
	fetcher := &stubArtistFetcher{}
	searcher := stubSearcher{Artists: []deezer.Artist{{ID: 123, Name: "Bad Bunny"}}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{}, WithArtistLinks(links, fetcher))

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !got.Matched {
		t.Fatalf("Matched = false, want true (name path decides after Tier 0 declines on conflicting relations)")
	}
	if fetcher.Called {
		t.Fatalf("ArtistByID was called, want Tier 0 to decline on conflicting relations")
	}
}

func TestMatch_Tier0_TwoRelationsSameDeezerIDStillResolves(t *testing.T) {
	links := &stubArtistDetailLookup{Detail: musicbrainz.ArtistDetail{
		Relations: []musicbrainz.ArtistRelation{
			{Type: "free streaming", URL: musicbrainz.ArtistRelationURL{Resource: "https://www.deezer.com/artist/6144"}},
			{Type: "streaming", URL: musicbrainz.ArtistRelationURL{Resource: "https://www.deezer.com/artist/6144"}},
		},
	}}
	fetcher := &stubArtistFetcher{Artist: deezer.Artist{ID: 6144, Picture: "https://example.test/pic.jpg"}}
	searcher := stubSearcherFunc(func(_ context.Context, _ string, _ int) ([]deezer.Artist, error) {
		t.Fatal("SearchArtists called, want Tier 0 to resolve")
		return nil, nil
	})
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{}, WithArtistLinks(links, fetcher))

	got, err := m.Match(context.Background(), "mbid-1", "some artist")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !got.Matched || got.DeezerID == nil || *got.DeezerID != "6144" {
		t.Fatalf("got = %+v, want a matched result with DeezerID 6144 (two relations naming the same id still resolve)", got)
	}
}

func TestMatch_Tier0_LookupArtistErrorDoesNotFailMatch(t *testing.T) {
	links := &stubArtistDetailLookup{Err: errors.New("mb down")}
	fetcher := &stubArtistFetcher{}
	searcher := stubSearcher{Artists: []deezer.Artist{{ID: 123, Name: "Bad Bunny"}}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{}, WithArtistLinks(links, fetcher))

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v, want a nil error (a MusicBrainz outage must degrade, not fail)", err)
	}
	if !got.Matched {
		t.Fatalf("Matched = false, want true (name path still decides)")
	}
}

func TestMatch_Tier0_ConfirmingFetchErrorDoesNotFailMatch(t *testing.T) {
	links := &stubArtistDetailLookup{Detail: musicbrainz.ArtistDetail{
		Relations: []musicbrainz.ArtistRelation{
			{Type: "free streaming", URL: musicbrainz.ArtistRelationURL{Resource: "https://www.deezer.com/artist/6144"}},
		},
	}}
	fetcher := &stubArtistFetcher{Err: errors.New("deezer down")}
	searcher := stubSearcher{Artists: []deezer.Artist{{ID: 123, Name: "Bad Bunny"}}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{}, WithArtistLinks(links, fetcher))

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v, want a nil error (a Deezer confirm failure -- including the *APIError dead-id case -- must degrade, not fail)", err)
	}
	if !got.Matched {
		t.Fatalf("Matched = false, want true (name path still decides)")
	}
}

func TestMatch_Tier0_ConfirmingFetchZeroIDTreatedUnconfirmed(t *testing.T) {
	links := &stubArtistDetailLookup{Detail: musicbrainz.ArtistDetail{
		Relations: []musicbrainz.ArtistRelation{
			{Type: "free streaming", URL: musicbrainz.ArtistRelationURL{Resource: "https://www.deezer.com/artist/6144"}},
		},
	}}
	fetcher := &stubArtistFetcher{Artist: deezer.Artist{}} // zero ID: unconfirmed
	searcher := stubSearcher{Artists: []deezer.Artist{{ID: 123, Name: "Bad Bunny"}}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{}, WithArtistLinks(links, fetcher))

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !got.Matched {
		t.Fatalf("Matched = false, want true (name path decides when the confirm fetch returns a zero ID)")
	}
	if got.DeezerID == nil || *got.DeezerID != "123" {
		t.Fatalf("DeezerID = %v, want \"123\" (from the name path, not Tier 0)", got.DeezerID)
	}
}

func TestMatch_Tier0_NilSeamsBehavesLikeTodayWithThreeArgNewMatcher(t *testing.T) {
	searcher := stubSearcher{Artists: []deezer.Artist{{ID: 123, Name: "Bad Bunny", Picture: "https://example.test/bb.jpg"}}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{})

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !got.Matched || got.DeezerID == nil || *got.DeezerID != "123" {
		t.Fatalf("got = %+v, want the unchanged three-arg NewMatcher behavior (both new tiers no-op)", got)
	}
}

func TestMatch_Tier1_AliasResolvesWhenPrimaryFindsZeroCandidates(t *testing.T) {
	links := &stubArtistDetailLookup{Detail: musicbrainz.ArtistDetail{
		Aliases: []musicbrainz.ArtistAlias{
			{Name: "Radio Head", Type: "Search hint"},
		},
	}}
	searcher := &stubSearcherByQuery{Results: map[string][]deezer.Artist{
		"Radio Head": {{ID: 777, Name: "Radio Head", Picture: "https://example.test/rh.jpg"}},
	}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{}, WithArtistLinks(links, &stubArtistFetcher{}))

	got, err := m.Match(context.Background(), "mbid-1", "Radiohead")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !got.Matched {
		t.Fatalf("Matched = false, want true (the alias should have resolved)")
	}
	if got.DeezerID == nil || *got.DeezerID != "777" {
		t.Fatalf("DeezerID = %v, want \"777\"", got.DeezerID)
	}
}

func TestMatch_Tier1_PrioritizedAliasTypesSearchedFirst(t *testing.T) {
	links := &stubArtistDetailLookup{Detail: musicbrainz.ArtistDetail{
		Aliases: []musicbrainz.ArtistAlias{
			{Name: "Zzz Other Alias", Type: "Artist name"},
			{Name: "Legal Name Alias", Type: "Legal name"},
			{Name: "Search Hint Alias", Type: "Search hint"},
		},
	}}
	searcher := &stubSearcherByQuery{Results: map[string][]deezer.Artist{}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{}, WithArtistLinks(links, &stubArtistFetcher{}))

	_, err := m.Match(context.Background(), "mbid-1", "primary name")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}

	// Queries[0] is the primary name search issued before any alias retry.
	// The two prioritized alias types (Legal name, Search hint) must be
	// searched -- in their original relative order -- before the
	// unprioritized "Artist name" alias.
	wantQueries := []string{"primary name", "Legal Name Alias", "Search Hint Alias", "Zzz Other Alias"}
	if !slices.Equal(searcher.Queries, wantQueries) {
		t.Fatalf("Queries = %v, want %v", searcher.Queries, wantQueries)
	}
}

func TestMatch_Tier1_DuplicateAndAlreadyTriedAliasesSkipped(t *testing.T) {
	links := &stubArtistDetailLookup{Detail: musicbrainz.ArtistDetail{
		Aliases: []musicbrainz.ArtistAlias{
			{Name: "Bad Bunny", Type: "Legal name"},      // normalizes the same as the already-tried primary name -- must be skipped
			{Name: "El Conejo Malo", Type: "Artist name"},
			{Name: "el conejo malo", Type: "Artist name"}, // same normalized form as the alias above -- searched only once
		},
	}}
	searcher := &stubSearcherByQuery{Results: map[string][]deezer.Artist{}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{}, WithArtistLinks(links, &stubArtistFetcher{}))

	_, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}

	wantQueries := []string{"bad bunny", "El Conejo Malo"}
	if !slices.Equal(searcher.Queries, wantQueries) {
		t.Fatalf("Queries = %v, want %v (no duplicate outbound search, and the already-tried primary name is skipped)", searcher.Queries, wantQueries)
	}
}

func TestMatch_Tier1_NoAliasResolvesFailsClosed(t *testing.T) {
	links := &stubArtistDetailLookup{Detail: musicbrainz.ArtistDetail{
		Aliases: []musicbrainz.ArtistAlias{
			{Name: "Nonexistent Alias", Type: "Search hint"},
		},
	}}
	searcher := &stubSearcherByQuery{Results: map[string][]deezer.Artist{}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{}, WithArtistLinks(links, &stubArtistFetcher{}))

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if got.Matched {
		t.Fatalf("Matched = true, want false (D-09 fail-closed, unchanged)")
	}
	if got.DeezerID != nil || got.ImageURL != nil {
		t.Fatalf("DeezerID=%v ImageURL=%v, want both nil", got.DeezerID, got.ImageURL)
	}
}

func TestMatch_Tier1_AliasSearchErrorAbsorbedContinuesToNextAlias(t *testing.T) {
	links := &stubArtistDetailLookup{Detail: musicbrainz.ArtistDetail{
		Aliases: []musicbrainz.ArtistAlias{
			{Name: "Errors Out", Type: "Search hint"},
			{Name: "Second Alias", Type: "Legal name"},
		},
	}}
	wantErr := errors.New("dz boom")
	searcher := &stubSearcherByQuery{
		Results: map[string][]deezer.Artist{
			"Second Alias": {{ID: 42, Name: "Second Alias"}},
		},
		ErrByQuery: map[string]error{
			"Errors Out": wantErr,
		},
	}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{}, WithArtistLinks(links, &stubArtistFetcher{}))

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v, want a nil error (an absorbed alias-attempt error must not surface)", err)
	}
	if !got.Matched {
		t.Fatalf("Matched = false, want true (the next alias should still be tried and resolve)")
	}
	if got.DeezerID == nil || *got.DeezerID != "42" {
		t.Fatalf("DeezerID = %v, want \"42\"", got.DeezerID)
	}
}

func TestMatch_Tier1_BoundedByMaxAliasAttempts(t *testing.T) {
	aliases := make([]musicbrainz.ArtistAlias, 0, maxAliasAttempts+3)
	for i := 0; i < maxAliasAttempts+3; i++ {
		aliases = append(aliases, musicbrainz.ArtistAlias{Name: fmt.Sprintf("Alias %d", i), Type: "Artist name"})
	}
	links := &stubArtistDetailLookup{Detail: musicbrainz.ArtistDetail{Aliases: aliases}}
	searcher := &stubSearcherByQuery{Results: map[string][]deezer.Artist{}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{}, WithArtistLinks(links, &stubArtistFetcher{}))

	_, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}

	// Queries[0] is the primary name search; the rest are alias attempts.
	aliasAttempts := len(searcher.Queries) - 1
	if aliasAttempts != maxAliasAttempts {
		t.Fatalf("alias search attempts = %d, want exactly maxAliasAttempts (%d)", aliasAttempts, maxAliasAttempts)
	}
}

func TestMatch_Tier1_AmbiguousTieDoesNotRetryAliases(t *testing.T) {
	links := &stubArtistDetailLookup{Detail: musicbrainz.ArtistDetail{
		Aliases: []musicbrainz.ArtistAlias{
			{Name: "Should Not Be Searched", Type: "Search hint"},
		},
	}}
	var queries []string
	searcher := stubSearcherFunc(func(_ context.Context, query string, _ int) ([]deezer.Artist, error) {
		queries = append(queries, query)
		return []deezer.Artist{
			{ID: 1, Name: "Bad Bunny"},
			{ID: 2, Name: "Bad Bunny"},
		}, nil
	})
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{}, WithArtistLinks(links, &stubArtistFetcher{}))

	got, err := m.Match(context.Background(), "mbid-1", "bad bunny")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if got.Matched {
		t.Fatalf("Matched = true, want false (an ambiguous tie is a considered D-09 stop, not a retry trigger)")
	}
	if len(queries) != 1 {
		t.Fatalf("queries = %v, want exactly 1 (no alias retry after an ambiguous tie -- DD-3)", queries)
	}
}
