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
	"log/slog"
	"net/url"
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

// ArtistDetailLookup is the narrow seam this package depends on for
// MusicBrainz's curated url-rels and aliases (D-09r Tier 0's Deezer link
// source and Tier 1's alias-retry source).
type ArtistDetailLookup interface {
	LookupArtist(ctx context.Context, mbid string) (musicbrainz.ArtistDetail, error)
}

// ArtistFetcher is the narrow seam this package depends on for Deezer's
// single-artist fetch (D-09r Tier 0's confirming call, DD-1).
type ArtistFetcher interface {
	ArtistByID(ctx context.Context, artistID string) (deezer.Artist, error)
}

// Matcher implements D-08's match rule against the three narrow seams
// above, plus D-09r's Tier 0/Tier 1 seams. Consumers always depend on
// ArtistSearcher/AlbumLister/ReleaseGroupLister/ArtistDetailLookup/
// ArtistFetcher -- never on *deezer.Client or *musicbrainz.Client directly.
type Matcher struct {
	search  ArtistSearcher
	albums  AlbumLister
	groups  ReleaseGroupLister
	links   ArtistDetailLookup
	artists ArtistFetcher
	logger  *slog.Logger
}

// Option configures optional Matcher behavior applied by NewMatcher.
type Option func(*Matcher)

// WithArtistLinks wires D-09r's Tier 0 (curated Deezer url-rel) and Tier 1
// (alias retry) into a Matcher. A Matcher built without this option behaves
// exactly as it did before D-09r -- both new tiers no-op and Match falls
// back to name-search-only, byte-identical to its pre-D-09r behavior.
func WithArtistLinks(links ArtistDetailLookup, artists ArtistFetcher) Option {
	return func(m *Matcher) {
		m.links = links
		m.artists = artists
	}
}

// WithLogger wires an operator-facing logger for D-09r's absorbed-error
// debug logging (T-09r-07). Falls back to slog.Default() when unset.
func WithLogger(logger *slog.Logger) Option {
	return func(m *Matcher) {
		m.logger = logger
	}
}

