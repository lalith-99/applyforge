// Package preferences owns job search preferences and restrictions, including
// the granular immigration-compatibility preferences described in
// MASTER_REQUIREMENTS.md's Immigration-Aware Job Matching section.
package preferences

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// ErrNotFound is returned when no preferences have been set yet for a user.
var ErrNotFound = errors.New("preferences not found")

// Preferences is the domain representation of a user's job search preferences.
type Preferences struct {
	UserID                              uuid.UUID
	Remote                              bool
	Hybrid                              bool
	Onsite                              bool
	PreferredLocations                  []string
	WillingnessToRelocate               bool
	EmploymentTypes                     []string
	MinimumSalary                       *int32
	ExcludedCompanies                   []string
	ExcludedLocations                   []string
	ExcludedIndustries                  []string
	ClearanceConstraints                *string
	WorkAuthorization                   *string
	ImmigrationStatus                   *string
	RequiresH1BTransfer                 bool
	RequiresNewH1BCapSponsorship        bool
	RequiresFutureEmploymentSponsorship bool
	GreenCardSupportPreferred           bool
	GreenCardSupportRequired            bool
	PermSupportPreferred                bool
	ImmigrationSupportMinConfidence     *string
	CreatedAt                           time.Time
	UpdatedAt                           time.Time
}

func fromRow(row db.JobPreference) Preferences {
	return Preferences{
		UserID:                              database.PGToUUID(row.UserID),
		Remote:                              row.Remote,
		Hybrid:                              row.Hybrid,
		Onsite:                              row.Onsite,
		PreferredLocations:                  row.PreferredLocations,
		WillingnessToRelocate:               row.WillingnessToRelocate,
		EmploymentTypes:                     row.EmploymentTypes,
		MinimumSalary:                       database.Int4OrNil(row.MinimumSalary),
		ExcludedCompanies:                   row.ExcludedCompanies,
		ExcludedLocations:                   row.ExcludedLocations,
		ExcludedIndustries:                  row.ExcludedIndustries,
		ClearanceConstraints:                database.TextOrNil(row.ClearanceConstraints),
		WorkAuthorization:                   database.TextOrNil(row.WorkAuthorization),
		ImmigrationStatus:                   database.TextOrNil(row.ImmigrationStatus),
		RequiresH1BTransfer:                 row.RequiresH1bTransfer,
		RequiresNewH1BCapSponsorship:        row.RequiresNewH1bCapSponsorship,
		RequiresFutureEmploymentSponsorship: row.RequiresFutureEmploymentSponsorship,
		GreenCardSupportPreferred:           row.GreenCardSupportPreferred,
		GreenCardSupportRequired:            row.GreenCardSupportRequired,
		PermSupportPreferred:                row.PermSupportPreferred,
		ImmigrationSupportMinConfidence:     database.TextOrNil(row.ImmigrationSupportMinConfidence),
		CreatedAt:                           row.CreatedAt.Time,
		UpdatedAt:                           row.UpdatedAt.Time,
	}
}

// UpsertInput carries the fields a client may set on their job preferences.
type UpsertInput struct {
	Remote                              bool
	Hybrid                              bool
	Onsite                              bool
	PreferredLocations                  []string
	WillingnessToRelocate               bool
	EmploymentTypes                     []string
	MinimumSalary                       *int32
	ExcludedCompanies                   []string
	ExcludedLocations                   []string
	ExcludedIndustries                  []string
	ClearanceConstraints                *string
	WorkAuthorization                   *string
	ImmigrationStatus                   *string
	RequiresH1BTransfer                 bool
	RequiresNewH1BCapSponsorship        bool
	RequiresFutureEmploymentSponsorship bool
	GreenCardSupportPreferred           bool
	GreenCardSupportRequired            bool
	PermSupportPreferred                bool
	ImmigrationSupportMinConfidence     *string
}

// Repository provides access to job preference records.
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

// Get returns the job preferences for a user.
func (r *Repository) Get(ctx context.Context, userID uuid.UUID) (Preferences, error) {
	row, err := r.q.GetJobPreferences(ctx, database.UUIDToPG(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Preferences{}, ErrNotFound
		}
		return Preferences{}, err
	}
	return fromRow(row), nil
}

// Upsert creates or updates the job preferences for a user.
func (r *Repository) Upsert(ctx context.Context, userID uuid.UUID, in UpsertInput) (Preferences, error) {
	row, err := r.q.UpsertJobPreferences(ctx, db.UpsertJobPreferencesParams{
		UserID:                              database.UUIDToPG(userID),
		Remote:                              in.Remote,
		Hybrid:                              in.Hybrid,
		Onsite:                              in.Onsite,
		PreferredLocations:                  orEmpty(in.PreferredLocations),
		WillingnessToRelocate:               in.WillingnessToRelocate,
		EmploymentTypes:                     orEmpty(in.EmploymentTypes),
		MinimumSalary:                       database.PGInt4(in.MinimumSalary),
		ExcludedCompanies:                   orEmpty(in.ExcludedCompanies),
		ExcludedLocations:                   orEmpty(in.ExcludedLocations),
		ExcludedIndustries:                  orEmpty(in.ExcludedIndustries),
		ClearanceConstraints:                database.PGText(in.ClearanceConstraints),
		WorkAuthorization:                   database.PGText(in.WorkAuthorization),
		ImmigrationStatus:                   database.PGText(in.ImmigrationStatus),
		RequiresH1bTransfer:                 in.RequiresH1BTransfer,
		RequiresNewH1bCapSponsorship:        in.RequiresNewH1BCapSponsorship,
		RequiresFutureEmploymentSponsorship: in.RequiresFutureEmploymentSponsorship,
		GreenCardSupportPreferred:           in.GreenCardSupportPreferred,
		GreenCardSupportRequired:            in.GreenCardSupportRequired,
		PermSupportPreferred:                in.PermSupportPreferred,
		ImmigrationSupportMinConfidence:     database.PGText(in.ImmigrationSupportMinConfidence),
	})
	if err != nil {
		return Preferences{}, err
	}
	return fromRow(row), nil
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
