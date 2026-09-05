package matching

import (
	"testing"
	"time"
)

// Golden matching fixtures (see MASTER_REQUIREMENTS.md §55). Expressed as Go
// literals rather than external JSON files since Input has no DB/HTTP
// dependencies — this keeps the fixtures and assertions in one auditable
// place while still covering every required scenario.

func candidateGoBackend() (skills map[string]bool, seniority string) {
	return skillSet("Go", "Kafka", "PostgreSQL", "Docker", "Kubernetes", "AWS"), "senior"
}

func candidateJavaBackend() (skills map[string]bool, seniority string) {
	return skillSet("Java", "Spring", "Spring Boot", "MySQL", "Docker"), "senior"
}

func candidateFrontend() (skills map[string]bool, seniority string) {
	return skillSet("React", "TypeScript", "JavaScript", "CSS", "GraphQL"), "mid"
}

func skillSet(skills ...string) map[string]bool {
	set := make(map[string]bool, len(skills))
	for _, s := range skills {
		set[normalizeKey(s)] = true
	}
	return set
}

func normalizeKey(s string) string { return toLower(s) }

func toLower(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		out = append(out, r)
	}
	return string(out)
}

func jobGoKafkaAWS() Input {
	return Input{
		CompanyName:  "Acme",
		RemoteType:   "remote",
		JobSeniority: "senior",
		RequiredSkills: []SkillRequirement{
			{NormalizedName: "go", Importance: "required"},
			{NormalizedName: "kafka", Importance: "required"},
			{NormalizedName: "postgresql", Importance: "required"},
		},
		PreferredSkills: []SkillRequirement{
			{NormalizedName: "kubernetes", Importance: "preferred"},
			{NormalizedName: "aws", Importance: "preferred"},
		},
		Responsibilities: []string{
			"Build distributed backend services in go processing kafka events",
			"Operate postgresql-backed services in production",
		},
		FirstSeenAt: time.Now(),
	}
}

func jobJavaSpring() Input {
	return Input{
		CompanyName:  "Acme",
		RemoteType:   "hybrid",
		JobSeniority: "senior",
		RequiredSkills: []SkillRequirement{
			{NormalizedName: "java", Importance: "required"},
			{NormalizedName: "spring", Importance: "required"},
			{NormalizedName: "mysql", Importance: "required"},
		},
		PreferredSkills: []SkillRequirement{
			{NormalizedName: "docker", Importance: "preferred"},
		},
		Responsibilities: []string{
			"Build enterprise services in java using spring",
		},
		FirstSeenAt: time.Now(),
	}
}

func jobReactFrontend() Input {
	return Input{
		CompanyName:  "Acme",
		RemoteType:   "remote",
		JobSeniority: "mid",
		RequiredSkills: []SkillRequirement{
			{NormalizedName: "react", Importance: "required"},
			{NormalizedName: "typescript", Importance: "required"},
			{NormalizedName: "graphql", Importance: "required"},
		},
		Responsibilities: []string{
			"Build react and typescript UIs consuming graphql apis",
		},
		FirstSeenAt: time.Now(),
	}
}

func jobGoSQSDynamoDB() Input {
	return Input{
		CompanyName:  "Acme",
		RemoteType:   "remote",
		JobSeniority: "senior",
		RequiredSkills: []SkillRequirement{
			{NormalizedName: "go", Importance: "required"},
			{NormalizedName: "amazon sqs", Importance: "required"},
			{NormalizedName: "dynamodb", Importance: "required"},
		},
		Responsibilities: []string{
			"Build go services using amazon sqs and dynamodb",
		},
		FirstSeenAt: time.Now(),
	}
}

func withCandidate(job Input, skills map[string]bool, seniority string) Input {
	job.CandidateSkills = skills
	job.CandidateSeniority = seniority
	return job
}

// kafkaToSQS / postgresToDynamo mirror the seeded transferable_skills rows so
// tests don't depend on the database.
func kafkaToSQS() TransferableSkill {
	return TransferableSkill{SourceSkill: "Kafka", TargetSkill: "Amazon SQS", TransferabilityScore: 55, Level: "MEDIUM", PrepClassification: "QUICK_PREP"}
}

func postgresToDynamo() TransferableSkill {
	return TransferableSkill{SourceSkill: "PostgreSQL", TargetSkill: "DynamoDB", TransferabilityScore: 30, Level: "LOW", PrepClassification: "STANDARD_PREP"}
}

func TestGolden_GoKafkaCandidate_ScoresStronglyAgainstGoKafkaRole(t *testing.T) {
	skills, seniority := candidateGoBackend()
	result := Score(withCandidate(jobGoKafkaAWS(), skills, seniority))

	if result.TotalScore < 80 {
		t.Fatalf("expected a strong score (>=80), got %d: %+v", result.TotalScore, result.Components)
	}
	if len(result.MissingRequiredSkills) != 0 {
		t.Fatalf("expected no missing required skills, got %v", result.MissingRequiredSkills)
	}
}

