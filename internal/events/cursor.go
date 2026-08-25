package events

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Cursor is the typed domain position ListEvents' composite keyset paging
// walks: a release_date/id pair rather than a single bigint (quick task
// 260825-g6i, HIST-01). ReleaseDate nil means "this position is inside the
// undated tail of the feed" -- a distinct state from ListParams.Cursor ==
// nil, which means "no cursor at all, first page." A dated position always
// carries both fields; the undated tail is walked by id alone, descending,
// since release_date IS NULL for every row in it.
//
// This type crosses the domain boundary (events.ListParams.Cursor,
// events.Page.NextCursor); only the HTTP boundary (internal/httpserver)
// speaks the encoded string, via EncodeCursor/DecodeCursor below.
type Cursor struct {
	ReleaseDate *string
	ID          int64
}

// cursorWireForm is the compact JSON shape EncodeCursor/DecodeCursor
// serialize a Cursor as -- short field names ("d", "i") keep the encoded
// token small, since it travels in a URL query string on every "Load more"
// request.
type cursorWireForm struct {
	ReleaseDate *string `json:"d,omitempty"`
	ID          int64   `json:"i"`
}

// maxCursorTokenLen bounds the encoded token's own length, checked before
// any base64 decode or JSON unmarshal so an abusive payload is never
// buffered or allocated against (T-g6i-01). A legitimate token -- base64url
// of a compact JSON object with at most a 10-character MusicBrainz partial
// date and a bigint id -- is well under 64 bytes.
const maxCursorTokenLen = 512

// maxCursorReleaseDateLen bounds the decoded ReleaseDate field alone (T-g6i-01):
// a MusicBrainz partial date is at most 10 characters ("YYYY-MM-DD").
const maxCursorReleaseDateLen = 32

// ErrInvalidCursor is the single sentinel every DecodeCursor failure path
// wraps. Callers (internal/httpserver) branch on this sentinel, never on
// the underlying base64/JSON error text -- that text never reaches a
// response body (T-g6i-05).
var ErrInvalidCursor = errors.New("invalid cursor")

// EncodeCursor marshals c into the opaque token travelling in the
// next_cursor response field and the cursor query parameter.
// base64.RawURLEncoding (not StdEncoding) is required: the token travels in
// a query string, so "+", "/" and "=" must not appear in it.
func EncodeCursor(c Cursor) string {
	wire := cursorWireForm(c)
	// json.Marshal on this fixed, package-internal struct shape never
	// errors -- no channel, func, or cyclic value is reachable here.
	data, _ := json.Marshal(wire)
	return base64.RawURLEncoding.EncodeToString(data)
}

// DecodeCursor parses s (the client-supplied cursor query parameter) back
// into a Cursor, or returns a wrapped ErrInvalidCursor. Every rejection
// happens before the string is trusted for anything -- length is checked
// first, ahead of any decode (T-g6i-01), and every field is validated
// after successful JSON unmarshal.
func DecodeCursor(s string) (Cursor, error) {
	if len(s) > maxCursorTokenLen {
		return Cursor{}, fmt.Errorf("%w: token exceeds %d characters", ErrInvalidCursor, maxCursorTokenLen)
	}

	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, fmt.Errorf("%w: %s", ErrInvalidCursor, "not valid base64url")
	}

	var wire cursorWireForm
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil {
		return Cursor{}, fmt.Errorf("%w: %s", ErrInvalidCursor, "not valid cursor JSON")
	}

	// ids are BIGSERIAL, so zero and negative are never real ids -- the
	// same reasoning internal/httpserver's parseOptionalPositiveInt64
	// already documents for the artist_id/cursor path.
	if wire.ID < 1 {
		return Cursor{}, fmt.Errorf("%w: %s", ErrInvalidCursor, "id must be positive")
	}
	if wire.ReleaseDate != nil && len(*wire.ReleaseDate) > maxCursorReleaseDateLen {
		return Cursor{}, fmt.Errorf("%w: %s", ErrInvalidCursor, "release date exceeds max length")
	}

	return Cursor(wire), nil
}
