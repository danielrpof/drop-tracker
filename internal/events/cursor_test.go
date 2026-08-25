package events

import (
	"errors"
	"strings"
	"testing"
)

func TestEncodeDecodeCursor_RoundTripsWithReleaseDate(t *testing.T) {
	date := "2026-05-01"
	c := Cursor{ReleaseDate: &date, ID: 42}

	token := EncodeCursor(c)
	got, err := DecodeCursor(token)
	if err != nil {
		t.Fatalf("DecodeCursor(%q): %v", token, err)
	}
	if got.ID != c.ID {
		t.Fatalf("ID = %d, want %d", got.ID, c.ID)
	}
	if got.ReleaseDate == nil || *got.ReleaseDate != date {
		t.Fatalf("ReleaseDate = %v, want %q", got.ReleaseDate, date)
	}
}

func TestEncodeDecodeCursor_RoundTripsWithNilReleaseDate(t *testing.T) {
	c := Cursor{ReleaseDate: nil, ID: 7}

	token := EncodeCursor(c)
	got, err := DecodeCursor(token)
	if err != nil {
		t.Fatalf("DecodeCursor(%q): %v", token, err)
	}
	if got.ID != c.ID {
		t.Fatalf("ID = %d, want %d", got.ID, c.ID)
	}
	if got.ReleaseDate != nil {
		t.Fatalf("ReleaseDate = %v, want nil", got.ReleaseDate)
	}
}

func TestEncodeCursor_ProducesURLSafeToken(t *testing.T) {
	// A token containing +, / or = would need further escaping in a query
	// string -- RawURLEncoding must never produce either.
	date := "2020"
	token := EncodeCursor(Cursor{ReleaseDate: &date, ID: 999999999})
	if strings.ContainsAny(token, "+/=") {
		t.Fatalf("token %q contains a non-URL-safe character", token)
	}
}

func TestDecodeCursor_RejectsInvalidInput(t *testing.T) {
	longDate := strings.Repeat("9", maxCursorReleaseDateLen+1)
	longToken := strings.Repeat("a", maxCursorTokenLen+1)

	cases := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"non-base64url input", "not valid base64!!"},
		{"valid base64 that is not JSON", "bm90IGpzb24"}, // base64url("not json")
		{"JSON missing the id", "eyJkIjoiMjAyMCJ9"},      // base64url(`{"d":"2020"}`)
		{"id of 0", "eyJpIjowfQ"},                        // base64url(`{"i":0}`)
		{"id negative", "eyJpIjotMX0"},                   // base64url(`{"i":-1}`)
		{"token longer than the length cap", longToken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeCursor(tc.input); err == nil {
				t.Fatalf("DecodeCursor(%q): want error, got nil", tc.input)
			} else if !errors.Is(err, ErrInvalidCursor) {
				t.Fatalf("DecodeCursor(%q): error = %v, want wrapping ErrInvalidCursor", tc.input, err)
			}
		})
	}

	// A release-date string longer than the cap, embedded in an otherwise
	// well-formed token, built via EncodeCursor rather than a hand-written
	// literal so this case stays correct if the wire encoding ever changes.
	t.Run("release-date string longer than the release-date cap", func(t *testing.T) {
		token := EncodeCursor(Cursor{ReleaseDate: &longDate, ID: 1})
		if _, err := DecodeCursor(token); err == nil {
			t.Fatalf("DecodeCursor(%q): want error, got nil", token)
		} else if !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("DecodeCursor(%q): error = %v, want wrapping ErrInvalidCursor", token, err)
		}
	})
}

func TestDecodeCursor_NeverPanics(t *testing.T) {
	inputs := []string{
		"", " ", "\x00", "%%%", strings.Repeat("z", 10000),
		"null", "[]", "{}", "true", "1.5",
	}
	for _, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("DecodeCursor(%q) panicked: %v", in, r)
				}
			}()
			_, _ = DecodeCursor(in)
		}()
	}
}
