package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestICIMSSource_Fetch_PaginatesAndEnrichesUSSoftware(t *testing.T) {
	var searchCalls, detailCalls int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if strings.Contains(r.URL.Path, "/jobs/123/") {
			detailCalls++
			_, _ = w.Write([]byte(icimsDetailFixture("Acme Corporation", "123", "Backend Engineer")))
			return
		}
		if r.URL.Path == "/jobs/search" {
			searchCalls++
			if r.URL.Query().Get("pr") == "0" {
				_, _ = w.Write([]byte(icimsListingFixture(server.URL, "123", "Backend Engineer", "US-CA-San Francisco")))
				return
			}
			_, _ = w.Write([]byte(`<html><body><ul></ul></body></html>`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	source := &ICIMSSource{
		BoardToken:       "careers-acme.icims.com",
		BaseURL:          server.URL,
		MaxPages:         5,
		MaxDetailFetches: 10,
		http:             server.Client(),
		maxBodyBytes:     defaultICIMSMaxBodyBytes,
	}

	raw, cursor, err := source.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if cursor != nil {
		t.Fatalf("expected nil cursor")
	}
	if searchCalls != 2 {
		t.Fatalf("expected two listing calls, got %d", searchCalls)
	}
	if detailCalls != 1 {
		t.Fatalf("expected one detail call for US software role, got %d", detailCalls)
	}
	if len(raw) != 1 {
		t.Fatalf("expected one job, got %d", len(raw))
	}
	job := raw[0]
	if job.ExternalID != "careers-acme.icims.com:123" || job.Title != "Backend Engineer" {
		t.Fatalf("unexpected job identity: %+v", job)
	}
	if job.CompanyName != "Acme Corporation" {
		t.Fatalf("expected hiring organization from JSON-LD, got %q", job.CompanyName)
	}
	if job.Country != "US" || job.State != "CA" || job.City != "San Francisco" {
		t.Fatalf("unexpected normalized location: %+v", job)
	}
	if job.Description != "Build distributed systems." || job.EmploymentType != "FULL_TIME" || job.PostedAt == nil {
		t.Fatalf("expected enriched detail fields: %+v", job)
	}
	seen := source.SeenExternalIDs()
	if len(seen) != 1 || seen[0] != "careers-acme.icims.com:123" {
		t.Fatalf("unexpected complete snapshot ids: %#v", seen)
	}
}

func TestICIMSSource_Fetch_DoesNotDetailFetchIrrelevantJobs(t *testing.T) {
	var detailCalls int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if strings.Contains(r.URL.Path, "/jobs/") && r.URL.Path != "/jobs/search" {
			detailCalls++
			t.Fatalf("non-US/non-software listing must not fetch detail")
		}
		if r.URL.Path == "/jobs/search" && r.URL.Query().Get("pr") == "0" {
			_, _ = w.Write([]byte(icimsListingFixture(server.URL, "9", "Accountant", "GB-London-London")))
			return
		}
		_, _ = w.Write([]byte(`<html></html>`))
	}))
	defer server.Close()

	source := &ICIMSSource{
		BoardToken:       "careers-acme.icims.com",
		BaseURL:          server.URL,
		MaxPages:         5,
		MaxDetailFetches: 10,
		http:             server.Client(),
		maxBodyBytes:     defaultICIMSMaxBodyBytes,
	}
	jobs, _, err := source.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(jobs) != 1 || detailCalls != 0 {
		t.Fatalf("unexpected jobs/detail calls jobs=%d details=%d", len(jobs), detailCalls)
	}
}

func TestICIMSSource_Fetch_RefusesPartialSnapshotAtPageBound(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		id := r.URL.Query().Get("pr")
		if id == "" {
			id = "0"
		}
		_, _ = w.Write([]byte(icimsListingFixture(server.URL, id, "Backend Engineer", "US-CA-San Francisco")))
	}))
	defer server.Close()

	source := &ICIMSSource{
		BoardToken:       "careers-acme.icims.com",
		BaseURL:          server.URL,
		MaxPages:         2,
		MaxDetailFetches: 1,
		http:             server.Client(),
		maxBodyBytes:     defaultICIMSMaxBodyBytes,
	}
	if _, _, err := source.Fetch(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "safety bound") {
		t.Fatalf("expected partial-snapshot safety error, got %v", err)
	}
}

