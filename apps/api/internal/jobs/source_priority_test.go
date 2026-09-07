package jobs

import "testing"

func TestSourcePriority_PrefersEmployerDirectSources(t *testing.T) {
	for _, direct := range []string{"GREENHOUSE", "LEVER", "ASHBY", "SMARTRECRUITERS", "WORKABLE"} {
		if sourcePriority(direct) <= sourcePriority("BRIGHTDATA") {
			t.Fatalf("direct source %s must outrank Bright Data", direct)
		}
		if sourcePriority(direct) <= sourcePriority("SERPAPI_GOOGLE_JOBS") {
			t.Fatalf("direct source %s must outrank Google Jobs discovery", direct)
		}
	}
	if sourcePriority("BRIGHTDATA") <= sourcePriority("SERPAPI_GOOGLE_JOBS") {
		t.Fatal("structured broad provider should outrank supplemental Google Jobs")
	}
	if sourcePriority("SERPAPI_GOOGLE_JOBS") <= sourcePriority("ARBEITNOW") {
		t.Fatal("Google Jobs discovery should outrank generic aggregator")
	}
}
