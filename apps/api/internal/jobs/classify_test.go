package jobs

import "testing"

func TestClassifyTitle(t *testing.T) {
	cases := []struct {
		title, family, classification string
	}{
		{"Lead Backend Engineer", "BACKEND", "IC_SOFTWARE"},
		{"Site Reliability Engineer", "SRE", "IC_SOFTWARE"},
		{"Engineering Manager, Backend", "EXCLUDED", "NON_SOFTWARE"},
		{"Senior QA Test Engineer", "EXCLUDED", "NON_SOFTWARE"},
		{"Business Analyst", "EXCLUDED", "NON_SOFTWARE"},
	}
	for _, testCase := range cases {
		got := classifyTitle(testCase.title)
		if got.Family != testCase.family || got.Classification != testCase.classification {
			t.Errorf("classifyTitle(%q) = %+v", testCase.title, got)
		}
	}
}
