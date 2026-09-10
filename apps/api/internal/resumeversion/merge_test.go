package resumeversion

import (
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/tailoring"
)

func strPtr(s string) *string { return &s }

func TestMergeContent_SummaryReplacedWhenApproved(t *testing.T) {
	base := aiclient.ResumeProfile{Summary: strPtr("Original summary.")}
	suggestions := []tailoring.Suggestion{
		{Section: "summary", SuggestedText: "Tailored summary.", UserStatus: tailoring.StatusApproved},
	}

	merged := mergeContent(base, suggestions)
	if merged.Summary == nil || *merged.Summary != "Tailored summary." {
		t.Fatalf("expected summary to be replaced, got %v", merged.Summary)
	}
}

func TestMergeContent_PendingSuggestionsIgnored(t *testing.T) {
	base := aiclient.ResumeProfile{Summary: strPtr("Original summary.")}
	suggestions := []tailoring.Suggestion{
		{Section: "summary", SuggestedText: "Tailored summary.", UserStatus: tailoring.StatusPending},
	}

	merged := mergeContent(base, suggestions)
	if merged.Summary == nil || *merged.Summary != "Original summary." {
		t.Fatalf("expected pending suggestion to be ignored, got %v", merged.Summary)
	}
}

func TestMergeContent_EditedTextTakesPrecedenceOverSuggestedText(t *testing.T) {
	base := aiclient.ResumeProfile{}
	suggestions := []tailoring.Suggestion{
		{
			Section:       "summary",
			SuggestedText: "AI suggested summary.",
			EditedText:    strPtr("User edited summary."),
			UserStatus:    tailoring.StatusEdited,
		},
	}

	merged := mergeContent(base, suggestions)
	if merged.Summary == nil || *merged.Summary != "User edited summary." {
		t.Fatalf("expected edited text to win, got %v", merged.Summary)
	}
}

func TestMergeContent_SkillsAddedWithoutDuplicates(t *testing.T) {
	base := aiclient.ResumeProfile{Skills: []string{"Go"}}
	suggestions := []tailoring.Suggestion{
		{Section: "skills", SuggestedText: "Add Kafka", SkillsAdded: []string{"Kafka"}, UserStatus: tailoring.StatusApproved},
		{Section: "skills", SuggestedText: "Add go", SkillsAdded: []string{"go"}, UserStatus: tailoring.StatusApproved},
	}

	merged := mergeContent(base, suggestions)
	if len(merged.Skills) != 2 {
		t.Fatalf("expected no duplicate skill (case-insensitive), got %v", merged.Skills)
	}
}

func TestMergeContent_ExperienceBulletReplacedByExactOriginalTextMatch(t *testing.T) {
	base := aiclient.ResumeProfile{
		Experiences: []aiclient.ExperienceEntry{
			{Company: strPtr("Acme"), Bullets: []string{"Built things.", "Led a team."}},
		},
	}
	suggestions := []tailoring.Suggestion{
		{
			Section:       "experience",
			OriginalText:  strPtr("Built things."),
			SuggestedText: "Built scalable event pipelines using Kafka.",
			UserStatus:    tailoring.StatusApproved,
		},
	}

	merged := mergeContent(base, suggestions)
	bullets := merged.Experiences[0].Bullets
	if bullets[0] != "Built scalable event pipelines using Kafka." {
		t.Fatalf("expected matched bullet to be replaced, got %q", bullets[0])
	}
	if bullets[1] != "Led a team." {
		t.Fatalf("expected unrelated bullet to remain untouched, got %q", bullets[1])
	}
}

func TestMergeContent_RejectedSuggestionsNeverApplied(t *testing.T) {
	base := aiclient.ResumeProfile{
		Experiences: []aiclient.ExperienceEntry{{Bullets: []string{"Built things."}}},
	}
	suggestions := []tailoring.Suggestion{
		{Section: "experience", OriginalText: strPtr("Built things."), SuggestedText: "Rejected replacement.", UserStatus: tailoring.StatusRejected},
	}

	merged := mergeContent(base, suggestions)
	if merged.Experiences[0].Bullets[0] != "Built things." {
		t.Fatalf("expected rejected suggestion to leave bullet untouched, got %q", merged.Experiences[0].Bullets[0])
	}
}

