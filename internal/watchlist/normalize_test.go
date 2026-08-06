package watchlist

// normalizeSet is unexported, so its test lives in package watchlist rather
// than the external watchlist_test package used by service_test.go's
// real-Postgres tests.

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeSet(t *testing.T) {
	tests := []struct {
		name    string
		values  []string
		want    []string
		wantErr error
		errVal  string
	}{
		{
			name:   "submission order normalises to allow-list order",
			values: []string{"single", "album"},
			want:   []string{"album", "single"},
		},
		{
			name:   "duplicates are de-duplicated",
			values: []string{"album", "album"},
			want:   []string{"album"},
		},
		{
			name:   "empty input yields a non-nil empty slice",
			values: []string{},
			want:   []string{},
		},
		{
			name:    "unknown value errors and names the offending value",
			values:  []string{"lp"},
			wantErr: ErrInvalidReleaseType,
			errVal:  "lp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeSet(tt.values, ReleaseTypes, ErrInvalidReleaseType)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want errors.Is(err, %v)", err, tt.wantErr)
				}
				if tt.errVal != "" && !strings.Contains(err.Error(), tt.errVal) {
					t.Fatalf("err.Error() = %q, want it to contain %q", err.Error(), tt.errVal)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got == nil {
				t.Fatal("got nil slice, want a non-nil slice")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
