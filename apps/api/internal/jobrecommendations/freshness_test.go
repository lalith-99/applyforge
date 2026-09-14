package jobrecommendations

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
	"github.com/lalithlochan/applyforge/apps/api/internal/matching"
)

func TestStrictFreshRecommendationCandidates(t *testing.T) {
	now := time.Date(2026, time.September, 14, 8, 0, 0, 0, time.UTC)
	freshPostedAt := now.Add(-23*time.Hour - 59*time.Minute)
	stalePostedAt := now.Add(-24*time.Hour - time.Second)
	futureWithinSkew := now.Add(4 * time.Minute)
	futureBeyondSkew := now.Add(6 * time.Minute)

	makeCandidate := func(id string, postedAt *time.Time) matching.RankedJob {
		return matching.RankedJob{Job: jobs.Job{ID: uuid.MustParse(id), PostedAt: postedAt}}
	}

	candidates := []matching.RankedJob{
		makeCandidate("00000000-0000-0000-0000-000000000001", &freshPostedAt),
		makeCandidate("00000000-0000-0000-0000-000000000002", nil),
		makeCandidate("00000000-0000-0000-0000-000000000003", &stalePostedAt),
		makeCandidate("00000000-0000-0000-0000-000000000004", &futureWithinSkew),
		makeCandidate("00000000-0000-0000-0000-000000000005", &futureBeyondSkew),
	}

	got := strictFreshRecommendationCandidates(candidates, now)
	if len(got) != 2 {
		t.Fatalf("expected 2 fresh candidates, got %d", len(got))
	}
	if got[0].Job.ID != candidates[0].Job.ID {
		t.Fatalf("expected recent posted job to survive")
	}
	if got[1].Job.ID != candidates[3].Job.ID {
		t.Fatalf("expected small provider clock skew to survive")
	}
}

func TestStrictFreshRecommendationCandidatesBoundary(t *testing.T) {
	now := time.Date(2026, time.September, 14, 8, 0, 0, 0, time.UTC)
	boundary := now.Add(-24 * time.Hour)
	candidate := matching.RankedJob{Job: jobs.Job{
		ID:       uuid.MustParse("00000000-0000-0000-0000-000000000010"),
		PostedAt: &boundary,
	}}

	got := strictFreshRecommendationCandidates([]matching.RankedJob{candidate}, now)
	if len(got) != 1 {
		t.Fatalf("expected exact 24-hour boundary to remain eligible")
	}
}
