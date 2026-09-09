package tailoring

import "testing"

func TestSuggestionEvidenceStates(t *testing.T) {
	tests := []struct {
		name                string
		section             string
		source              string
		wantStatus          string
		wantAttestation     bool
	}{
		{
			name:            "verified master rewrite",
			section:         "experience",
			source:          "MASTER_RESUME",
			wantStatus:      EvidenceVerified,
			wantAttestation: false,
		},
		{
			name:            "AI experience draft requires attestation",
			section:         "experience",
			source:          "AI_SUGGESTED",
			wantStatus:      EvidenceCandidateAttestationRequired,
			wantAttestation: true,
		},
		{
			name:            "AI skill addition is build before use",
			section:         "skills",
			source:          "AI_SUGGESTED",
			wantStatus:      EvidenceBuildBeforeUse,
			wantAttestation: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, requiresAttestation := suggestionEvidence(tt.section, tt.source)
			if status != tt.wantStatus {
				t.Fatalf("expected evidence status %q, got %q", tt.wantStatus, status)
			}
			if requiresAttestation != tt.wantAttestation {
				t.Fatalf("expected requiresAttestation=%v, got %v", tt.wantAttestation, requiresAttestation)
			}
		})
	}
}

func TestUpdateNeedsAttestation(t *testing.T) {
	verified := Suggestion{RequiresAttestation: false}
	aiDraft := Suggestion{RequiresAttestation: true}

	if updateNeedsAttestation(verified, StatusApproved, false) {
		t.Fatal("verified rewrite should not require attestation")
	}
	if !updateNeedsAttestation(aiDraft, StatusApproved, false) {
		t.Fatal("AI draft approval should require attestation")
	}
	if updateNeedsAttestation(aiDraft, StatusApproved, true) {
		t.Fatal("attested AI draft should be approvable")
	}
	if !updateNeedsAttestation(aiDraft, StatusEdited, false) {
		t.Fatal("edited AI draft should still require attestation")
	}
	if updateNeedsAttestation(aiDraft, StatusRejected, false) {
		t.Fatal("rejecting a draft should never require attestation")
	}
}
