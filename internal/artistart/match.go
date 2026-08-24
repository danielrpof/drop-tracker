// Package artistart owns D-08's artist-art match rule and D-09's
// fail-closed policy in exactly one place. Every artists.image_url row in
// this database is NULL and always has been: MusicBrainz has no artist
// images, and adds only ever flow from a MusicBrainz search result. Deezer
// has artist pictures but no MusicBrainz id, and this project has
// deliberately never had cross-source identity resolution -- this package
// is the one place that resolution happens, so bug #3's two call sites (the
// add-time match and the backfill sweep, sibling plan 13-03) can never
// drift apart by each carrying their own copy of the rule.
package artistart

import (
	"context"
	"strconv"
	"strings"

	"github.com/danielrpof/drop-tracker/internal/deezer"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
)

// searchLimit bounds the Deezer artist search this package issues, matching
// the existing searchResultLimit precedent from Phase 6
// (internal/httpserver/search.go).
const searchLimit = 10

// Result is the outcome of a Match call. DeezerID and ImageURL are *string
// specifically so they drop into sqlc.UpsertArtistParams unchanged, where
// the existing COALESCE clauses read nil as "the caller said nothing about
// this field" rather than "blank it". Matched: false is D-09's fail-closed
// outcome, and both pointers are always nil in that case.
type Result struct {
	DeezerID *string
	ImageURL *string
	Matched  bool
}

// ArtistSearcher is the narrow seam this package depends on for Deezer
// artist search -- declared here, in the consumer, mirroring
// detection.RecordingSource/ReleaseDetailSource (internal/detection/detector.go
// lines 21-39), so tests substitute a stub and no concrete client type
// appears in this package's own constructor signature.
type ArtistSearcher interface {
	SearchArtists(ctx context.Context, query string, limit int) ([]deezer.Artist, error)
}

// AlbumLister is the narrow seam this package depends on for Deezer album
// listing, used only by Task 2's tie-break.
type AlbumLister interface {
	ArtistAlbums(ctx context.Context, artistID string, limit int) ([]deezer.Album, error)
}

// ReleaseGroupLister is the narrow seam this package depends on for
// MusicBrainz release-group browsing, used only by Task 2's tie-break.
type ReleaseGroupLister interface {
	ReleaseGroupsByArtist(ctx context.Context, mbid string) ([]musicbrainz.ReleaseGroup, error)
}

// Matcher implements D-08's match rule against the three narrow seams
// above. Consumers always depend on ArtistSearcher/AlbumLister/
// ReleaseGroupLister -- never on *deezer.Client or *musicbrainz.Client
// directly.
type Matcher struct {
	search ArtistSearcher
	albums AlbumLister
	groups ReleaseGroupLister
}

// NewMatcher builds a Matcher backed by search, albums and groups.
func NewMatcher(search ArtistSearcher, albums AlbumLister, groups ReleaseGroupLister) *Matcher {
	return &Matcher{search: search, albums: albums, groups: groups}
}

// diacriticFolds maps common Latin-1 Supplement diacritic runes (and their
// uppercase forms) to their ASCII base letter. This is a best-effort
// ASCII fold for the common Western-Latin case, not a full Unicode
// normalizer -- a rune with no entry here passes through unchanged, and a
// future contributor must not assume it handles non-Latin scripts.
var diacriticFolds = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a',
	'À': 'A', 'Á': 'A', 'Â': 'A', 'Ã': 'A', 'Ä': 'A', 'Å': 'A',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
	'È': 'E', 'É': 'E', 'Ê': 'E', 'Ë': 'E',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i',
	'Ì': 'I', 'Í': 'I', 'Î': 'I', 'Ï': 'I',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'Ò': 'O', 'Ó': 'O', 'Ô': 'O', 'Õ': 'O', 'Ö': 'O',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u',
	'Ù': 'U', 'Ú': 'U', 'Û': 'U', 'Ü': 'U',
	'ñ': 'n', 'Ñ': 'N',
	'ç': 'c', 'Ç': 'C',
	'ý': 'y', 'ÿ': 'y', 'Ý': 'Y',
}

