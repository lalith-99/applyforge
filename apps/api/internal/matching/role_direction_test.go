package matching

import "testing"

func TestRecommendationRoleDirectionEligible_RejectsDistinctOffDirectionSpecialties(t *testing.T) {
	targets := []string{"Java Full Stack Developer", "Backend Engineer", "Frontend Developer", "Software Engineer"}

	for _, title := range []string{
		"Site Reliability Engineer - Kubernetes",
		"Application Security Engineer",
		"Machine Learning Engineer",
		"Data Platform Engineer",
		"Senior Android Engineer",
		"Embedded Software Engineer",
	} {
		if recommendationRoleDirectionEligible(title, targets) {
			t.Fatalf("expected %q to be rejected for Java/backend/full-stack/frontend targets", title)
		}
	}
}

func TestRecommendationRoleDirectionEligible_PreservesGenericAndTransferableSoftwareRoles(t *testing.T) {
	targets := []string{"Java Full Stack Developer", "Backend Engineer", "Frontend Developer"}

	for _, title := range []string{
		"Senior Software Engineer - Payments",
		"Java Backend Engineer",
		"Full Stack Software Engineer",
		"React Frontend Developer",
		"Platform Software Engineer",
		"Cloud Software Engineer",
		"DevOps Engineer - Developer Platform",
		"Distributed Systems Engineer",
	} {
		if !recommendationRoleDirectionEligible(title, targets) {
			t.Fatalf("expected transferable role %q to remain eligible", title)
		}
	}
}

func TestRecommendationRoleDirectionEligible_AllowsExplicitSpecialtyTargets(t *testing.T) {
	tests := []struct {
		target string
		title  string
	}{
		{"Site Reliability Engineer", "Senior SRE - Compute"},
		{"Security Engineer", "Product Security Engineer"},
		{"Machine Learning Engineer", "ML Engineer, Recommendations"},
		{"Data Engineer", "Data Platform Engineer"},
		{"Android Developer", "Senior Android Engineer"},
		{"Embedded Software Engineer", "Firmware Engineer"},
	}

	for _, tt := range tests {
		if !recommendationRoleDirectionEligible(tt.title, []string{tt.target}) {
			t.Fatalf("explicit target %q should allow %q", tt.target, tt.title)
		}
	}
}

func TestRecommendationRoleDirectionEligible_GenericTargetDoesNotOverFilter(t *testing.T) {
	for _, title := range []string{
		"Site Reliability Engineer",
		"Data Engineer",
		"Security Engineer",
	} {
		if !recommendationRoleDirectionEligible(title, []string{"Software Engineer"}) {
			t.Fatalf("generic target should not prefilter %q before JD scoring", title)
		}
	}
}

func TestRecommendationHighConfidenceSpecialty_NormalizesATSTitlePunctuation(t *testing.T) {
	for title, expected := range map[string]string{
		"Security/Engineer":         "security",
		"Machine-Learning Engineer": "ml",
		"Data_Platform Engineer":    "data",
		"Site Reliability|Engineer": "sre",
	} {
		got, ok := recommendationHighConfidenceSpecialty(title)
		if !ok || got != expected {
			t.Fatalf("expected %q => %q, got %q (known=%v)", title, expected, got, ok)
		}
	}
}
