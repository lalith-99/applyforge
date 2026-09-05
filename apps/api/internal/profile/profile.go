// Package profile owns the user's personal/career profile used during onboarding.
package profile

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// ErrNotFound is returned when no profile has been created yet for a user.
var ErrNotFound = errors.New("profile not found")

// Profile is the domain representation of a user's personal/career profile.
type Profile struct {
	UserID                      uuid.UUID
	FirstName                   *string
	LastName                    *string
	City                        *string
	State                       *string
	Country                     *string
	PrimaryTargetTitles         []string
	AlternativeTargetTitles     []string
	Seniority                   *string
	YearsExperience             *int32
	PreferredIndustries         []string
	PreferredTechnologies       []string
	DesiredCompensationMin      *int32
	DesiredCompensationMax      *int32
	DesiredCompensationCurrency string
	OnboardingCompletedAt       *time.Time
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

func fromRow(row db.UserProfile) Profile {
	return Profile{
		UserID:                      database.PGToUUID(row.UserID),
		FirstName:                   database.TextOrNil(row.FirstName),
		LastName:                    database.TextOrNil(row.LastName),
		City:                        database.TextOrNil(row.City),
		State:                       database.TextOrNil(row.State),
		Country:                     database.TextOrNil(row.Country),
		PrimaryTargetTitles:         row.PrimaryTargetTitles,
		AlternativeTargetTitles:     row.AlternativeTargetTitles,
		Seniority:                   database.TextOrNil(row.Seniority),
		YearsExperience:             database.Int4OrNil(row.YearsExperience),
		PreferredIndustries:         row.PreferredIndustries,
		PreferredTechnologies:       row.PreferredTechnologies,
		DesiredCompensationMin:      database.Int4OrNil(row.DesiredCompensationMin),
		DesiredCompensationMax:      database.Int4OrNil(row.DesiredCompensationMax),
		DesiredCompensationCurrency: row.DesiredCompensationCurrency,
		OnboardingCompletedAt:       database.TimeOrNil(row.OnboardingCompletedAt),
		CreatedAt:                   row.CreatedAt.Time,
		UpdatedAt:                   row.UpdatedAt.Time,
	}
}

// UpsertInput carries the fields a client may set on their profile.
type UpsertInput struct {
	FirstName                   *string
	LastName                    *string
	City                        *string
	State                       *string
	Country                     *string
	PrimaryTargetTitles         []string
	AlternativeTargetTitles     []string
	Seniority                   *string
	YearsExperience             *int32
	PreferredIndustries         []string
	PreferredTechnologies       []string
	DesiredCompensationMin      *int32
	DesiredCompensationMax      *int32
	DesiredCompensationCurrency string
	MarkOnboardingComplete      bool
}

// Repository provides access to profile records.
type Repository struct {
	q *db.Queries
}

// NewRepository builds a Repository from a database pool.
func NewRepository(pool *database.Pool) *Repository {
	return newRepository(pool.Queries())
}

func newRepository(q *db.Queries) *Repository {
	return &Repository{q: q}
}

// Get returns the profile for a user.
func (r *Repository) Get(ctx context.Context, userID uuid.UUID) (Profile, error) {
	row, err := r.q.GetUserProfile(ctx, database.UUIDToPG(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Profile{}, ErrNotFound
		}
		return Profile{}, err
	}
	return fromRow(row), nil
}

// Upsert creates or updates the profile for a user.
func (r *Repository) Upsert(ctx context.Context, userID uuid.UUID, in UpsertInput) (Profile, error) {
	currency := in.DesiredCompensationCurrency
	if currency == "" {
		currency = "USD"
	}

	var onboardingCompletedAt *time.Time
	if in.MarkOnboardingComplete {
		now := time.Now().UTC()
		onboardingCompletedAt = &now
	}

	row, err := r.q.UpsertUserProfile(ctx, db.UpsertUserProfileParams{
		UserID:                      database.UUIDToPG(userID),
		FirstName:                   database.PGText(in.FirstName),
		LastName:                    database.PGText(in.LastName),
		City:                        database.PGText(in.City),
		State:                       database.PGText(in.State),
		Country:                     database.PGText(in.Country),
		PrimaryTargetTitles:         orEmpty(in.PrimaryTargetTitles),
		AlternativeTargetTitles:     orEmpty(in.AlternativeTargetTitles),
		Seniority:                   database.PGText(in.Seniority),
		YearsExperience:             database.PGInt4(in.YearsExperience),
		PreferredIndustries:         orEmpty(in.PreferredIndustries),
		PreferredTechnologies:       orEmpty(in.PreferredTechnologies),
		DesiredCompensationMin:      database.PGInt4(in.DesiredCompensationMin),
		DesiredCompensationMax:      database.PGInt4(in.DesiredCompensationMax),
		DesiredCompensationCurrency: currency,
		OnboardingCompletedAt:       database.PGTimestamptz(onboardingCompletedAt),
	})
	if err != nil {
		return Profile{}, err
	}
	return fromRow(row), nil
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
