package jobrequirements

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
)

// Service resolves JobRequirements for a job, parsing via the AI worker only
// when nothing is cached yet or the job's content has changed since the last
// parse (see MASTER_REQUIREMENTS.md §47: JD parsing once per unique content_hash).
type Service struct {
	repo     *Repository
	aiClient *aiclient.Client
}

// NewService builds a Service.
func NewService(repo *Repository, aiClient *aiclient.Client) *Service {
	return &Service{repo: repo, aiClient: aiClient}
}

// GetOrParse returns cached requirements if they're still fresh for
// contentHash, otherwise parses via the AI worker and caches the result.
func (s *Service) GetOrParse(ctx context.Context, jobID uuid.UUID, title, description, contentHash string) (Requirements, error) {
	cached, err := s.repo.Get(ctx, jobID)
	if err == nil && cached.ContentHash == contentHash {
		return cached, nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Requirements{}, err
	}

	parsed, err := s.aiClient.ParseJobRequirements(ctx, title, description)
	if err != nil {
		return Requirements{}, err
	}

	return s.repo.Upsert(ctx, jobID, contentHash, parsed)
}
