package resumeversion
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
