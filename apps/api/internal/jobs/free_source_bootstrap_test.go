package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSourceCompanyKey(t *testing.T) {
	tests := map[string]string{
		"legal suffixes":       sourceCompanyKey("Capital One Services, LLC"),
		"parenthetical suffix": sourceCompanyKey("Blackstone (Campus Careers)"),
		"ampersand":            sourceCompanyKey("Black & Veatch Corporation"),
		"former name":          sourceCompanyKey("Ascendion, Inc. (Formerly known as Collabera, Inc.)"),
	}
	wants := map[string]string{
		"legal suffixes":       "capital one services",
		"parenthetical suffix": "blackstone",
		"ampersand":            "black and veatch",
		"former name":          "ascendion",
	}
	for name, got := range tests {
		if got != wants[name] {
			t.Fatalf("%s: got %q want %q", name, got, wants[name])
		}
	}
}

func TestSourceCompanyExactKeysIncludesBrandOverride(t *testing.T) {
	keys := sourceCompanyExactKeys("Goldman Sachs Services LLC")
	found := false
	for _, key := range keys {
		if key == "goldman sachs" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("keys = %v, want goldman sachs brand alias", keys)
	}
}

func TestFreeSourceEntryScoreDemotesSecondaryWorkdaySites(t *testing.T) {
	inventory := freeSourceInventory{Name: "workday", SourceType: "WORKDAY", AutoMonitor: true}

	primary := freeSourceEntryScore(inventory, "Adobe", "https://adobe.wd5.myworkdayjobs.com/external_experienced")
	if primary < 80 {
		t.Fatalf("primary external Workday board score = %d, want auto-monitorable", primary)
	}

	secondary := freeSourceEntryScore(inventory, "Salesforce (Mulesoft Careersite)", "https://salesforce.wd12.myworkdayjobs.com/Mulesoft_Careersite")
	if secondary >= 80 {
		t.Fatalf("secondary Workday board score = %d, want review-only", secondary)
	}

	internal := freeSourceEntryScore(inventory, "Rivian", "https://rivian.wd5.myworkdayjobs.com/internal")
	if internal >= 80 {
		t.Fatalf("internal Workday board score = %d, want review-only", internal)
	}
}

func TestFetchFreeSourceInventory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/greenhouse.csv") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte("name,slug,url\nDatabricks,databricks,https://job-boards.greenhouse.io/databricks\n"))
	}))
	defer server.Close()

	entries, err := fetchFreeSourceInventory(context.Background(), FreeSourceBootstrapConfig{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}, freeSourceInventory{Name: "greenhouse", SourceType: "GREENHOUSE", AutoMonitor: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].CompanyKey != "databricks" || entries[0].Slug != "databricks" {
		t.Fatalf("unexpected entry: %+v", entries[0])
	}
	discovery, ok := entries[0].toDiscoveredSource()
	if !ok || !discovery.Monitorable || discovery.SourceType != "GREENHOUSE" {
		t.Fatalf("unexpected discovery: %+v ok=%v", discovery, ok)
	}
}

func TestBuiltInOfficialCareerPortal(t *testing.T) {
	tests := map[string]string{
		"Amazon Web Services, Inc.": "https://www.amazon.jobs/en",
		"Apple Inc.":                "https://jobs.apple.com/en-us/search",
		"Netflix, Inc.":             "https://jobs.netflix.com/",
		"Salesforce, Inc.":          "https://careers.salesforce.com/en/jobs/",
		"Tesla, Inc.":               "https://www.tesla.com/careers/search",
	}
	for company, want := range tests {
		got, ok := builtInOfficialCareerPortal(company)
		if !ok || got != want {
			t.Fatalf("%s: got %q ok=%v want %q", company, got, ok, want)
		}
	}
}
