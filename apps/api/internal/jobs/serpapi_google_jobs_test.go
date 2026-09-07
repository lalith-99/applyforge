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

func TestSerpAPIGoogleJobsSource_FetchesBoundedPagesAndMapsJobs(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/search.json" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("engine") != "google_jobs" || r.URL.Query().Get("gl") != "us" {
			t.Fatalf("missing Google Jobs localization parameters: %s", r.URL.RawQuery)
		}
		if !strings.Contains(r.URL.Query().Get("q"), "Java Developer") ||
			!strings.Contains(r.URL.Query().Get("q"), "Spring Boot Developer") {
			t.Fatalf("java shard query missing expected titles: %q", r.URL.Query().Get("q"))
		}
		if r.URL.Query().Get("api_key") != "test-key" {
			t.Fatal("missing SerpApi key")
		}

		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jobs_results": []map[string]any{{
					"title":        "Java Full Stack Developer",
					"company_name": "Acme",
					"location":     "Austin, TX",
					"description":  "Build Java and Spring Boot services.",
					"job_id":       "job-1",
					"detected_extensions": map[string]any{
						"posted_at":     "3 hours ago",
						"schedule_type": "Full-time",
					},
					"apply_options": []map[string]any{
						{"title": "LinkedIn", "link": "https://linkedin.example/job-1"},
						{"title": "Workday", "link": "https://acme.example/workday/job-1"},
					},
					"share_link": "https://google.example/job-1",
				}},
				"serpapi_pagination": map[string]any{"next_page_token": "next-1"},
			})
			return
		}
		if r.URL.Query().Get("next_page_token") != "next-1" {
			t.Fatalf("expected pagination token, got %q", r.URL.Query().Get("next_page_token"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jobs_results": []map[string]any{{
				"title":        "Spring Boot Engineer",
				"company_name": "Beta",
				"location":     "Anywhere",
				"description":  "Backend engineering.",
				"job_id":       "job-2",
				"detected_extensions": map[string]any{
					"posted_at":      "20 hours ago",
					"schedule_type":  "Full-time",
					"work_from_home": true,
				},
				"apply_options": []map[string]any{{"title": "Indeed", "link": "https://indeed.example/job-2"}},
			}},
		})
	}))
	defer server.Close()

	now := time.Date(2026, 9, 7, 18, 0, 0, 0, time.UTC)
	source := NewSerpAPIGoogleJobsSource("us-java-24h", SerpAPIGoogleJobsConfig{
		APIKey:   "test-key",
		BaseURL:  server.URL + "/search.json",
		MaxPages: 5,
	})
	source.now = func() time.Time { return now }

	found, _, err := source.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if calls != 2 || len(found) != 2 {
		t.Fatalf("expected 2 calls and 2 jobs, calls=%d jobs=%d", calls, len(found))
	}
	if found[0].ApplyURL != "https://acme.example/workday/job-1" {
		t.Fatalf("expected employer ATS apply URL, got %q", found[0].ApplyURL)
	}
	if found[0].PostedAt == nil || now.Sub(*found[0].PostedAt) != 3*time.Hour {
		t.Fatalf("expected 3-hour relative posted time: %+v", found[0].PostedAt)
	}
	if found[1].RemoteType != "remote" || found[1].Country != "United States" {
		t.Fatalf("expected US remote mapping: %+v", found[1])
	}
}

func TestGoogleJobsQueryForShard_IsSpecific(t *testing.T) {
	java := googleJobsQueryForShard("us-java-24h")
	goQuery := googleJobsQueryForShard("us-go-24h")
	if !strings.Contains(java, "Java Developer") || strings.Contains(java, "Golang Developer") {
		t.Fatalf("unexpected Java query: %q", java)
	}
	if !strings.Contains(goQuery, "Golang Developer") {
		t.Fatalf("unexpected Go query: %q", goQuery)
	}
}
