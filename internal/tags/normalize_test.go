package tags_test

// Whitebox-adjacent: NormalizeName is exported, but this file lives beside
// service_test.go's package tags_test convention (internal/watchlist's
// normalize_test.go precedent is whitebox because normalizeSet is
// unexported; NormalizeName here is exported, so package tags_test is fine).

import (
	"errors"
	"strings"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/tags"
)

func TestNormalizeName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{"trims and collapses", "  Reggaeton  ", "Reggaeton", nil},
		{"collapses internal whitespace", "hip   hop", "hip hop", nil},
		{"NFC-normalizes", "ré", "ré", nil}, // "re" + combining acute -> NFC "ré"
		{"empty is required", "", "", tags.ErrNameRequired},
		{"whitespace-only is required", " \t\n ", "", tags.ErrNameRequired},
		{"32 multi-byte runes accepted", strings.Repeat("é", 32), strings.Repeat("é", 32), nil},
		{"33 multi-byte runes too long", strings.Repeat("é", 33), "", tags.ErrNameTooLong},
		{"control character invalid", "bad\u0007name", "", tags.ErrNameInvalid},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tags.NormalizeName(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("NormalizeName(%q) err = %v, want %v", tc.input, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeName(%q) unexpected err: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
