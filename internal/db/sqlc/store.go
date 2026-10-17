package db

import "github.com/jackc/pgx/v5/pgxpool"

// Store provides all functions to execute db queries.
// The API layer depends on this interface, so it can be mocked in tests.
type Store interface {
	Querier
}

// SQLStore is the real implementation backed by a Postgres connection pool.
type SQLStore struct {
	*Queries
	pool *pgxpool.Pool
}

// NewStore creates a new Store.
func NewStore(pool *pgxpool.Pool) Store {
	return &SQLStore{
		Queries: New(pool),
		pool:    pool,
	}
}

// compile-time check: SQLStore must implement Store
var _ Store = (*SQLStore)(nil)
