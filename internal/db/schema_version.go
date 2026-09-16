package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// RowQuerier is the minimal surface SchemaVersion needs from a database
// handle; *pgxpool.Pool satisfies it. Declaring the seam here keeps the
// hand-rolled schema_migrations read testable without a live Postgres.
type RowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// SchemaVersion reads golang-migrate's schema_migrations bookkeeping row --
// (version bigint, dirty boolean). That table is created outside
// internal/db/migrations/, so sqlc never sees it and the read is hand-rolled
// pgx. One helper shared by /ready and plan 18-04's /status instance block
// (D-15) -- one schema-version concept, not two.
func SchemaVersion(ctx context.Context, q RowQuerier) (version uint, dirty bool, err error) {
	var v int64
	err = q.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&v, &dirty)
	if errors.Is(err, pgx.ErrNoRows) {
		// An empty table is unreachable given cmd/server's boot order
		// (RunMigrations runs first and always leaves a row); handled anyway
		// as applied-version zero rather than a panic or a 500.
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read schema_migrations: %w", err)
	}
	if v < 0 {
		// golang-migrate only ever writes a non-negative version; a negative
		// value means the row was tampered with out of band.
		return 0, false, fmt.Errorf("read schema_migrations: negative version %d", v)
	}
	return uint(v), dirty, nil
}

// SchemaVersionReader adapts a RowQuerier to the parameterless SchemaVersion
// call httpserver's SchemaVersioner seam expects.
type SchemaVersionReader struct {
	q RowQuerier
}

// NewSchemaVersionReader builds a reader over q -- typically the process-wide
// *pgxpool.Pool.
func NewSchemaVersionReader(q RowQuerier) *SchemaVersionReader {
	return &SchemaVersionReader{q: q}
}

// SchemaVersion delegates to the package function against the wrapped querier.
func (r *SchemaVersionReader) SchemaVersion(ctx context.Context) (uint, bool, error) {
	return SchemaVersion(ctx, r.q)
}
