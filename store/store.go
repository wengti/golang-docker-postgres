package store

import "github.com/jackc/pgx/v5/pgxpool"

// Store is the data-access layer. It owns the connection pool and exposes
// methods for reading and writing application data, so the rest of the app
// never touches SQL or the pool directly.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store backed by the given connection pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}