func TestMergeContent_EquivalentSkillAliasesAreNotDuplicated(t *testing.T) {
	base := aiclient.ResumeProfile{Skills: []string{"React.js", "Java 21", "PostgreSQL"}}
	suggestions := []tailoring.Suggestion{
		{Section: "skills", SuggestedText: "Add React", SkillsAdded: []string{"React"}, UserStatus: tailoring.StatusApproved},
		{Section: "skills", SuggestedText: "Add Java", SkillsAdded: []string{"Java"}, UserStatus: tailoring.StatusApproved},
		{Section: "skills", SuggestedText: "Add Postgres", SkillsAdded: []string{"Postgres"}, UserStatus: tailoring.StatusApproved},
	}

	merged := mergeContent(base, suggestions)
	if len(merged.Skills) != 3 {
		t.Fatalf("expected equivalent skill aliases to be deduplicated, got %v", merged.Skills)
	}
}

func TestMergeContent_ApprovedAISkillCanBeAddedWithoutExperienceSupport(t *testing.T) {
	base := aiclient.ResumeProfile{Skills: []string{"Java"}}
	suggestions := []tailoring.Suggestion{
		{
			Section:       "skills",
			SuggestedText: "Add Linux",
			SkillsAdded:   []string{"Linux"},
			Source:        "AI_SUGGESTED",
			UserStatus:    tailoring.StatusApproved,
		},
	}

	merged := mergeContent(base, suggestions)
	if len(merged.Skills) != 2 || merged.Skills[1] != "Linux" {
		t.Fatalf("expected approved standalone AI skill to be retained, got %v", merged.Skills)
	}
}

func TestMergeContent_ApprovedSkillAndSupportingExperienceBothApply(t *testing.T) {
	base := aiclient.ResumeProfile{
		Skills: []string{"Kubernetes"},
		Experiences: []aiclient.ExperienceEntry{
			{Bullets: []string{"Managed Kubernetes application deployments."}},
		},
	}
	suggestions := []tailoring.Suggestion{
		{
			Section:       "skills",
			SuggestedText: "Add GitOps",
			SkillsAdded:   []string{"GitOps"},
			Source:        "AI_SUGGESTED",
			UserStatus:    tailoring.StatusApproved,
		},
		{
			Section:       "experience",
			OriginalText:  strPtr("Managed Kubernetes application deployments."),
			SuggestedText: "Managed Kubernetes application deployments using GitOps workflows and Argo CD.",
			SkillsAdded:   []string{"GitOps"},
			Source:        "AI_SUGGESTED",
			UserStatus:    tailoring.StatusApproved,
		},
	}

	merged := mergeContent(base, suggestions)
	if len(merged.Skills) != 2 || merged.Skills[1] != "GitOps" {
		t.Fatalf("expected GitOps skill in final resume, got %v", merged.Skills)
	}
	if merged.Experiences[0].Bullets[0] !=
		"Managed Kubernetes application deployments using GitOps workflows and Argo CD." {
		t.Fatalf("supporting experience rewrite was not applied: %v", merged.Experiences[0].Bullets)
	}
}

func TestMergeContent_PreservesAISelectedSkillCategory(t *testing.T) {
	base := aiclient.ResumeProfile{Skills: []string{"Java"}}
	suggestions := []tailoring.Suggestion{
		{
			Section:         "skills",
			SuggestedText:   "Add Prompt Engineering",
			SkillsAdded:     []string{"Prompt Engineering"},
			SkillCategories: map[string]string{"Prompt Engineering": "AI / GenAI"},
			Source:          "AI_SUGGESTED",
			UserStatus:      tailoring.StatusApproved,
		},
	}

	merged := mergeContent(base, suggestions)
	if len(merged.Skills) != 2 || merged.Skills[1] != "Prompt Engineering" {
		t.Fatalf("expected prompt engineering skill, got %v", merged.Skills)
	}
	if merged.SkillCategories["Prompt Engineering"] != "AI / GenAI" {
		t.Fatalf("expected AI-selected category to survive merge, got %v", merged.SkillCategories)
	}
}

