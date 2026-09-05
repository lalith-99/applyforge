package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// InterviewQuestion mirrors app/learning/models.py InterviewQuestion.
type InterviewQuestion struct {
	Question          string `json:"question"`
	ConciseAnswer     string `json:"concise_answer"`
	DeeperExplanation string `json:"deeper_explanation"`
}

// QuickPrepModule mirrors app/learning/models.py QuickPrepModule.
type QuickPrepModule struct {
	Skill                 string              `json:"skill"`
	WhatItIs              string              `json:"what_it_is"`
	WhyItMatters          string              `json:"why_it_matters"`
	TransferableFrom      []string            `json:"transferable_from"`
	CoreConcepts          []string            `json:"core_concepts"`
	ScreeningPoints       []string            `json:"screening_points"`
	InterviewQuestions    []InterviewQuestion `json:"interview_questions"`
	CommonMistakes        []string            `json:"common_mistakes"`
	ArchitectureQuestions []string            `json:"architecture_questions"`
	ExampleCode           *string             `json:"example_code"`
}

// GenerateQuickPrep requests a Quick Prep module for a skill.
func (c *Client) GenerateQuickPrep(ctx context.Context, skill string, transferableFrom []string) (QuickPrepModule, error) {
	if transferableFrom == nil {
		transferableFrom = []string{}
	}
	var out QuickPrepModule
	err := c.postJSON(ctx, "/v1/learning/quick-prep", map[string]any{
		"skill":             skill,
		"transferable_from": transferableFrom,
	}, &out)
	return out, err
}

// DefendBulletResponse mirrors app/learning/models.py DefendBulletResponse.
type DefendBulletResponse struct {
	Questions []InterviewQuestion `json:"questions"`
}

// DefendBullet requests likely interview questions for a resume bullet.
func (c *Client) DefendBullet(ctx context.Context, bulletText string, skills []string) (DefendBulletResponse, error) {
	if skills == nil {
		skills = []string{}
	}
	var out DefendBulletResponse
	err := c.postJSON(ctx, "/v1/learning/defend-bullet", map[string]any{
		"bullet_text": bulletText,
		"skills":      skills,
	}, &out)
	return out, err
}

// LearningPlanResult mirrors app/learning/models.py LearningPlanResponse.
type LearningPlanResult struct {
	Skills                  []string            `json:"skills"`
	CurrentReadiness        int                 `json:"current_readiness"`
	TargetReadiness         int                 `json:"target_readiness"`
	Topics                  []string            `json:"topics"`
	PracticeQuestions       []InterviewQuestion `json:"practice_questions"`
	Projects                []string            `json:"projects"`
	ArchitectureQuestions   []string            `json:"architecture_questions"`
	EstimatedEffortCategory string              `json:"estimated_effort_category"`
}

// GenerateLearningPlan aggregates Quick Prep content for a job's missing skills.
func (c *Client) GenerateLearningPlan(ctx context.Context, jobTitle string, missingSkills []string, currentReadiness, targetReadiness int) (LearningPlanResult, error) {
	if missingSkills == nil {
		missingSkills = []string{}
	}
	var out LearningPlanResult
	err := c.postJSON(ctx, "/v1/learning/learning-plan", map[string]any{
		"job_title":         jobTitle,
		"missing_skills":    missingSkills,
		"current_readiness": currentReadiness,
		"target_readiness":  targetReadiness,
	}, &out)
	return out, err
}

// postJSON is a small shared helper for the simple JSON-in/JSON-out learning endpoints.
func (c *Client) postJSON(ctx context.Context, path string, body any, out any) error {
	reqBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call ai-worker %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ai-worker %s failed: %s: %s", path, resp.Status, readBody(resp.Body))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
