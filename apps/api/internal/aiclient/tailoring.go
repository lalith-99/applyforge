package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// TailoringExperience mirrors app/tailoring/models.py ExperienceInput.
type TailoringExperience struct {
	Company        string   `json:"company"`
	Title          string   `json:"title"`
	Bullets        []string `json:"bullets"`
	DetectedSkills []string `json:"detected_skills"`
}

// TailoringTransferableMatch mirrors app/tailoring/models.py TransferableMatchInput.
type TailoringTransferableMatch struct {
	SourceSkill        string `json:"source_skill"`
	TargetSkill        string `json:"target_skill"`
	Level              string `json:"level"`
	PrepClassification string `json:"prep_classification"`
}

// TailoringRequest mirrors app/tailoring/models.py TailoringRequest.
type TailoringRequest struct {
	Mode                string                       `json:"mode"`
	JobTitle            string                       `json:"job_title"`
	MasterSkills        []string                     `json:"master_skills"`
	MasterSummary       *string                      `json:"master_summary"`
	Experiences         []TailoringExperience        `json:"experiences"`
	RequiredSkills      []string                     `json:"required_skills"`
	PreferredSkills     []string                     `json:"preferred_skills"`
	Responsibilities    []string                     `json:"responsibilities"`
	TransferableMatches []TailoringTransferableMatch `json:"transferable_matches"`
}

// TailoringSuggestion mirrors app/tailoring/models.py TailoringSuggestion.
type TailoringSuggestion struct {
	Section               string   `json:"section"`
	OriginalText          *string  `json:"original_text"`
	SuggestedText         string   `json:"suggested_text"`
	RequirementsAddressed []string `json:"requirements_addressed"`
	SkillsAdded           []string `json:"skills_added"`
	KeywordsAdded         []string `json:"keywords_added"`
	Source                string   `json:"source"`
	Reason                string   `json:"reason"`
	Confidence            float64  `json:"confidence"`
	RiskLevel             string   `json:"risk_level"`
}

// TailoringResponse mirrors app/tailoring/models.py TailoringResponse.
type TailoringResponse struct {
	SummarySuggestion     *TailoringSuggestion  `json:"summary_suggestion"`
	SkillSuggestions      []TailoringSuggestion `json:"skill_suggestions"`
	ExperienceSuggestions []TailoringSuggestion `json:"experience_suggestions"`
	KeywordCoverageBefore float64               `json:"keyword_coverage_before"`
	KeywordCoverageAfter  float64               `json:"keyword_coverage_after"`
}

// SuggestTailoring requests resume tailoring suggestions for a job.
func (c *Client) SuggestTailoring(ctx context.Context, req TailoringRequest) (TailoringResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return TailoringResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/tailoring/suggest", bytes.NewReader(body))
	if err != nil {
		return TailoringResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return TailoringResponse{}, fmt.Errorf("call ai-worker tailoring/suggest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return TailoringResponse{}, fmt.Errorf("ai-worker tailoring/suggest failed: %s: %s", resp.Status, readBody(resp.Body))
	}

	var out TailoringResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return TailoringResponse{}, err
	}
	return out, nil
}
