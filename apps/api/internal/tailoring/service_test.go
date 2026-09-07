package tailoring

import (
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
