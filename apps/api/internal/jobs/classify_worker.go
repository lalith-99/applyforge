package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/background"
)

const JobTypeClassifyRole = "classify_job_role"

type ClassifyRolePayload struct {
	JobID string `json:"job_id"`
}

type ClassifyRoleWorker struct {
	repo     *Repository
	aiClient *aiclient.Client
	queue    *background.Queue
}

func NewClassifyRoleWorker(repo *Repository, aiClient *aiclient.Client, queue *background.Queue) *ClassifyRoleWorker {
	return &ClassifyRoleWorker{repo: repo, aiClient: aiClient, queue: queue}
}

func (w *ClassifyRoleWorker) Handle(ctx context.Context, job background.Job) error {
	var payload ClassifyRolePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	jobID, err := uuid.Parse(payload.JobID)
	if err != nil {
		return fmt.Errorf("invalid job id: %w", err)
	}

	j, err := w.repo.GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("load job: %w", err)
	}
	if strings.TrimSpace(j.Description) == "" {
		return nil
	}

	result, err := w.aiClient.ClassifyJobRole(ctx, j.Title, j.Description)
	if err != nil {
		return fmt.Errorf("classify role: %w", err)
	}
	classification := sanitizeAIClassification(result)
	if err := w.repo.UpdateRoleClassification(ctx, j.ID, classification); err != nil {
		return fmt.Errorf("persist role classification: %w", err)
	}

	if classification.Classification != "IC_SOFTWARE" || w.queue == nil || !isFreshForEagerAI(j.PostedAt, time.Now().UTC()) {
		return nil
	}

	if err := w.queue.Enqueue(ctx, JobTypeEnrich, EnrichPayload{JobID: j.ID.String()}, 3); err != nil {
		return fmt.Errorf("enqueue enrich after role classification: %w", err)
	}
	if err := w.queue.Enqueue(ctx, JobTypeEmbed, EmbedPayload{JobID: j.ID.String()}, 3); err != nil {
		return fmt.Errorf("enqueue embed after role classification: %w", err)
	}
	return nil
}

func sanitizeAIClassification(in aiclient.JobRoleClassification) RoleClassification {
	allowedFamilies := map[string]bool{
		"SOFTWARE_ENGINEERING": true,
		"BACKEND":              true,
		"FRONTEND":             true,
		"FULLSTACK":            true,
		"PLATFORM":             true,
		"INFRASTRUCTURE":       true,
		"SRE":                  true,
		"DEVOPS":               true,
		"CLOUD":                true,
		"DATA_ENGINEERING":     true,
		"ML_ENGINEERING":       true,
		"AI_ENGINEERING":       true,
		"SECURITY_ENGINEERING": true,
		"MOBILE":               true,
		"EMBEDDED":             true,
		"SYSTEMS":              true,
	}
	switch in.Classification {
	case "IC_SOFTWARE":
		if !allowedFamilies[in.Family] {
			return RoleClassification{Family: "UNKNOWN", Classification: "UNKNOWN", Confidence: 0.25}
		}
	case "NON_SOFTWARE":
		return RoleClassification{Family: "EXCLUDED", Classification: "NON_SOFTWARE", Confidence: clampConfidence(in.Confidence)}
	case "UNKNOWN":
		return RoleClassification{Family: "UNKNOWN", Classification: "UNKNOWN", Confidence: clampConfidence(in.Confidence)}
	default:
		return RoleClassification{Family: "UNKNOWN", Classification: "UNKNOWN", Confidence: 0.25}
	}
	return RoleClassification{Family: in.Family, Classification: in.Classification, Confidence: clampConfidence(in.Confidence)}
}

func clampConfidence(value float32) float32 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
