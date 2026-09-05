// Package analytics aggregates a user's job-search activity into a
// dashboard: conversion funnel, response rate, and match-score analytics
// (see MASTER_REQUIREMENTS.md §42).
package analytics

import (
	"context"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/applications"
	"github.com/lalithlochan/applyforge/apps/api/internal/database"
	db "github.com/lalithlochan/applyforge/apps/api/internal/database/gen"
)

// funnelOrder is the conversion funnel stage order. SAVED is every tracked
// application (the funnel's baseline); later stages count distinct
// applications that have ever reached that status at least once (from
// application_events), not just applications currently sitting in it.
var funnelOrder = []string{
	applications.StatusSaved,
	applications.StatusApplied,
	applications.StatusRecruiterScreen,
	applications.StatusAssessment,
	applications.StatusTechnicalInterview,
	applications.StatusFinalInterview,
	applications.StatusOffer,
}

// StatusCount is a single (status, count) pair.
type StatusCount struct {
	Status string
	Count  int64
}

// FunnelStage is a single stage of the conversion funnel.
type FunnelStage struct {
	Status string
	Count  int64
}

// Dashboard is the full aggregated analytics response.
type Dashboard struct {
	JobsDiscovered       int64
	TotalApplications    int64
	TailoringRunsCount   int64
	HighMatchesCount     int64
	ApplicationsByStatus []StatusCount
	Funnel               []FunnelStage
	ResponseRatePercent  float64
	AverageMatchScore    *float64
}

// Repository provides read access to the aggregation queries backing the
// analytics dashboard.
type Repository struct {
	q *db.Queries
}

// NewRepository builds a Repository from a database pool.
func NewRepository(pool *database.Pool) *Repository {
	return &Repository{q: pool.Queries()}
}

// NewRepositoryFromQueries builds a Repository directly from generated
// Queries — used by tests that run against a transaction-scoped connection.
func NewRepositoryFromQueries(q *db.Queries) *Repository {
	return &Repository{q: q}
}

func (r *Repository) jobsDiscovered(ctx context.Context) (int64, error) {
	return r.q.CountJobsDiscovered(ctx)
}

func (r *Repository) tailoringRunsForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	return r.q.CountTailoringRunsForUser(ctx, database.UUIDToPG(userID))
}

func (r *Repository) highMatchesForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	return r.q.CountHighMatchesForUser(ctx, database.UUIDToPG(userID))
}

func (r *Repository) applicationsByStatus(ctx context.Context, userID uuid.UUID) ([]StatusCount, error) {
	rows, err := r.q.CountApplicationsByStatusForUser(ctx, database.UUIDToPG(userID))
	if err != nil {
		return nil, err
	}
	out := make([]StatusCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, StatusCount{Status: row.Status, Count: row.Total})
	}
	return out, nil
}

// eventCountsByToStatus returns, for each status, how many distinct
// applications have ever transitioned into it.
func (r *Repository) eventCountsByToStatus(ctx context.Context, userID uuid.UUID) (map[string]int64, error) {
	rows, err := r.q.CountApplicationEventsByToStatusForUser(ctx, database.UUIDToPG(userID))
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		if row.ToStatus.Valid {
			out[row.ToStatus.String] = row.Total
		}
	}
	return out, nil
}
