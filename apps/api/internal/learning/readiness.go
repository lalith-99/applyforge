package learning

import "github.com/lalithlochan/applyforge/apps/api/internal/matching"

// ReadinessComponents breaks Interview Readiness down per MASTER_REQUIREMENTS.md §35.
// This is product guidance, not a scientific assessment — it is derived from the
// same deterministic match Result already computed for the job, since there is
// no dedicated signal (yet) for "question preparedness" usage, etc.
type ReadinessComponents struct {
	CoreLanguage         float64
	BackendFundamentals  float64
	RequiredTechnology   float64
	SystemDesignDomain   float64
	ExperienceExamples   float64
	QuestionPreparedness float64
}

// InterviewReadiness computes a deterministic 0-100 Interview Readiness score
// and its component breakdown from an already-computed job Match result.
func InterviewReadiness(result matching.Result) (int, ReadinessComponents) {
	requiredTotal := len(result.MatchedSkills) + len(result.MissingRequiredSkills) + len(result.TransferableSkills)
	requiredCoverage := 0.7
	if requiredTotal > 0 {
		requiredCoverage = float64(len(result.MatchedSkills)) / float64(requiredTotal)
	}

	components := ReadinessComponents{
		CoreLanguage:         20 * requiredCoverage,
		BackendFundamentals:  20 * ratio(result.Components.ResponsibilityAlignment, 20),
		RequiredTechnology:   25 * ratio(result.Components.MustHaveSkillCoverage, 30),
		SystemDesignDomain:   15 * ratio(result.Components.DomainAlignment, 10),
		ExperienceExamples:   10 * ratio(result.Components.PreferredSkills, 10),
		QuestionPreparedness: 10 * 0.7, // no usage-tracking signal yet; neutral-leaning-positive default
	}

	total := components.CoreLanguage + components.BackendFundamentals + components.RequiredTechnology +
		components.SystemDesignDomain + components.ExperienceExamples + components.QuestionPreparedness

	score := int(total + 0.5)
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score, components
}

// ratio computes component/weight as a 0-1 fraction, defaulting to a neutral
// value when the weight is zero (shouldn't happen with fixed weights, but
// keeps this safe if MASTER weights ever change).
func ratio(component, weight float64) float64 {
	if weight == 0 {
		return 0.7
	}
	r := component / weight
	if r > 1 {
		r = 1
	}
	if r < 0 {
		r = 0
	}
	return r
}
