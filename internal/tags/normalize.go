// Package tags implements the artist tag domain: normalization, the
// get-or-create/attach/detach service, and error translation for the
// per-artist cap trigger (ADR 0004).
package tags

import "errors"

// MaxNameRunes is the tag name length cap (TAG-04), counted in Unicode code
// points after NFC normalization -- matching Postgres char_length.
const MaxNameRunes = 32

var (
	// ErrNameRequired is returned for an empty or whitespace-only name.
	ErrNameRequired = errors.New("tag name is required")
	// ErrNameTooLong is returned when NormalizeName exceeds MaxNameRunes.
	ErrNameTooLong = errors.New("tag name too long")
	// ErrNameInvalid is returned when a name contains a control character.
	ErrNameInvalid = errors.New("tag name contains invalid characters")
)

// NormalizeName is a placeholder -- RED phase, implemented in the GREEN
// commit that follows.
func NormalizeName(raw string) (string, error) {
	return raw, nil
}