func TestICIMSSource_VerifyCompanyOwnership(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if strings.Contains(r.URL.Path, "/jobs/123/") {
			_, _ = w.Write([]byte(icimsDetailFixture("Deloitte", "123", "Software Engineer")))
			return
		}
		_, _ = w.Write([]byte(icimsListingFixture(server.URL, "123", "Software Engineer", "US-TX-Dallas")))
	}))
	defer server.Close()

	source := &ICIMSSource{
		BoardToken:   "careers-deloitte.icims.com",
		BaseURL:      server.URL,
		MaxPages:     5,
		http:         server.Client(),
		maxBodyBytes: defaultICIMSMaxBodyBytes,
	}
	verification, err := source.VerifyCompanyOwnership(context.Background(), "Deloitte Consulting LLP")
	if err != nil {
		t.Fatalf("VerifyCompanyOwnership: %v", err)
	}
	if !verification.Verified || verification.ObservedName != "Deloitte" {
		t.Fatalf("unexpected verification: %+v", verification)
	}
}

func TestICIMSSource_VerifyCompanyOwnershipRejectsMismatch(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if strings.Contains(r.URL.Path, "/jobs/123/") {
			_, _ = w.Write([]byte(icimsDetailFixture("Another Company", "123", "Software Engineer")))
			return
		}
		_, _ = w.Write([]byte(icimsListingFixture(server.URL, "123", "Software Engineer", "US-TX-Dallas")))
	}))
	defer server.Close()

	source := &ICIMSSource{
		BoardToken:   "careers-candidate.icims.com",
		BaseURL:      server.URL,
		http:         server.Client(),
		maxBodyBytes: defaultICIMSMaxBodyBytes,
	}
	verification, err := source.VerifyCompanyOwnership(context.Background(), "Deloitte Consulting LLP")
	if err != nil {
		t.Fatalf("VerifyCompanyOwnership: %v", err)
	}
	if verification.Verified || verification.ObservedName != "Another Company" {
		t.Fatalf("mismatched employer must not verify: %+v", verification)
	}
}

func TestNewICIMSSourceRejectsNonICIMSHost(t *testing.T) {
	if _, err := NewICIMSSource("example.com"); err == nil {
		t.Fatal("expected non-iCIMS host to be rejected")
	}
}

func icimsListingFixture(baseURL, id, title, location string) string {
	return `<html><body><ul>` +
		`<li class="iCIMS_JobCardItem">` +
		`<div class="col-xs-6 header left"><span class="sr-only field-label">Job Locations</span><span>` + location + `</span></div>` +
		`<div class="col-xs-12 title"><a href="` + baseURL + `/jobs/` + id + `/role/job?in_iframe=1" class="iCIMS_Anchor"><h3>` + title + `</h3></a></div>` +
		`<div class="col-xs-12 description">Short summary</div>` +
		`</li></ul></body></html>`
}

func icimsDetailFixture(company, id, title string) string {
	return `<html><body><script type="application/ld+json">` +
		`{"@context":"https://schema.org","@type":"JobPosting","title":"` + title + `","identifier":{"value":"` + id + `"},` +
		`"hiringOrganization":{"@type":"Organization","name":"` + company + `"},` +
		`"description":"<p>Build distributed systems.</p>","employmentType":"FULL_TIME","datePosted":"2026-09-11T12:00:00Z",` +
		`"jobLocation":{"@type":"Place","address":{"@type":"PostalAddress","addressLocality":"San Francisco","addressRegion":"CA","addressCountry":"US"}}}` +
		`</script></body></html>`
}
