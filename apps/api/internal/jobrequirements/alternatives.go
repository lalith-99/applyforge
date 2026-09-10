package jobrequirements

import (
	"regexp"
	"sort"
	"strings"

	"github.com/lalithlochan/applyforge/apps/api/internal/aiclient"
)

// Some job requirements are alternatives rather than AND requirements, e.g.
// "Expertise in Python or Golang" or "one or more of Python, Go, Rust, Java,
// C++". Flattening those into independent required skills either over-penalizes
// candidates or, after the previous workaround, removes the requirement
// entirely. Alternative groups preserve the actual boolean semantics while
// still fitting inside the existing JSONB skill-requirement payload.
type alternativeSkillDef struct {
	Display string
	Aliases []string
}

var alternativeSkillDefs = []alternativeSkillDef{
	{Display: "Python", Aliases: []string{"python"}},
	{Display: "Go", Aliases: []string{"golang", "go"}},
	{Display: "Rust", Aliases: []string{"rust"}},
	{Display: "Java", Aliases: []string{"java"}},
	{Display: "C++", Aliases: []string{"c++"}},
	{Display: "C#", Aliases: []string{"c#"}},
	{Display: "JavaScript", Aliases: []string{"javascript", "js"}},
	{Display: "TypeScript", Aliases: []string{"typescript"}},
	{Display: "AWS", Aliases: []string{"amazon web services", "aws"}},
	{Display: "Azure", Aliases: []string{"microsoft azure", "azure"}},
	{Display: "GCP", Aliases: []string{"google cloud platform", "google cloud", "gcp"}},
}

var (
	alternativeMarkerPattern = regexp.MustCompile(`(?i)\b(one\s+or\s+more|one\s+of|any\s+of|either)\b`)
	directOrPattern          = regexp.MustCompile(`(?i)\b(?:python|golang|go|rust|java|javascript|typescript|aws|azure|gcp|c\+\+|c#)\b\s*(?:/|,)?\s*or\s*\b(?:python|golang|go|rust|java|javascript|typescript|aws|azure|gcp|c\+\+|c#)\b`)
	clauseSplitPattern       = regexp.MustCompile(`(?:\r?\n|\.\s+|\s+-\s+)`)
)

func canonicalAlternativeSkill(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	for _, def := range alternativeSkillDefs {
		for _, alias := range def.Aliases {
			if value == alias {
				return def.Display
			}
		}
	}
	return strings.TrimSpace(raw)
}

func clauseAlternativeSkills(clause string) []string {
	lower := strings.ToLower(clause)
	found := make([]string, 0, 5)
	seen := map[string]bool{}
	for _, def := range alternativeSkillDefs {
		for _, alias := range def.Aliases {
			pattern := regexp.MustCompile(`(?i)(?:^|[^a-z0-9+#.])` + regexp.QuoteMeta(alias) + `(?:$|[^a-z0-9+#])`)
			if pattern.MatchString(lower) {
				if !seen[def.Display] {
					found = append(found, def.Display)
					seen[def.Display] = true
				}
				break
			}
		}
	}
	return found
}

func extractAlternativeRequirements(description string) []aiclient.SkillRequirement {
	var groups []aiclient.SkillRequirement
	seen := map[string]bool{}

	for _, rawClause := range clauseSplitPattern.Split(description, -1) {
		clause := strings.TrimSpace(strings.Trim(rawClause, "-*• \t"))
		if clause == "" {
			continue
		}

		isAlternative := alternativeMarkerPattern.MatchString(clause) || directOrPattern.MatchString(clause)
		if !isAlternative {
			continue
		}
		alternatives := clauseAlternativeSkills(clause)
		if len(alternatives) < 2 {
			continue
		}

		// Keep a deterministic display/key regardless of source ordering.
		// Source order itself carries no matching meaning for an OR group.
		sort.SliceStable(alternatives, func(i, j int) bool {
			return strings.ToLower(alternatives[i]) < strings.ToLower(alternatives[j])
		})
		key := strings.ToLower(strings.Join(alternatives, " or "))
		if seen[key] {
			continue
		}
		seen[key] = true
		groups = append(groups, aiclient.SkillRequirement{
			NormalizedName: strings.Join(alternatives, " or "),
			OriginalText:   clause,
			Importance:     "required",
			Category:       "alternative",
			Confidence:     0.95,
			Alternatives:   alternatives,
		})
	}
	return groups
}

func sameAlternativeSkill(raw string, alternatives []string) bool {
	candidate := canonicalAlternativeSkill(raw)
	for _, alternative := range alternatives {
		if strings.EqualFold(candidate, canonicalAlternativeSkill(alternative)) {
			return true
		}
	}
	return false
}

// withAlternativeRequirements overlays OR semantics recovered directly from
// the original JD. This runs on both cached and newly parsed requirements, so
// it also repairs jobs parsed by older parser versions without a DB migration.
func withAlternativeRequirements(reqs Requirements, description string) Requirements {
	groups := extractAlternativeRequirements(description)
	if len(groups) == 0 {
		return reqs
	}

	for _, group := range groups {
		filtered := make([]aiclient.SkillRequirement, 0, len(reqs.RequiredSkills)+1)
		for _, existing := range reqs.RequiredSkills {
			// Only collapse an individual skill when it clearly came from an
			// alternative clause. Independent requirements for the same language
			// elsewhere in the posting remain intact.
			fromAlternativeClause := alternativeMarkerPattern.MatchString(existing.OriginalText) ||
				directOrPattern.MatchString(existing.OriginalText) ||
				strings.EqualFold(strings.TrimSpace(existing.OriginalText), strings.TrimSpace(existing.NormalizedName))
			if fromAlternativeClause && sameAlternativeSkill(existing.NormalizedName, group.Alternatives) {
				continue
			}
			filtered = append(filtered, existing)
		}
		reqs.RequiredSkills = filtered

		exists := false
		for _, existing := range reqs.RequiredSkills {
			if strings.EqualFold(existing.NormalizedName, group.NormalizedName) {
				exists = true
				break
			}
		}
		if !exists {
			reqs.RequiredSkills = append(reqs.RequiredSkills, group)
		}
	}
	return reqs
}
