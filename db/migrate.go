package db

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// schema holds the contents of schema.sql, embedded into the binary at build
// time by the //go:embed directive below. This means the SQL ships inside the
// compiled binary — no need to locate the file on disk at runtime.

//go:embed schema.sql
var schema string

// Migrate runs the schema script against the database, creating tables that do
// not yet exist. It is idempotent (the script uses IF NOT EXISTS), so it is
// safe to call on every startup.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("running schema migration: %w", err)
	}
	return nil
}
