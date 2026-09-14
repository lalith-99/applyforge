package airank_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
		w.Header().Set("X-ApplyForge-AI-Mode", "provider")
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

func TestService_RankCachesUnchangedSemanticInputs(t *testing.T) {
	var calls atomic.Int32
	jobID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-ApplyForge-AI-Mode", "provider")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{"rankings": []map[string]any{{
				"job_id": jobID.String(), "fit_score": 91, "recommendation": "APPLY_NOW",
			}}},
		})
	}))
	defer server.Close()

	store := newFakePolicyStore(true)
	svc := airank.NewService(aiclient.New(server.URL)).WithPolicyStore(
		store,
		"rank-v1:test-model",
		30*24*time.Hour,
		airank.BudgetConfig{DailyUSD: 1, MonthlyUSD: 10, EstimatedBatchUSD: 0.01},
	)
	candidates := []matching.RankedJob{{
		Job: jobStub(jobID, "Backend Engineer"),
		Result: matching.Result{
			TotalScore:         80,
			MatchedSkills:      []string{"Go", "Kafka"},
			TransferableSkills: nil,
		},
	}}

	first, err := svc.Rank(context.Background(), "Senior backend engineer", []string{"Backend"}, candidates)
	if err != nil || len(first) != 1 || !first[0].HasJudgment {
		t.Fatalf("first Rank: ranked=%+v err=%v", first, err)
	}
	second, err := svc.Rank(context.Background(), "Senior backend engineer", []string{"Backend"}, candidates)
	if err != nil || len(second) != 1 || !second[0].HasJudgment {
		t.Fatalf("second Rank: ranked=%+v err=%v", second, err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected one provider call across identical runs, got %d", got)
	}
	if store.reserveCalls != 1 || store.consumed != 1 {
		t.Fatalf("expected one consumed budget reservation, got reserves=%d consumed=%d", store.reserveCalls, store.consumed)
	}
}

func TestService_RankBudgetExhaustionUsesDeterministicFallback(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	store := newFakePolicyStore(false)
	svc := airank.NewService(aiclient.New(server.URL)).WithPolicyStore(
		store, "rank-v1:test-model", time.Hour,
		airank.BudgetConfig{DailyUSD: 1, MonthlyUSD: 10, EstimatedBatchUSD: 0.01},
	)
	ranked, err := svc.Rank(context.Background(), "candidate", nil, []matching.RankedJob{{
		Job: jobStub(uuid.New(), "Backend Engineer"), Result: matching.Result{TotalScore: 77},
	}})
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("provider must not be called after the hard budget gate rejects a batch")
	}
	if len(ranked) != 1 || ranked[0].HasJudgment || ranked[0].Result.TotalScore != 77 {
		t.Fatalf("expected deterministic fallback, got %+v", ranked)
	}
}

func TestService_RankDoesNotCacheOrChargeHeuristicWorkerOutput(t *testing.T) {
	jobID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-ApplyForge-AI-Mode", "heuristic")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{"rankings": []map[string]any{{
				"job_id": jobID.String(), "fit_score": 88, "recommendation": "APPLY_NOW",
			}}},
		})
	}))
	defer server.Close()

	store := newFakePolicyStore(true)
	svc := airank.NewService(aiclient.New(server.URL)).WithPolicyStore(
		store, "rank-v1:test-model", time.Hour,
		airank.BudgetConfig{DailyUSD: 1, MonthlyUSD: 10, EstimatedBatchUSD: 0.01},
	)
	ranked, err := svc.Rank(context.Background(), "candidate", nil, []matching.RankedJob{{
		Job: jobStub(jobID, "Backend Engineer"), Result: matching.Result{TotalScore: 77},
	}})
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if len(ranked) != 1 || ranked[0].HasJudgment {
		t.Fatalf("heuristic worker output must remain deterministic fallback, got %+v", ranked)
	}
	if store.released != 1 || store.consumed != 0 || len(store.cache) != 0 {
		t.Fatalf("heuristic output must release budget and skip cache: %+v", store)
	}
}

type fakePolicyStore struct {
	mu           sync.Mutex
	cache        map[string]airank.Judgment
	allow        bool
	reserveCalls int
	consumed     int
	released     int
}

func newFakePolicyStore(allow bool) *fakePolicyStore {
	return &fakePolicyStore{cache: make(map[string]airank.Judgment), allow: allow}
}

func (s *fakePolicyStore) LoadJudgments(_ context.Context, hashes []string, _ time.Duration) (map[string]airank.Judgment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make(map[string]airank.Judgment)
	for _, hash := range hashes {
		if judgment, ok := s.cache[hash]; ok {
			result[hash] = judgment
		}
	}
	return result, nil
}

func (s *fakePolicyStore) SaveJudgments(_ context.Context, entries []airank.CachedJudgment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range entries {
		s.cache[entry.InputHash] = entry.Judgment
	}
	return nil
}

func (s *fakePolicyStore) ReserveRankingBudget(context.Context, airank.BudgetConfig) (airank.BudgetReservation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reserveCalls++
	if !s.allow {
		return airank.BudgetReservation{}, false, nil
	}
	return airank.BudgetReservation{ID: uuid.New()}, true, nil
}

func (s *fakePolicyStore) FinalizeRankingBudget(_ context.Context, _ airank.BudgetReservation, consumed bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if consumed {
		s.consumed++
	} else {
		s.released++
	}
	return nil
}

func jobStub(id uuid.UUID, title string) jobs.Job {
	return jobs.Job{ID: id, Title: title, CompanyName: "Acme"}
}
