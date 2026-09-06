package airank_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/airank"
	"github.com/lalithlochan/applyforge/apps/api/internal/jobs"
	"github.com/lalithlochan/applyforge/apps/api/internal/matching"
)

func TestService_Rank_MergesJudgmentsAndSortsByFitScore(t *testing.T) {
	jobA := uuid.New()
	jobB := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req aiclient.RankJobsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(req.Jobs) != 2 {
			t.Fatalf("expected 2 jobs in batch, got %d", len(req.Jobs))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"rankings": []map[string]any{
					{"job_id": jobA.String(), "fit_score": 40, "recommendation": "SKIP"},
					{"job_id": jobB.String(), "fit_score": 95, "recommendation": "APPLY_NOW"},
				},
			},
		})
	}))
	defer server.Close()

	aiClient := aiclient.New(server.URL)
	svc := airank.NewService(aiClient)

	candidates := []matching.RankedJob{
		{Job: jobStub(jobA, "Backend Engineer A"), Result: matching.Result{TotalScore: 80}},
		{Job: jobStub(jobB, "Backend Engineer B"), Result: matching.Result{TotalScore: 60}},
	}

	ranked, err := svc.Rank(context.Background(), "Senior backend engineer", []string{"Backend Engineer"}, candidates)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if len(ranked) != 2 {
		t.Fatalf("expected 2 ranked jobs, got %d", len(ranked))
	}
	// Job B has the higher AI fit_score (95) despite the lower deterministic
	// score (60) - AI judgment should win the final ordering.
	if ranked[0].Job.ID != jobB {
		t.Fatalf("expected job B ranked first, got %s", ranked[0].Job.ID)
	}
	if !ranked[0].HasJudgment || ranked[0].Judgment.Recommendation != "APPLY_NOW" {
		t.Fatalf("expected job B to have an APPLY_NOW judgment, got %+v", ranked[0])
	}
}

func TestService_Rank_FallsBackToTotalScoreOnAIFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	aiClient := aiclient.New(server.URL)
	svc := airank.NewService(aiClient)

	jobA := uuid.New()
	candidates := []matching.RankedJob{
		{Job: jobStub(jobA, "Backend Engineer"), Result: matching.Result{TotalScore: 77}},
	}

	ranked, err := svc.Rank(context.Background(), "", nil, candidates)
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if len(ranked) != 1 || ranked[0].HasJudgment {
		t.Fatalf("expected 1 job with no judgment (AI call failed), got %+v", ranked)
	}
}

func jobStub(id uuid.UUID, title string) jobs.Job {
	return jobs.Job{ID: id, Title: title, CompanyName: "Acme"}
}
