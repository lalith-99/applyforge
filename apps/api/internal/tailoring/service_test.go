package tailoring

import (
	"encoding/json"
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/resume"
)

func TestSanitizeKnownSkillSuggestionsKeepsUnknownAIDraftForAttestation(t *testing.T) {
	resp := aiclient.TailoringResponse{
		ExperienceSuggestions: []aiclient.TailoringSuggestion{
			{
				Section:       "experience",
				SuggestedText: "Deployed Spring Boot microservices on Azure using containerized CI/CD workflows.",
				SkillsAdded:   []string{"Azure"},
				Source:        "AI_SUGGESTED",
				RiskLevel:     "HIGH",
			},
		},
	}

	sanitizeKnownSkillSuggestions(&resp, map[string]bool{"java": true, "spring boot": true})

	if len(resp.ExperienceSuggestions) != 1 {
		t.Fatalf("unknown AI experience draft should survive for attestation: %+v", resp.ExperienceSuggestions)
	}
	if resp.ExperienceSuggestions[0].SkillsAdded[0] != "Azure" {
		t.Fatalf("expected Azure attestation metadata to remain, got %+v", resp.ExperienceSuggestions[0])
	}
}

func TestMasterResumeSkillInventoryUsesSelectedResume(t *testing.T) {
	parsed, err := json.Marshal(aiclient.ResumeProfile{
		Skills:  []string{"Java", "Kubernetes", "AWS", "Apache Kafka"},
		Summary: stringPtr("Java engineer with Kubernetes experience."),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := resume.Resume{ParsedProfile: parsed}
	experiences := []resume.Experience{
		{DetectedSkills: []string{"Spring Boot"}, Technologies: []string{"Kafka"}},
	}

	skillSet, masterSkills, summary := masterResumeSkillInventory(res, experiences, nil)
	for _, want := range []string{"java", "kubernetes", "aws", "spring boot", "kafka"} {
		if !skillSet[want] {
			t.Fatalf("selected resume skill set missing %q: %v", want, skillSet)
		}
	}
	if len(masterSkills) != 5 {
		t.Fatalf("masterSkills = %v, want 5 canonical selected-resume skills", masterSkills)
	}
	if summary == nil || *summary != "Java engineer with Kubernetes experience." {
		t.Fatalf("unexpected summary: %v", summary)
	}
}

func TestSanitizeKnownSkillSuggestionsPreservesExperienceRewrites(t *testing.T) {
	resp := aiclient.TailoringResponse{
		SkillSuggestions: []aiclient.TailoringSuggestion{
			{
				Section:       "skills",
				SuggestedText: "Add Kubernetes to your skills section",
				SkillsAdded:   []string{"Kubernetes"},
				Source:        "AI_SUGGESTED",
			},
			{
				Section:       "skills",
				SuggestedText: "Add Rust to your skills section",
				SkillsAdded:   []string{"Rust"},
				Source:        "AI_SUGGESTED",
			},
		},
		ExperienceSuggestions: []aiclient.TailoringSuggestion{
			{
				Section:       "experience",
				SuggestedText: "Operated Kubernetes services and implemented Rust components.",
				SkillsAdded:   []string{"Kubernetes", "Rust"},
				KeywordsAdded: []string{"Kubernetes", "Rust"},
				Source:        "AI_SUGGESTED",
				RiskLevel:     "HIGH",
			},
			{
				Section:       "experience",
				SuggestedText: "Reframed existing Kubernetes deployment work.",
				SkillsAdded:   []string{"Kubernetes"},
				Source:        "AI_SUGGESTED",
				RiskLevel:     "HIGH",
			},
		},
	}

	sanitizeKnownSkillSuggestions(&resp, map[string]bool{"kubernetes": true})

	if len(resp.SkillSuggestions) != 1 || resp.SkillSuggestions[0].SkillsAdded[0] != "Rust" {
		t.Fatalf("known resume skill was not filtered from skill suggestions: %+v", resp.SkillSuggestions)
	}
	if len(resp.ExperienceSuggestions) != 2 {
		t.Fatalf("experience rewrites must never be dropped for known-skill metadata: %+v", resp.ExperienceSuggestions)
	}
	if len(resp.ExperienceSuggestions[0].SkillsAdded) != 1 ||
		resp.ExperienceSuggestions[0].SkillsAdded[0] != "Rust" {
		t.Fatalf("expected only unknown Rust metadata to remain: %+v", resp.ExperienceSuggestions[0])
	}
	if resp.ExperienceSuggestions[1].Source != "MASTER_RESUME" ||
		resp.ExperienceSuggestions[1].RiskLevel != "LOW" {
		t.Fatalf("known-only rewrite should be downgraded to verified: %+v", resp.ExperienceSuggestions[1])
	}
}

func TestTailoringSkillKeyCanonicalizesCommonAliases(t *testing.T) {
	tests := map[string]string{
		"Apache Kafka": "kafka",
		"Kafka":        "kafka",
		"Argo CD":      "argocd",
		"Golang":       "go",
		"React.js":     "react",
	}
	for input, want := range tests {
		if got := tailoringSkillKey(input); got != want {
			t.Fatalf("tailoringSkillKey(%q) = %q, want %q", input, got, want)
		}
	}
}

func stringPtr(value string) *string {
	return &value
}