func TestMergeContent_AddExperienceBulletTargetsExistingRole(t *testing.T) {
	base := aiclient.ResumeProfile{
		Experiences: []aiclient.ExperienceEntry{
			{Company: strPtr("CMS"), Title: strPtr("Software Development Engineer"), Bullets: []string{"Existing CMS bullet."}},
			{Company: strPtr("Thoughtworks"), Title: strPtr("Software Developer"), Bullets: []string{"Existing TW bullet."}},
		},
	}
	newBullet := "Built agentic testing workflows using prompt engineering and tool calling."
	suggestions := []tailoring.Suggestion{
		{
			Section:       "experience",
			Operation:     tailoring.OperationAdd,
			SuggestedText: newBullet,
			TargetCompany: strPtr("CMS"),
			TargetTitle:   strPtr("Software Development Engineer"),
			Source:        "AI_SUGGESTED",
			UserStatus:    tailoring.StatusApproved,
		},
	}

	merged := mergeContent(base, suggestions)
	if len(merged.Experiences[0].Bullets) != 2 || merged.Experiences[0].Bullets[1] != newBullet {
		t.Fatalf("expected new bullet under CMS only, got %v", merged.Experiences[0].Bullets)
	}
	if len(merged.Experiences[1].Bullets) != 1 {
		t.Fatalf("unexpected bullet added to other role: %v", merged.Experiences[1].Bullets)
	}
}

func TestMergeContent_PendingOrUnknownRoleAddIsIgnored(t *testing.T) {
	base := aiclient.ResumeProfile{
		Experiences: []aiclient.ExperienceEntry{
			{Company: strPtr("CMS"), Title: strPtr("Software Development Engineer"), Bullets: []string{"Existing."}},
		},
	}
	suggestions := []tailoring.Suggestion{
		{
			Section:       "experience",
			Operation:     tailoring.OperationAdd,
			SuggestedText: "Pending bullet.",
			TargetCompany: strPtr("CMS"),
			TargetTitle:   strPtr("Software Development Engineer"),
			UserStatus:    tailoring.StatusPending,
		},
		{
			Section:       "experience",
			Operation:     tailoring.OperationAdd,
			SuggestedText: "Unknown-role bullet.",
			TargetCompany: strPtr("Invented Company"),
			TargetTitle:   strPtr("Invented Role"),
			UserStatus:    tailoring.StatusApproved,
		},
	}

	merged := mergeContent(base, suggestions)
	if len(merged.Experiences[0].Bullets) != 1 {
		t.Fatalf("expected no new bullets, got %v", merged.Experiences[0].Bullets)
	}
}

func TestMergeContent_DeduplicatesApprovedAddedBullet(t *testing.T) {
	newBullet := "Built agentic testing workflows using prompt engineering and tool calling."
	base := aiclient.ResumeProfile{
		Experiences: []aiclient.ExperienceEntry{
			{Company: strPtr("CMS"), Title: strPtr("Software Development Engineer"), Bullets: []string{newBullet}},
		},
	}
	suggestions := []tailoring.Suggestion{
		{
			Section:       "experience",
			Operation:     tailoring.OperationAdd,
			SuggestedText: newBullet,
			TargetCompany: strPtr("CMS"),
			TargetTitle:   strPtr("Software Development Engineer"),
			UserStatus:    tailoring.StatusApproved,
		},
	}

	merged := mergeContent(base, suggestions)
	if len(merged.Experiences[0].Bullets) != 1 {
		t.Fatalf("expected duplicate add to be ignored, got %v", merged.Experiences[0].Bullets)
	}
}
