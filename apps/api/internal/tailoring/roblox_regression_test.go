package tailoring

import (
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
)

func TestTargetSkillKeys_ExpandsAlternativeRequirements(t *testing.T) {
	allowed := targetSkillKeys(
		[]aiclient.SkillRequirement{
			{
				NormalizedName: "Go or Python",
				Alternatives:   []string{"Go", "Python"},
				Importance:     "required",
			},
		},
		nil,
	)

	if !allowed["go"] || !allowed["python"] || len(allowed) != 2 {
		t.Fatalf("expected both alternatives to be valid target technologies, got %v", allowed)
	}
}

func TestSanitizeTailoringSuggestions_DropsBroadStandaloneConceptsButKeepsGo(t *testing.T) {
	response := aiclient.TailoringResponse{
		SkillSuggestions: []aiclient.TailoringSuggestion{
			{
				Section:       "skills",
				SuggestedText: "Add Go (Golang)",
				SkillsAdded:   []string{"Go (Golang)"},
				KeywordsAdded: []string{"Go (Golang)"},
				Source:        "AI_SUGGESTED",
				RiskLevel:     "HIGH",
			},
			{
				Section:       "skills",
				SuggestedText: "Add Privacy Engineering",
				SkillsAdded:   []string{"Privacy Engineering"},
				KeywordsAdded: []string{"Privacy Engineering"},
				Source:        "AI_SUGGESTED",
				RiskLevel:     "HIGH",
			},
			{
				Section:       "skills",
				SuggestedText: "Add Data Protection",
				SkillsAdded:   []string{"Data Protection"},
				KeywordsAdded: []string{"Data Protection"},
				Source:        "AI_SUGGESTED",
				RiskLevel:     "HIGH",
			},
			{
				Section:       "skills",
				SuggestedText: "Add System Design",
				SkillsAdded:   []string{"System Design"},
				KeywordsAdded: []string{"System Design"},
				Source:        "AI_SUGGESTED",
				RiskLevel:     "HIGH",
			},
		},
	}

	sanitizeTailoringSuggestions(
		&response,
		map[string]bool{"python": true},
		map[string]bool{"go": true, "python": true},
	)

	if len(response.SkillSuggestions) != 1 {
		t.Fatalf("expected only the concrete target technology to remain, got %+v", response.SkillSuggestions)
	}
	if tailoringSkillKey(response.SkillSuggestions[0].SkillsAdded[0]) != "go" {
		t.Fatalf("expected Go suggestion to remain, got %+v", response.SkillSuggestions[0])
	}
}
