package httpapi

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
)

// RateLimitStore atomically increments a shared fixed-window bucket.
// Returning allowed=false means the request exceeded the supplied limit.
type RateLimitStore interface {
	Increment(ctx context.Context, scope, key string, limit int, window time.Duration) (allowed bool, err error)
}

type postgresRateLimitStore struct {
	db      *database.Pool
	counter atomic.Uint64
}

// NewPostgresRateLimitStore creates a distributed limiter store shared by all
// API replicas that point at the same Postgres database.
func NewPostgresRateLimitStore(db *database.Pool) RateLimitStore {
	if db == nil {
		return nil
	}
	return &postgresRateLimitStore{db: db}
}

func (s *postgresRateLimitStore) Increment(
	ctx context.Context,
	scope, key string,
	limit int,
	window time.Duration,
) (bool, error) {
	if limit <= 0 || window <= 0 {
		return true, nil
	}

	now := time.Now().UTC()
	bucketStart := now.Truncate(window)
	expiresAt := bucketStart.Add(2 * window)

	var count int
	err := s.db.QueryRow(ctx, `
		INSERT INTO rate_limit_buckets (
			scope, limit_key, bucket_start, count, expires_at
		) VALUES ($1, $2, $3, 1, $4)
		ON CONFLICT (scope, limit_key, bucket_start)
		DO UPDATE SET
			count = rate_limit_buckets.count + 1,
			expires_at = GREATEST(rate_limit_buckets.expires_at, EXCLUDED.expires_at)
		RETURNING count
	`, scope, key, bucketStart, expiresAt).Scan(&count)
	if err != nil {
		return false, err
	}

	// Cleanup is amortized so ordinary requests do not each perform a second
	// database write. Expired rows are never consulted by current buckets.
	if s.counter.Add(1)%1000 == 0 {
		_, _ = s.db.Exec(
			context.WithoutCancel(ctx),
			"DELETE FROM rate_limit_buckets WHERE expires_at < now()",
		)
	}

	return count <= limit, nil
}
