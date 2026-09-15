package jobs

import "testing"

func TestDecodeEmployerListDataset(t *testing.T) {
	dataset, err := decodeEmployerListDataset([]byte(`{
		"updated":"2026-09-15",
		"rows":[{
			"n":"Databricks, Inc.",
			"u":"https://job-boards.greenhouse.io/databricks",
			"hq":"San Francisco, CA",
			"st":["CA","NY"],
			"ind":"Software",
			"a":1234,
			"fy":2023,
			"li":"https://www.linkedin.com/jobs/databricks-jobs",
			"tier":"1000+"
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if dataset.Updated != "2026-09-15" || len(dataset.Rows) != 1 {
		t.Fatalf("unexpected dataset: %+v", dataset)
	}
	row := dataset.Rows[0]
	if row.Name != "Databricks, Inc." || row.Approvals == nil || *row.Approvals != 1234 {
		t.Fatalf("unexpected row: %+v", row)
	}
}

func TestEmployerListDiscoveriesPromotesDirectATS(t *testing.T) {
	discoveries := employerListDiscoveries(employerListRow{
		Name:       "Databricks, Inc.",
		CareersURL: "https://job-boards.greenhouse.io/databricks",
	})
	if len(discoveries) != 1 {
		t.Fatalf("got %d discoveries, want 1", len(discoveries))
	}
	got := discoveries[0]
	if got.SourceType != "GREENHOUSE" || got.BoardToken != "databricks" || !got.Monitorable {
		t.Fatalf("unexpected discovery: %+v", got)
	}
	if got.DiscoveryMethod != "CAREER_PAGE" || got.Confidence > 0.92 {
		t.Fatalf("unexpected provenance/confidence: %+v", got)
	}
}

func TestEmployerListDiscoveriesKeepsGenericCareerPageForInspection(t *testing.T) {
	discoveries := employerListDiscoveries(employerListRow{
		Name:       "Example Corp",
		CareersURL: "https://careers.example.com/jobs",
	})
	if len(discoveries) != 1 {
		t.Fatalf("got %d discoveries, want 1", len(discoveries))
	}
	got := discoveries[0]
	if got.SourceType != "CUSTOM" || got.BoardToken != employerListGenericCareerToken || got.Monitorable {
		t.Fatalf("unexpected generic career discovery: %+v", got)
	}
	if got.DiscoveryMethod != "CAREER_PAGE" {
		t.Fatalf("discovery method = %q, want CAREER_PAGE", got.DiscoveryMethod)
	}
}

func TestEmployerListDuckyRedirectIsNavigationOnly(t *testing.T) {
	row := employerListRow{
		Name:       "Example Corp",
		CareersURL: "https://duckduckgo.com/?q=!ducky+Example+Corp+careers",
	}
	if !isDuckDuckGoDuckyURL(row.CareersURL) {
		t.Fatal("expected DuckDuckGo !ducky URL to be detected")
	}
	if discoveries := employerListDiscoveries(row); len(discoveries) != 0 {
		t.Fatalf("ducky redirect produced monitor candidates: %+v", discoveries)
	}
}

func TestEmployerListRowScorePrefersDirectCareerURL(t *testing.T) {
	approvals := 5000
	direct := employerListRow{
		CareersURL: "https://careers.example.com/jobs",
		Approvals:  &approvals,
	}
	ducky := employerListRow{
		CareersURL: "https://duckduckgo.com/?q=!ducky+Example+careers",
		Approvals:  &approvals,
	}
	if employerListRowScore(direct) <= employerListRowScore(ducky) {
		t.Fatalf("direct career URL should outrank DuckDuckGo redirect: direct=%d ducky=%d",
			employerListRowScore(direct), employerListRowScore(ducky))
	}
}
