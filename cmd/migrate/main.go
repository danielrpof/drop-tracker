// Command migrate is a go run HEAD-schema helper for CI (D-01): it applies
// the current branch's embedded schema through the exact
// internal/db.RunMigrations path cmd/server runs at boot, against a
// database named by DATABASE_URL. It reads no paths from argv -- only the
// DATABASE_URL environment variable -- so it needs no gosec G304 carve-out
// (RESEARCH.md Open Question 3).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/danielrpof/drop-tracker/internal/db"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run reads DATABASE_URL and applies every embedded migration to it via
// db.RunMigrations -- the same embedded-iofs + bounded-retry boot path
// cmd/server uses, including the ahead-of-source no-op guard -- so the
// n1-boot CI job exercises identical migration behavior, not a second code
// path the app never runs.
func run(ctx context.Context) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := db.RunMigrations(ctx, dsn, logger); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
