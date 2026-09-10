package resumeversion

import "testing"

func TestCategoryForSkillUsesCanonicalAliases(t *testing.T) {
	categories := map[string]string{"Go": "Languages"}
	if got := categoryForSkill("Go (Golang)", categories); got != "Languages" {
		t.Fatalf("categoryForSkill(Go (Golang)) = %q, want Languages", got)
	}
}
