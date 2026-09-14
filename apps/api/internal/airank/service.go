// Package airank layers AI-judged relevance on top of matching's
// deterministic scores (Phase H). Kept separate from package matching
// deliberately - matching's own doc comment states scoring is never
// delegated to an LLM; this package is a distinct, later stage, not a
// replacement for it.
package airank

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/aiusage"
	"github.com/lalithlochan/applyforge/apps/api/internal/matching"
)

const (
	// batchSize caps how many jobs go into a single AI ranking call, per the
	// "batch 10-20 jobs together" guidance - one request judging many jobs at
	// once is both cheaper and lets the model compare/prioritize across them,
	// versus one request per job.
	batchSize = 20

	// maxProviderRankCandidates bounds worst-case model spend on a cold cache.
	// Recommend already sends candidates in deterministic/pre-AI priority order,
	// so judging the first 40 preserves the strongest opportunities while the
	// remainder stays available through deterministic fallback. Cached judgments
	// are still used for every candidate and do not consume this provider budget.
	maxProviderRankCandidates = 40
)

// Judgment is the AI's fit assessment for one job.
type Judgment struct {
	FitScore                  int
	InterviewProbabilityScore int
	CareerAlignment           int
	SkillGapSeverity          string
	StrongEvidence            []string
	Gaps                      []string
	Recommendation            string
	Reason                    string
}

// RankedJob pairs a deterministic match.RankedJob with its AI Judgment.
// HasJudgment is false if the AI ranking call failed for this job's batch
// (callers should fall back to sorting by Result.TotalScore in that case).
type RankedJob struct {
	matching.RankedJob
	Judgment    Judgment
	HasJudgment bool
}

// Service batches deterministically-scored jobs to the AI worker for
// relevance judgment.
type Service struct {
	aiClient     *aiclient.Client
	store        PolicyStore
	usage        *aiusage.Repository
	cacheVersion string
	cacheTTL     time.Duration
	budget       BudgetConfig
}

// NewService builds a Service.
func NewService(aiClient *aiclient.Client) *Service {
	return &Service{aiClient: aiClient}
}

// WithPolicyStore enables durable semantic-input caching and a hard ranking
// budget. Leaving it unset preserves deterministic fallback behavior for
// tests and small tools that do not have a database.
func (s *Service) WithPolicyStore(store PolicyStore, cacheVersion string, cacheTTL time.Duration, budget BudgetConfig) *Service {
	s.store = store
	s.cacheVersion = strings.TrimSpace(cacheVersion)
	if s.cacheVersion == "" {
		s.cacheVersion = "v1"
	}
	s.cacheTTL = cacheTTL
	if s.cacheTTL <= 0 {
		s.cacheTTL = 30 * 24 * time.Hour
	}
	s.budget = budget
	return s
}

func (s *Service) WithUsageTracking(usage *aiusage.Repository) *Service {
	s.usage = usage
	return s
}

