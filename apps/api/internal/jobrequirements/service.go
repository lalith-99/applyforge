package jobrequirements

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/aiusage"
)

// Service resolves JobRequirements for a job, parsing via the AI worker only
// when nothing is cached yet or the job's content has changed since the last
// parse (see MASTER_REQUIREMENTS.md §47: JD parsing once per unique content_hash).
type Service struct {
	repo     *Repository
	aiClient *aiclient.Client
	usage    *aiusage.Repository // optional; nil is fine (e.g. in tests)
}

// NewService builds a Service.
func NewService(repo *Repository, aiClient *aiclient.Client) *Service {
	return &Service{repo: repo, aiClient: aiClient}
}

// WithUsageTracking attaches an aiusage.Repository so cache hits (which
// never reach aiclient, and so aren't tracked by its usage recorder) are
// still recorded. Returns the same Service for chaining at construction.
func (s *Service) WithUsageTracking(usage *aiusage.Repository) *Service {
	s.usage = usage
	return s
}

const requirementsParserCacheVersion = "req-v2"

func versionedRequirementsHash(contentHash string) string {
	return requirementsParserCacheVersion + ":" + contentHash
}

// GetOrParse returns cached requirements if they're still fresh for both the
// job content and the current parser semantics. Alternative/OR semantics are
// recovered from the original description on every read so an older cached
// parse can never collapse "Python or Go" into either zero or two hard
// requirements.
func (s *Service) GetOrParse(ctx context.Context, jobID uuid.UUID, title, description, contentHash string) (Requirements, error) {
	cacheHash := versionedRequirementsHash(contentHash)
	cached, err := s.repo.Get(ctx, jobID)
	if err == nil && cached.ContentHash == cacheHash {
		if s.usage != nil {
			s.usage.RecordAsync(ctx, aiusage.Entry{Operation: "parse_job_requirements", Status: "SUCCESS", CacheHit: true})
		}
		return withAlternativeRequirements(cached, description), nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Requirements{}, err
	}

	parsed, err := s.aiClient.ParseJobRequirements(ctx, title, description)
	if err != nil {
		return Requirements{}, err
	}

	stored, err := s.repo.Upsert(ctx, jobID, cacheHash, parsed)
	if err != nil {
		return Requirements{}, err
	}
	return withAlternativeRequirements(stored, description), nil
}
