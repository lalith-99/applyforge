package jobs

import (
	"net"
	"net/url"
	"testing"
	"time"
)

func TestParseCareerPageDocumentExtractsJobsAndATSLinks(t *testing.T) {
	base, err := url.Parse("https://careers.acme.example/jobs")
	if err != nil {
		t.Fatal(err)
	}

	body := []byte(`<!doctype html>
<html>
<head>
  <script type="application/ld+json">
  {
    "@context": "https://schema.org",
    "@graph": [
      {
        "@type": "JobPosting",
        "title": "Senior Java Engineer",
        "identifier": {"@type": "PropertyValue", "value": "REQ-101"},
        "description": "<p>Build Java and Spring Boot services.</p>",
        "datePosted": "2026-09-08",
        "employmentType": ["FULL_TIME"],
        "jobLocation": {
          "@type": "Place",
          "address": {
            "addressLocality": "Austin",
            "addressRegion": "TX",
            "addressCountry": {"name": "US"}
          }
        },
        "url": "/jobs/req-101"
      },
      {
        "@type": ["Thing", "JobPosting"],
        "title": "Remote Go Engineer",
        "identifier": "REQ-102",
        "description": "<div>Build Go platform services.</div>",
        "datePosted": "2026-09-07T12:00:00Z",
        "employmentType": "FULL_TIME",
        "jobLocationType": "TELECOMMUTE",
        "url": "https://careers.acme.example/jobs/req-102"
      }
    ]
  }
  </script>
</head>
<body>
  <a href="https://acme.wd1.myworkdayjobs.com/en-US/External/job/Austin/Senior-Engineer_R1">
    Another careers site
  </a>
</body>
</html>`)

	inspection, err := parseCareerPageDocument(base, body)
	if err != nil {
		t.Fatalf("parseCareerPageDocument: %v", err)
	}
	if len(inspection.Jobs) != 2 {
		t.Fatalf("expected 2 structured jobs, got %d", len(inspection.Jobs))
	}

	first := inspection.Jobs[0]
	if first.ExternalID != "REQ-101" {
		t.Fatalf("unexpected external id %q", first.ExternalID)
	}
	if first.Title != "Senior Java Engineer" {
		t.Fatalf("unexpected title %q", first.Title)
	}
	if first.Description != "Build Java and Spring Boot services." {
		t.Fatalf("unexpected description %q", first.Description)
	}
	if first.City != "Austin" || first.State != "TX" || first.Country != "US" {
		t.Fatalf("unexpected location %+v", first)
	}
	if first.ApplyURL != "https://careers.acme.example/jobs/req-101" {
		t.Fatalf("unexpected apply URL %q", first.ApplyURL)
	}
	if first.PostedAt == nil || first.PostedAt.Format("2006-01-02") != "2026-09-08" {
		t.Fatalf("unexpected date posted %v", first.PostedAt)
	}

	second := inspection.Jobs[1]
	if second.RemoteType != "remote" || second.LocationText != "Remote" {
		t.Fatalf("expected remote job, got %+v", second)
	}

	foundWorkday := false
	for _, source := range inspection.Sources {
		if source.SourceType == "WORKDAY" {
			foundWorkday = true
			if !source.Monitorable {
				t.Fatal("expected discovered Workday site to be monitorable")
			}
		}
	}
	if !foundWorkday {
		t.Fatalf("expected Workday source discovery, got %+v", inspection.Sources)
	}
}

func TestParseJobPostingJSONLDHandlesTopLevelArray(t *testing.T) {
	base, _ := url.Parse("https://jobs.example.com/")
	raw := `[
	  {
	    "@type": "JobPosting",
	    "title": "Platform Engineer",
	    "identifier": {"value": "P-1"},
	    "datePosted": "2026-09-08",
	    "jobLocation": {"address": {"addressCountry": "United States"}}
	  }
	]`

	jobs := parseJobPostingJSONLD(base, raw)
	if len(jobs) != 1 || jobs[0].ExternalID != "P-1" {
		t.Fatalf("unexpected jobs %+v", jobs)
	}
}

func TestValidatePublicHTTPURLRejectsPrivateTargets(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1/admin",
		"http://10.0.0.10/",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/",
		"file:///etc/passwd",
	}
	for _, raw := range blocked {
		if _, err := validatePublicHTTPURL(raw); err == nil {
			t.Fatalf("expected %s to be rejected", raw)
		}
	}

	if _, err := validatePublicHTTPURL("https://careers.example.com/jobs"); err != nil {
		t.Fatalf("expected public hostname syntax to be allowed: %v", err)
	}
}

func TestIsPublicInternetIP(t *testing.T) {
	cases := []struct {
		ip      string
		allowed bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"100.64.0.1", false},
		{"192.0.2.1", false},
		{"10.0.0.1", false},
		{"127.0.0.1", false},
		{"::1", false},
	}
	for _, tc := range cases {
		if got := isPublicInternetIP(net.ParseIP(tc.ip)); got != tc.allowed {
			t.Fatalf("%s: expected allowed=%v, got %v", tc.ip, tc.allowed, got)
		}
	}
}

func TestParseJSONLDDate(t *testing.T) {
	got := parseJSONLDDate("2026-09-08T10:30:00Z")
	if got == nil || !got.Equal(time.Date(2026, 9, 8, 10, 30, 0, 0, time.UTC)) {
		t.Fatalf("unexpected parsed date %v", got)
	}
}
