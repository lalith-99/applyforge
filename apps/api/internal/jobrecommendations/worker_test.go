package jobrecommendations

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/airank"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
	"github.com/lalithlochan/applyforge/apps/api/internal/matching"
)

func rankedJobStub(recommendation string, hasJudgment bool, totalScore int) airank.RankedJob {
	postedAt := time.Now().Add(-2 * time.Hour)
	return airank.RankedJob{
		RankedJob: matching.RankedJob{
			Job: jobs.Job{ID: uuid.New(), PostedAt: &postedAt, FirstSeenAt: postedAt},
			Result: matching.Result{
				TotalScore:       totalScore,
				OpportunityScore: totalScore,
			},
		},
		Judgment: airank.Judgment{
			FitScore:                  90,
			InterviewProbabilityScore: 85,
			CareerAlignment:           90,
			Recommendation:            recommendation,
		},
		HasJudgment: hasJudgment,
	}
}

func TestToRecommendations_DropsAISkippedJobs(t *testing.T) {
	ranked := []airank.RankedJob{
		rankedJobStub("SKIP", true, 75),
		rankedJobStub("APPLY_NOW", true, 80),
	}

	recs := toRecommendations(ranked, 1)

	if len(recs) != 1 {
		t.Fatalf("expected the SKIP-judged job to be dropped, got %d recommendations", len(recs))
	}
	if recs[0].AIRecommendation == nil || *recs[0].AIRecommendation != "APPLY_NOW" {
		t.Fatalf("expected the remaining recommendation to be APPLY_NOW, got %+v", recs[0])
	}
}

func TestToRecommendations_DropsWeakCareerOrInterviewFit(t *testing.T) {
	weakCareer := rankedJobStub("CONSIDER", true, 82)
	weakCareer.Judgment.CareerAlignment = 40
	weakInterview := rankedJobStub("CONSIDER", true, 82)
	weakInterview.Judgment.InterviewProbabilityScore = 45

	recs := toRecommendations([]airank.RankedJob{weakCareer, weakInterview}, 1)
	if len(recs) != 0 {
		t.Fatalf("expected weak career/interview fits to be excluded, got %+v", recs)
	}
}

func TestToRecommendations_KeepsStrongFallbackWithoutAI(t *testing.T) {
	ranked := []airank.RankedJob{
		rankedJobStub("", false, 80),
	}

	recs := toRecommendations(ranked, 1)

	if len(recs) != 1 {
		t.Fatalf("expected a strong deterministic fallback to remain recommendable, got %d", len(recs))
	}
	if recs[0].AIRecommendation != nil {
		t.Fatalf("expected no AI recommendation when the AI call failed, got %+v", recs[0])
	}
}

func TestToRecommendations_DoesNotPadWeakFallback(t *testing.T) {
	ranked := []airank.RankedJob{
		rankedJobStub("", false, 55),
	}

	recs := toRecommendations(ranked, 1)
	if len(recs) != 0 {
		t.Fatalf("expected weak fallback job to be omitted rather than padding the daily list")
	}
}

func TestDailyPriorityScore_ExplicitH1BSupportOutranksUnknown(t *testing.T) {
	explicit := rankedJobStub("APPLY_NOW", true, 80)
	explicit.Result.ImmigrationRelevant = true
	explicit.Result.ImmigrationPriorityScore = 100

	unknown := rankedJobStub("APPLY_NOW", true, 80)
	unknown.Result.ImmigrationRelevant = true
	unknown.Result.ImmigrationPriorityScore = 45

	explicitScore := dailyPriorityScore(explicit, 95)
	unknownScore := dailyPriorityScore(unknown, 95)
	if explicitScore <= unknownScore {
		t.Fatalf("explicit H-1B support must outrank unknown support: explicit=%d unknown=%d", explicitScore, unknownScore)
	}
}

func TestDailyPriorityScore_FresherEquivalentJobRanksHigher(t *testing.T) {
	job := rankedJobStub("STRONG_CONSIDER", true, 80)
	fresh := dailyPriorityScore(job, 100)
	older := dailyPriorityScore(job, 80)
	if fresh <= older {
		t.Fatalf("fresher equivalent job must rank higher: fresh=%d older=%d", fresh, older)
	}
}

func TestToRecommendations_CapsDailyShortlistAt20(t *testing.T) {
	ranked := make([]airank.RankedJob, 0, 30)
	for i := 0; i < 30; i++ {
		ranked = append(ranked, rankedJobStub("APPLY_NOW", true, 85))
	}

	recs := toRecommendations(ranked, 1)
	if len(recs) != DailyRecommendationLimit {
		t.Fatalf("expected daily shortlist cap %d, got %d", DailyRecommendationLimit, len(recs))
	}
}
