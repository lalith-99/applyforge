// Package jobs owns job ingestion: pluggable JobSource connectors,
// normalization, deduplication, and the canonical Job model (see
// MASTER_REQUIREMENTS.md §13-§16).
package jobs

import (
	"context"
	"time"
)

// Cursor lets a JobSource resume pagination across polls. Sources that don't
// paginate can ignore it.
type Cursor struct {
	Token string
}

// RawJob is the source-specific representation of a job posting before
// normalization.
type RawJob struct {
	ExternalID     string
	Title          string
	CompanyName    string
	Description    string
	LocationText   string
	Country        string
	State          string
	City           string
	RemoteType     string
	EmploymentType string
	ApplyURL       string
	SourceURL      string
	PostedAt       *time.Time
}

// JobSource fetches raw postings from a single external board.
type JobSource interface {
	Name() string
	Fetch(ctx context.Context, cursor *Cursor) ([]RawJob, *Cursor, error)
}
