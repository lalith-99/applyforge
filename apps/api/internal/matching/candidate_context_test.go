package matching

import (
	"slices"
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/candidateskills"
)

func TestBuildCandidateSkillSets_SeparatesCurrentFromTargetSkills(t *testing.T) {
	current, target, transferableKeys := buildCandidateSkillSets([]candidateskills.Skill{
		{NormalizedName: "Go", Status: candidateskills.StatusVerifiedProfessional},
		{NormalizedName: "Kafka", Status: candidateskills.StatusVerifiedProject},
		{NormalizedName: "Spring AI", Status: candidateskills.StatusTargetSkill},
		{NormalizedName: "Next.js", Status: candidateskills.StatusUserApproved},
	})

	if !current["go"] || !current["kafka"] {
		t.Fatalf("expected verified skills in current set: %#v", current)
	}
	if current["spring ai"] || current["next.js"] {
		t.Fatalf("target-only skills must not be current capabilities: %#v", current)
	}
	if !target["spring ai"] || !target["next.js"] {
		t.Fatalf("expected target/user-approved skills in target set: %#v", target)
	}
	if !slices.Contains(transferableKeys, "go") || !slices.Contains(transferableKeys, "kafka") {
		t.Fatalf("verified skills should seed transferability: %#v", transferableKeys)
	}
	if slices.Contains(transferableKeys, "spring ai") || slices.Contains(transferableKeys, "next.js") {
		t.Fatalf("target-only skills must not seed transferable-skill credit: %#v", transferableKeys)
	}
}

func TestBuildCandidateSkillSets_NormalizesAndSkipsEmptyNames(t *testing.T) {
	current, _, transferableKeys := buildCandidateSkillSets([]candidateskills.Skill{
		{NormalizedName: "  PostgreSQL  ", Status: candidateskills.StatusFamiliar},
		{NormalizedName: " ", Status: candidateskills.StatusVerifiedProfessional},
	})

	if !current["postgresql"] {
		t.Fatalf("expected normalized current skill: %#v", current)
	}
	if len(transferableKeys) != 1 || transferableKeys[0] != "postgresql" {
		t.Fatalf("unexpected transferable keys: %#v", transferableKeys)
	}
}
