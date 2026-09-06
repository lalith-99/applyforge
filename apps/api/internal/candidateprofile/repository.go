// Package candidateprofile materializes a CandidateIntelligenceProfile
// (Phase F): a single AI-synthesized summary of a candidate's target roles,
// skills, and experience, generated once per resume/profile change instead
// of being re-derived from scattered tables on every match/rank request.
package candidateprofile

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// ErrNotFound is returned when a user has no generated profile yet.
var ErrNotFound = errors.New("candidate profile not found")

// TransferableSkill is a skill the candidate doesn't explicitly have but
// plausibly could learn, with supporting evidence.
type TransferableSkill struct {
	Skill    string `json:"skill"`
	Evidence string `json:"evidence"`
	Strength string `json:"strength"`
}

// Profile is one materialized version of a candidate's intelligence profile.
type Profile struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	Version               int32
	TargetRoles           []string
	Seniority             *string
	YearsExperience       *int32
	CoreSkills            []string
	SecondarySkills       []string
	TransferableSkills    []TransferableSkill
	Domains               []string
	ArchitectureStrengths []string
	LeadershipSignals     []string
	ExperienceEvidence    []string
	Summary               string
	SourceContentHash     string
	CreatedAt             time.Time
}

// Repository provides access to candidate_profile_versions.
type Repository struct {
	q *db.Queries
}

// NewRepository builds a Repository from a database pool.
func NewRepository(pool *database.Pool) *Repository {
	return &Repository{q: pool.Queries()}
}

// NewRepositoryFromQueries builds a Repository from an existing sqlc Queries
// value (e.g. bound to a transaction). Primarily for integration tests.
func NewRepositoryFromQueries(q *db.Queries) *Repository {
	return &Repository{q: q}
}

// GetLatest returns the highest-version profile for a user.
func (r *Repository) GetLatest(ctx context.Context, userID uuid.UUID) (Profile, error) {
	row, err := r.q.GetLatestCandidateProfileVersion(ctx, database.UUIDToPG(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Profile{}, ErrNotFound
		}
		return Profile{}, err
	}
	return profileFromRow(row.ID, row.UserID, row.Version, row.TargetRoles, row.Seniority,
		row.YearsExperience, row.CoreSkills, row.SecondarySkills, row.TransferableSkills,
		row.Domains, row.ArchitectureStrengths, row.LeadershipSignals, row.ExperienceEvidence,
		row.Summary, row.SourceContentHash, row.CreatedAt.Time)
}

// GetLatestEmbedding returns the most recent non-null embedding for a user's
// profile (Phase G's candidate-side semantic retrieval input).
func (r *Repository) GetLatestEmbedding(ctx context.Context, userID uuid.UUID) ([]float32, error) {
	vec, err := r.q.GetLatestCandidateProfileEmbedding(ctx, database.UUIDToPG(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return vec.Slice(), nil
}

// Create inserts the next version of a user's profile (version = 1 + the
// current latest, or 1 if none exists yet).
func (r *Repository) Create(ctx context.Context, userID uuid.UUID, p Profile) (Profile, error) {
	nextVersion := int32(1)
	if latest, err := r.GetLatest(ctx, userID); err == nil {
		nextVersion = latest.Version + 1
	} else if !errors.Is(err, ErrNotFound) {
		return Profile{}, err
	}

	transferableJSON, err := json.Marshal(p.TransferableSkills)
	if err != nil {
		return Profile{}, err
	}

	row, err := r.q.CreateCandidateProfileVersion(ctx, db.CreateCandidateProfileVersionParams{
		UserID:                database.UUIDToPG(userID),
		Version:               nextVersion,
		TargetRoles:           emptyIfNil(p.TargetRoles),
		Seniority:             database.PGText(p.Seniority),
		YearsExperience:       database.PGInt4(p.YearsExperience),
		CoreSkills:            emptyIfNil(p.CoreSkills),
		SecondarySkills:       emptyIfNil(p.SecondarySkills),
		TransferableSkills:    transferableJSON,
		Domains:               emptyIfNil(p.Domains),
		ArchitectureStrengths: emptyIfNil(p.ArchitectureStrengths),
		LeadershipSignals:     emptyIfNil(p.LeadershipSignals),
		ExperienceEvidence:    emptyIfNil(p.ExperienceEvidence),
		Summary:               p.Summary,
		SourceContentHash:     p.SourceContentHash,
	})
	if err != nil {
		return Profile{}, err
	}
	return profileFromRow(row.ID, row.UserID, row.Version, row.TargetRoles, row.Seniority,
		row.YearsExperience, row.CoreSkills, row.SecondarySkills, row.TransferableSkills,
		row.Domains, row.ArchitectureStrengths, row.LeadershipSignals, row.ExperienceEvidence,
		row.Summary, row.SourceContentHash, row.CreatedAt.Time)
}

// UpdateEmbedding stores a semantic embedding for a profile version.
func (r *Repository) UpdateEmbedding(ctx context.Context, profileID uuid.UUID, vector []float32, model string) error {
	return r.q.UpdateCandidateProfileEmbedding(ctx, db.UpdateCandidateProfileEmbeddingParams{
		ID:             database.UUIDToPG(profileID),
		Embedding:      pgvector.NewVector(vector),
		EmbeddingModel: database.PGText(&model),
	})
}

// emptyIfNil normalizes a nil slice to empty, since pgx encodes nil as SQL
// NULL rather than an empty array, which would violate the NOT NULL DEFAULT
// '{}' columns here (the DEFAULT only applies when a column is omitted from
// the INSERT entirely, not when NULL is passed explicitly).
func emptyIfNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func profileFromRow(id, userID pgtype.UUID, version int32, targetRoles []string, seniority pgtype.Text,
	yearsExperience pgtype.Int4, coreSkills, secondarySkills []string, transferableJSON []byte,
	domains, architectureStrengths, leadershipSignals, experienceEvidence []string, summary,
	sourceContentHash string, createdAt time.Time,
) (Profile, error) {
	var transferable []TransferableSkill
	if len(transferableJSON) > 0 {
		if err := json.Unmarshal(transferableJSON, &transferable); err != nil {
			return Profile{}, err
		}
	}
	return Profile{
		ID:                    database.PGToUUID(id),
		UserID:                database.PGToUUID(userID),
		Version:               version,
		TargetRoles:           targetRoles,
		Seniority:             database.TextOrNil(seniority),
		YearsExperience:       database.Int4OrNil(yearsExperience),
		CoreSkills:            coreSkills,
		SecondarySkills:       secondarySkills,
		TransferableSkills:    transferable,
		Domains:               domains,
		ArchitectureStrengths: architectureStrengths,
		LeadershipSignals:     leadershipSignals,
		ExperienceEvidence:    experienceEvidence,
		Summary:               summary,
		SourceContentHash:     sourceContentHash,
		CreatedAt:             createdAt,
	}, nil
}
