package matching

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// Repository persists computed match results for fast re-reads.
type Repository struct {
	q *db.Queries
}

// NewRepository builds a Repository from a database pool.
func NewRepository(pool *database.Pool) *Repository {
	return &Repository{q: pool.Queries()}
}

// ListTransferableFromSkills returns transferable-skill rows whose
// source_skill matches one of the candidate's known (lowercase) skill names.
func (r *Repository) ListTransferableFromSkills(ctx context.Context, candidateSkillKeys []string) ([]TransferableSkill, error) {
	if len(candidateSkillKeys) == 0 {
		return nil, nil
	}
	rows, err := r.q.ListTransferableSkillsFromSources(ctx, candidateSkillKeys)
	if err != nil {
		return nil, err
	}
	out := make([]TransferableSkill, 0, len(rows))
	for _, row := range rows {
		out = append(out, TransferableSkill{
			SourceSkill:          row.SourceSkill,
			TargetSkill:          row.TargetSkill,
			TransferabilityScore: int(row.TransferabilityScore),
			Level:                row.Level,
			PrepClassification:   row.PrepClassification,
		})
	}
	return out, nil
}

// Save upserts a computed Result for a (job, user) pair.
func (r *Repository) Save(ctx context.Context, jobID, userID uuid.UUID, result Result) error {
	componentScoresJSON, _ := json.Marshal(result.Components)
	transferableJSON, _ := json.Marshal(result.TransferableSkills)

	_, err := r.q.UpsertJobMatch(ctx, db.UpsertJobMatchParams{
		JobID:                    database.UUIDToPG(jobID),
		UserID:                   database.UUIDToPG(userID),
		TotalScore:               int32(result.TotalScore),
		Grade:                    result.Grade,
		ComponentScores:          componentScoresJSON,
		MatchedSkills:            orEmpty(result.MatchedSkills),
		TransferableSkills:       transferableJSON,
		MissingRequiredSkills:    orEmpty(result.MissingRequiredSkills),
		MissingPreferredSkills:   orEmpty(result.MissingPreferredSkills),
		PositiveEvidence:         orEmpty(result.PositiveEvidence),
		Concerns:                 orEmpty(result.Concerns),
		Explanation:              result.Explanation,
		OpportunityScore:         int32(result.OpportunityScore),
		CurrentProfileMatch:      int32(result.CurrentProfileMatch),
		TargetProfileMatch:       int32(result.TargetProfileMatch),
		SuggestedTargetAdditions: orEmpty(result.SuggestedTargetAdditions),
		Eligible:                 result.Eligibility.Eligible,
		HardFailures:             orEmpty(result.Eligibility.HardFailures),
		Warnings:                 orEmpty(result.Eligibility.Warnings),
	})
	return err
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
