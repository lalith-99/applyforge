package aiclient

import (
	"context"
)

// CritiqueSuggestion mirrors app/tailoring/critic_models.py's SuggestionInput.
type CritiqueSuggestion struct {
	Section       string   `json:"section"`
	SuggestedText string   `json:"suggested_text"`
	SkillsAdded   []string `json:"skills_added"`
	Source        string   `json:"source"`
	RiskLevel     string   `json:"risk_level"`
}

// CritiqueRequest mirrors app/tailoring/critic_models.py's CritiqueRequest.
type CritiqueRequest struct {
	JobTitle            string               `json:"job_title"`
	MasterResumeSummary *string              `json:"master_resume_summary"`
	MasterSkills        []string             `json:"master_skills"`
	RequiredSkills      []string             `json:"required_skills"`
	PreferredSkills     []string             `json:"preferred_skills"`
	Suggestions         []CritiqueSuggestion `json:"suggestions"`
}

// CritiqueResult mirrors app/tailoring/critic_models.py's CritiqueResult.
type CritiqueResult struct {
	UnsupportedClaims        []string `json:"unsupported_claims"`
	MissingHighValueKeywords []string `json:"missing_high_value_keywords"`
	WeakBullets              []string `json:"weak_bullets"`
	Repetition               []string `json:"repetition"`
	ATSScore                 int      `json:"ats_score"`
	TechnicalMatchScore      int      `json:"technical_match_score"`
	HumanReadability         int      `json:"human_readability"`
	RecommendRegeneration    bool     `json:"recommend_regeneration"`
	Feedback                 string   `json:"feedback"`
}

// Critique requests an AI review of generated tailoring suggestions (Phase K).
func (c *Client) Critique(ctx context.Context, req CritiqueRequest) (CritiqueResult, error) {
	var out struct {
		Result CritiqueResult `json:"result"`
	}
	err := c.postJSON(ctx, "critique_tailoring", "/v1/tailoring/critique", req, &out)
	return out.Result, err
}