// Rank judges each candidate job's genuine fit via the AI worker, batching
// batchSize jobs per call, and returns them sorted by FitScore descending
// (falling back to Result.TotalScore for any job whose batch failed).
func (s *Service) Rank(ctx context.Context, candidateSummary string, targetRoles []string, candidates []matching.RankedJob) ([]RankedJob, error) {
	ranked := make([]RankedJob, len(candidates))
	for i, c := range candidates {
		ranked[i] = RankedJob{RankedJob: c}
	}

	inputHashes := make([]string, len(ranked))
	misses := make([]int, 0, len(ranked))
	cacheAvailable := s.store != nil
	if cacheAvailable {
		for i := range ranked {
			inputHashes[i] = rankingInputHash(s.cacheVersion, candidateSummary, targetRoles, ranked[i].RankedJob)
		}
		loadStarted := time.Now()
		cached, err := s.store.LoadJudgments(ctx, inputHashes, s.cacheTTL)
		if err != nil {
			// Fail closed for model spend. If the cache/budget database is not
			// available, deterministic ranking still produces recommendations.
			slog.Error("load AI ranking cache failed; using deterministic ranking", "error", err)
			cacheAvailable = false
		} else {
			cacheHits := 0
			for i, hash := range inputHashes {
				if judgment, ok := cached[hash]; ok {
					ranked[i].Judgment = judgment
					ranked[i].HasJudgment = true
					cacheHits++
					continue
				}
				misses = append(misses, i)
			}
			if cacheHits > 0 && s.usage != nil {
				s.usage.RecordAsync(ctx, aiusage.Entry{
					Operation: "rank_jobs",
					Status:    "SUCCESS",
					LatencyMS: time.Since(loadStarted).Milliseconds(),
					CacheHit:  true,
				})
			}
		}
	}
	if s.store == nil {
		for i := range ranked {
			misses = append(misses, i)
		}
	} else if !cacheAvailable {
		misses = nil
	}

	// Keep provider spend bounded on cold/mostly-cold runs. The caller supplies
	// candidates in descending pre-AI priority, so truncating misses here judges
	// the strongest uncached opportunities first. Every omitted candidate remains
	// in ranked and continues through deterministic fallback; nothing is dropped.
	if len(misses) > maxProviderRankCandidates {
		slog.Info("capping AI ranking cache misses",
			"cache_misses", len(misses),
			"provider_candidates", maxProviderRankCandidates,
			"deterministic_fallback_candidates", len(misses)-maxProviderRankCandidates,
		)
		misses = misses[:maxProviderRankCandidates]
	}

	for start := 0; start < len(misses); start += batchSize {
		end := min(start+batchSize, len(misses))
		batchIndexes := misses[start:end]

		reservation := BudgetReservation{}
		if s.store != nil {
			var allowed bool
			var err error
			reservation, allowed, err = s.store.ReserveRankingBudget(ctx, s.budget)
			if err != nil {
				slog.Error("reserve AI ranking budget failed; using deterministic ranking", "error", err)
				continue
			}
			if !allowed {
				slog.Info("AI ranking budget exhausted; using deterministic ranking")
				continue
			}
		}

		req := aiclient.RankJobsRequest{
			CandidateSummary: candidateSummary,
			TargetRoles:      targetRoles,
			Jobs:             make([]aiclient.JobRankingInput, len(batchIndexes)),
		}
		for i, rankedIndex := range batchIndexes {
			req.Jobs[i] = toRankingInput(ranked[rankedIndex].RankedJob)
		}

		response, err := s.aiClient.RankJobs(ctx, req)
		if err != nil {
			if finalizeErr := s.finalizeBudget(ctx, reservation, false); finalizeErr != nil {
				slog.Error("release failed AI ranking budget reservation", "error", finalizeErr)
			}
			// This batch's jobs simply stay HasJudgment=false; the caller
			// sorts by TotalScore for those. A transient AI failure should
			// never make the whole funnel return nothing.
			continue
		}
		// Only an explicit provider marker is cacheable and chargeable. Missing
		// or unknown provenance fails closed to deterministic ranking.
		providerBacked := strings.EqualFold(response.Mode, "provider")
		if finalizeErr := s.finalizeBudget(ctx, reservation, providerBacked); finalizeErr != nil {
			slog.Error("finalize AI ranking budget reservation failed", "error", finalizeErr)
		}
		if !providerBacked {
			continue
		}

		byID := make(map[string]aiclient.JobRankingResult, len(response.Rankings))
		for _, r := range response.Rankings {
			byID[r.JobID] = r
		}
		cacheEntries := make([]CachedJudgment, 0, len(batchIndexes))
		for _, rankedIndex := range batchIndexes {
			r, ok := byID[ranked[rankedIndex].Job.ID.String()]
			if !ok {
				continue
			}
			judgment := Judgment{
				FitScore:                  r.FitScore,
				InterviewProbabilityScore: r.InterviewProbabilityScore,
				CareerAlignment:           r.CareerAlignment,
				SkillGapSeverity:          r.SkillGapSeverity,
				StrongEvidence:            r.StrongEvidence,
				Gaps:                      r.Gaps,
				Recommendation:            r.Recommendation,
				Reason:                    r.Reason,
			}
			ranked[rankedIndex].Judgment = judgment
			ranked[rankedIndex].HasJudgment = true
			if s.store != nil {
				cacheEntries = append(cacheEntries, CachedJudgment{
					InputHash:    inputHashes[rankedIndex],
					JobID:        ranked[rankedIndex].Job.ID,
					CacheVersion: s.cacheVersion,
					Judgment:     judgment,
				})
			}
		}
		if s.store != nil {
			if err := s.store.SaveJudgments(ctx, cacheEntries); err != nil {
				slog.Error("save AI ranking cache failed", "error", err)
			}
		}
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		return rankValue(ranked[i]) > rankValue(ranked[j])
	})
	return ranked, nil
}

func (s *Service) finalizeBudget(ctx context.Context, reservation BudgetReservation, consumed bool) error {
	if s.store == nil {
		return nil
	}
	return s.store.FinalizeRankingBudget(ctx, reservation, consumed)
}

func rankingInputHash(cacheVersion, candidateSummary string, targetRoles []string, candidate matching.RankedJob) string {
	roles := sortedStrings(targetRoles)
	input := toRankingInput(candidate)
	input.MatchedSkills = sortedStrings(input.MatchedSkills)
	input.MissingRequiredSkills = sortedStrings(input.MissingRequiredSkills)
	input.MissingPreferredSkills = sortedStrings(input.MissingPreferredSkills)
	input.TransferableNotes = sortedStrings(input.TransferableNotes)
	payload, _ := json.Marshal(struct {
		CacheVersion     string                   `json:"cache_version"`
		CandidateSummary string                   `json:"candidate_summary"`
		TargetRoles      []string                 `json:"target_roles"`
		Job              aiclient.JobRankingInput `json:"job"`
	}{
		CacheVersion:     cacheVersion,
		CandidateSummary: strings.TrimSpace(candidateSummary),
		TargetRoles:      roles,
		Job:              input,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func rankValue(r RankedJob) int {
	if r.HasJudgment {
		return r.Judgment.FitScore
	}
	return r.Result.TotalScore
}

func toRankingInput(c matching.RankedJob) aiclient.JobRankingInput {
	notes := make([]string, 0, len(c.Result.TransferableSkills))
	for _, t := range c.Result.TransferableSkills {
		notes = append(notes, fmt.Sprintf("%s -> %s (%s)", t.SourceSkill, t.TargetSkill, t.Level))
	}
	return aiclient.JobRankingInput{
		JobID:                  c.Job.ID.String(),
		Title:                  c.Job.Title,
		CompanyName:            c.Job.CompanyName,
		Seniority:              c.Job.Seniority,
		RemoteType:             c.Job.RemoteType,
		MatchedSkills:          emptyIfNil(c.Result.MatchedSkills),
		MissingRequiredSkills:  emptyIfNil(c.Result.MissingRequiredSkills),
		MissingPreferredSkills: emptyIfNil(c.Result.MissingPreferredSkills),
		TransferableNotes:      emptyIfNil(notes),
		DeterministicScore:     c.Result.TotalScore,
		ImmigrationStatus:      c.Result.Eligibility.Immigration.Status,
		ImmigrationConfidence:  c.Result.Eligibility.Immigration.Confidence,
		ImmigrationEvidence:    c.Result.Eligibility.Immigration.Evidence,
		ImmigrationRelevant:    c.Result.ImmigrationRelevant,
	}
}

func emptyIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
