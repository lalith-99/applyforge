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

// Generate builds and stores a new profile version for userID from the latest
// parsed resume plus current onboarding/profile/preferences data.
func (s *Service) Generate(ctx context.Context, userID uuid.UUID) (Profile, error) {
	req, sourceHash, err := s.currentSource(ctx, userID)
	if err != nil {
		return Profile{}, err
	}

	synthesized, err := s.aiClient.BuildCandidateProfile(ctx, req)
	if err != nil {
		return Profile{}, fmt.Errorf("synthesize candidate profile: %w", err)
	}

	// Explicit user/profile data wins over an empty synthesis field. This is
	// especially important for ranking direction: a transient AI fallback must
	// never erase target roles, seniority, years, or resume skills that are
	// already present in ApplyForge's source-of-truth records.
	if len(synthesized.TargetRoles) == 0 && len(req.TargetRoles) > 0 {
		synthesized.TargetRoles = append([]string{}, req.TargetRoles...)
	}
	if synthesized.Seniority == nil || strings.TrimSpace(*synthesized.Seniority) == "" {
		synthesized.Seniority = req.Seniority
	}
	if (synthesized.YearsExperience == nil || *synthesized.YearsExperience <= 0) &&
		req.YearsExperience != nil && *req.YearsExperience > 0 {
		years := *req.YearsExperience
		synthesized.YearsExperience = &years
	}
	if len(synthesized.CoreSkills) == 0 && len(req.MasterSkills) > 0 {
		synthesized.CoreSkills = append([]string{}, req.MasterSkills...)
	}
	if strings.TrimSpace(synthesized.Summary) == "" && req.MasterSummary != nil {
		synthesized.Summary = *req.MasterSummary
	}

	transferable := make([]TransferableSkill, 0, len(synthesized.TransferableSkills))
	for _, t := range synthesized.TransferableSkills {
		transferable = append(transferable, TransferableSkill{
			Skill: t.Skill, Evidence: t.Evidence, Strength: t.Strength,
		})
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
		SourceContentHash:     sourceHash,
	}

	return s.repo.Create(ctx, userID, p)
}

// NeedsRebuild compares the latest materialized candidate profile with the
// current resume/onboarding/preferences source data. It repairs older profiles
// that were generated before onboarding was completed without rebuilding a
// healthy profile on every API restart.
func (s *Service) NeedsRebuild(ctx context.Context, userID uuid.UUID) (bool, error) {
	req, sourceHash, err := s.currentSource(ctx, userID)
	if err != nil {
		return false, err
	}

	latest, err := s.repo.GetLatest(ctx, userID)
	if err != nil {
		if err == ErrNotFound {
			return true, nil
		}
		return false, err
	}

	if latest.SourceContentHash != sourceHash {
		return true, nil
	}
	if len(latest.TargetRoles) == 0 && len(req.TargetRoles) > 0 {
		return true, nil
	}
	if (latest.Seniority == nil || strings.TrimSpace(*latest.Seniority) == "") &&
		req.Seniority != nil && strings.TrimSpace(*req.Seniority) != "" {
		return true, nil
	}
	if (latest.YearsExperience == nil || *latest.YearsExperience <= 0) &&
		req.YearsExperience != nil && *req.YearsExperience > 0 {
		return true, nil
	}
	if len(latest.CoreSkills) == 0 && len(req.MasterSkills) > 0 {
		return true, nil
	}
	return false, nil
}

func (s *Service) currentSource(ctx context.Context, userID uuid.UUID) (aiclient.CandidateProfileRequest, string, error) {
	resumes, err := s.resumes.ListForUser(ctx, userID)
	if err != nil {
		return aiclient.CandidateProfileRequest{}, "", fmt.Errorf("list resumes: %w", err)
	}

	// candidate_skills is the normalized/deduped source of truth for what the
	// candidate currently knows.
	skillRows, err := s.candidateSkills.ListForUser(ctx, userID)
	if err != nil {
		return aiclient.CandidateProfileRequest{}, "", fmt.Errorf("list candidate skills: %w", err)
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
		break
	}

	prefs, err := s.preferencesRepo.Get(ctx, userID)
	if err != nil && err != preferences.ErrNotFound {
		return aiclient.CandidateProfileRequest{}, "", fmt.Errorf("load preferences: %w", err)
	}

	prof, err := s.profileRepo.Get(ctx, userID)
	if err != nil && err != profile.ErrNotFound {
		return aiclient.CandidateProfileRequest{}, "", fmt.Errorf("load profile: %w", err)
	}

	targetRoles := dedupeNonEmpty(append(
		append([]string{}, prof.PrimaryTargetTitles...),
		prof.AlternativeTargetTitles...,
	))

	req := aiclient.CandidateProfileRequest{
		TargetRoles:           targetRoles,
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
	return req, candidateSourceHash(req), nil
}

// EmbeddingText builds a normalized text representation of a profile for
// semantic embedding.
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

func candidateSourceHash(req aiclient.CandidateProfileRequest) string {
	h := sha256.New()
	write := func(value string) {
		h.Write([]byte(value))
		h.Write([]byte{0})
	}
	write(strings.Join(req.TargetRoles, "|"))
	write(derefOr(req.Seniority, ""))
	if req.YearsExperience != nil {
		write(fmt.Sprintf("%d", *req.YearsExperience))
	} else {
		write("")
	}
	write(strings.Join(req.MasterSkills, "|"))
	write(derefOr(req.MasterSummary, ""))
	write(strings.Join(req.PreferredIndustries, "|"))
	write(strings.Join(req.PreferredTechnologies, "|"))
	write(derefOr(req.WorkAuthorization, ""))
	write(derefOr(req.ImmigrationStatus, ""))
	for _, exp := range req.Experiences {
		write(exp.Company)
		write(exp.Title)
		write(strings.Join(exp.Bullets, "|"))
		write(strings.Join(exp.Technologies, "|"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func dedupeNonEmpty(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
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