func TestGolden_JavaCandidate_ScoresStronglyAgainstJavaSpringRole(t *testing.T) {
	skills, seniority := candidateJavaBackend()
	result := Score(withCandidate(jobJavaSpring(), skills, seniority))

	if result.TotalScore < 80 {
		t.Fatalf("expected a strong score (>=80), got %d: %+v", result.TotalScore, result.Components)
	}
}

func TestGolden_ReactCandidate_DoesNotScoreHighlyAgainstGoBackendRole(t *testing.T) {
	skills, seniority := candidateFrontend()
	result := Score(withCandidate(jobGoKafkaAWS(), skills, seniority))

	if result.TotalScore >= 70 {
		t.Fatalf("expected a low score (<70) for a frontend candidate against a Go backend role, got %d", result.TotalScore)
	}
}

func TestGolden_KafkaTransfersTowardSQS_ButIsNotIdenticalCredit(t *testing.T) {
	skills, seniority := candidateGoBackend() // has Kafka, not Amazon SQS
	input := withCandidate(jobGoSQSDynamoDB(), skills, seniority)
	input.TransferableFromSkills = []TransferableSkill{kafkaToSQS(), postgresToDynamo()}

	withTransfer := Score(input)

	// Same job, but candidate has Amazon SQS directly instead of via transfer.
	directSkills, _ := candidateGoBackend()
	directSkills["amazon sqs"] = true
	directInput := withCandidate(jobGoSQSDynamoDB(), directSkills, seniority)
	directInput.TransferableFromSkills = input.TransferableFromSkills
	withDirect := Score(directInput)

	if len(withTransfer.TransferableSkills) == 0 {
		t.Fatalf("expected at least one transferable skill match for Kafka -> Amazon SQS")
	}
	if withTransfer.TotalScore >= withDirect.TotalScore {
		t.Fatalf("expected transferable credit (%d) to score lower than direct skill credit (%d)", withTransfer.TotalScore, withDirect.TotalScore)
	}
	if withTransfer.CurrentProfileMatch >= withDirect.CurrentProfileMatch {
		t.Fatalf("transferable skills must not count toward Current Profile Match: got current=%d for transfer case vs %d for direct",
			withTransfer.CurrentProfileMatch, withDirect.CurrentProfileMatch)
	}
}

func TestGolden_PostgresTransfersTowardDynamoDB_ButIsNotIdenticalCredit(t *testing.T) {
	skills, seniority := candidateGoBackend() // has PostgreSQL, not DynamoDB
	input := withCandidate(jobGoSQSDynamoDB(), skills, seniority)
	input.TransferableFromSkills = []TransferableSkill{kafkaToSQS(), postgresToDynamo()}

	result := Score(input)

	found := false
	for _, tr := range result.TransferableSkills {
		if tr.SourceSkill == "PostgreSQL" && tr.TargetSkill == "dynamodb" {
			found = true
			if tr.Level != "LOW" {
				t.Fatalf("expected LOW transferability level for PostgreSQL -> DynamoDB, got %s", tr.Level)
			}
		}
	}
	if !found {
		t.Fatalf("expected a PostgreSQL -> DynamoDB transferable match, got %+v", result.TransferableSkills)
	}
	if contains(result.MatchedSkills, "dynamodb") {
		t.Fatalf("DynamoDB must not appear as a directly matched skill when only transferable")
	}
}

func TestGolden_SmallWordingChanges_DoNotCauseUnstableScoreSwings(t *testing.T) {
	skills, seniority := candidateGoBackend()

	base := jobGoKafkaAWS()
	reworded := jobGoKafkaAWS()
	reworded.Responsibilities = []string{
		"Build distributed backend services in Go processing Kafka events!",
		"Operate PostgreSQL-backed services in production.",
	}

	scoreBase := Score(withCandidate(base, skills, seniority)).TotalScore
	scoreReworded := Score(withCandidate(reworded, skills, seniority)).TotalScore

	diff := scoreBase - scoreReworded
	if diff < 0 {
		diff = -diff
	}
	if diff > 2 {
		t.Fatalf("expected minor wording changes to leave score essentially unchanged, got %d vs %d", scoreBase, scoreReworded)
	}
}

func TestEligibility_ExcludedCompany_IsHardFailure(t *testing.T) {
	skills, seniority := candidateGoBackend()
	input := withCandidate(jobGoKafkaAWS(), skills, seniority)
	input.ExcludedCompanies = []string{"Acme"}

	result := Score(input)
	if result.Eligibility.Eligible {
		t.Fatalf("expected excluded company to be a hard failure")
	}
	if result.OpportunityScore != 0 {
		t.Fatalf("expected opportunity score to be 0 when ineligible, got %d", result.OpportunityScore)
	}
}

func contains(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}
