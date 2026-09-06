package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/background"
)

// JobTypeEmbed is the background job type enqueued once per newly ingested
// job (see Ingest), producing a semantic embedding for retrieval (Phase E).
const JobTypeEmbed = "embed_job"

// EmbedPayload is the JSON payload enqueued for an embed_job job.
type EmbedPayload struct {
	JobID string `json:"job_id"`
}

// EmbedWorker processes embed_job background jobs: it builds a normalized
// text representation of the job and stores its embedding.
type EmbedWorker struct {
	repo     *Repository
	aiClient *aiclient.Client
}

// NewEmbedWorker builds an EmbedWorker.
func NewEmbedWorker(repo *Repository, aiClient *aiclient.Client) *EmbedWorker {
	return &EmbedWorker{repo: repo, aiClient: aiClient}
}

// Handle implements background.Handler for JobTypeEmbed.
func (w *EmbedWorker) Handle(ctx context.Context, job background.Job) error {
	var payload EmbedPayload
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

	resp, err := w.aiClient.Embed(ctx, embeddingText(j))
	if err != nil {
		return fmt.Errorf("embed job: %w", err)
	}

	return w.repo.UpdateEmbedding(ctx, j.ID, resp.Embedding, resp.Model)
}

// embeddingText builds a normalized representation of a job for embedding -
// title and company carry the most retrieval signal, so they're repeated
// ahead of the (truncated) description rather than relying on the model to
// weight a single long blob evenly.
func embeddingText(j Job) string {
	var b strings.Builder
	b.WriteString(j.Title)
	b.WriteString(" at ")
	b.WriteString(j.CompanyName)
	b.WriteString(".\n\n")

	desc := j.Description
	const maxDescRunes = 4000 // keep embedding input compact and cost-bounded
	if len(desc) > maxDescRunes {
		desc = desc[:maxDescRunes]
	}
	b.WriteString(desc)
	return b.String()
}
