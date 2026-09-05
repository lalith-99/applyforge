// Package users owns the User domain model and its persistence.
package users

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// ErrNotFound is returned when a user does not exist.
var ErrNotFound = errors.New("user not found")

// User is the domain representation of an account.
type User struct {
	ID              uuid.UUID
	Email           string
	PasswordHash    *string
	GoogleID        *string
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func fromRow(row db.User) User {
	return User{
		ID:              database.PGToUUID(row.ID),
		Email:           row.Email,
		PasswordHash:    database.TextOrNil(row.PasswordHash),
		GoogleID:        database.TextOrNil(row.GoogleID),
		EmailVerifiedAt: database.TimeOrNil(row.EmailVerifiedAt),
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}
}

// Repository provides access to user records.
type Repository struct {
	q *db.Queries
}

// NewRepository builds a Repository from a database pool.
func NewRepository(pool *database.Pool) *Repository {
	return newRepository(pool.Queries())
}

// NewRepositoryFromQueries builds a Repository from an existing sqlc
// Queries value (e.g. one bound to a transaction). This is primarily useful
// for other packages' integration tests that need a fixture user; production
// code should use NewRepository.
func NewRepositoryFromQueries(q *db.Queries) *Repository {
	return newRepository(q)
}

func newRepository(q *db.Queries) *Repository {
	return &Repository{q: q}
}

// CreateWithPassword creates a new email/password account.
func (r *Repository) CreateWithPassword(ctx context.Context, email, passwordHash string) (User, error) {
	row, err := r.q.CreateUserWithPassword(ctx, db.CreateUserWithPasswordParams{
		Email:        email,
		PasswordHash: database.PGText(&passwordHash),
	})
	if err != nil {
		return User{}, err
	}
	return fromRow(row), nil
}

// CreateWithGoogle creates a new account authenticated via Google.
func (r *Repository) CreateWithGoogle(ctx context.Context, email, googleID string) (User, error) {
	row, err := r.q.CreateUserWithGoogle(ctx, db.CreateUserWithGoogleParams{
		Email:    email,
		GoogleID: database.PGText(&googleID),
	})
	if err != nil {
		return User{}, err
	}
	return fromRow(row), nil
}

// GetByEmail looks up a user by email (case-insensitive).
func (r *Repository) GetByEmail(ctx context.Context, email string) (User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return fromRow(row), nil
}

// GetByGoogleID looks up a user by their Google account id.
func (r *Repository) GetByGoogleID(ctx context.Context, googleID string) (User, error) {
	row, err := r.q.GetUserByGoogleID(ctx, database.PGText(&googleID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return fromRow(row), nil
}

// GetByID looks up a user by id.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (User, error) {
	row, err := r.q.GetUserByID(ctx, database.UUIDToPG(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return fromRow(row), nil
}

// LinkGoogleAccount attaches a Google id to an existing (email/password) account.
func (r *Repository) LinkGoogleAccount(ctx context.Context, id uuid.UUID, googleID string) (User, error) {
	row, err := r.q.LinkGoogleAccount(ctx, db.LinkGoogleAccountParams{
		ID:       database.UUIDToPG(id),
		GoogleID: database.PGText(&googleID),
	})
	if err != nil {
		return User{}, err
	}
	return fromRow(row), nil
}
