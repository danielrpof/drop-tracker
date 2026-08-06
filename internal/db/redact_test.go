package db

// redact_test.go is in package db (not db_test) because both functions
// under test, redactError and redactDSN, are unexported -- the same split
// internal/watchlist already uses (normalize_test.go is in-package,
// service_test.go is package watchlist_test for the real-Postgres tests).
// migrate_test.go and sqlc_test.go stay package db_test.

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// dsnFixtures enumerates every DSN form config.Config.DatabaseURL accepts,
// pinning redactError (this file) and redactDSN
// (TestRedactDSN_NeverEchoesPassword) against one shared list. This is what
// closes CR-01/G-02-2: a DSN form added here for one helper is
// automatically asserted against the other, so the two functions cannot
// silently diverge in coverage again.
//
// Every fixture password is the project's existing non-entropic
// test-fixture marker, "local-test-fixture-password" (see
// migrate_test.go's closedPortDSN/closedPortKeywordValueDSN), or a phrase
// built from it -- never a new realistic-looking secret, since the
// gitleaks pre-commit hook is active and would flag one.
var dsnFixtures = []struct {
	name string
	dsn  string
	// mustNotContain lists the exact substrings that may never survive
	// redaction.
	mustNotContain []string
}{
	{
		name:           "URL form, userinfo",
		dsn:            "postgres://drop_tracker:local-test-fixture-password@127.0.0.1:5432/drop_tracker?sslmode=disable",
		mustNotContain: []string{"local-test-fixture-password"},
	},
	{
		name:           "URL form, password as a query parameter, no userinfo",
		dsn:            "postgres://127.0.0.1:5432/drop_tracker?password=local-test-fixture-password&sslmode=disable",
		mustNotContain: []string{"local-test-fixture-password"},
	},
	{
		name:           "keyword/value form, canonical spacing",
		dsn:            "host=127.0.0.1 port=5432 user=drop_tracker password=local-test-fixture-password dbname=drop_tracker sslmode=disable",
		mustNotContain: []string{"local-test-fixture-password"},
	},
	{
		name:           "keyword/value form, whitespace around the =",
		dsn:            "host=127.0.0.1 port=5432 user=drop_tracker password = local-test-fixture-password dbname=drop_tracker sslmode=disable",
		mustNotContain: []string{"local-test-fixture-password"},
	},
	{
		name: "keyword/value form, single-quoted value containing spaces",
		dsn:  "host=127.0.0.1 port=5432 user=drop_tracker password='local-test-fixture-password has spaces' dbname=drop_tracker sslmode=disable",
		// Both the whole quoted phrase and its final word are checked, so
		// a redaction that stops at the value's first space still fails.
		mustNotContain: []string{"local-test-fixture-password has spaces", "spaces"},
	},
	{
		name:           "keyword/value form, differently-cased keyword",
		dsn:            "host=127.0.0.1 port=5432 user=drop_tracker Password=local-test-fixture-password dbname=drop_tracker sslmode=disable",
		mustNotContain: []string{"local-test-fixture-password"},
	},
	{
		// What a wrapped multi-layer driver/migration error looks like:
		// one segment in URL-userinfo form, another in keyword/value form.
		name:           "combined: URL-form userinfo DSN and a keyword/value fragment in one message",
		dsn:            `initial connect via postgres://drop_tracker:local-test-fixture-password@127.0.0.1:5432/drop_tracker failed; retry target host=127.0.0.1 password=local-test-fixture-password-retry dbname=drop_tracker`,
		mustNotContain: []string{"local-test-fixture-password"},
	},
}

// wrapAsDriverError embeds dsn in the middle of a message with text on both
// sides, the way a real driver/dial error would -- never the bare DSN
// alone.
func wrapAsDriverError(dsn string) error {
	return fmt.Errorf("connect to database: dial %s: connection refused", dsn)
}

func TestRedactError_NeverEchoesPassword(t *testing.T) {
	for _, f := range dsnFixtures {
		t.Run(f.name, func(t *testing.T) {
			got := redactError(wrapAsDriverError(f.dsn))
			for _, secret := range f.mustNotContain {
				if strings.Contains(got, secret) {
					t.Fatalf("fixture %q: redactError(err) = %q, must not contain %q", f.name, got, secret)
				}
			}
		})
	}
}

// TestRedactError_KeepsDiagnosticContext is the counterweight to
// TestRedactError_NeverEchoesPassword: scrubbing by destroying the whole
// message would satisfy a "secret absent" assertion while making a failed
// migration at container start undiagnosable -- a different bug, equally
// real.
func TestRedactError_KeepsDiagnosticContext(t *testing.T) {
	const dsn = "host=127.0.0.1 port=5432 user=drop_tracker password=local-test-fixture-password dbname=drop_tracker sslmode=disable"

	got := redactError(wrapAsDriverError(dsn))

	for _, want := range []string{"host=127.0.0.1", "dbname=drop_tracker", "connect to database", "connection refused"} {
		if !strings.Contains(got, want) {
			t.Fatalf("redactError(err) = %q, want it to still contain %q", got, want)
		}
	}
	if !strings.Contains(got, "password=<redacted>") {
		t.Fatalf("redactError(err) = %q, want a visible redaction placeholder in place of the password", got)
	}
}

// TestRedactError_LeavesNonDSNTextAlone guards against a pattern loose
// enough to eat ordinary diagnostic text: error text that merely mentions
// the word "password" without assigning one, such as a Postgres
// authentication-failure message, must pass through byte-identical.
func TestRedactError_LeavesNonDSNTextAlone(t *testing.T) {
	tests := []string{
		`password authentication failed for user "drop_tracker"`,
		"please rotate your password soon, it is about to expire",
	}

	for _, msg := range tests {
		t.Run(msg, func(t *testing.T) {
			got := redactError(errors.New(msg))
			if got != msg {
				t.Fatalf("redactError(err) = %q, want byte-identical %q", got, msg)
			}
		})
	}
}
