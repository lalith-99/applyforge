package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// ErrSessionNotFound is returned when a session token is unknown or expired.
var ErrSessionNotFound = errors.New("session not found")

// SessionTTL controls how long an issued session remains valid.
const SessionTTL = 30 * 24 * time.Hour

// Session is the domain representation of an issued session.
type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
}

type sessionRepository struct {
	q *db.Queries
}

func newSessionRepository(pool *database.Pool) *sessionRepository {
	return &sessionRepository{q: pool.Queries()}
}

func newSessionRepositoryFromQueries(q *db.Queries) *sessionRepository {
	return &sessionRepository{q: q}
}

func (r *sessionRepository) create(ctx context.Context, userID uuid.UUID, tokenHash, userAgent, ip string) (Session, error) {
	row, err := r.q.CreateSession(ctx, db.CreateSessionParams{
		UserID:    database.UUIDToPG(userID),
		TokenHash: tokenHash,
		ExpiresAt: database.PGTimestamptz(ptr(time.Now().UTC().Add(SessionTTL))),
		UserAgent: database.PGText(ptrOrNil(userAgent)),
		IpAddress: database.PGText(ptrOrNil(ip)),
	})
	if err != nil {
		return Session{}, err
	}
	return Session{
		ID:        database.PGToUUID(row.ID),
		UserID:    database.PGToUUID(row.UserID),
		ExpiresAt: row.ExpiresAt.Time,
	}, nil
}

func (r *sessionRepository) getByTokenHash(ctx context.Context, tokenHash string) (Session, error) {
	row, err := r.q.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, err
	}
	_ = r.q.TouchSession(ctx, row.ID)
	return Session{
		ID:        database.PGToUUID(row.ID),
		UserID:    database.PGToUUID(row.UserID),
		ExpiresAt: row.ExpiresAt.Time,
	}, nil
}

func (r *sessionRepository) deleteByTokenHash(ctx context.Context, tokenHash string) error {
	return r.q.DeleteSessionByTokenHash(ctx, tokenHash)
}

func ptr(t time.Time) *time.Time { return &t }

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
