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
			Job: jobs.Job{ID: uuid.New(), CompanyID: uuid.New(), PostedAt: &postedAt, FirstSeenAt: postedAt},
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
		rankedJobStub("SKIP", true, 80),
		rankedJobStub("APPLY_NOW", true, 80),
	}

	recs := toRecommendations(ranked, 1)
	if len(recs) != 1 {
		t.Fatalf("expected SKIP job to be excluded, got %d recommendations", len(recs))
	}
}

func TestToRecommendations_BackfillsWhenStrongListIsTooSmall(t *testing.T) {
	ranked := make([]airank.RankedJob, 0, 12)
	for i := 0; i < 2; i++ {
		ranked = append(ranked, rankedJobStub("APPLY_NOW", true, 82))
	}
	for i := 0; i < 10; i++ {
		job := rankedJobStub("CONSIDER", true, 72)
		job.Judgment.FitScore = 68
		job.Judgment.CareerAlignment = 65
		job.Judgment.InterviewProbabilityScore = 65
		ranked = append(ranked, job)
	}

	recs := toRecommendations(ranked, 1)
	if len(recs) != DailyRecommendationTargetMinimum {
		t.Fatalf("expected backfill to reach %d recommendations, got %d", DailyRecommendationTargetMinimum, len(recs))
	}
}

func TestToRecommendations_DoesNotDiluteTenStrongJobs(t *testing.T) {
	ranked := make([]airank.RankedJob, 0, 15)
	for i := 0; i < 10; i++ {
		ranked = append(ranked, rankedJobStub("STRONG_CONSIDER", true, 82))
	}
	for i := 0; i < 5; i++ {
		job := rankedJobStub("CONSIDER", true, 70)
		job.Judgment.FitScore = 62
		job.Judgment.CareerAlignment = 60
		job.Judgment.InterviewProbabilityScore = 60
		ranked = append(ranked, job)
	}

	recs := toRecommendations(ranked, 1)
	if len(recs) != 10 {
		t.Fatalf("expected exactly the 10 strong jobs without dilution, got %d", len(recs))
	}
}

func TestToRecommendations_DoesNotBackfillBelowQualityFloor(t *testing.T) {
	weak := rankedJobStub("CONSIDER", true, 45)
	weak.Judgment.FitScore = 40
	weak.Judgment.CareerAlignment = 40
	weak.Judgment.InterviewProbabilityScore = 40

	recs := toRecommendations([]airank.RankedJob{weak}, 1)
	if len(recs) != 0 {
		t.Fatalf("expected weak job below quality floor to be omitted")
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

func TestToRecommendations_CapsSameCompanyAtTwo(t *testing.T) {
	companyID := uuid.New()
	ranked := make([]airank.RankedJob, 0, 8)
	for i := 0; i < 5; i++ {
		job := rankedJobStub("APPLY_NOW", true, 90-i)
		job.Job.CompanyID = companyID
		ranked = append(ranked, job)
	}
	for i := 0; i < 3; i++ {
		ranked = append(ranked, rankedJobStub("APPLY_NOW", true, 80-i))
	}

	recs := toRecommendations(ranked, 1)
	if len(recs) != 5 {
		t.Fatalf("expected 2 jobs from dominant company plus 3 other employers, got %d", len(recs))
	}

	dominantCount := 0
	jobIDs := map[uuid.UUID]bool{}
	for _, rankedJob := range ranked {
		if rankedJob.Job.CompanyID == companyID {
			jobIDs[rankedJob.Job.ID] = true
		}
	}
	for _, rec := range recs {
		if jobIDs[rec.JobID] {
			dominantCount++
		}
	}
	if dominantCount != DailyRecommendationMaxPerCompany {
		t.Fatalf("expected %d recommendations from dominant company, got %d", DailyRecommendationMaxPerCompany, dominantCount)
	}
}
