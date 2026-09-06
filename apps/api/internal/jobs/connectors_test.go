package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// These are contract tests: each connector is checked against a fixture
// response shaped like the real API, via httptest, so CI never depends on
// live external network access.

func TestGreenhouseSource_Fetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jobs":[{"id":123,"title":"Backend Engineer","updated_at":"2026-01-01T00:00:00Z","location":{"name":"Remote"},"absolute_url":"https://boards.greenhouse.io/acme/jobs/123","content":"<p>Build things</p>"}]}`))
	}))
	defer server.Close()

	source := NewGreenhouseSource("acme")
	source.BaseURL = server.URL

	raw, cursor, err := source.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if cursor != nil {
		t.Fatalf("expected nil cursor")
	}
	if len(raw) != 1 {
		t.Fatalf("expected 1 job, got %d", len(raw))
	}
	if raw[0].ExternalID != "123" || raw[0].Title != "Backend Engineer" || raw[0].Description != "Build things" {
		t.Fatalf("unexpected job: %+v", raw[0])
	}
	if raw[0].PostedAt == nil {
		t.Fatalf("expected posted_at to be parsed")
	}
}

func TestLeverSource_Fetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"abc","text":"Backend Engineer","createdAt":1700000000000,"hostedUrl":"https://jobs.lever.co/acme/abc","categories":{"location":"Remote","commitment":"Full-time"},"descriptionPlain":"Build things"}]`))
	}))
	defer server.Close()

	source := NewLeverSource("acme")
	source.BaseURL = server.URL

	raw, _, err := source.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(raw) != 1 {
		t.Fatalf("expected 1 job, got %d", len(raw))
	}
	if raw[0].ExternalID != "abc" || raw[0].EmploymentType != "Full-time" {
		t.Fatalf("unexpected job: %+v", raw[0])
	}
}

func TestLeverSource_Fetch_EmptyBoard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	source := NewLeverSource("empty-board")
	source.BaseURL = server.URL

	raw, _, err := source.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(raw))
	}
}

func TestAshbySource_Fetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jobs":[{"id":"xyz","title":"Backend Engineer","location":"NYC","employmentType":"FullTime","workplaceType":"Remote","publishedAt":"2026-01-01T00:00:00.000+00:00","jobUrl":"https://jobs.ashbyhq.com/acme/xyz","applyUrl":"https://jobs.ashbyhq.com/acme/xyz/apply","descriptionHtml":"<p>Build things</p>"}]}`))
	}))
	defer server.Close()

	source := NewAshbySource("acme")
	source.BaseURL = server.URL

	raw, _, err := source.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(raw) != 1 {
		t.Fatalf("expected 1 job, got %d", len(raw))
	}
	if raw[0].ExternalID != "xyz" || raw[0].RemoteType != "remote" {
		t.Fatalf("unexpected job: %+v", raw[0])
	}
}

func TestConnectors_FetchError_OnNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	gh := NewGreenhouseSource("missing")
	gh.BaseURL = server.URL
	if _, _, err := gh.Fetch(context.Background(), nil); err == nil {
		t.Fatalf("expected error for 404 response")
	}
}

func TestArbeitnowSource_Fetch_PaginatesAcrossCompanies(t *testing.T) {
	var page1URL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write([]byte(`{"data":[{"slug":"job-2","company_name":"Beta Inc","title":"Data Engineer","description":"<p>Crunch data</p>","remote":false,"url":"https://arbeitnow.com/jobs/job-2","location":"Berlin","created_at":1700000000}],"links":{"next":null}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"slug":"job-1","company_name":"Acme GmbH","title":"Backend Engineer","description":"<p>Build things</p>","remote":true,"url":"https://arbeitnow.com/jobs/job-1","location":"Remote","created_at":1700000000}],"links":{"next":"` + page1URL + `?page=2"}}`))
	}))
	defer server.Close()
	page1URL = server.URL

	source := NewArbeitnowSource()
	source.BaseURL = server.URL
	source.MaxPages = 5

	raw, cursor, err := source.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if cursor != nil {
		t.Fatalf("expected nil cursor")
	}
	if len(raw) != 2 {
		t.Fatalf("expected 2 jobs across both pages, got %d", len(raw))
	}
	if raw[0].CompanyName != "Acme GmbH" || raw[0].RemoteType != "remote" || raw[0].Description != "Build things" {
		t.Fatalf("unexpected job 1: %+v", raw[0])
	}
	if raw[1].CompanyName != "Beta Inc" || raw[1].RemoteType != "onsite" {
		t.Fatalf("unexpected job 2: %+v", raw[1])
	}
}

func TestArbeitnowSource_Fetch_StopsAtMaxPages(t *testing.T) {
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"slug":"job","company_name":"Acme","title":"Engineer","description":"","remote":false,"url":"https://x","location":"","created_at":0}],"links":{"next":"http://` + r.Host + `?page=next"}}`))
	}))
	defer server.Close()

	source := NewArbeitnowSource()
	source.BaseURL = server.URL
	source.MaxPages = 2

	if _, _, err := source.Fetch(context.Background(), nil); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected exactly MaxPages=2 requests, got %d", callCount)
	}
}
