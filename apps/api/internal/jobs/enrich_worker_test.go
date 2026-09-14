package jobs

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestShouldEagerEnrich(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	us := "US"
	ca := "CA"
	recent := now.Add(-2 * time.Hour)
	stale := now.Add(-25 * time.Hour)
	future := now.Add(6 * time.Minute)
	duplicateID := uuid.New()

	tests := []struct {
		name string
		job  Job
		want bool
	}{
		{
			name: "fresh active US software role",
			job:  Job{Title: "Senior Java Backend Engineer", Status: "ACTIVE", CountryCode: &us, PostedAt: &recent},
			want: true,
		},
		{
			name: "unknown country is deferred to retrieval filters",
			job:  Job{Title: "Golang Developer", Status: "ACTIVE", PostedAt: &recent},
			want: true,
		},
		{
			name: "canonical duplicate",
			job:  Job{Title: "Software Engineer", Status: "ACTIVE", CountryCode: &us, PostedAt: &recent, CanonicalJobID: &duplicateID},
			want: false,
		},
		{
			name: "inactive job",
			job:  Job{Title: "Software Engineer", Status: "CLOSED", CountryCode: &us, PostedAt: &recent},
			want: false,
		},
		{
			name: "known non US job",
			job:  Job{Title: "Software Engineer", Status: "ACTIVE", CountryCode: &ca, PostedAt: &recent},
			want: false,
		},
		{
			name: "missing posting date",
			job:  Job{Title: "Software Engineer", Status: "ACTIVE", CountryCode: &us},
			want: false,
		},
		{
			name: "stale posting",
			job:  Job{Title: "Software Engineer", Status: "ACTIVE", CountryCode: &us, PostedAt: &stale},
			want: false,
		},
		{
			name: "implausibly future dated posting",
			job:  Job{Title: "Software Engineer", Status: "ACTIVE", CountryCode: &us, PostedAt: &future},
			want: false,
		},
		{
			name: "cheap title exclusion",
			job:  Job{Title: "QA Automation Test Engineer", Status: "ACTIVE", CountryCode: &us, PostedAt: &recent},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldEagerEnrich(tt.job, now); got != tt.want {
				t.Fatalf("shouldEagerEnrich() = %v, want %v", got, tt.want)
			}
		})
	}
}
