package aiclient

import (
	"context"
)

// CandidateExperience mirrors app/candidates/models.py's ExperienceInput.
type CandidateExperience struct {
	Company      string   `json:"company"`
	Title        string   `json:"title"`
	Bullets      []string `json:"bullets"`
	Technologies []string `json:"technologies"`
}

// CandidateProfileRequest mirrors app/candidates/models.py's CandidateProfileRequest.
type CandidateProfileRequest struct {
	TargetRoles           []string              `json:"target_roles"`
	Seniority             *string               `json:"seniority"`
	YearsExperience       *int                  `json:"years_experience"`
	MasterSkills          []string              `json:"master_skills"`
	MasterSummary         *string               `json:"master_summary"`
	Experiences           []CandidateExperience `json:"experiences"`
	PreferredIndustries   []string              `json:"preferred_industries"`
	PreferredTechnologies []string              `json:"preferred_technologies"`
	WorkAuthorization     *string               `json:"work_authorization"`
	ImmigrationStatus     *string               `json:"immigration_status"`
}

// TransferableSkillSignal mirrors app/candidates/models.py's TransferableSkillSignal.
type TransferableSkillSignal struct {
	Skill    string `json:"skill"`
	Evidence string `json:"evidence"`
	Strength string `json:"strength"`
}

// CandidateIntelligenceProfile mirrors app/candidates/models.py's CandidateIntelligenceProfile.
type CandidateIntelligenceProfile struct {
	TargetRoles           []string                  `json:"target_roles"`
	Seniority             *string                   `json:"seniority"`
	YearsExperience       *int                      `json:"years_experience"`
	CoreSkills            []string                  `json:"core_skills"`
	SecondarySkills       []string                  `json:"secondary_skills"`
	TransferableSkills    []TransferableSkillSignal `json:"transferable_skills"`
	Domains               []string                  `json:"domains"`
	ArchitectureStrengths []string                  `json:"architecture_strengths"`
	LeadershipSignals     []string                  `json:"leadership_signals"`
	ExperienceEvidence    []string                  `json:"experience_evidence"`
	Summary               string                    `json:"summary"`
}

// BuildCandidateProfile requests an AI-synthesized CandidateIntelligenceProfile (Phase F).
func (c *Client) BuildCandidateProfile(ctx context.Context, req CandidateProfileRequest) (CandidateIntelligenceProfile, error) {
	var out struct {
		Profile CandidateIntelligenceProfile `json:"profile"`
	}
	err := c.postJSON(ctx, "build_candidate_profile", "/v1/candidates/profile", req, &out)
	return out.Profile, err
}
