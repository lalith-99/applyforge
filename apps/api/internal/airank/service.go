// Package airank layers AI-judged relevance on top of matching's
// deterministic scores (Phase H). Kept separate from package matching
// deliberately - matching's own doc comment states scoring is never
// delegated to an LLM; this package is a distinct, later stage, not a
// replacement for it.
package airank

import (
	"context"
	"fmt"
	"sort"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/matching"
)

// batchSize caps how many jobs go into a single AI ranking call, per the
// "batch 10-20 jobs together" guidance - one request judging many jobs at
// once is both cheaper and lets the model compare/prioritize across them,
// versus one request per job.
const batchSize = 20

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
	aiClient *aiclient.Client
}

// NewService builds a Service.
func NewService(aiClient *aiclient.Client) *Service {
	return &Service{aiClient: aiClient}
}

// Rank judges each candidate job's genuine fit via the AI worker, batching
// batchSize jobs per call, and returns them sorted by FitScore descending
// (falling back to Result.TotalScore for any job whose batch failed).
func (s *Service) Rank(ctx context.Context, candidateSummary string, targetRoles []string, candidates []matching.RankedJob) ([]RankedJob, error) {
	ranked := make([]RankedJob, len(candidates))
	for i, c := range candidates {
		ranked[i] = RankedJob{RankedJob: c}
	}

	for start := 0; start < len(ranked); start += batchSize {
		end := start + batchSize
		if end > len(ranked) {
			end = len(ranked)
		}
		batch := ranked[start:end]

		req := aiclient.RankJobsRequest{
			CandidateSummary: candidateSummary,
			TargetRoles:      targetRoles,
			Jobs:             make([]aiclient.JobRankingInput, len(batch)),
		}
		for i, c := range batch {
			req.Jobs[i] = toRankingInput(c.RankedJob)
		}

		results, err := s.aiClient.RankJobs(ctx, req)
		if err != nil {
			// This batch's jobs simply stay HasJudgment=false; the caller
			// sorts by TotalScore for those. A transient AI failure should
			// never make the whole funnel return nothing.
			continue
		}

		byID := make(map[string]aiclient.JobRankingResult, len(results))
		for _, r := range results {
			byID[r.JobID] = r
		}
		for i := range batch {
			r, ok := byID[batch[i].Job.ID.String()]
			if !ok {
				continue
			}
			batch[i].Judgment = Judgment{
				FitScore:                  r.FitScore,
				InterviewProbabilityScore: r.InterviewProbabilityScore,
				CareerAlignment:           r.CareerAlignment,
				SkillGapSeverity:          r.SkillGapSeverity,
				StrongEvidence:            r.StrongEvidence,
				Gaps:                      r.Gaps,
				Recommendation:            r.Recommendation,
				Reason:                    r.Reason,
			}
			batch[i].HasJudgment = true
		}
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		return rankValue(ranked[i]) > rankValue(ranked[j])
	})
	return ranked, nil
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
	}
}

func emptyIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
