package aiclient

import (
	"context"
)

// JobRankingInput mirrors app/candidates/ranking_models.py's JobRankingInput.
type JobRankingInput struct {
	JobID                  string   `json:"job_id"`
	Title                  string   `json:"title"`
	CompanyName            string   `json:"company_name"`
	Seniority              *string  `json:"seniority"`
	RemoteType             *string  `json:"remote_type"`
	MatchedSkills          []string `json:"matched_skills"`
	MissingRequiredSkills  []string `json:"missing_required_skills"`
	MissingPreferredSkills []string `json:"missing_preferred_skills"`
	TransferableNotes      []string `json:"transferable_notes"`
	DeterministicScore     int      `json:"deterministic_score"`
}

// RankJobsRequest mirrors app/candidates/ranking_models.py's RankJobsRequest.
type RankJobsRequest struct {
	CandidateSummary string            `json:"candidate_summary"`
	TargetRoles      []string          `json:"target_roles"`
	Jobs             []JobRankingInput `json:"jobs"`
}

// JobRankingResult mirrors app/candidates/ranking_models.py's JobRankingResult.
type JobRankingResult struct {
	JobID                     string   `json:"job_id"`
	FitScore                  int      `json:"fit_score"`
	InterviewProbabilityScore int      `json:"interview_probability_score"`
	CareerAlignment           int      `json:"career_alignment"`
	SkillGapSeverity          string   `json:"skill_gap_severity"`
	StrongEvidence            []string `json:"strong_evidence"`
	Gaps                      []string `json:"gaps"`
	Recommendation            string   `json:"recommendation"`
	Reason                    string   `json:"reason"`
}

// RankJobs requests AI job-fit judgments for a batch of jobs (Phase H).
func (c *Client) RankJobs(ctx context.Context, req RankJobsRequest) ([]JobRankingResult, error) {
	var out struct {
		Result struct {
			Rankings []JobRankingResult `json:"rankings"`
		} `json:"result"`
	}
	err := c.postJSON(ctx, "rank_jobs", "/v1/candidates/rank-jobs", req, &out)
	return out.Result.Rankings, err
}
