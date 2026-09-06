package jobrecommendations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/airank"
	"github.com/lalithlochan/applyforge/apps/api/internal/background"
	"github.com/lalithlochan/applyforge/apps/api/internal/candidateprofile"
	"github.com/lalithlochan/applyforge/apps/api/internal/matching"
)

// JobTypeCompute is the background job type enqueued whenever a user's
// candidate profile changes (see candidateprofile.BuildWorker).
const JobTypeCompute = "compute_recommendations"

// ComputePayload is the JSON payload enqueued for a compute_recommendations job.
type ComputePayload struct {
	UserID string `json:"user_id"`
}

// PoolSize is how many candidates the funnel considers before AI reranking
// and the final cut - generous enough that the true top results are almost
// certainly included, small enough to keep AI reranking costs bounded.
const PoolSize = 50

// ComputeWorker runs the full funnel (Phase G's Recommend + Phase H's
// airank.Rank) for a user and materializes the result.
type ComputeWorker struct {
	matchingSvc *matching.Service
	airankSvc   *airank.Service
	profiles    *candidateprofile.Repository
	repo        *Repository
}

// NewComputeWorker builds a ComputeWorker.
func NewComputeWorker(matchingSvc *matching.Service, airankSvc *airank.Service, profiles *candidateprofile.Repository, repo *Repository) *ComputeWorker {
	return &ComputeWorker{matchingSvc: matchingSvc, airankSvc: airankSvc, profiles: profiles, repo: repo}
}

// Handle implements background.Handler for JobTypeCompute.
func (w *ComputeWorker) Handle(ctx context.Context, job background.Job) error {
	var payload ComputePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}

	userID, err := uuid.Parse(payload.UserID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}

	candidates, err := w.matchingSvc.Recommend(ctx, userID, PoolSize)
	if err != nil {
		if errors.Is(err, candidateprofile.ErrNotFound) {
			// No embedded profile yet (e.g. resume never uploaded) - nothing
			// to recommend. Not an error worth retrying.
			return nil
		}
		return fmt.Errorf("recommend: %w", err)
	}
	if len(candidates) == 0 {
		return w.repo.ReplaceForUser(ctx, userID, nil)
	}

	prof, err := w.profiles.GetLatest(ctx, userID)
	if err != nil {
		return fmt.Errorf("load candidate profile: %w", err)
	}

	ranked, err := w.airankSvc.Rank(ctx, prof.Summary, prof.TargetRoles, candidates)
	if err != nil {
		return fmt.Errorf("rank: %w", err)
	}

	version := prof.Version
	recs := make([]Recommendation, 0, len(ranked))
	for _, r := range ranked {
		rec := Recommendation{
			JobID:                   r.Job.ID,
			DeterministicScore:      int32(r.Result.TotalScore),
			FinalScore:              int32(r.Result.TotalScore),
			CandidateProfileVersion: &version,
		}
		if r.HasJudgment {
			fitScore := int32(r.Judgment.FitScore)
			recommendation := r.Judgment.Recommendation
			rec.AIFitScore = &fitScore
			rec.AIRecommendation = &recommendation
			rec.AIReason = r.Judgment.Reason
			rec.FinalScore = fitScore
		}
		recs = append(recs, rec)
	}

	return w.repo.ReplaceForUser(ctx, userID, recs)
}
