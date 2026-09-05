package auth

import "testing"

func TestNewSessionToken_UniqueAndNonEmpty(t *testing.T) {
	a, err := newSessionToken()
	if err != nil {
		t.Fatalf("newSessionToken: %v", err)
	}
	b, err := newSessionToken()
	if err != nil {
		t.Fatalf("newSessionToken: %v", err)
	}
	if a == "" || b == "" {
		t.Fatalf("expected non-empty tokens")
	}
	if a == b {
		t.Fatalf("expected distinct tokens across calls")
	}
}

func TestHashToken_Deterministic(t *testing.T) {
	if hashToken("same-input") != hashToken("same-input") {
		t.Fatalf("expected hashToken to be deterministic")
	}
	if hashToken("input-a") == hashToken("input-b") {
		t.Fatalf("expected different inputs to hash differently")
	}
}
