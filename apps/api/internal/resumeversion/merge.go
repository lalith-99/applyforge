package resumeversion

import (
	"strings"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
	"github.com/lalithlochan/applyforge/apps/api/internal/tailoring"
)

// mergeContent applies approved/edited tailoring suggestions onto a base
// resume profile to produce the final content for a resume version. Pure
// and DB-free so it's fully unit-testable in isolation from persistence.
func resumeSkillKey(skill string) string {
	key := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(skill)), " "))
	switch key {
	case "react.js", "reactjs":
		return "react"
	case "node.js", "node js":
		return "nodejs"
	case "golang", "go (golang)":
		return "go"
	case "postgres":
		return "postgresql"
	case "amazon web services":
		return "aws"
	case "apache kafka":
		return "kafka"
	case "argo cd":
		return "argocd"
	}

	for _, prefix := range []string{"java", "spring boot", "angular", "python"} {
		if strings.HasPrefix(key, prefix+" ") {
			suffix := strings.TrimPrefix(key, prefix+" ")
			if suffix != "" {
				numeric := true
				for _, r := range suffix {
					if (r < '0' || r > '9') && r != '.' {
						numeric = false
						break
					}
				}
				if numeric {
					return prefix
				}
			}
		}
	}
	return key
}

func categoryForSkill(skill string, categories map[string]string) string {
	if len(categories) == 0 {
		return ""
	}
	key := resumeSkillKey(skill)
	for candidate, category := range categories {
		if resumeSkillKey(candidate) == key {
			return category
		}
	}
	return ""
}

func sameRole(exp aiclient.ExperienceEntry, company, title *string) bool {
	if company == nil && title == nil {
		return false
	}
	if company != nil {
		if exp.Company == nil || !strings.EqualFold(strings.TrimSpace(*exp.Company), strings.TrimSpace(*company)) {
			return false
		}
	}
	if title != nil {
		if exp.Title == nil || !strings.EqualFold(strings.TrimSpace(*exp.Title), strings.TrimSpace(*title)) {
			return false
		}
	}
	return true
}

func containsBullet(bullets []string, text string) bool {
	needle := strings.TrimSpace(text)
	for _, bullet := range bullets {
		if strings.EqualFold(strings.TrimSpace(bullet), needle) {
			return true
		}
	}
	return false
}

func mergeContent(base aiclient.ResumeProfile, suggestions []tailoring.Suggestion) aiclient.ResumeProfile {
	merged := base
	merged.Skills = append([]string{}, base.Skills...)
	merged.SkillCategories = make(map[string]string, len(base.SkillCategories))
	for skill, category := range base.SkillCategories {
		merged.SkillCategories[skill] = category
	}
	merged.Experiences = make([]aiclient.ExperienceEntry, len(base.Experiences))
	for i, exp := range base.Experiences {
		merged.Experiences[i] = exp
		merged.Experiences[i].Bullets = append([]string{}, exp.Bullets...)
	}

	existingSkills := make(map[string]bool, len(merged.Skills))
	for _, s := range merged.Skills {
		existingSkills[resumeSkillKey(s)] = true
	}
	for _, s := range suggestions {
		if s.UserStatus != tailoring.StatusApproved && s.UserStatus != tailoring.StatusEdited {
			continue
		}
		finalText := s.SuggestedText
		if s.EditedText != nil {
			finalText = *s.EditedText
		}

		switch s.Section {
		case "summary":
			merged.Summary = &finalText
		case "skills":
			for _, skill := range s.SkillsAdded {
				key := resumeSkillKey(skill)
				if !existingSkills[key] {
					merged.Skills = append(merged.Skills, skill)
					existingSkills[key] = true
				}
				if category := categoryForSkill(skill, s.SkillCategories); category != "" {
					merged.SkillCategories[skill] = category
				}
			}
		case "experience":
			if s.Operation == tailoring.OperationAdd {
				for i := range merged.Experiences {
					if !sameRole(merged.Experiences[i], s.TargetCompany, s.TargetTitle) {
						continue
					}
					if !containsBullet(merged.Experiences[i].Bullets, finalText) {
						merged.Experiences[i].Bullets = append(merged.Experiences[i].Bullets, finalText)
					}
					break
				}
				continue
			}

			// REWRITE: original_text is the exact bullet text the AI worker was
			// given, so it can be matched back to the bullet it replaces.
			if s.OriginalText == nil {
				continue
			}
			for i := range merged.Experiences {
				for j, bullet := range merged.Experiences[i].Bullets {
					if bullet == *s.OriginalText {
						merged.Experiences[i].Bullets[j] = finalText
					}
				}
			}
		}
	}

	return merged
}
