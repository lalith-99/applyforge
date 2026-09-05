// Package database wires up the PostgreSQL connection pool used across the API.
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// Pool wraps a pgx connection pool.
type Pool struct {
	*pgxpool.Pool
}

// New creates a connection pool for the given DSN. It does not block on
// connectivity; callers should use Ping to verify the database is reachable.
func New(ctx context.Context, dsn string) (*Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}
	return &Pool{pool}, nil
}

// Queries returns a sqlc-generated Querier bound to this pool.
func (p *Pool) Queries() *db.Queries {
	return db.New(p.Pool)
}

// Ping verifies the database is reachable.
func (p *Pool) Ping(ctx context.Context) error {
	if err := p.Pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}
