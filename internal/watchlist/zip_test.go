package watchlist

// Whitebox: zipTags is unexported, mirroring normalize_test.go's own
// package watchlist convention for testing a private function.

import "testing"

func TestZipTags(t *testing.T) {
	t.Run("empty input yields non-nil empty slice", func(t *testing.T) {
		got, err := zipTags(nil, nil)
		if err != nil {
			t.Fatalf("zipTags(nil, nil) err = %v", err)
		}
		if got == nil {
			t.Fatal("zipTags(nil, nil) = nil, want a non-nil empty slice")
		}
		if len(got) != 0 {
			t.Fatalf("zipTags(nil, nil) = %+v, want empty", got)
		}
	})

	t.Run("matched pairs zip index for index", func(t *testing.T) {
		got, err := zipTags([]int64{1, 2}, []string{"afrobeat", "banda"})
		if err != nil {
			t.Fatalf("zipTags err = %v", err)
		}
		want := []TagRef{{ID: 1, Name: "afrobeat"}, {ID: 2, Name: "banda"}}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("zipTags = %+v, want %+v", got, want)
		}
	})

	t.Run("length mismatch is an error, not a silent mispair", func(t *testing.T) {
		if _, err := zipTags([]int64{1, 2}, []string{"only-one"}); err == nil {
			t.Fatal("zipTags with mismatched lengths: want error, got nil")
		}
	})
}
