package buildinfo

import "testing"

func TestVersion_DefaultsToDev(t *testing.T) {
	if Version != "dev" {
		t.Fatalf("Version = %q, want %q (the flagless-build fallback)", Version, "dev")
	}
}

func TestShort_Truncates(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"40-char SHA truncates to 12", "deadbeefcafe0123456789abcdef0123456789ab", "deadbeefcafe"},
		{"exactly 12 chars unchanged", "deadbeefcafe", "deadbeefcafe"},
		{"dev unchanged", "dev", "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := Version
			Version = tt.in
			t.Cleanup(func() { Version = old })
			if got := Short(); got != tt.want {
				t.Fatalf("Short() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestVersion_Injected is the gate for the silently-ignored -X path failure mode
// (18-RESEARCH.md Pitfall 7): if the fully-qualified module path in the link
// flag is wrong, the Go linker ignores it with no error and Version stays "dev".
// This test is only meaningful when run with the flag, e.g.:
//
//	go test -ldflags "-X github.com/danielrpof/drop-tracker/internal/buildinfo.Version=deadbeefcafe0123456789abcdef0123456789ab" \
//	  ./internal/buildinfo/ -run TestVersion_Injected -count=1 -v
//
// A SKIP under that command means the linker ignored the flag — treat it as a failure.
func TestVersion_Injected(t *testing.T) {
	const injected = "deadbeefcafe0123456789abcdef0123456789ab"
	if Version == "dev" {
		t.Skip(`Version is still "dev" — run under go test -ldflags "-X .../internal/buildinfo.Version=<value>"; a SKIP there means the -X import path is wrong`)
	}
	if Version != injected {
		t.Fatalf("Version = %q, want the injected %q", Version, injected)
	}
}
