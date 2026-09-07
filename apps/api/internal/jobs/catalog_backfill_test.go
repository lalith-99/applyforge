package jobs

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBetterCanonicalCandidate_PrefersActiveThenSourceQuality(t *testing.T) {
	now := time.Now().UTC()
	closedDirect := fingerprintMember{
		ID: uuid.New(), Source: "GREENHOUSE", Status: "CLOSED", FirstSeenAt: now.Add(-time.Hour),
	}
	activeAggregator := fingerprintMember{
		ID: uuid.New(), Source: "ARBEITNOW", Status: "ACTIVE", FirstSeenAt: now,
	}
	if !betterCanonicalCandidate(activeAggregator, closedDirect) {
		t.Fatal("active duplicate must beat a closed direct-source row")
	}

	activeBroad := fingerprintMember{
		ID: uuid.New(), Source: "BRIGHTDATA", Status: "ACTIVE", FirstSeenAt: now.Add(-2 * time.Hour),
	}
	activeDirect := fingerprintMember{
		ID: uuid.New(), Source: "GREENHOUSE", Status: "ACTIVE", FirstSeenAt: now,
	}
	if !betterCanonicalCandidate(activeDirect, activeBroad) {
		t.Fatal("employer-direct ATS must beat broad provider when both are active")
	}
}

func TestBetterCanonicalCandidate_UsesEarliestDiscoveryAsTieBreaker(t *testing.T) {
	now := time.Now().UTC()
	older := fingerprintMember{
		ID: uuid.New(), Source: "LEVER", Status: "ACTIVE", FirstSeenAt: now.Add(-time.Hour),
	}
	newer := fingerprintMember{
		ID: uuid.New(), Source: "LEVER", Status: "ACTIVE", FirstSeenAt: now,
	}
	if betterCanonicalCandidate(newer, older) {
		t.Fatal("newer same-priority source should not replace earlier canonical")
	}
	if !betterCanonicalCandidate(older, newer) {
		t.Fatal("earlier same-priority source should be preferred")
	}
}
