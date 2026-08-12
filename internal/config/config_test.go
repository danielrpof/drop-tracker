package config_test

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/config"
)

// repoRoot resolves the repository root relative to this test file's own
// location, so file-based tests pass regardless of the working directory
// `go test` is invoked from.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file location via runtime.Caller")
	}
	// this file lives at internal/config/config_test.go; repo root is two
	// directories up.
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// setRequired sets only the one variable Load cannot proceed without,
// leaving every other variable exactly as the ambient test environment has
// it (t.Setenv auto-restores on test end, so no manual cleanup is needed).
func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
}

func TestLoad_Defaults(t *testing.T) {
	setRequired(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, want 8080", cfg.HTTPPort)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "json")
	}
	if cfg.DiscordWebhookURL != "" {
		t.Errorf("DiscordWebhookURL = %q, want empty string (no default)", cfg.DiscordWebhookURL)
	}
	if cfg.PollInterval.String() != "15m0s" {
		t.Errorf("PollInterval = %v, want 15m0s", cfg.PollInterval)
	}
	if cfg.MusicBrainzUserAgent == "" {
		t.Error("MusicBrainzUserAgent = \"\", must never default to empty (MusicBrainz throttles missing/default UAs)")
	}
	if cfg.MusicBrainzRateLimitPerSec != 1 {
		t.Errorf("MusicBrainzRateLimitPerSec = %v, want 1", cfg.MusicBrainzRateLimitPerSec)
	}
	if cfg.DeezerRateLimitPer5s != 50 {
		t.Errorf("DeezerRateLimitPer5s = %d, want 50", cfg.DeezerRateLimitPer5s)
	}
}

func TestLoad_ExplicitValueEqualToDefault(t *testing.T) {
	setRequired(t)
	unset, err := config.Load()
	if err != nil {
		t.Fatalf("Load() with LOG_LEVEL unset returned unexpected error: %v", err)
	}

	t.Setenv("LOG_LEVEL", "info")
	explicit, err := config.Load()
	if err != nil {
		t.Fatalf("Load() with LOG_LEVEL=info returned unexpected error: %v", err)
	}

	if !reflect.DeepEqual(unset, explicit) {
		t.Errorf("Config with LOG_LEVEL unset (%+v) differs from LOG_LEVEL=info explicit (%+v); "+
			"an explicitly-set value equal to the default must not conflict with the unset path", unset, explicit)
	}
}

func TestLoad_OptionalUnsetIsNotAnError(t *testing.T) {
	setRequired(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() with DISCORD_WEBHOOK_URL unset returned unexpected error: %v", err)
	}
	if cfg.DiscordWebhookURL != "" {
		t.Errorf("DiscordWebhookURL = %q, want empty string", cfg.DiscordWebhookURL)
	}
}

// unsetViaSetenv sets a sentinel value with t.Setenv (which registers the
// auto-restore hook) and then immediately unsets it, so the "absent"
// condition is still reverted automatically at test end.
func unsetViaSetenv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "sentinel-will-be-unset")
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("os.Unsetenv(%q): %v", key, err)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	unsetViaSetenv(t, "DATABASE_URL")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() with DATABASE_URL unset returned nil error, want an error naming DATABASE_URL")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("error = %q, want it to mention DATABASE_URL", err.Error())
	}
}

func TestLoad_EmptyRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal(`Load() with DATABASE_URL="" returned nil error, want an error naming DATABASE_URL`)
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("error = %q, want it to mention DATABASE_URL", err.Error())
	}
}

func TestLoad_AggregatesAllMissing(t *testing.T) {
	unsetViaSetenv(t, "DATABASE_URL")
	t.Setenv("HTTP_PORT", "not-a-number")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() with DATABASE_URL absent and HTTP_PORT invalid returned nil error")
	}

	// caarlos0/env v11.4.1 names a missing/empty required variable by its
	// `env` tag ("DATABASE_URL") but a type-conversion failure by the Go
	// struct field name ("HTTPPort", not "HTTP_PORT") — verified directly
	// against ParseError.Error() in the vendored module source. Both
	// substrings independently confirm the aggregate names every offending
	// setting; neither assertion implies anything about message order.
	msg := err.Error()
	if !strings.Contains(msg, "DATABASE_URL") {
		t.Errorf("aggregate error %q does not mention DATABASE_URL", msg)
	}
	if !strings.Contains(msg, "HTTPPort") {
		t.Errorf("aggregate error %q does not mention HTTPPort", msg)
	}
}

