package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	reqBody, err := json.Marshal(map[string]string{"title": title, "description": description})
	if err != nil {
		return JobRequirements{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/jobs/parse-requirements", bytes.NewReader(reqBody))
	if err != nil {
		return JobRequirements{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return JobRequirements{}, fmt.Errorf("call ai-worker parse-requirements: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return JobRequirements{}, fmt.Errorf("ai-worker parse-requirements failed: %s: %s", resp.Status, readBody(resp.Body))
	}

	var out struct {
		Requirements JobRequirements `json:"requirements"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return JobRequirements{}, err
	}
	return out.Requirements, nil
}
