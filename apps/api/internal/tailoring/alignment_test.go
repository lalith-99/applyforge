package tailoring

import "testing"

func TestComputeAlignment_FullCoverage(t *testing.T) {
	skillSet := map[string]bool{"go": true, "kafka": true, "kubernetes": true}
	score := ComputeAlignment(skillSet, []string{"go", "kafka"}, []string{"kubernetes"}, []string{"build go services using kafka"})
	if score < 90 {
		t.Fatalf("expected near-full alignment score, got %d", score)
	}
}

func TestComputeAlignment_NoCoverage(t *testing.T) {
	skillSet := map[string]bool{"java": true}
	score := ComputeAlignment(skillSet, []string{"go", "kafka"}, []string{"kubernetes"}, []string{"build go services"})
	if score > 40 {
		t.Fatalf("expected low alignment score, got %d", score)
	}
}

func TestComputeAlignment_ImprovesWithMoreSkills(t *testing.T) {
	before := map[string]bool{"go": true}
	after := map[string]bool{"go": true, "kafka": true, "kubernetes": true}
	required := []string{"go", "kafka"}
	preferred := []string{"kubernetes"}
	responsibilities := []string{"build go services using kafka and kubernetes"}

	scoreBefore := ComputeAlignment(before, required, preferred, responsibilities)
	scoreAfter := ComputeAlignment(after, required, preferred, responsibilities)

	if scoreAfter <= scoreBefore {
		t.Fatalf("expected alignment to improve as more required/preferred skills are added: before=%d after=%d", scoreBefore, scoreAfter)
	}
}
