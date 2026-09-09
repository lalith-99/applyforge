package tailoring

import (
	"encoding/json"
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/resume"
)

func TestEnsureAddedSkillsReachABullet_AddsGrowthBulletForOrphanedSkill(t *testing.T) {
	bullet := "Built Go microservices for telecom file processing."
	aiResp := aiclient.TailoringResponse{
		SkillSuggestions: []aiclient.TailoringSuggestion{
			{Section: "skills", SkillsAdded: []string{"Rust"}, Source: "AI_SUGGESTED"},
		},
	}
	experiences := []resume.Experience{{Bullets: []string{bullet}}}

	ensureAddedSkillsReachABullet(&aiResp, experiences)

	if len(aiResp.ExperienceSuggestions) != 1 {
		t.Fatalf("expected one synthesized experience suggestion, got %d", len(aiResp.ExperienceSuggestions))
	}
	got := aiResp.ExperienceSuggestions[0]
	if got.RiskLevel != "HIGH" || got.Source != "AI_SUGGESTED" {
		t.Fatalf("expected an honest, high-risk growth suggestion, got %+v", got)
	}
	if got.OriginalText == nil || *got.OriginalText != bullet {
		t.Fatalf("expected original text to be the real existing bullet, got %v", got.OriginalText)
	}
}

func TestEnsureAddedSkillsReachABullet_SkipsSkillsAlreadyMentioned(t *testing.T) {
	aiResp := aiclient.TailoringResponse{
		SkillSuggestions: []aiclient.TailoringSuggestion{
			{Section: "skills", SkillsAdded: []string{"Rust"}, Source: "AI_SUGGESTED"},
		},
		ExperienceSuggestions: []aiclient.TailoringSuggestion{
			{Section: "experience", SuggestedText: "Applying growing Rust expertise to systems work."},
		},
	}
	experiences := []resume.Experience{{Bullets: []string{"Some bullet."}}}

	ensureAddedSkillsReachABullet(&aiResp, experiences)

	if len(aiResp.ExperienceSuggestions) != 1 {
		t.Fatalf("expected no additional suggestion since Rust is already mentioned, got %d", len(aiResp.ExperienceSuggestions))
	}
}

func TestMasterResumeSkillInventoryUsesSelectedResume(t *testing.T) {
	parsed, err := json.Marshal(aiclient.ResumeProfile{
		Skills:  []string{"Java", "Kubernetes", "AWS"},
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
		t.Fatalf("masterSkills = %v, want 5 selected-resume skills", masterSkills)
	}
	if summary == nil || *summary != "Java engineer with Kubernetes experience." {
		t.Fatalf("unexpected summary: %v", summary)
	}
}

func TestSanitizeKnownSkillSuggestionsDropsLearnFirstForResumeSkill(t *testing.T) {
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
				SuggestedText: "Add Argo CD to your skills section",
				SkillsAdded:   []string{"Argo CD"},
				Source:        "AI_SUGGESTED",
			},
		},
		ExperienceSuggestions: []aiclient.TailoringSuggestion{
			{
				Section:       "experience",
				SuggestedText: "Actively building Kubernetes proficiency.",
				SkillsAdded:   []string{"Kubernetes"},
				Source:        "AI_SUGGESTED",
			},
			{
				Section:       "experience",
				SuggestedText: "Reframed existing Kubernetes deployment work.",
				Source:        "MASTER_RESUME",
			},
		},
	}

	sanitizeKnownSkillSuggestions(&resp, map[string]bool{"kubernetes": true})

	if len(resp.SkillSuggestions) != 1 || resp.SkillSuggestions[0].SkillsAdded[0] != "Argo CD" {
		t.Fatalf("known resume skill was not filtered from skill suggestions: %+v", resp.SkillSuggestions)
	}
	if len(resp.ExperienceSuggestions) != 1 || resp.ExperienceSuggestions[0].Source != "MASTER_RESUME" {
		t.Fatalf("known-skill AI growth suggestion was not filtered: %+v", resp.ExperienceSuggestions)
	}
}

func stringPtr(value string) *string {
	return &value
}
