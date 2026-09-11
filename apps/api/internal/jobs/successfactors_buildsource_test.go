package jobs

import "testing"

func TestSourcePriority_SuccessFactorsIsDirectATS(t *testing.T) {
	if got := sourcePriority("SUCCESSFACTORS"); got != 100 {
		t.Fatalf("sourcePriority(SUCCESSFACTORS) = %d, want 100", got)
	}
}
