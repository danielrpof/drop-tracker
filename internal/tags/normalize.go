// Package tags implements the artist tag domain: normalization, the
// get-or-create/attach/detach service, and error translation for the
// per-artist cap trigger (ADR 0004).
package tags

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// MaxNameRunes is the tag name length cap (TAG-04), counted in Unicode code
// points after NFC normalization -- matching Postgres char_length.
const MaxNameRunes = 32

var (
	// ErrNameRequired is returned for an empty or whitespace-only name.
	ErrNameRequired = errors.New("tag name is required")
	// ErrNameTooLong is returned when NormalizeName exceeds MaxNameRunes.
	ErrNameTooLong = fmt.Errorf("tag name must be at most %d characters", MaxNameRunes)
	// ErrNameInvalid is returned when a name contains a control character.
	ErrNameInvalid = errors.New("tag name contains invalid characters")
)

// NormalizeName applies NFC (so a decomposed and precomposed form of the
// same tag are one identity), trims outer whitespace and collapses internal
// runs to a single space, then validates: empty/blank is ErrNameRequired,
// a control character other than whitespace is ErrNameInvalid, and more
// than MaxNameRunes is ErrNameTooLong. Rune-counted after NFC, matching
// Postgres char_length (TAG-04 encoding probe) -- never byte-counted, so a
// multi-byte name is not rejected for its byte length.
func NormalizeName(raw string) (string, error) {
	s := norm.NFC.String(raw)
	s = strings.Join(strings.Fields(s), " ")

	if s == "" {
		return "", ErrNameRequired
	}
	if strings.ContainsFunc(s, unicode.IsControl) {
		return "", ErrNameInvalid
	}
	if utf8.RuneCountInString(s) > MaxNameRunes {
		return "", ErrNameTooLong
	}
	return s, nil
}
