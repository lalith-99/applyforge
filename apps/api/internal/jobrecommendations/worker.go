package jobrecommendations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

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
const PoolSize = 60

const (
	DailyRecommendationLimit          = 20
	DailyRecommendationTargetMinimum  = 10
	MinStrongDailyRecommendationScore = 65
	MinBackfillDailyScore             = 60
)

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
	recs := toRecommendations(ranked, version)
	return w.repo.ReplaceForUser(ctx, userID, recs)
}

// toRecommendations converts the reranked candidate pool into the user's
// daily shortlist. Strong APPLY_NOW / STRONG_CONSIDER opportunities lead the
// list. If fewer than 10 survive, we backfill with the best reasonable
// CONSIDER/fallback jobs rather than returning an unusably tiny list. SKIP is
// always excluded and we never backfill below the minimum quality floor.
func toRecommendations(ranked []airank.RankedJob, version int32) []Recommendation {
	strong := make([]Recommendation, 0, DailyRecommendationLimit)
	backfill := make([]Recommendation, 0, DailyRecommendationLimit)

	for _, r := range ranked {
		if r.HasJudgment && r.Judgment.Recommendation == "SKIP" {
			continue
		}

		freshness := dailyFreshnessScore(r.Job.PostedAt, r.Job.FirstSeenAt)
		finalScore := dailyPriorityScore(r, freshness)
		if finalScore < MinBackfillDailyScore {
			continue
		}

		rec := Recommendation{
			JobID:                    r.Job.ID,
			DeterministicScore:       int32(r.Result.TotalScore),
			FinalScore:               int32(finalScore),
			CandidateProfileVersion:  &version,
			ImmigrationStatus:        r.Result.Eligibility.Immigration.Status,
			ImmigrationConfidence:    r.Result.Eligibility.Immigration.Confidence,
			ImmigrationEvidence:      r.Result.Eligibility.Immigration.Evidence,
			ImmigrationPriorityScore: int32(r.Result.ImmigrationPriorityScore),
		}
		strongCandidate := finalScore >= MinStrongDailyRecommendationScore
		if r.HasJudgment {
			fitScore := int32(r.Judgment.FitScore)
			recommendation := r.Judgment.Recommendation
			rec.AIFitScore = &fitScore
			rec.AIRecommendation = &recommendation
			rec.AIReason = r.Judgment.Reason
			strongCandidate = strongCandidate &&
				(recommendation == "APPLY_NOW" || recommendation == "STRONG_CONSIDER")
		} else {
			strongCandidate = strongCandidate && r.Result.TotalScore >= MinStrongDailyRecommendationScore
		}

		if strongCandidate {
			strong = append(strong, rec)
		} else {
			backfill = append(backfill, rec)
		}
	}

	sortRecommendations(strong)
	sortRecommendations(backfill)

	if len(strong) >= DailyRecommendationLimit {
		return strong[:DailyRecommendationLimit]
	}

	recs := append([]Recommendation{}, strong...)
	// Once we already have 10+ strong jobs, do not dilute the shortlist just
	// to reach 20. If we have fewer than 10, add the highest-quality reasonable
	// alternatives until the list is useful or the backfill pool is exhausted.
	for _, rec := range backfill {
		if len(recs) >= DailyRecommendationTargetMinimum {
			break
		}
		recs = append(recs, rec)
	}
	return recs
}

func sortRecommendations(recs []Recommendation) {
	sort.SliceStable(recs, func(i, j int) bool {
		if recs[i].FinalScore == recs[j].FinalScore {
			return recs[i].DeterministicScore > recs[j].DeterministicScore
		}
		return recs[i].FinalScore > recs[j].FinalScore
	})
}

// dailyPriorityScore is intentionally separate from matching.Score. The
// deterministic Job Match score remains auditable and unchanged; this score
// answers a different product question: "which fresh jobs should I apply to
// first today?"
func dailyPriorityScore(r airank.RankedJob, freshness int) int {
	fit := r.Result.TotalScore
	career := r.Result.TotalScore
	interview := r.Result.TotalScore
	if r.HasJudgment {
		fit = r.Judgment.FitScore
		career = r.Judgment.CareerAlignment
		interview = r.Judgment.InterviewProbabilityScore
	}

	var score float64
	if r.Result.ImmigrationRelevant {
		score =
			0.30*float64(fit) +
				0.20*float64(r.Result.TotalScore) +
				0.15*float64(career) +
				0.10*float64(interview) +
				0.10*float64(freshness) +
				0.15*float64(r.Result.ImmigrationPriorityScore)
	} else {
		score =
			0.35*float64(fit) +
				0.25*float64(r.Result.TotalScore) +
				0.15*float64(career) +
				0.10*float64(interview) +
				0.15*float64(freshness)
	}
	return clampDailyScore(score)
}

func dailyFreshnessScore(postedAt *time.Time, firstSeenAt time.Time) int {
	reference := firstSeenAt
	if postedAt != nil {
		reference = *postedAt
	}
	age := time.Since(reference)
	switch {
	case age <= time.Hour:
		return 100
	case age <= 6*time.Hour:
		return 95
	case age <= 12*time.Hour:
		return 90
	case age <= 24*time.Hour:
		return 80
	default:
		return 0
	}
}

func clampDailyScore(score float64) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return int(score + 0.5)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
