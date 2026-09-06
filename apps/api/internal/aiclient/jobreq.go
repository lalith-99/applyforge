package aiclient

import (
	"context"
)

// SkillRequirement mirrors app/jobs/models.py SkillRequirement.
type SkillRequirement struct {
	NormalizedName string  `json:"normalized_name"`
	OriginalText   string  `json:"original_text"`
	Importance     string  `json:"importance"`
	Category       string  `json:"category"`
	Confidence     float64 `json:"confidence"`
}

// JobRequirements mirrors app/jobs/models.py JobRequirements.
type JobRequirements struct {
	RoleFamily                    *string            `json:"role_family"`
	NormalizedTitle               *string            `json:"normalized_title"`
	Seniority                     *string            `json:"seniority"`
	RequiredSkills                []SkillRequirement `json:"required_skills"`
	PreferredSkills               []SkillRequirement `json:"preferred_skills"`
	RequiredExperienceYears       *int               `json:"required_experience_years"`
	Responsibilities              []string           `json:"responsibilities"`
	Domains                       []string           `json:"domains"`
	EducationRequirements         []string           `json:"education_requirements"`
	Certifications                []string           `json:"certifications"`
	ClearanceRequirements         *string            `json:"clearance_requirements"`
	WorkAuthorizationRequirements *string            `json:"work_authorization_requirements"`
	Keywords                      []string           `json:"keywords"`
}

// ParseJobRequirements extracts structured requirements from a job title/description.
func (c *Client) ParseJobRequirements(ctx context.Context, title, description string) (JobRequirements, error) {
	var out struct {
		Requirements JobRequirements `json:"requirements"`
	}
	err := c.postJSON(ctx, "parse_job_requirements", "/v1/jobs/parse-requirements", map[string]string{
		"title": title, "description": description,
	}, &out)
	return out.Requirements, err
}
