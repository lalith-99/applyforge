package jobrecommendations

import (
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/airank"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
	"github.com/lalithlochan/applyforge/apps/api/internal/matching"
)

func rankedJobStub(recommendation string, hasJudgment bool, totalScore int) airank.RankedJob {
	return airank.RankedJob{
		RankedJob: matching.RankedJob{
			Job:    jobs.Job{ID: uuid.New()},
			Result: matching.Result{TotalScore: totalScore},
		},
		Judgment:    airank.Judgment{FitScore: 90, Recommendation: recommendation},
		HasJudgment: hasJudgment,
	}
}

func TestToRecommendations_DropsAISkippedJobs(t *testing.T) {
	ranked := []airank.RankedJob{
		rankedJobStub("SKIP", true, 65),
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

func TestToRecommendations_KeepsJobsWithNoJudgment(t *testing.T) {
	ranked := []airank.RankedJob{
		rankedJobStub("", false, 70),
	}

	recs := toRecommendations(ranked, 1)

	if len(recs) != 1 {
		t.Fatalf("expected a job with a failed AI call to still be kept via fallback ordering, got %d", len(recs))
	}
	if recs[0].AIRecommendation != nil {
		t.Fatalf("expected no AI recommendation set when the AI call failed, got %+v", recs[0])
	}
}


func TestBlendImmigrationPriority_ExplicitSupportOutranksUnknown(t *testing.T) {
	explicit := matching.Result{
		ImmigrationRelevant:      true,
		ImmigrationPriorityScore: 100,
	}
	unknown := matching.Result{
		ImmigrationRelevant:      true,
		ImmigrationPriorityScore: 45,
	}

	explicitScore := blendImmigrationPriority(80, explicit)
	unknownScore := blendImmigrationPriority(80, unknown)
	if explicitScore != 84 {
		t.Fatalf("expected explicit support blend 84, got %d", explicitScore)
	}
	if unknownScore != 73 {
		t.Fatalf("expected unknown blend 73, got %d", unknownScore)
	}
	if explicitScore <= unknownScore {
		t.Fatalf("explicit H-1B support must outrank unknown support: explicit=%d unknown=%d", explicitScore, unknownScore)
	}
}

func TestBlendImmigrationPriority_DoesNotAffectUsersWithoutImmigrationConstraint(t *testing.T) {
	result := matching.Result{
		ImmigrationRelevant:      false,
		ImmigrationPriorityScore: 100,
	}
	if got := blendImmigrationPriority(81, result); got != 81 {
		t.Fatalf("expected unchanged technical score for non-immigration user, got %d", got)
	}
}
