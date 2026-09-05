package learning

import (
	"testing"

	"github.com/lalithlochan/applyforge/apps/api/internal/matching"
)

func TestInterviewReadiness_StrongMatchScoresHigh(t *testing.T) {
	result := matching.Result{
		MatchedSkills:         []string{"go", "postgresql", "kafka"},
		MissingRequiredSkills: nil,
		TransferableSkills:    nil,
		Components: matching.ComponentScores{
			ResponsibilityAlignment: 20,
			MustHaveSkillCoverage:   30,
			DomainAlignment:         10,
			PreferredSkills:         10,
		},
	}

	score, components := InterviewReadiness(result)
	if score < 90 {
		t.Fatalf("expected a high readiness score for a strong match, got %d", score)
	}
	if components.RequiredTechnology <= 0 {
		t.Fatalf("expected a positive RequiredTechnology component, got %v", components.RequiredTechnology)
	}
}

func TestInterviewReadiness_WeakMatchScoresLow(t *testing.T) {
	result := matching.Result{
		MatchedSkills:         []string{"go"},
		MissingRequiredSkills: []string{"kafka", "kubernetes", "postgresql"},
		Components: matching.ComponentScores{
			ResponsibilityAlignment: 2,
			MustHaveSkillCoverage:   5,
			DomainAlignment:         1,
			PreferredSkills:         1,
		},
	}

	strong := matching.Result{
		MatchedSkills: []string{"go", "kafka", "kubernetes", "postgresql"},
		Components: matching.ComponentScores{
			ResponsibilityAlignment: 20,
			MustHaveSkillCoverage:   30,
			DomainAlignment:         10,
			PreferredSkills:         10,
		},
	}

	weakScore, _ := InterviewReadiness(result)
	strongScore, _ := InterviewReadiness(strong)

	if weakScore >= strongScore {
		t.Fatalf("expected weak match readiness (%d) to be lower than strong match readiness (%d)", weakScore, strongScore)
	}
}

func TestInterviewReadiness_ScoreBoundedZeroToHundred(t *testing.T) {
	score, _ := InterviewReadiness(matching.Result{})
	if score < 0 || score > 100 {
		t.Fatalf("expected score in [0,100], got %d", score)
	}
}
