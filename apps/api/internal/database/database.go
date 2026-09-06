// Package database wires up the PostgreSQL connection pool used across the API.
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxvec "github.com/pgvector/pgvector-go/pgx"

	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// Pool wraps a pgx connection pool.
type Pool struct {
	*pgxpool.Pool
}

// New creates a connection pool for the given DSN. It does not block on
// connectivity; callers should use Ping to verify the database is reachable.
func New(ctx context.Context, dsn string) (*Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pgx pool config: %w", err)
	}
	// Registers pgvector's "vector" type per-connection so sqlc-generated
	// code can scan/encode pgvector.Vector directly (see Phase E job
	// embeddings). A no-op for connections that never touch vector columns.
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvec.RegisterTypes(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
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
