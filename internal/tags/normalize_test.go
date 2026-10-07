package tags_test

// External test package: NormalizeName is exported, so no whitebox access is
// needed (unlike internal/watchlist's normalizeSet tests).

import (
	"errors"
	"strconv"
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

func TestLimitSentinelsEmbedTheirConstants(t *testing.T) {
	if !strings.Contains(tags.ErrNameTooLong.Error(), strconv.Itoa(tags.MaxNameRunes)) {
		t.Fatalf("ErrNameTooLong = %q, want it to contain %d", tags.ErrNameTooLong, tags.MaxNameRunes)
	}
	if !strings.Contains(tags.ErrTagCapReached.Error(), strconv.Itoa(tags.MaxTagsPerArtist)) {
		t.Fatalf("ErrTagCapReached = %q, want it to contain %d", tags.ErrTagCapReached, tags.MaxTagsPerArtist)
	}
}
