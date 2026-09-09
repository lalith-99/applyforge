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
	DailyRecommendationLimit    = 20
	MinDailyRecommendationScore = 65
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
// daily shortlist. We deliberately prefer fewer strong jobs over padding the
// list with weak opportunities: explicit AI SKIPs, weak fit/career/interview
// signals, and low composite scores are dropped. The final list is capped at
// 20 fresh opportunities.
func toRecommendations(ranked []airank.RankedJob, version int32) []Recommendation {
	recs := make([]Recommendation, 0, minInt(len(ranked), DailyRecommendationLimit))
	for _, r := range ranked {
		if r.HasJudgment {
			if r.Judgment.Recommendation == "SKIP" ||
				r.Judgment.FitScore < 60 ||
				r.Judgment.CareerAlignment < 50 ||
				r.Judgment.InterviewProbabilityScore < 50 {
				continue
			}
		} else if r.Result.TotalScore < MinDailyRecommendationScore {
			continue
		}

		freshness := dailyFreshnessScore(r.Job.PostedAt, r.Job.FirstSeenAt)
		finalScore := dailyPriorityScore(r, freshness)
		if finalScore < MinDailyRecommendationScore {
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
		if r.HasJudgment {
			fitScore := int32(r.Judgment.FitScore)
			recommendation := r.Judgment.Recommendation
			rec.AIFitScore = &fitScore
			rec.AIRecommendation = &recommendation
			rec.AIReason = r.Judgment.Reason
		}
		recs = append(recs, rec)
	}

	sort.SliceStable(recs, func(i, j int) bool {
		if recs[i].FinalScore == recs[j].FinalScore {
			return recs[i].DeterministicScore > recs[j].DeterministicScore
		}
		return recs[i].FinalScore > recs[j].FinalScore
	})
	if len(recs) > DailyRecommendationLimit {
		recs = recs[:DailyRecommendationLimit]
	}
	return recs
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
