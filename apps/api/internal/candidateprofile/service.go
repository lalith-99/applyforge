package candidateprofile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/candidateskills"
	"github.com/lalithlochan/applyforge/apps/api/internal/preferences"
	"github.com/lalithlochan/applyforge/apps/api/internal/profile"
	"github.com/lalithlochan/applyforge/apps/api/internal/resume"
)

// Service gathers a candidate's resume/skills/preferences/profile data,
// synthesizes a CandidateIntelligenceProfile via the AI worker, and
// materializes it as a new version.
type Service struct {
	repo            *Repository
	resumes         *resume.Repository
	candidateSkills *candidateskills.Repository
	preferencesRepo *preferences.Repository
	profileRepo     *profile.Repository
	aiClient        *aiclient.Client
}

// NewService builds a Service.
func NewService(repo *Repository, resumes *resume.Repository, candidateSkillsRepo *candidateskills.Repository, preferencesRepo *preferences.Repository, profileRepo *profile.Repository, aiClient *aiclient.Client) *Service {
	return &Service{
		repo:            repo,
		resumes:         resumes,
		candidateSkills: candidateSkillsRepo,
		preferencesRepo: preferencesRepo,
		profileRepo:     profileRepo,
		aiClient:        aiClient,
	}
}

// Generate builds and stores a new profile version for userID from their
// most recently parsed resume plus current preferences/profile data. Safe
// to call repeatedly (e.g. after every resume re-parse or preferences
// change) - content_hash-based skipping isn't done here since generation is
// already only triggered on genuine upstream changes (see callers), unlike
// the JD-parsing cache which is queried far more often than its inputs change.
func (s *Service) Generate(ctx context.Context, userID uuid.UUID) (Profile, error) {
	resumes, err := s.resumes.ListForUser(ctx, userID)
	if err != nil {
		return Profile{}, fmt.Errorf("list resumes: %w", err)
	}

	// candidate_skills is the normalized/deduped source of truth for "what
	// the candidate knows" (it's what the resume parser itself populates,
	// then a user can edit/approve) - preferred over reading raw
	// ResumeProfile.Skills again, which is a coarser earlier-stage view.
	skillRows, err := s.candidateSkills.ListForUser(ctx, userID)
	if err != nil {
		return Profile{}, fmt.Errorf("list candidate skills: %w", err)
	}
	masterSkills := make([]string, 0, len(skillRows))
	for _, sk := range skillRows {
		masterSkills = append(masterSkills, sk.DisplayName)
	}

	var masterSummary *string
	var experiences []aiclient.CandidateExperience
	for _, r := range resumes {
		if r.Status != resume.StatusParsed {
			continue
		}
		var parsed aiclient.ResumeProfile
		if err := unmarshalProfile(r.ParsedProfile, &parsed); err != nil {
			continue
		}
		masterSummary = parsed.Summary
		for _, e := range parsed.Experiences {
			experiences = append(experiences, aiclient.CandidateExperience{
				Company:      derefOr(e.Company, ""),
				Title:        derefOr(e.Title, ""),
				Bullets:      emptyIfNil(e.Bullets),
				Technologies: append(append([]string{}, e.DetectedSkills...), e.Technologies...),
			})
		}
		break // ListForUser is newest-first; only the latest resume feeds the profile
	}

	prefs, err := s.preferencesRepo.Get(ctx, userID)
	if err != nil && err != preferences.ErrNotFound {
		return Profile{}, fmt.Errorf("load preferences: %w", err)
	}

	prof, err := s.profileRepo.Get(ctx, userID)
	if err != nil && err != profile.ErrNotFound {
		return Profile{}, fmt.Errorf("load profile: %w", err)
	}

	req := aiclient.CandidateProfileRequest{
		TargetRoles:           emptyIfNil(prof.PrimaryTargetTitles),
		Seniority:             prof.Seniority,
		YearsExperience:       int32PtrToIntPtr(prof.YearsExperience),
		MasterSkills:          emptyIfNil(masterSkills),
		MasterSummary:         masterSummary,
		Experiences:           emptyIfNil(experiences),
		PreferredIndustries:   emptyIfNil(prof.PreferredIndustries),
		PreferredTechnologies: emptyIfNil(prof.PreferredTechnologies),
		WorkAuthorization:     prefs.WorkAuthorization,
		ImmigrationStatus:     prefs.ImmigrationStatus,
	}

	synthesized, err := s.aiClient.BuildCandidateProfile(ctx, req)
	if err != nil {
		return Profile{}, fmt.Errorf("synthesize candidate profile: %w", err)
	}

	transferable := make([]TransferableSkill, 0, len(synthesized.TransferableSkills))
	for _, t := range synthesized.TransferableSkills {
		transferable = append(transferable, TransferableSkill{Skill: t.Skill, Evidence: t.Evidence, Strength: t.Strength})
	}

	p := Profile{
		TargetRoles:           synthesized.TargetRoles,
		Seniority:             synthesized.Seniority,
		YearsExperience:       intPtrToInt32Ptr(synthesized.YearsExperience),
		CoreSkills:            synthesized.CoreSkills,
		SecondarySkills:       synthesized.SecondarySkills,
		TransferableSkills:    transferable,
		Domains:               synthesized.Domains,
		ArchitectureStrengths: synthesized.ArchitectureStrengths,
		LeadershipSignals:     synthesized.LeadershipSignals,
		ExperienceEvidence:    synthesized.ExperienceEvidence,
		Summary:               synthesized.Summary,
		SourceContentHash:     contentHash(masterSkills, masterSummary, prof, prefs),
	}

	return s.repo.Create(ctx, userID, p)
}

// EmbeddingText builds a normalized text representation of a profile for
// semantic embedding (Phase G's candidate-side retrieval input).
func EmbeddingText(p Profile) string {
	var b strings.Builder
	if p.Seniority != nil {
		b.WriteString(*p.Seniority + " ")
	}
	b.WriteString(strings.Join(p.TargetRoles, ", "))
	b.WriteString(".\n\n")
	b.WriteString(p.Summary)
	b.WriteString("\n\nCore skills: ")
	b.WriteString(strings.Join(p.CoreSkills, ", "))
	if len(p.Domains) > 0 {
		b.WriteString("\nDomains: ")
		b.WriteString(strings.Join(p.Domains, ", "))
	}
	return b.String()
}

func contentHash(masterSkills []string, masterSummary *string, prof profile.Profile, prefs preferences.Preferences) string {
	h := sha256.New()
	h.Write([]byte(strings.Join(masterSkills, "|")))
	h.Write([]byte(derefOr(masterSummary, "")))
	h.Write([]byte(strings.Join(prof.PrimaryTargetTitles, "|")))
	h.Write([]byte(derefOr(prof.Seniority, "")))
	h.Write([]byte(derefOr(prefs.WorkAuthorization, "")))
	return hex.EncodeToString(h.Sum(nil))
}

func derefOr(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}

func int32PtrToIntPtr(v *int32) *int {
	if v == nil {
		return nil
	}
	i := int(*v)
	return &i
}

func intPtrToInt32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}
	i := int32(*v)
	return &i
}

func unmarshalProfile(raw []byte, out *aiclient.ResumeProfile) error {
	if len(raw) == 0 {
		return fmt.Errorf("empty parsed profile")
	}
	return json.Unmarshal(raw, out)
}
