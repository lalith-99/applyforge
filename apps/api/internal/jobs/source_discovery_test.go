package jobs

import "testing"

func TestDetectCompanySources(t *testing.T) {
	cases := []struct {
		url         string
		sourceType  string
		token       string
		monitorable bool
	}{
		{"https://boards.greenhouse.io/acme/jobs/123", "GREENHOUSE", "acme", true},
		{"https://job-boards.greenhouse.io/acme/jobs/123", "GREENHOUSE", "acme", true},
		{"https://jobs.lever.co/acme/abc", "LEVER", "acme", true},
		{"https://jobs.ashbyhq.com/acme/abc", "ASHBY", "acme", true},
		{"https://jobs.smartrecruiters.com/Acme/123", "SMARTRECRUITERS", "Acme", true},
		{"https://apply.workable.com/acme/j/ABC/", "WORKABLE", "acme", true},
		{"https://acme.wd1.myworkdayjobs.com/Careers/job/123", "WORKDAY", "acme.wd1.myworkdayjobs.com|acme|Careers", true},
	}

	for _, tc := range cases {
		got := DetectCompanySources(tc.url)
		if len(got) != 1 {
			t.Fatalf("%s: expected one discovery, got %d", tc.url, len(got))
		}
		if got[0].SourceType != tc.sourceType || got[0].BoardToken != tc.token || got[0].Monitorable != tc.monitorable {
			t.Fatalf("%s: got %+v", tc.url, got[0])
		}
	}
}

func TestDetectCompanySourcesIgnoresGenericAndDeduplicates(t *testing.T) {
	if got := DetectCompanySources("https://example.com/careers/123"); len(got) != 0 {
		t.Fatalf("expected generic URL to be ignored, got %+v", got)
	}
	got := DetectCompanySources(
		"https://jobs.lever.co/acme/1",
		"https://jobs.lever.co/acme/2",
	)
	if len(got) != 1 {
		t.Fatalf("expected same board to deduplicate, got %d", len(got))
	}
}

func TestDetectCompanySourcesPreservesRegistryURL(t *testing.T) {
	cases := []struct {
		raw        string
		sourceType string
	}{
		{"https://jobs-acme.icims.com/jobs/123/software-engineer/job?mode=job", "ICIMS"},
		{"https://career8.successfactors.com/sfcareer/jobreqcareer?jobId=42&company=pfizer", "SUCCESSFACTORS"},
		{"https://acme.fa.us2.oraclecloud.com/hcmUI/CandidateExperience/en/sites/CX_1/job/123", "ORACLE"},
	}

	for _, tc := range cases {
		got := DetectCompanySources(tc.raw)
		if len(got) != 1 {
			t.Fatalf("%s: expected one discovery, got %+v", tc.raw, got)
		}
		if got[0].SourceType != tc.sourceType {
			t.Fatalf("%s: unexpected source type %s", tc.raw, got[0].SourceType)
		}
		if got[0].SourceURL != tc.raw {
			t.Fatalf("%s: expected exact source URL, got %s", tc.raw, got[0].SourceURL)
		}
		if got[0].Monitorable {
			t.Fatalf("%s: registry-only source must not be directly monitorable", tc.raw)
		}
	}
}

func TestDetectCompanySources_SuccessFactorsKeepsCompanyQueryInBoardToken(t *testing.T) {
	raw := "https://career8.successfactors.com/sfcareer/jobreqcareer?jobId=42&company=pfizer"
	got := DetectCompanySources(raw)
	if len(got) != 1 {
		t.Fatalf("expected one SuccessFactors discovery, got %+v", got)
	}
	if got[0].BoardToken != raw {
		t.Fatalf("board token must preserve SuccessFactors company identity query; got %q", got[0].BoardToken)
	}
}
