package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSuccessFactorsSource_FetchRSS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sitemal.xml" {
			t.Fatalf("unexpected feed path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(successFactorsRSSFixture("Acme Corporation", "Acme Corporation", "sf-123")))
	}))
	defer server.Close()

	source, err := NewSuccessFactorsSource(server.URL)
	if err != nil {
		t.Fatalf("NewSuccessFactorsSource: %v", err)
	}
	source.http = server.Client()

	jobs, cursor, err := source.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if cursor != nil {
		t.Fatalf("expected nil cursor")
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one job, got %d", len(jobs))
	}
	job := jobs[0]
	if !strings.HasSuffix(job.ExternalID, ":sf-123") {
		t.Fatalf("expected tenant-scoped external id, got %q", job.ExternalID)
	}
	if job.Title != "Backend Engineer" || job.LocationText != "Austin, TX, US" {
		t.Fatalf("unexpected job: %+v", job)
	}
	if job.Description != "Build APIs" || job.EmploymentType != "Full-time" || job.PostedAt == nil {
		t.Fatalf("expected parsed RSS metadata: %+v", job)
	}
	if job.CompanyName != "" {
		t.Fatalf("direct feed must preserve configured canonical company ownership, got CompanyName=%q", job.CompanyName)
	}
	seen := source.SeenExternalIDs()
	if len(seen) != 1 || seen[0] != job.ExternalID {
		t.Fatalf("unexpected seen IDs: %#v", seen)
	}
}

func TestSuccessFactorsSource_VerifyRSSCompanyOwnership(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(successFactorsRSSFixture("Jobs at Deloitte", "Deloitte", "d-1")))
	}))
	defer server.Close()

	source, err := NewSuccessFactorsSource(server.URL)
	if err != nil {
		t.Fatalf("NewSuccessFactorsSource: %v", err)
	}
	source.http = server.Client()

	verification, err := source.VerifyCompanyOwnership(context.Background(), "Deloitte Consulting LLP")
	if err != nil {
		t.Fatalf("VerifyCompanyOwnership: %v", err)
	}
	if !verification.Verified || verification.ObservedName != "Deloitte" {
		t.Fatalf("unexpected verification: %+v", verification)
	}
}

func TestSuccessFactorsSource_VerifyRSSRejectsMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(successFactorsRSSFixture("Jobs at Another Company", "Another Company", "x-1")))
	}))
	defer server.Close()

	source, err := NewSuccessFactorsSource(server.URL)
	if err != nil {
		t.Fatalf("NewSuccessFactorsSource: %v", err)
	}
	source.http = server.Client()
	verification, err := source.VerifyCompanyOwnership(context.Background(), "Deloitte Consulting LLP")
	if err != nil {
		t.Fatalf("VerifyCompanyOwnership: %v", err)
	}
	if verification.Verified || verification.ObservedName != "Another Company" {
		t.Fatalf("mismatched employer must remain unverified: %+v", verification)
	}
}

func TestSuccessFactorsSource_FetchLegacyXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<Job-Listing><Job><ReqId>42</ReqId><JobTitle>Software Engineer</JobTitle><Job-Description><![CDATA[<p>Build services</p>]]></Job-Description><City>New York</City><State>NY</State><Country>United States</Country><Posted-Date>2026-09-10</Posted-Date><EmploymentType>Full-time</EmploymentType><CompanyName>Pfizer</CompanyName></Job></Job-Listing>`))
	}))
	defer server.Close()

	source := &SuccessFactorsSource{
		BoardToken:      "https://career8.successfactors.com/career?company=pfizer",
		FeedURL:         server.URL,
		Mode:            successFactorsLegacy,
		TenantKey:       "career8.successfactors.com|pfizer",
		LegacyCompanyID: "pfizer",
		http:            server.Client(),
		maxBodyBytes:    defaultSuccessFactorsMaxBodyBytes,
	}

	jobs, _, err := source.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one job, got %d", len(jobs))
	}
	job := jobs[0]
	if job.ExternalID != "career8.successfactors.com|pfizer:42" {
		t.Fatalf("unexpected legacy external id %q", job.ExternalID)
	}
	if job.LocationText != "New York, NY, United States" || job.Description != "Build services" || job.PostedAt == nil {
		t.Fatalf("unexpected legacy job: %+v", job)
	}

	verification, err := source.VerifyCompanyOwnership(context.Background(), "Pfizer Inc")
	if err != nil {
		t.Fatalf("VerifyCompanyOwnership: %v", err)
	}
	if !verification.Verified || verification.ObservedName != "Pfizer" {
		t.Fatalf("unexpected legacy verification: %+v", verification)
	}
}

func TestSuccessFactorsSource_VerifyLegacyRequiresProviderIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<Job-Listing><Job><ReqId>42</ReqId><JobTitle>Software Engineer</JobTitle></Job></Job-Listing>`))
	}))
	defer server.Close()

	source := &SuccessFactorsSource{
		BoardToken:      "https://career8.successfactors.com/career?company=acme",
		FeedURL:         server.URL,
		Mode:            successFactorsLegacy,
		TenantKey:       "career8.successfactors.com|acme",
		LegacyCompanyID: "acme",
		http:            server.Client(),
		maxBodyBytes:    defaultSuccessFactorsMaxBodyBytes,
	}
	if _, err := source.VerifyCompanyOwnership(context.Background(), "Acme"); err == nil || !strings.Contains(err.Error(), "employer identity") {
		t.Fatalf("expected identity-verification failure, got %v", err)
	}
}

func TestNewSuccessFactorsSource_ResolvesLegacyFeed(t *testing.T) {
	source, err := NewSuccessFactorsSource("https://career8.successfactors.com/career?company=acme&rcm_site_locale=en_US")
	if err != nil {
		t.Fatalf("NewSuccessFactorsSource: %v", err)
	}
	if source.Mode != successFactorsLegacy || source.LegacyCompanyID != "acme" {
		t.Fatalf("unexpected legacy source: %+v", source)
	}
	if !strings.Contains(source.FeedURL, "career_ns=job_listing_summary") || !strings.Contains(source.FeedURL, "resultType=XML") || !strings.Contains(source.FeedURL, "rcm_site_locale=en_US") {
		t.Fatalf("unexpected legacy feed URL %q", source.FeedURL)
	}
}

func TestNewSuccessFactorsSource_RejectsBareSlug(t *testing.T) {
	if _, err := NewSuccessFactorsSource("acme"); err == nil {
		t.Fatal("expected a bare ambiguous slug to be rejected")
	}
}

func successFactorsRSSFixture(channelTitle, employer, id string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:g="http://base.google.com/ns/1.0">
  <channel>
    <title>` + channelTitle + `</title>
    <item>
      <g:id>` + id + `</g:id>
      <g:employer>` + employer + `</g:employer>
      <g:location>Austin, TX, US</g:location>
      <g:job_type>Full-time</g:job_type>
      <title>Backend Engineer</title>
      <link>https://careers.example.com/job/` + id + `</link>
      <description><![CDATA[<p>Build APIs</p>]]></description>
      <pubDate>Thu, 10 Sep 2026 12:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>`
}