// configEnvKeys reflects over Config and returns the set of env key names
// (the text before the first comma in each `env` struct tag, so
// "DATABASE_URL,notEmpty" yields "DATABASE_URL").
func configEnvKeys(t *testing.T) map[string]bool {
	t.Helper()
	keys := map[string]bool{}
	typ := reflect.TypeOf(config.Config{})
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("env")
		if tag == "" {
			continue
		}
		name := strings.SplitN(tag, ",", 2)[0]
		keys[name] = true
	}
	return keys
}

// envExampleKeys parses .env.example with a line scanner: blank lines and
// comment lines are skipped, and the text before the first "=" is taken as
// the key.
func envExampleKeys(t *testing.T) map[string]bool {
	t.Helper()
	path := filepath.Join(repoRoot(t), ".env.example")

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open .env.example: %v", err)
	}
	defer func() { _ = f.Close() }()

	keys := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		keys[line[:idx]] = true
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan .env.example: %v", err)
	}
	return keys
}

// setDiff returns the sorted keys present in a but not in b.
func setDiff(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// TestLoad_TypeErrorNeverEchoesSecretFields guards T-01-09: caarlos0/env's
// aggregate error format (`parse error on field %q of type %q: %v`) embeds
// the underlying strconv error, which quotes the raw invalid input verbatim
// for a field that failed *type conversion* -- so a malformed HTTP_PORT is
// expected to appear in the error text (that's not a secret, just an
// operator typo). The only fields that hold secret material (DatabaseURL,
// DiscordWebhookURL) are plain, unvalidated strings with no custom parser,
// so they can never fail type conversion and can never have a value echoed
// this way. This test cements that structural invariant rather than
// asserting a blanket "no value is ever echoed" claim, which is false.
func TestLoad_TypeErrorNeverEchoesSecretFields(t *testing.T) {
	const secret = "postgres://user:s3cr3t-p4ssw0rd@localhost:5432/db?sslmode=disable"
	t.Setenv("DATABASE_URL", secret)
	t.Setenv("HTTP_PORT", "not-a-number")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() with HTTP_PORT=not-a-number returned nil error")
	}

	msg := err.Error()
	if !strings.Contains(msg, "not-a-number") {
		t.Errorf("aggregate error %q does not mention the invalid HTTP_PORT value; "+
			"expected library behavior for a non-secret typed field changed", msg)
	}
	if strings.Contains(msg, secret) || strings.Contains(msg, "s3cr3t-p4ssw0rd") {
		t.Errorf("aggregate error %q leaked the DATABASE_URL secret value", msg)
	}
}

func TestEnvExampleCompleteness(t *testing.T) {
	structKeys := configEnvKeys(t)
	fileKeys := envExampleKeys(t)

	inStructNotFile := setDiff(structKeys, fileKeys)
	inFileNotStruct := setDiff(fileKeys, structKeys)

	if len(inStructNotFile) > 0 || len(inFileNotStruct) > 0 {
		t.Errorf("Config and .env.example have drifted apart.\n"+
			"Fields in Config with no documented .env.example key: %v\n"+
			"Keys in .env.example with no matching Config field: %v",
			inStructNotFile, inFileNotStruct)
	}
}

func TestDotEnvIsNotTracked(t *testing.T) {
	root := repoRoot(t)

	f, err := os.Open(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("open .gitignore: %v", err)
	}
	defer func() { _ = f.Close() }()

	found := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if scanner.Text() == ".env" {
			found = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan .gitignore: %v", err)
	}
	if !found {
		t.Error(`.gitignore does not contain an exact-match ".env" line`)
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH, skipping git ls-files check")
		return
	}

	cmd := exec.Command("git", "ls-files", ".env")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files .env: %v", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("git ls-files .env returned %q, want empty output (no .env file tracked)", out)
	}
}
