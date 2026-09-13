package jobs

import "testing"

type completeStaticSource struct {
	staticJobSource
	seen []string
}

func (s completeStaticSource) SeenExternalIDs() []string { return s.seen }

func TestSnapshotExternalIDsCombinesHydratedAndListingOnlyIDs(t *testing.T) {
	source := completeStaticSource{
		staticJobSource: staticJobSource{jobs: []RawJob{
			{ExternalID: " hydrated "},
			{ExternalID: "duplicate"},
		}},
		seen: []string{"duplicate", "listing-only", ""},
	}

	ids := snapshotExternalIDs(source.jobs, source)
	got := make(map[string]bool, len(ids))
	for _, id := range ids {
		got[id] = true
	}
	for _, want := range []string{"hydrated", "duplicate", "listing-only"} {
		if !got[want] {
			t.Fatalf("expected %q in snapshot IDs, got %v", want, ids)
		}
	}
	if len(got) != 3 {
		t.Fatalf("expected IDs to be trimmed and deduplicated, got %v", ids)
	}
}

func TestSourceSupportsClosure(t *testing.T) {
	for _, source := range []string{"GREENHOUSE", "LEVER", "ASHBY", "WORKDAY", "ICIMS"} {
		if !sourceSupportsClosure(source) {
			t.Fatalf("expected %s to support complete-snapshot closure", source)
		}
	}
	for _, source := range []string{"ARBEITNOW", "BRIGHTDATA", "SERPAPI_GOOGLE_JOBS", "CAREER_PAGE"} {
		if sourceSupportsClosure(source) {
			t.Fatalf("expected %s to keep capped/partial inventory open", source)
		}
	}
}

var _ JobSource = completeStaticSource{}
var _ CompleteSnapshotSource = completeStaticSource{}
