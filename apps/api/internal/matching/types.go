// Package matching implements deterministic job/candidate scoring (see
// MASTER_REQUIREMENTS.md §19-§22). Scoring is never delegated to an LLM —
// only upstream extraction (JD parsing, transferable-skill data) uses AI or
// curated data; this package is pure, testable arithmetic over that data.
package matching

import "time"

// SkillRequirement is a normalized requirement extracted from a job posting.
type SkillRequirement struct {
	NormalizedName string
	Importance     string // "required" | "preferred"
}

// TransferableSkill describes how much a known skill transfers toward a
// missing one (seeded in the transferable_skills table).
type TransferableSkill struct {
	SourceSkill          string
	TargetSkill          string
	TransferabilityScore int
	Level                string // VERY_HIGH | HIGH | MEDIUM | LOW | NONE
	PrepClassification   string // QUICK_PREP | STANDARD_PREP | DEEPER_GAP
}

// Input bundles everything the scorer needs. All fields are plain data so
// the scorer has no database/HTTP dependencies and is fully unit-testable.
type Input struct {
	// Candidate
	CandidateSkills          map[string]bool // normalized skill name -> present on resume/profile
	CandidateTargetSkills    map[string]bool // normalized skill name -> user-approved target skill
	TransferableFromSkills   []TransferableSkill
	CandidateSeniority       string
	PreferredRemote          bool
	PreferredHybrid          bool
	PreferredOnsite          bool
	PreferredEmploymentTypes []string
	ExcludedCompanies        []string
	ExcludedLocations        []string

	// Job
	CompanyName      string
	LocationText     string
	RemoteType       string
	EmploymentType   string
	JobSeniority     string
	RequiredSkills   []SkillRequirement
	PreferredSkills  []SkillRequirement
	Responsibilities []string
	HasEducationReqs bool
	HasCertReqs      bool
	PostedAt         *time.Time
	FirstSeenAt      time.Time
}

// ComponentScores breaks total_score down per MASTER_REQUIREMENTS.md §20.
type ComponentScores struct {
	MustHaveSkillCoverage   float64
	ResponsibilityAlignment float64
	RoleSeniority           float64
	PreferredSkills         float64
	DomainAlignment         float64
	LocationWorkArrangement float64
	EducationCertifications float64
	CandidatePreferences    float64
}

// Total sums the weighted components (already weighted, so this is a plain sum).
func (c ComponentScores) Total() float64 {
	return c.MustHaveSkillCoverage + c.ResponsibilityAlignment + c.RoleSeniority +
		c.PreferredSkills + c.DomainAlignment + c.LocationWorkArrangement +
		c.EducationCertifications + c.CandidatePreferences
}

// TransferableMatch is a skill the candidate doesn't have directly but has a
// meaningful transferable path toward.
type TransferableMatch struct {
	SourceSkill        string
	TargetSkill        string
	Level              string
	PrepClassification string
}

// Result is the full deterministic assessment of a candidate against a job.
type Result struct {
	TotalScore               int
	Grade                    string
	Components               ComponentScores
	MatchedSkills            []string
	TransferableSkills       []TransferableMatch
	MissingRequiredSkills    []string
	MissingPreferredSkills   []string
	PositiveEvidence         []string
	Concerns                 []string
	Explanation              string
	OpportunityScore         int
	CurrentProfileMatch      int
	TargetProfileMatch       int
	SuggestedTargetAdditions []string
	Eligibility              EligibilityResult
}

// EligibilityResult is computed before scoring (see MASTER_REQUIREMENTS.md §19).
type EligibilityResult struct {
	Eligible     bool
	HardFailures []string
	Warnings     []string
}

// Grade thresholds (configurable — see MASTER_REQUIREMENTS.md §20).
var GradeThresholds = []struct {
	Min   int
	Grade string
}{
	{95, "Exceptional"},
	{90, "Excellent"},
	{80, "Strong"},
	{70, "Possible"},
	{60, "Weak"},
	{0, "Poor"},
}

func gradeFor(score int) string {
	for _, t := range GradeThresholds {
		if score >= t.Min {
			return t.Grade
		}
	}
	return "Poor"
}
