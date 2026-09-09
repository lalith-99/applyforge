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
		{"Software Engineer Intern", "EXCLUDED", "NON_SOFTWARE"},
		{"Software Developer Internship", "EXCLUDED", "NON_SOFTWARE"},
		{"Backend Engineer Co-op", "EXCLUDED", "NON_SOFTWARE"},
		{"Software Engineering Apprentice", "EXCLUDED", "NON_SOFTWARE"},
		{"Student Software Developer", "EXCLUDED", "NON_SOFTWARE"},
		{"Business Analyst", "EXCLUDED", "NON_SOFTWARE"},
		{"Mechanical Engineer", "EXCLUDED", "NON_SOFTWARE"},
		{"Electrical Engineer", "EXCLUDED", "NON_SOFTWARE"},
		{"Solutions Engineer", "EXCLUDED", "NON_SOFTWARE"},
		{"Machine Learning Engineer", "ML_ENGINEERING", "IC_SOFTWARE"},
		{"Firmware Engineer", "EMBEDDED", "IC_SOFTWARE"},
		{"Software Development Engineer II", "SOFTWARE_ENGINEERING", "IC_SOFTWARE"},
		{"Java Developer", "SOFTWARE_ENGINEERING", "IC_SOFTWARE"},
		{"Java Full Stack Developer", "FULLSTACK", "IC_SOFTWARE"},
		{"Spring Boot Developer", "BACKEND", "IC_SOFTWARE"},
		{"Golang Developer", "BACKEND", "IC_SOFTWARE"},
		{"Go Developer", "BACKEND", "IC_SOFTWARE"},
		{"Python Developer", "SOFTWARE_ENGINEERING", "IC_SOFTWARE"},
		{".NET Developer", "SOFTWARE_ENGINEERING", "IC_SOFTWARE"},
		{"React Developer", "FRONTEND", "IC_SOFTWARE"},
		{"Kubernetes Engineer", "PLATFORM", "IC_SOFTWARE"},
		{"MLOps Engineer", "ML_ENGINEERING", "IC_SOFTWARE"},
		{"Distributed Systems Engineer", "SYSTEMS", "IC_SOFTWARE"},
		{"Salesforce Developer", "SOFTWARE_ENGINEERING", "IC_SOFTWARE"},
		{"Enterprise Application Developer", "SOFTWARE_ENGINEERING", "IC_SOFTWARE"},
		{"Integration Engineer", "SOFTWARE_ENGINEERING", "IC_SOFTWARE"},
		{"Software Engineering Consultant", "CONSULTING_ENGINEERING", "IC_SOFTWARE"},
		{"Java Consultant", "CONSULTING_ENGINEERING", "IC_SOFTWARE"},
		{"DevOps Consultant", "DEVOPS", "IC_SOFTWARE"},
		{"Sales Engineer", "EXCLUDED", "NON_SOFTWARE"},
		{"Network Engineer", "UNKNOWN", "UNKNOWN"},
	}
	for _, testCase := range cases {
		got := classifyTitle(testCase.title)
		if got.Family != testCase.family || got.Classification != testCase.classification {
			t.Errorf("classifyTitle(%q) = %+v", testCase.title, got)
		}
	}
}
