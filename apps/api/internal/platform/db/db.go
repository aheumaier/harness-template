// Package db owns the Postgres connection pool. Domains receive the pool; they never open their own.
package db

import (
	"context"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool connects and pings, so a bad DATABASE_URL fails at startup rather than on first request.
// Every query is a span, a child of the span in its context (the HTTP server span in a request), via the
// global tracer provider. Query parameters are not recorded (they may hold personal data).
func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
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
