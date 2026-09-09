package candidateprofile

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/background"
)

// JobTypeBuild is the background job type enqueued whenever a candidate's
// resume finishes parsing or their preferences/profile change.
const JobTypeBuild = "build_candidate_profile"

// BuildPayload is the JSON payload enqueued for a build_candidate_profile job.
type BuildPayload struct {
	UserID string `json:"user_id"`
}

// BuildWorker processes build_candidate_profile background jobs: it
// (re)generates the profile, then embeds it for semantic retrieval.
type BuildWorker struct {
	service  *Service
	repo     *Repository
	aiClient *aiclient.Client
	onBuilt  func(ctx context.Context, userID uuid.UUID) // optional; see SetOnBuilt
}

// NewBuildWorker builds a BuildWorker.
func NewBuildWorker(service *Service, repo *Repository, aiClient *aiclient.Client) *BuildWorker {
	return &BuildWorker{service: service, repo: repo, aiClient: aiClient}
}

// SetOnBuilt registers a callback invoked after a profile is successfully
// built and embedded (e.g. to enqueue a job_recommendations recompute - kept
// as a callback, not a direct import, so package candidateprofile doesn't
// need to know about jobrecommendations, which already imports it).
func (w *BuildWorker) SetOnBuilt(fn func(ctx context.Context, userID uuid.UUID)) {
	w.onBuilt = fn
}

// Handle implements background.Handler for JobTypeBuild.
func (w *BuildWorker) Handle(ctx context.Context, job background.Job) error {
	var payload BuildPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}

	userID, err := uuid.Parse(payload.UserID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}

	p, err := w.service.Generate(ctx, userID)
	if err != nil {
		return fmt.Errorf("generate candidate profile: %w", err)
	}

	resp, embedErr := w.aiClient.Embed(ctx, EmbeddingText(p))
	if embedErr == nil {
		if err := w.repo.UpdateEmbedding(ctx, p.ID, resp.Embedding, resp.Model); err != nil {
			slog.Warn("candidate profile embedding could not be stored; continuing with lexical recommendations",
				"user_id", userID, "profile_id", p.ID, "error", err)
		}
	} else {
		// Semantic retrieval is optional. Do not leave a successfully-generated
		// profile stranded just because embeddings are unavailable; Recommend
		// has a lexical fallback and can still produce a useful shortlist.
		slog.Warn("candidate profile embedding failed; continuing with lexical recommendations",
			"user_id", userID, "profile_id", p.ID, "error", embedErr)
	}

	if w.onBuilt != nil {
		w.onBuilt(ctx, userID)
	}
	return nil
}
