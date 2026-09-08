package jobs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseWorkdayCareerURL(t *testing.T) {
	cases := []struct {
		raw   string
		token string
	}{
		{
			raw:   "https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite/job/US-CA/Senior-Engineer_JR123",
			token: "nvidia.wd5.myworkdayjobs.com|nvidia|NVIDIAExternalCareerSite",
		},
		{
			raw:   "https://example.wd1.myworkdaysite.com/External/jobs",
			token: "example.wd1.myworkdaysite.com|example|External",
		},
	}

	for _, tc := range cases {
		got, ok := parseWorkdayCareerURL(tc.raw)
		if !ok {
			t.Fatalf("%s: expected Workday discovery", tc.raw)
		}
		if got.BoardToken != tc.token {
			t.Fatalf("%s: expected token %q, got %q", tc.raw, tc.token, got.BoardToken)
		}
		if !got.Monitorable || got.SourceType != "WORKDAY" {
			t.Fatalf("%s: unexpected discovery %+v", tc.raw, got)
		}
	}
}

func TestParseWorkdayCareerURLNeedsSite(t *testing.T) {
	if _, ok := parseWorkdayCareerURL("https://nvidia.wd5.myworkdayjobs.com"); ok {
		t.Fatal("host-only Workday URL must not be promoted without an exact site")
	}
}

func TestWorkdaySourceFetchUsesFullSnapshotAndFreshDetails(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	detailCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wday/cxs/acme/External/jobs":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST list request, got %s", r.Method)
			}
			var request struct {
				Limit  int `json:"limit"`
				Offset int `json:"offset"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode list request: %v", err)
			}
			if request.Limit != 20 || request.Offset != 0 {
				t.Fatalf("unexpected list request %+v", request)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"total": 2,
				"jobPostings": [
					{
						"title": "Senior Backend Engineer",
						"externalPath": "/job/Austin-TX/Senior-Backend-Engineer_R123",
						"locationsText": "Austin, TX",
						"postedOn": "Posted 2 Days Ago",
						"bulletFields": ["R123"]
					},
					{
						"title": "Old Engineer",
						"externalPath": "/job/Austin-TX/Old-Engineer_R100",
						"locationsText": "Austin, TX",
						"postedOn": "Posted 30 Days Ago",
						"bulletFields": ["R100"]
					}
				]
			}`))
		case "/wday/cxs/acme/External/job/Senior-Backend-Engineer_R123":
			detailCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"jobPostingInfo": {
					"title": "Senior Backend Engineer",
					"jobReqId": "R123",
					"jobPostingId": "Senior-Backend-Engineer_R123",
					"jobDescription": "<p>Build distributed Java and Go services.</p>",
					"startDate": "2026-09-06",
					"location": "Austin, Texas, United States of America",
					"timeType": "Full time",
					"remoteType": "Hybrid",
					"jobRequisitionLocation": {
						"country": {
							"alpha2Code": "US",
							"descriptor": "United States of America"
						}
					}
				}
			}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	source, err := NewWorkdaySource("acme.wd1.myworkdayjobs.com|acme|External")
	if err != nil {
		t.Fatalf("NewWorkdaySource: %v", err)
	}
	source.BaseURL = server.URL
	source.now = func() time.Time { return now }
	source.DetailMaxAge = 7 * 24 * time.Hour

	jobs, _, err := source.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if detailCalls != 1 {
		t.Fatalf("expected one fresh detail request, got %d", detailCalls)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one hydrated fresh job, got %d", len(jobs))
	}
	if jobs[0].ExternalID != "R123" || jobs[0].RemoteType != "hybrid" {
		t.Fatalf("unexpected job %+v", jobs[0])
	}
	if jobs[0].Description != "Build distributed Java and Go services." {
		t.Fatalf("unexpected description %q", jobs[0].Description)
	}

	seen := source.SeenExternalIDs()
	if len(seen) != 2 || seen[0] != "R123" || seen[1] != "R100" {
		t.Fatalf("unexpected full snapshot ids %+v", seen)
	}
}

func TestParseWorkdayPostedOn(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	got := parseWorkdayPostedOn("Posted Yesterday", now)
	if got == nil || !got.Equal(now.Add(-24*time.Hour)) {
		t.Fatalf("unexpected yesterday timestamp %v", got)
	}
	got = parseWorkdayPostedOn("Posted 3 Days Ago", now)
	if got == nil || !got.Equal(now.Add(-72*time.Hour)) {
		t.Fatalf("unexpected relative timestamp %v", got)
	}
}
