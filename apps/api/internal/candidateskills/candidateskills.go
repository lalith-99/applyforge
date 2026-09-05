// Package candidateskills owns the CandidateSkill domain model: the
// first-class record of what a candidate knows, where it came from, and
// whether the user has approved it for resume inclusion (see
// MASTER_REQUIREMENTS.md §12).
package candidateskills

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// Source values (mirrors the candidate_skills.source check constraint).
const (
	SourceMasterResume     = "MASTER_RESUME"
	SourceUserProfile      = "USER_PROFILE"
	SourceAIRecommendation = "AI_RECOMMENDATION"
	SourceJobTargeting     = "JOB_TARGETING"
	SourceProject          = "PROJECT"
	SourceManualEntry      = "MANUAL_ENTRY"
)

// Status values (mirrors the candidate_skills.status check constraint).
const (
	StatusVerifiedProfessional = "VERIFIED_PROFESSIONAL"
	StatusVerifiedProject      = "VERIFIED_PROJECT"
	StatusFamiliar             = "FAMILIAR"
	StatusLearning             = "LEARNING"
	StatusTargetSkill          = "TARGET_SKILL"
	StatusUserApproved         = "USER_APPROVED"
	StatusUnknown              = "UNKNOWN"
)

// Skill is the domain representation of a candidate skill.
type Skill struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	NormalizedName string
	DisplayName    string
	Category       *string
	Proficiency    *string
	Source         string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func fromRow(row db.CandidateSkill) Skill {
	return Skill{
		ID:             database.PGToUUID(row.ID),
		UserID:         database.PGToUUID(row.UserID),
		NormalizedName: row.NormalizedName,
		DisplayName:    row.DisplayName,
		Category:       database.TextOrNil(row.Category),
		Proficiency:    database.TextOrNil(row.Proficiency),
		Source:         row.Source,
		Status:         row.Status,
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}
}

// Repository provides access to candidate skill records.
type Repository struct {
	q *db.Queries
}

// NewRepository builds a Repository from a database pool.
func NewRepository(pool *database.Pool) *Repository {
	return &Repository{q: pool.Queries()}
}

// Upsert creates or updates a candidate skill for a user, keyed by normalized name.
func (r *Repository) Upsert(ctx context.Context, userID uuid.UUID, normalizedName, displayName string, category, proficiency *string, source, status string) (Skill, error) {
	row, err := r.q.UpsertCandidateSkill(ctx, db.UpsertCandidateSkillParams{
		UserID:         database.UUIDToPG(userID),
		NormalizedName: normalizedName,
		DisplayName:    displayName,
		Category:       database.PGText(category),
		Proficiency:    database.PGText(proficiency),
		Source:         source,
		Status:         status,
	})
	if err != nil {
		return Skill{}, err
	}
	return fromRow(row), nil
}

// ListForUser returns all candidate skills for a user.
func (r *Repository) ListForUser(ctx context.Context, userID uuid.UUID) ([]Skill, error) {
	rows, err := r.q.ListCandidateSkillsForUser(ctx, database.UUIDToPG(userID))
	if err != nil {
		return nil, err
	}
	skills := make([]Skill, 0, len(rows))
	for _, row := range rows {
		skills = append(skills, fromRow(row))
	}
	return skills, nil
}

// UpdateStatus updates the status of an existing candidate skill.
func (r *Repository) UpdateStatus(ctx context.Context, userID uuid.UUID, normalizedName, status string) (Skill, error) {
	row, err := r.q.UpdateCandidateSkillStatus(ctx, db.UpdateCandidateSkillStatusParams{
		UserID:         database.UUIDToPG(userID),
		NormalizedName: normalizedName,
		Status:         status,
	})
	if err != nil {
		return Skill{}, err
	}
	return fromRow(row), nil
}
