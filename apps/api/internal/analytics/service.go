package analytics

import (
	"context"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/applications"
)

// Service builds the aggregated analytics Dashboard for a user.
type Service struct {
	repo             *Repository
	applicationsRepo *applications.Repository
}

// NewService builds a Service.
func NewService(repo *Repository, applicationsRepo *applications.Repository) *Service {
	return &Service{repo: repo, applicationsRepo: applicationsRepo}
}

// Dashboard computes the full analytics dashboard for a user.
func (s *Service) Dashboard(ctx context.Context, userID uuid.UUID) (Dashboard, error) {
	jobsDiscovered, err := s.repo.jobsDiscovered(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	tailoringRuns, err := s.repo.tailoringRunsForUser(ctx, userID)
	if err != nil {
		return Dashboard{}, err
	}
	highMatches, err := s.repo.highMatchesForUser(ctx, userID)
	if err != nil {
		return Dashboard{}, err
	}
	byStatus, err := s.repo.applicationsByStatus(ctx, userID)
	if err != nil {
		return Dashboard{}, err
	}
	eventCounts, err := s.repo.eventCountsByToStatus(ctx, userID)
	if err != nil {
		return Dashboard{}, err
	}

	userApplications, err := s.applicationsRepo.ListForUser(ctx, userID)
	if err != nil {
		return Dashboard{}, err
	}

	var totalApplications int64
	for _, sc := range byStatus {
		totalApplications += sc.Count
	}

	funnel := make([]FunnelStage, 0, len(funnelOrder))
	for _, status := range funnelOrder {
		count := eventCounts[status]
		if status == applications.StatusSaved {
			count = totalApplications
		}
		funnel = append(funnel, FunnelStage{Status: status, Count: count})
	}

	appliedCount := funnel[1].Count // funnelOrder[1] == StatusApplied
	var responseRate float64
	if appliedCount > 0 {
		responseRate = float64(eventCounts[applications.StatusRecruiterScreen]) / float64(appliedCount) * 100
	}

	var sum int32
	var scored int
	for _, app := range userApplications {
		if app.MatchScore != nil {
			sum += *app.MatchScore
			scored++
		}
	}
	var avgMatchScore *float64
	if scored > 0 {
		avg := float64(sum) / float64(scored)
		avgMatchScore = &avg
	}

	return Dashboard{
		JobsDiscovered:       jobsDiscovered,
		TotalApplications:    totalApplications,
		TailoringRunsCount:   tailoringRuns,
		HighMatchesCount:     highMatches,
		ApplicationsByStatus: byStatus,
		Funnel:               funnel,
		ResponseRatePercent:  responseRate,
		AverageMatchScore:    avgMatchScore,
	}, nil
}