// foldDiacritics replaces every rune present in diacriticFolds with its
// ASCII base letter, rune-by-rune. It works correctly whether called before
// or after lowercasing, since diacriticFolds carries both cases -- called
// before strings.ToLower in normalizeArtistName. Hand-rolled rather than
// pulling in the x/text Unicode normalization module (13-RESEARCH.md's
// Package Legitimacy Audit claims zero new Go modules for this plan, and
// this keeps that claim true).
func foldDiacritics(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if folded, ok := diacriticFolds[r]; ok {
			b.WriteRune(folded)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// normalizeArtistName normalizes both artist names and title strings (this
// function serves both -- titlesMatch, added by Task 2, reuses it for
// album/release-group titles, since the folding rules wanted for titles --
// case, whitespace, typographic apostrophe, diacritics -- are the same ones
// wanted for names): trim, fold the U+2019 right single quotation mark to
// an ASCII apostrophe, fold diacritics to their ASCII base letter, lowercase,
// and collapse every run of whitespace to a single space.
//
// Equality on this normalized form IS D-08's "close name equality" --
// deliberately strict rather than fuzzy, because D-09 requires failing
// closed and a fuzzy threshold is exactly how a wrong-artist photo gets
// attached. "Strict" and "unaccented" are different axes, though: folding a
// diacritic to its base letter is not fuzzy matching -- "Rosalía" and
// "Rosalia" are the same artist rendered two ways by two different upstream
// catalogues, not two different artists that happen to look similar, so
// folding them together doesn't weaken D-09's fail-closed guarantee
// (grilling round Q3).
func normalizeArtistName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "’", "'")
	s = foldDiacritics(s)
	s = strings.ToLower(s)
	return strings.Join(strings.Fields(s), " ")
}

// nilIfEmpty returns nil for an empty string and a pointer to the value
// otherwise, mirroring internal/httpserver/search.go's package-local helper
// of the same shape.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Match implements D-08's decision order -- normalized-name equality first,
// shared album title (equality-or-containment via titlesMatch) only to
// break a tie, nothing else ever -- and D-09's fail-closed policy on every
// ambiguous, empty, or unresolvable outcome. Popularity ordering from
// SearchArtists (Deezer's own NbFan-descending sort) is deliberately never
// consulted at any step.
func (m *Matcher) Match(ctx context.Context, mbid, name string) (Result, error) {
	normalizedName := normalizeArtistName(name)
	if normalizedName == "" {
		return Result{}, nil
	}

	candidates, err := m.search.SearchArtists(ctx, name, searchLimit)
	if err != nil {
		return Result{}, err
	}

	var tied []deezer.Artist
	for _, c := range candidates {
		if normalizeArtistName(c.Name) == normalizedName {
			tied = append(tied, c)
		}
	}

	switch len(tied) {
	case 0:
		return Result{}, nil
	case 1:
		return resultFor(tied[0]), nil
	default:
		// Two or more same-normalized-name candidates: never pick the first
		// (SearchArtists already sorts by fan count descending, so "take the
		// first" would silently become a popularity-decides-identity rule
		// D-08/D-09 both forbid) and never pick the highest NbFan for the
		// same reason. Task 2 replaces this branch with D-08's
		// shared-album-title tie-break; until then this falls closed (D-09).
		return m.tieBreak(ctx, mbid, tied)
	}
}

// resultFor builds a matched Result from a single winning Deezer candidate.
func resultFor(c deezer.Artist) Result {
	id := strconv.FormatInt(c.ID, 10)
	return Result{Matched: true, DeezerID: &id, ImageURL: nilIfEmpty(c.Picture)}
}

// maxTieBreakCandidates bounds the tie-break (RED-phase placeholder value;
// GREEN wires this into tieBreak's actual bound check).
const maxTieBreakCandidates = 5

// titlesMatch is a RED-phase placeholder that always reports no match, so
// TestTitlesMatch and every positive tie-break test fail until GREEN
// implements the real equality-or-guarded-containment comparison.
func titlesMatch(_, _ string) bool {
	return false
}

// tieBreak is the marked seam Task 2 replaces with D-08's shared-album-title
// tie-break (titlesMatch, maxTieBreakCandidates, minTieBreakTitleLength).
// Until Task 2 lands, an ambiguous same-name tie falls closed to a
// non-match (D-09) rather than guessing.
func (m *Matcher) tieBreak(_ context.Context, _ string, _ []deezer.Artist) (Result, error) {
	return Result{}, nil
}
