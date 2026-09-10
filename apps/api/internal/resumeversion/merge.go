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
	key := strings.ToLower(strings.TrimSpace(skill))
	switch key {
	case "react.js", "reactjs":
		return "react"
	case "node.js", "node js":
		return "nodejs"
	case "golang":
		return "go"
	case "postgres":
		return "postgresql"
	case "amazon web services":
		return "aws"
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

func mergeContent(base aiclient.ResumeProfile, suggestions []tailoring.Suggestion) aiclient.ResumeProfile {
	merged := base
	merged.Skills = append([]string{}, base.Skills...)
	merged.Experiences = append([]aiclient.ExperienceEntry{}, base.Experiences...)

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
			}
		case "experience":
			// original_text is the exact bullet text the AI worker was given,
			// so it can be matched back to the bullet it replaces.
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
