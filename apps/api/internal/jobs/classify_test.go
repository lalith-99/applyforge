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
		{"Mechanical Engineer", "EXCLUDED", "NON_SOFTWARE"},
		{"Electrical Engineer", "EXCLUDED", "NON_SOFTWARE"},
		{"Solutions Engineer", "EXCLUDED", "NON_SOFTWARE"},
		{"Machine Learning Engineer", "ML_ENGINEERING", "IC_SOFTWARE"},
		{"Firmware Engineer", "EMBEDDED", "IC_SOFTWARE"},
		{"Software Development Engineer II", "SOFTWARE_ENGINEERING", "IC_SOFTWARE"},
		{"Network Engineer", "UNKNOWN", "UNKNOWN"},
	}
	for _, testCase := range cases {
		got := classifyTitle(testCase.title)
		if got.Family != testCase.family || got.Classification != testCase.classification {
			t.Errorf("classifyTitle(%q) = %+v", testCase.title, got)
		}
	}
}
