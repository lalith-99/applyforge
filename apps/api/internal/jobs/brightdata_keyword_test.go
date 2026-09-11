package jobs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestBrightDataSource_LinkedInKeywordFetch(t *testing.T) {
	var triggerInputs []brightDataKeywordInput
	var triggerQuery url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing authorization header: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/datasets/v3/trigger":
			triggerQuery = r.URL.Query()
			if err := json.NewDecoder(r.Body).Decode(&triggerInputs); err != nil {
				t.Fatalf("decode trigger inputs: %v", err)
			}
			_, _ = w.Write([]byte(`{"snapshot_id":"snap-keyword"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/datasets/v3/progress/snap-keyword":
			_, _ = w.Write([]byte(`{"status":"ready"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/datasets/v3/snapshot/snap-keyword":
			_, _ = w.Write([]byte(`[
				{
					"job_posting_id":"4462190011",
					"job_title":"Java Developer",
					"company_name":"Acme",
					"job_location":"Dallas, TX, United States",
					"job_summary":"Build Spring Boot services.",
					"job_employment_type":"Full-time",
					"apply_link":"https://jobs.lever.co/acme/123",
					"url":"https://www.linkedin.com/jobs/view/4462190011",
					"country_code":"US",
					"job_posted_date":"2026-09-10T12:00:00Z"
				}
			]`))
		default:
			http.Error(w, "unexpected route", http.StatusNotFound)
		}
	}))
	defer server.Close()

	source := NewBrightDataSource("us-single-user-core-24h", BrightDataConfig{
		APIKey:          "test-key",
		DatasetID:       "gd_lpfll7v5hcqtkxl6l",
		BaseURL:         server.URL,
		Mode:            brightDataModeLinkedInKeyword,
		RecordsLimit:    24,
		SnapshotTimeout: time.Second,
		PollInterval:    time.Millisecond,
		CountryValue:    "US",
	})

	jobs, _, err := source.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(triggerInputs) != 12 {
		t.Fatalf("expected 12 balanced keyword inputs, got %d", len(triggerInputs))
	}
	if got := triggerQuery.Get("dataset_id"); got != "gd_lpfll7v5hcqtkxl6l" {
		t.Fatalf("dataset_id = %q", got)
	}
	if triggerQuery.Get("type") != "discover_new" || triggerQuery.Get("discover_by") != "keyword" {
		t.Fatalf("unexpected trigger query: %v", triggerQuery)
	}
	if got := triggerQuery.Get("limit_per_input"); got != "2" {
		t.Fatalf("limit_per_input = %q, want 2", got)
	}
	for _, input := range triggerInputs {
		if input.Location != "United States" || input.Country != "US" || input.TimeRange != "Past 24 hours" {
			t.Fatalf("unexpected discovery scope: %+v", input)
		}
		if input.JobType != "Full-time" || !input.SelectiveSearch {
			t.Fatalf("unexpected employment/search settings: %+v", input)
		}
	}

	if len(jobs) != 1 {
		t.Fatalf("expected one mapped job, got %d", len(jobs))
	}
	job := jobs[0]
	if job.ExternalID != "4462190011" || job.CompanyName != "Acme" || job.Title != "Java Developer" {
		t.Fatalf("unexpected mapped identity: %+v", job)
	}
	if job.ApplyURL != "https://jobs.lever.co/acme/123" || job.SourceURL != "https://www.linkedin.com/jobs/view/4462190011" {
		t.Fatalf("unexpected mapped URLs: %+v", job)
	}
	if job.EmploymentType != "Full-time" || job.Country != "US" || job.PostedAt == nil {
		t.Fatalf("missing Bright Data scraper metadata: %+v", job)
	}
}

func TestBrightDataLimitPerInput_BoundsSingleUserFreeTier(t *testing.T) {
	inputs := brightDataKeywordInputsForShard("us-single-user-core-24h", "US")
	if len(inputs) != 12 {
		t.Fatalf("core input count = %d, want 12", len(inputs))
	}
	perInput := brightDataLimitPerInput(140, len(inputs))
	if perInput != 11 {
		t.Fatalf("limit per input = %d, want 11", perInput)
	}
	if total := perInput * len(inputs); total > 140 {
		t.Fatalf("daily keyword budget = %d, exceeds configured cap", total)
	}
	if monthly := perInput * len(inputs) * 31; monthly >= 5000 {
		t.Fatalf("31-day requested record budget = %d, should stay below 5000", monthly)
	}
}

func TestBrightDataConfigFromEnv_DefaultsToLinkedInKeyword(t *testing.T) {
	t.Setenv("BRIGHTDATA_API_KEY", "test-key")
	t.Setenv("BRIGHTDATA_MODE", "")
	t.Setenv("BRIGHTDATA_JOBS_DATASET_ID", "")
	t.Setenv("BRIGHTDATA_LINKEDIN_KEYWORD_DATASET_ID", "")
	t.Setenv("BRIGHTDATA_JOBS_RECORDS_LIMIT", strconv.Itoa(140))

	cfg, err := BrightDataConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != brightDataModeLinkedInKeyword {
		t.Fatalf("mode = %q, want %q", cfg.Mode, brightDataModeLinkedInKeyword)
	}
	if cfg.DatasetID != defaultBrightDataLinkedInKeywordDatasetID {
		t.Fatalf("dataset = %q, want %q", cfg.DatasetID, defaultBrightDataLinkedInKeywordDatasetID)
	}
	if cfg.RecordsLimit != 140 {
		t.Fatalf("records limit = %d", cfg.RecordsLimit)
	}
}

func TestBrightDataConfigFromEnv_MarketplaceModeRequiresDataset(t *testing.T) {
	t.Setenv("BRIGHTDATA_API_KEY", "test-key")
	t.Setenv("BRIGHTDATA_MODE", brightDataModeMarketplaceFilter)
	t.Setenv("BRIGHTDATA_JOBS_DATASET_ID", "")

	if _, err := BrightDataConfigFromEnv(); err == nil {
		t.Fatal("expected marketplace mode to require BRIGHTDATA_JOBS_DATASET_ID")
	}
}
