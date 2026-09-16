package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWorkdayFetchKeepsHealthyJobsWhenSomeDetailsFail(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wday/cxs/acme/External/jobs":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"total": 3,
				"jobPostings": [
					{
						"title": "Healthy Engineer",
						"externalPath": "/job/Austin-TX/Healthy-Engineer_R100",
						"locationsText": "Austin, TX",
						"postedOn": "Posted Today",
						"bulletFields": ["R100"]
					},
					{
						"title": "Transient Failure",
						"externalPath": "/job/Austin-TX/Transient-Failure_R200",
						"locationsText": "Austin, TX",
						"postedOn": "Posted Today",
						"bulletFields": ["R200"]
					},
					{
						"title": "Malformed Listing",
						"externalPath": "",
						"locationsText": "Austin, TX",
						"postedOn": "Posted Today",
						"bulletFields": ["R300"]
					}
				]
			}`))
		case "/wday/cxs/acme/External/job/Healthy-Engineer_R100":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"jobPostingInfo": {
					"title": "Healthy Engineer",
					"jobDescription": "<p>Build reliable systems.</p>",
					"startDate": "2026-09-16",
					"location": "Austin, Texas, United States of America",
					"timeType": "Full time",
					"jobRequisitionLocation": {
						"country": {"alpha2Code": "US", "descriptor": "United States of America"}
					}
				}
			}`))
		case "/wday/cxs/acme/External/job/Transient-Failure_R200":
			http.Error(w, "temporary upstream failure", http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	source, err := NewWorkdaySource("acme.wd1.myworkdayjobs.com|acme|External")
	if err != nil {
		t.Fatalf("NewWorkdaySource: %v", err)
	}
	source.BaseURL = server.URL
	source.now = func() time.Time { return now }

	jobs, _, err := source.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch should tolerate partial detail failures: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ExternalID != "R100" {
		t.Fatalf("expected only healthy hydrated job, got %+v", jobs)
	}

	seen := source.SeenExternalIDs()
	if len(seen) != 3 || seen[0] != "R100" || seen[1] != "R200" || seen[2] != "R300" {
		t.Fatalf("failed detail rows must remain in full snapshot seen IDs, got %+v", seen)
	}
}

func TestWorkdayFetchFailsWhenEveryFreshDetailFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wday/cxs/acme/External/jobs":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"total": 1,
				"jobPostings": [{
					"title": "Unavailable Engineer",
					"externalPath": "/job/Austin-TX/Unavailable-Engineer_R500",
					"locationsText": "Austin, TX",
					"postedOn": "Posted Today",
					"bulletFields": ["R500"]
				}]
			}`))
		case "/wday/cxs/acme/External/job/Unavailable-Engineer_R500":
			http.Error(w, "temporary upstream failure", http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	source, err := NewWorkdaySource("acme.wd1.myworkdayjobs.com|acme|External")
	if err != nil {
		t.Fatalf("NewWorkdaySource: %v", err)
	}
	source.BaseURL = server.URL
	source.now = func() time.Time { return time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC) }

	_, _, err = source.Fetch(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "detail hydration failed for all 1 fresh postings") {
		t.Fatalf("expected all-detail-failure error, got %v", err)
	}
}
