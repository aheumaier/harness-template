// Package db owns the Postgres connection pool. Domains receive the pool; they never open their own.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool connects and pings, so a bad DATABASE_URL fails at startup rather than on first request.
func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}

// Pool is the shared connection pool. cmd/ and services refer to this alias so that only
// platform and store packages import the driver (rules.md#go-db-only-in-store).
// It satisfies the DBTX interface that sqlc generates in each domain's store package.
type Pool = pgxpool.Pool