// NewMatcher builds a Matcher backed by search, albums and groups. The
// three positional parameters keep their names, order and types so every
// pre-D-09r call site compiles and runs unchanged. cmd/server/main.go is
// the one production site that must pass WithArtistLinks -- omitting it
// silently disables both D-09r tiers in production while every unit test
// still passes.
func NewMatcher(search ArtistSearcher, albums AlbumLister, groups ReleaseGroupLister, opts ...Option) *Matcher {
	m := &Matcher{search: search, albums: albums, groups: groups}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// debug logs msg at slog.LevelDebug through m.logger, falling back to
// slog.Default() when unset -- debug level specifically so D-09r's
// absorbed-error lines are observable to an operator who turns the level
// down, but stay silent under the default handler and in tests.
func (m *Matcher) debug(msg string, args ...any) {
	logger := m.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Debug(msg, args...)
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

// deezerArtistIDFromURL extracts the numeric Deezer artist id from a
// MusicBrainz relation's target URL, or returns "" when rawURL does not
// point at a Deezer artist page. This function only ever *parses* the
// relation URL -- nothing in this package ever issues a request to it
// (T-09r-01, see the plan's threat register): the confirming Deezer request
// D-09r Tier 0 makes is always built from the package-const defaultBaseURL
// plus this digits-only, strconv.ParseInt-validated id.
//
// The host check is an exact allow-list (deezer.com or www.deezer.com),
// never a suffix match -- a suffix rule is the classic way a lookalike host
// (deezer.com.evil.test) slips through (T-09r-02). Splitting the path on
// "/" and discarding empty segments absorbs both a trailing slash and a
// locale prefix (e.g. "/en/"), so only the shape ".../artist/{digits}"
// (with anything preceding "artist") ever resolves to a non-empty id.
func deezerArtistIDFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host != "deezer.com" && host != "www.deezer.com" {
		return ""
	}

	var segments []string
	for _, seg := range strings.Split(u.Path, "/") {
		if seg != "" {
			segments = append(segments, seg)
		}
	}
	if len(segments) < 2 || segments[len(segments)-2] != "artist" {
		return ""
	}

	id, err := strconv.ParseInt(segments[len(segments)-1], 10, 64)
	if err != nil || id <= 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}

// lookupArtistDetail fetches mbid's curated url-rels and aliases through
// m.links, returning the zero ArtistDetail when m.links is nil or mbid is
// empty. This method has no error return by design: a MusicBrainz outage
// must degrade D-09r to exactly its pre-D-09r behavior, never fail a
// watchlist add or burn a Backfill Errored slot. Called exactly once per
// Match so Tier 0 and Tier 1 share one MusicBrainz round trip.
func (m *Matcher) lookupArtistDetail(ctx context.Context, mbid string) musicbrainz.ArtistDetail {
	if m.links == nil || strings.TrimSpace(mbid) == "" {
		return musicbrainz.ArtistDetail{}
	}
	detail, err := m.links.LookupArtist(ctx, mbid)
	if err != nil {
		m.debug("artistart: musicbrainz artist lookup failed", "mbid", mbid, "error", err)
		return musicbrainz.ArtistDetail{}
	}
	return detail
}

// matchLinkedDeezerArtist implements D-09r Tier 0: resolving a curated
// MusicBrainz->Deezer url-rel without any Deezer name search. It reports
// false whenever it declines to resolve, in which case the caller falls
// through to the name path. No name check is applied here -- a MusicBrainz
// url-rel is a human-curated identity assertion, not a similarity
// heuristic, so D-08's strict-equality rule and D-09's fail-closed rule --
// both of which exist to stop a *heuristic* from guessing wrong -- are not
// the applicable guard on this path.
func (m *Matcher) matchLinkedDeezerArtist(ctx context.Context, detail musicbrainz.ArtistDetail) (Result, bool) {
	if m.artists == nil {
		return Result{}, false
	}

	ids := make(map[string]struct{})
	for _, rel := range detail.Relations {
		relType := strings.ToLower(strings.TrimSpace(rel.Type))
		if relType != "free streaming" && relType != "streaming" {
			continue
		}
		id := deezerArtistIDFromURL(rel.URL.Resource)
		if id == "" {
			continue
		}
		ids[id] = struct{}{}
	}

	if len(ids) != 1 {
		// Zero distinct ids: nothing to resolve. Two or more distinct ids:
		// conflicting curated data is ambiguity, and D-09 says ambiguity
		// resolves to no photo rather than to a guess (T-09r-03).
		return Result{}, false
	}
	var deezerID string
	for id := range ids {
		deezerID = id
	}

	artist, err := m.artists.ArtistByID(ctx, deezerID)
	if err != nil {
		m.debug("artistart: deezer artist confirm fetch failed", "deezer_id", deezerID, "error", err)
		return Result{}, false
	}
	if artist.ID == 0 {
		// Unconfirmed (e.g. a pulled/renumbered Deezer artist page).
		return Result{}, false
	}
	// Built from the *fetched* artist, not the URL-derived id, so a
	// Deezer-side merge or redirect lands on the canonical id.
	return resultFor(artist), true
}

// matchNameStrict holds D-08's original decision order -- normalized-name
// equality first, shared album title (equality-or-containment via
// titlesMatch) only to break a tie, nothing else ever -- and D-09's
// fail-closed policy on every ambiguous, empty, or unresolvable outcome.
// Popularity ordering from SearchArtists (Deezer's own NbFan-descending
// sort) is deliberately never consulted at any step. The strict-candidate
// count is returned alongside Result so Match's D-09r Tier 1 alias retry
// can distinguish "found nothing" (count 0, retry aliases) from "found an
// ambiguous tie" (count > 1, a considered D-09 stop -- DD-3).
func (m *Matcher) matchNameStrict(ctx context.Context, mbid, query string) (Result, int, error) {
	normalizedQuery := normalizeArtistName(query)
	if normalizedQuery == "" {
		return Result{}, 0, nil
	}

	candidates, err := m.search.SearchArtists(ctx, query, searchLimit)
	if err != nil {
		return Result{}, 0, err
	}

	var tied []deezer.Artist
	for _, c := range candidates {
		if normalizeArtistName(c.Name) == normalizedQuery {
			tied = append(tied, c)
		}
	}

	switch len(tied) {
	case 0:
		return Result{}, 0, nil
	case 1:
		return resultFor(tied[0]), 1, nil
	default:
		// Two or more same-normalized-name candidates: never pick the first
		// (SearchArtists already sorts by fan count descending, so "take the
		// first" would silently become a popularity-decides-identity rule
		// D-08/D-09 both forbid) and never pick the highest NbFan for the
		// same reason. D-08's shared-album-title tie-break decides instead;
		// until it resolves, this falls closed (D-09).
		result, err := m.tieBreak(ctx, mbid, tied)
		return result, len(tied), err
	}
}

// maxAliasAttempts bounds D-09r Tier 1's alias retry: each attempt is a
// rate-limited Deezer search running synchronously inside an interactive
// HTTP add, and some MusicBrainz artists carry dozens of aliases, mirroring
// maxTieBreakCandidates's own rationale (T-09r-04).
const maxAliasAttempts = 5

// aliasQueryNames builds Tier 1's ordered, deduped, bounded alias search
// list from detail's MusicBrainz aliases. alreadyTried is the
// already-normalized primary name. Aliases typed "Search hint" or "Legal
// name" are prioritized -- returned before every other alias type, in each
// pass's original relative order. An alias whose normalizeArtistName form
// is empty or equals alreadyTried is skipped (no duplicate outbound search
// against the name Match already tried), and a normalized form already
// added is skipped too, so no outbound search is ever repeated. The
// returned names are the RAW alias names -- exactly what SearchArtists
// receives, mirroring how Match passes the raw name today.
func aliasQueryNames(detail musicbrainz.ArtistDetail, alreadyTried string) []string {
	var prioritized, rest []musicbrainz.ArtistAlias
	for _, alias := range detail.Aliases {
		aliasType := strings.ToLower(strings.TrimSpace(alias.Type))
		if aliasType == "search hint" || aliasType == "legal name" {
			prioritized = append(prioritized, alias)
		} else {
			rest = append(rest, alias)
		}
	}

	seen := make(map[string]struct{})
	var names []string
	addFrom := func(aliases []musicbrainz.ArtistAlias) {
		for _, alias := range aliases {
			if len(names) >= maxAliasAttempts {
				return
			}
			normalized := normalizeArtistName(alias.Name)
			if normalized == "" || normalized == alreadyTried {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			names = append(names, alias.Name)
		}
	}
	addFrom(prioritized)
	addFrom(rest)
	return names
}

// matchByAliases implements D-09r Tier 1: re-running matchNameStrict's
// identical strict pipeline against names in order, returning the first
// Matched result. An error from any single alias attempt is logged and
// absorbed -- not surfaced -- so the loop continues to the next alias;
// surfacing it would convert an artist that previously fail-closed cleanly
// into a Backfill Errored outcome, and Backfill deliberately skips
// RecordArtMatchAttempt on error, so the artist would be re-queried every
// sweep forever instead of once per D-12's 24-hour cooldown. When no name
// resolves, returns the zero Result and a nil error -- D-09 fail-closed,
// unchanged.
func (m *Matcher) matchByAliases(ctx context.Context, mbid string, names []string) (Result, error) {
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		result, _, err := m.matchNameStrict(ctx, mbid, name)
		if err != nil {
			m.debug("artistart: alias match attempt failed", "mbid", mbid, "alias", name, "error", err)
			continue
		}
		if result.Matched {
			return result, nil
		}
	}
	return Result{}, nil
}

// Match implements D-09r's decision order ahead of and behind D-08's
// original strict name-equality rule, without changing that rule or this
// method's signature: Tier 0 (a curated MusicBrainz->Deezer url-rel) first,
// then D-08's strict name-equality search, then Tier 1 (the identical
// strict pipeline retried over prioritized, deduped, bounded MusicBrainz
// aliases) only when the strict search found zero candidates -- an
// ambiguous tie is a considered D-09 stop and is never retried under a
// different name string (DD-3). D-09's fail-closed policy holds on every
// ambiguous, empty, or unresolvable outcome throughout.
func (m *Matcher) Match(ctx context.Context, mbid, name string) (Result, error) {
	normalizedName := normalizeArtistName(name)
	if normalizedName == "" {
		return Result{}, nil
	}

	detail := m.lookupArtistDetail(ctx, mbid)

	if result, ok := m.matchLinkedDeezerArtist(ctx, detail); ok {
		return result, nil
	}

	result, count, err := m.matchNameStrict(ctx, mbid, name)
	if err != nil {
		return Result{}, err
	}
	if result.Matched || count > 0 {
		return result, nil
	}

	return m.matchByAliases(ctx, mbid, aliasQueryNames(detail, normalizedName))
}

// resultFor builds a matched Result from a single winning Deezer candidate.
func resultFor(c deezer.Artist) Result {
	id := strconv.FormatInt(c.ID, 10)
	return Result{Matched: true, DeezerID: &id, ImageURL: nilIfEmpty(c.Picture)}
}

// maxTieBreakCandidates bounds the tie-break: the tie-break costs one
// MusicBrainz release-group browse plus one Deezer album fetch per tied
// candidate, and this runs synchronously inside an HTTP add request in the
// sibling plan, so an implausibly large tie set is treated as unresolvable
// rather than expensive. When the tied-candidate count exceeds this, Match
// returns the zero Result with a nil error and issues no fetches at all.
const maxTieBreakCandidates = 5

// albumLimit bounds each tied candidate's Deezer album fetch during the
// tie-break.
const albumLimit = 50

// minTieBreakTitleLength guards titlesMatch's containment loosening against
// a degenerate short-substring collision (e.g. a one-character or
// two-character normalized title "matching" almost anything via
// containment).
const minTieBreakTitleLength = 4

// titlesMatch reports whether two already-normalizeArtistName-normalized
// title strings should be treated as the same release for D-08's tie-break
// (grilling round Q6): true on exact equality, OR when one is a non-empty
// substring of the other AND the shorter of the two strings is at least
// minTieBreakTitleLength runes long.
//
// Real-world edition suffixes ("(deluxe)", "(remastered 2020)") are common
// enough that exact-only comparison would make this tie-break path almost
// never resolve anything in practice, which would defeat the reason it
// exists; requiring the shorter title to clear a minimum length keeps this
// from degenerating into "any short string matches everything." This
// loosening only ever applies to the already-narrow tie-break signal -- it
// never touches D-08's primary name-equality check (Match's own candidate
// filter) and never weakens D-09's fail-closed default, since an unresolved
// or still-ambiguous tie still falls through to a non-match exactly as
// before.
func titlesMatch(a, b string) bool {
	if a == b {
		return true
	}
	if a == "" || b == "" {
		return false
	}
	shorter, longer := a, b
	if len(longer) < len(shorter) {
		shorter, longer = longer, shorter
	}
	if len([]rune(shorter)) < minTieBreakTitleLength {
		return false
	}
	return strings.Contains(longer, shorter)
}

// tieBreak implements D-08's shared-album-title tie-break (grilling round
// Q6) for two or more same-normalized-name Deezer candidates. tied must
// have length >= 2 (Match only calls this in that case).
func (m *Matcher) tieBreak(ctx context.Context, mbid string, tied []deezer.Artist) (Result, error) {
	if len(tied) > maxTieBreakCandidates {
		return Result{}, nil
	}

	groups, err := m.groups.ReleaseGroupsByArtist(ctx, mbid)
	if err != nil {
		return Result{}, err
	}

	groupTitles := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		normalized := normalizeArtistName(g.Title)
		if normalized == "" {
			continue
		}
		groupTitles[normalized] = struct{}{}
	}
	if len(groupTitles) == 0 {
		// Nothing to break the tie with, and D-08 forbids resolving it by
		// any other signal.
		return Result{}, nil
	}

	var winners []deezer.Artist
	for _, c := range tied {
		albums, err := m.albums.ArtistAlbums(ctx, strconv.FormatInt(c.ID, 10), albumLimit)
		if err != nil {
			return Result{}, err
		}

		for _, a := range albums {
			normalizedAlbumTitle := normalizeArtistName(a.Title)
			if normalizedAlbumTitle == "" {
				continue
			}
			matched := false
			for groupTitle := range groupTitles {
				if titlesMatch(normalizedAlbumTitle, groupTitle) {
					matched = true
					break
				}
			}
			if matched {
				winners = append(winners, c)
				break
			}
		}
	}

	if len(winners) != 1 {
		// Zero winners or two-or-more winners: D-09 fail-closed. An
		// ambiguous tie-break resolves to no photo, never to a guess.
		return Result{}, nil
	}
	return resultFor(winners[0]), nil
}
