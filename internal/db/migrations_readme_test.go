package db_test

import (
	"os"
	"strings"
	"testing"
)

// migrationsReadmePath is relative to this package's directory (internal/db),
// matching how `go test` sets the working directory for package tests.
const migrationsReadmePath = "migrations/README.md"

// migrationsReadmeMinLines guards against a phrase-stuffed stub satisfying
// every required-phrase check below without actually carrying the content
// PLAN.md 16-05's <action> specifies (five real sections).
const migrationsReadmeMinLines = 60

// TestMigrationsReadme_ContainsRequiredPhrases asserts internal/db/migrations/README.md
// still documents every load-bearing rule this plan's <action> requires. A future edit
// that guts a section should turn this test red rather than silently drift out of sync
// with what cmd/migration-check actually enforces. See
// .planning/phases/16-rollback-safe-migrations/16-05-PLAN.md.
func TestMigrationsReadme_ContainsRequiredPhrases(t *testing.T) {
	content, err := os.ReadFile(migrationsReadmePath)
	if err != nil {
		t.Fatalf("read %s: %v", migrationsReadmePath, err)
	}
	text := string(content)

	tests := []struct {
		name   string
		phrase string
	}{
		{"backward-incompatible finding class named", "backward-incompatible"},
		{"unsafe-forward finding class named", "unsafe-forward"},
		{"N-1 invariant phrasing present", "N-1"},
		{"000006 expand migration cited", "000006"},
		{"000007 backfill migration cited", "000007"},
		{"idempotent backfill rule stated", "idempotent"},
		{"immutable released-migration rule stated", "immutable"},
		{"CONCURRENTLY / concurrent index-build rule stated", "concurrently"},
		{"allow-destructive annotation prefix documented", "migration-check:allow-destructive"},
	}

	lowerText := strings.ToLower(text)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(lowerText, strings.ToLower(tc.phrase)) {
				t.Errorf(
					"%s is missing required phrase %q — this test protects PLAN.md 16-05's <action> section contents; "+
						"if you intentionally removed the phrase, update both the README and this test together",
					migrationsReadmePath, tc.phrase,
				)
			}
		})
	}
}

// TestMigrationsReadme_IsNonTrivial ensures a stub file stuffed with the phrases above
// (but none of the actual explanatory content) cannot pass the phrase checks alone.
func TestMigrationsReadme_IsNonTrivial(t *testing.T) {
	content, err := os.ReadFile(migrationsReadmePath)
	if err != nil {
		t.Fatalf("read %s: %v", migrationsReadmePath, err)
	}

	lines := strings.Count(string(content), "\n")
	if lines < migrationsReadmeMinLines {
		t.Errorf("%s has %d lines, want at least %d — a phrase-stuffed stub must not satisfy this test",
			migrationsReadmePath, lines, migrationsReadmeMinLines)
	}
}
