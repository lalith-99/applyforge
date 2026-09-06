// Package jobrecommendations precomputes and stores each user's ranked job
// list (Phase I), so reading recommendations becomes a simple indexed
// SELECT instead of running the full funnel (hard filter -> semantic
// retrieval -> deterministic score -> AI rerank) on every request.
package jobrecommendations

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// Recommendation is one precomputed ranked job for a user.
type Recommendation struct {
	JobID                   uuid.UUID
	DeterministicScore      int32
	AIFitScore              *int32
	AIRecommendation        *string
	AIReason                string
	FinalScore              int32
	CandidateProfileVersion *int32
}

// RecommendationWithJob is a Recommendation joined with the job summary
// fields needed to render it without a second lookup.
type RecommendationWithJob struct {
	Recommendation
	Title          string
	CompanyName    string
	LocationText   *string
	RemoteType     *string
	EmploymentType *string
	ApplyURL       *string
	ComputedAt     time.Time
}

// Repository provides access to job_recommendations.
type Repository struct {
	q *db.Queries
}

// NewRepository builds a Repository from a database pool.
func NewRepository(pool *database.Pool) *Repository {
	return &Repository{q: pool.Queries()}
}

// ReplaceForUser atomically replaces a user's whole recommendation set.
// Delete-then-insert is simple and cheap here: the set is small (N<=~50)
// and always fully recomputed together, never updated piecemeal.
func (r *Repository) ReplaceForUser(ctx context.Context, userID uuid.UUID, recs []Recommendation) error {
	if err := r.q.ReplaceJobRecommendations(ctx, database.UUIDToPG(userID)); err != nil {
		return err
	}
	for _, rec := range recs {
		if err := r.q.InsertJobRecommendation(ctx, db.InsertJobRecommendationParams{
			UserID:                  database.UUIDToPG(userID),
			JobID:                   database.UUIDToPG(rec.JobID),
			DeterministicScore:      rec.DeterministicScore,
			AiFitScore:              database.PGInt4(rec.AIFitScore),
			AiRecommendation:        database.PGText(rec.AIRecommendation),
			AiReason:                rec.AIReason,
			FinalScore:              rec.FinalScore,
			CandidateProfileVersion: database.PGInt4(rec.CandidateProfileVersion),
		}); err != nil {
			return err
		}
	}
	return nil
}

// ListForUser returns a user's precomputed recommendations, highest
// final_score first.
func (r *Repository) ListForUser(ctx context.Context, userID uuid.UUID, limit int32) ([]RecommendationWithJob, error) {
	rows, err := r.q.ListJobRecommendations(ctx, db.ListJobRecommendationsParams{
		UserID: database.UUIDToPG(userID),
		Limit:  limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]RecommendationWithJob, 0, len(rows))
	for _, row := range rows {
		out = append(out, RecommendationWithJob{
			Recommendation: Recommendation{
				JobID:                   database.PGToUUID(row.JobID),
				DeterministicScore:      row.DeterministicScore,
				AIFitScore:              database.Int4OrNil(row.AiFitScore),
				AIRecommendation:        database.TextOrNil(row.AiRecommendation),
				AIReason:                row.AiReason,
				FinalScore:              row.FinalScore,
				CandidateProfileVersion: database.Int4OrNil(row.CandidateProfileVersion),
			},
			Title:          row.Title,
			CompanyName:    row.CompanyName,
			LocationText:   database.TextOrNil(row.LocationText),
			RemoteType:     database.TextOrNil(row.RemoteType),
			EmploymentType: database.TextOrNil(row.EmploymentType),
			ApplyURL:       database.TextOrNil(row.ApplyUrl),
			ComputedAt:     row.ComputedAt.Time,
		})
	}
	return out, nil
}
