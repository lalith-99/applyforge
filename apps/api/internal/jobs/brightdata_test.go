package jobs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBrightDataSource_Fetch(t *testing.T) {
	var triggerCalls, statusCalls, downloadCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing authorization header: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/datasets/filter":
			triggerCalls++
			_, _ = w.Write([]byte(`{"snapshot_id":"snap-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/datasets/snapshots/snap-1":
			statusCalls++
			_, _ = w.Write([]byte(`{"status":"ready"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/datasets/snapshots/snap-1/download":
			downloadCalls++
			_, _ = w.Write([]byte(`[
				{
					"job_id":"job-1",
					"job_title":"Senior Backend Engineer",
					"company_name":"Acme",
					"job_description":"Build distributed services in Go.",
					"location":"Austin, TX, United States",
					"country_code":"US",
					"city":"Austin",
					"state":"TX",
					"employment_type":"Full-time",
					"posted_date":"2026-09-07T12:00:00Z",
					"application_url":"https://example.com/apply/1",
					"job_url":"https://example.com/jobs/1"
				}
			]`))
		default:
			http.Error(w, "unexpected route", http.StatusNotFound)
		}
	}))
	defer server.Close()

	source := NewBrightDataSource("us-software-24h", BrightDataConfig{
		APIKey:          "test-key",
		DatasetID:       "dataset-1",
		BaseURL:         server.URL,
		RecordsLimit:    10000,
		SnapshotTimeout: time.Second,
		PollInterval:    time.Millisecond,
		TitleField:      "job_title",
		PostedDateField: "posted_date",
	})
	source.now = func() time.Time {
		return time.Date(2026, 9, 7, 14, 0, 0, 0, time.UTC)
	}

	jobs, _, err := source.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if triggerCalls != 1 || statusCalls != 1 || downloadCalls != 1 {
		t.Fatalf("unexpected provider calls trigger=%d status=%d download=%d", triggerCalls, statusCalls, downloadCalls)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one mapped job, got %d", len(jobs))
	}
	job := jobs[0]
	if job.ExternalID != "job-1" || job.CompanyName != "Acme" || job.Country != "US" || job.State != "TX" || job.City != "Austin" {
		t.Fatalf("unexpected mapping: %+v", job)
	}
	if job.PostedAt == nil || job.ApplyURL == "" || job.RemoteType != "onsite" {
		t.Fatalf("missing posting metadata: %+v", job)
	}
}

func TestBrightDataRawJob_SkipsRowsWithoutEmployerOrTitle(t *testing.T) {
	for _, record := range []map[string]any{
		{"company_name": "Acme"},
		{"job_title": "Backend Engineer"},
	} {
		if _, ok := brightDataRawJob(record); ok {
			t.Fatalf("expected invalid record to be skipped: %+v", record)
		}
	}
}

func TestBrightDataRawJob_DerivesStableIDAndRemote(t *testing.T) {
	record := map[string]any{
		"job_title":       "Platform Engineer",
		"company_name":    "Example",
		"job_description": "Build platforms",
		"location":        "Remote - United States",
		"posted_date":     "2026-09-07",
		"url":             "https://example.com/jobs/platform",
	}
	first, ok := brightDataRawJob(record)
	if !ok {
		t.Fatal("expected valid record")
	}
	second, _ := brightDataRawJob(record)
	if !strings.HasPrefix(first.ExternalID, "bd-") || first.ExternalID != second.ExternalID {
		t.Fatalf("expected deterministic fallback id: %q %q", first.ExternalID, second.ExternalID)
	}
	if first.RemoteType != "remote" {
		t.Fatalf("expected remote classification: %+v", first)
	}
}

func TestBrightDataTitleFilters_UsesConfiguredShard(t *testing.T) {
	filters := brightDataTitleFilters("job_title", "us-java-24h")
	encoded, err := json.Marshal(filters)
	if err != nil {
		t.Fatalf("marshal filters: %v", err)
	}
	text := string(encoded)
	if !strings.Contains(text, "Java Developer") || !strings.Contains(text, "Spring Boot Developer") {
		t.Fatalf("java shard is missing Java/Spring titles: %s", text)
	}
	if strings.Contains(text, "Golang Developer") {
		t.Fatalf("java shard must not silently use the Go shard: %s", text)
	}
}
