package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect builds a connection string from environment variables and opens a
// pgx connection pool. It pings the database to verify connectivity before
// returning, so a non-nil pool is always ready to use.
func Connect(ctx context.Context) (*pgxpool.Pool, error) {
	user, err := requireEnv("POSTGRES_USER")
	if err != nil {
		return nil, err
	}
	password, err := requireEnv("POSTGRES_PASSWORD")
	if err != nil {
		return nil, err
	}
	dbName, err := requireEnv("POSTGRES_DB")
	if err != nil {
		return nil, err
	}
	host, err := requireEnv("DB_HOST")
	if err != nil {
		return nil, err
	}
	port, err := requireEnv("POSTGRES_PORT")
	if err != nil {
		return nil, err
	}
	sslMode, err := requireEnv("DB_SSLMODE")
	if err != nil {
		return nil, err
	}

	// DSN (Data Source Name): the single connection string bundling everything
	// needed to reach and authenticate to the database — scheme, credentials,
	// host, port, database name, and connection options like sslmode.
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbName, sslMode,
	)

	// Open a pool of reusable, warm connections rather than connecting per
	// request. Requests borrow a connection, use it, and return it; if all are
	// busy, callers wait — which keeps the app fast while protecting Postgres
	// from too many concurrent connections. Note: New is lazy and does not
	// actually dial the database yet (that is what the Ping below verifies).
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	// Force an actual round-trip to the server to fail fast: this proves the DB
	// is reachable and the credentials/database are valid now, at startup,
	// rather than surfacing a confusing error inside the first request handler.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}

// requireEnv returns the value of the named environment variable, or an error
// if it is unset or empty. This fails fast on misconfiguration instead of
// silently falling back to a default that may point at the wrong database.
func requireEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("required environment variable %q is not set", key)
	}
	return value, nil
}
