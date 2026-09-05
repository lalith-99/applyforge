// Package jobrequirements owns JobRequirements: structured, AI-extracted job
// posting requirements, cached per job content_hash so JD parsing only runs
// once per unique job content (see MASTER_REQUIREMENTS.md §17, §47).
package jobrequirements

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// ErrNotFound is returned when no requirements have been parsed for a job yet.
var ErrNotFound = errors.New("job requirements not found")

// Requirements is the domain representation of parsed job requirements.
type Requirements struct {
	JobID                         uuid.UUID
	ContentHash                   string
	RoleFamily                    *string
	NormalizedTitle               *string
	Seniority                     *string
	RequiredSkills                []aiclient.SkillRequirement
	PreferredSkills               []aiclient.SkillRequirement
	RequiredExperienceYears       *int32
	Responsibilities              []string
	Domains                       []string
	EducationRequirements         []string
	Certifications                []string
	ClearanceRequirements         *string
	WorkAuthorizationRequirements *string
	Keywords                      []string
	ParsedAt                      time.Time
}

func fromRow(row db.JobRequirement) Requirements {
	var required, preferred []aiclient.SkillRequirement
	_ = json.Unmarshal(row.RequiredSkills, &required)
	_ = json.Unmarshal(row.PreferredSkills, &preferred)

	return Requirements{
		JobID:                         database.PGToUUID(row.JobID),
		ContentHash:                   row.ContentHash,
		RoleFamily:                    database.TextOrNil(row.RoleFamily),
		NormalizedTitle:               database.TextOrNil(row.NormalizedTitle),
		Seniority:                     database.TextOrNil(row.Seniority),
		RequiredSkills:                required,
		PreferredSkills:               preferred,
		RequiredExperienceYears:       database.Int4OrNil(row.RequiredExperienceYears),
		Responsibilities:              decodeStringList(row.Responsibilities),
		Domains:                       row.Domains,
		EducationRequirements:         row.EducationRequirements,
		Certifications:                row.Certifications,
		ClearanceRequirements:         database.TextOrNil(row.ClearanceRequirements),
		WorkAuthorizationRequirements: database.TextOrNil(row.WorkAuthorizationRequirements),
		Keywords:                      row.Keywords,
		ParsedAt:                      row.ParsedAt.Time,
	}
}

func decodeStringList(raw []byte) []string {
	var out []string
	_ = json.Unmarshal(raw, &out)
	return out
}

// Repository provides access to job_requirements records.
type Repository struct {
	q *db.Queries
}

// NewRepository builds a Repository from a database pool.
func NewRepository(pool *database.Pool) *Repository {
	return &Repository{q: pool.Queries()}
}

// NewRepositoryFromQueries builds a Repository from an existing sqlc Queries
// value (e.g. bound to a transaction). Primarily for other packages'
// integration tests.
func NewRepositoryFromQueries(q *db.Queries) *Repository {
	return &Repository{q: q}
}

// Get returns cached requirements for a job, if present.
func (r *Repository) Get(ctx context.Context, jobID uuid.UUID) (Requirements, error) {
	row, err := r.q.GetJobRequirementsByJobID(ctx, database.UUIDToPG(jobID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Requirements{}, ErrNotFound
		}
		return Requirements{}, err
	}
	return fromRow(row), nil
}

// Upsert stores newly parsed requirements for a job.
func (r *Repository) Upsert(ctx context.Context, jobID uuid.UUID, contentHash string, reqs aiclient.JobRequirements) (Requirements, error) {
	requiredJSON, _ := json.Marshal(reqs.RequiredSkills)
	preferredJSON, _ := json.Marshal(reqs.PreferredSkills)
	responsibilitiesJSON, _ := json.Marshal(orEmptyStrings(reqs.Responsibilities))

	var experienceYears *int32
	if reqs.RequiredExperienceYears != nil {
		v := int32(*reqs.RequiredExperienceYears)
		experienceYears = &v
	}

	row, err := r.q.UpsertJobRequirements(ctx, db.UpsertJobRequirementsParams{
		JobID:                         database.UUIDToPG(jobID),
		ContentHash:                   contentHash,
		RoleFamily:                    database.PGText(reqs.RoleFamily),
		NormalizedTitle:               database.PGText(reqs.NormalizedTitle),
		Seniority:                     database.PGText(reqs.Seniority),
		RequiredSkills:                requiredJSON,
		PreferredSkills:               preferredJSON,
		RequiredExperienceYears:       database.PGInt4(experienceYears),
		Responsibilities:              responsibilitiesJSON,
		Domains:                       orEmptyStrings(reqs.Domains),
		EducationRequirements:         orEmptyStrings(reqs.EducationRequirements),
		Certifications:                orEmptyStrings(reqs.Certifications),
		ClearanceRequirements:         database.PGText(reqs.ClearanceRequirements),
		WorkAuthorizationRequirements: database.PGText(reqs.WorkAuthorizationRequirements),
		Keywords:                      orEmptyStrings(reqs.Keywords),
	})
	if err != nil {
		return Requirements{}, err
	}
	return fromRow(row), nil
}

func orEmptyStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
