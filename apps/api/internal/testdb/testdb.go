// Package testdb provides a helper for writing repository integration tests
// against a real Postgres instance. Tests are skipped automatically when
// DATABASE_URL is not set (e.g. a developer machine without Postgres
// running), and always run inside a transaction that is rolled back at the
// end of the test so no data is left behind.
package testdb

import (
	"context"
	"os"
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// OpenTx returns sqlc Queries bound to a transaction that is rolled back
// when the test finishes.
func OpenTx(t *testing.T) *db.Queries {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := database.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not reachable: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	return db.New(tx)
}
