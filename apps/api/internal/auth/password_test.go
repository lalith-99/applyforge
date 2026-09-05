package auth

import "testing"

func TestHashPassword_VerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "correct-horse-battery-staple" {
		t.Fatalf("expected hash to differ from plaintext")
	}

	if !VerifyPassword(hash, "correct-horse-battery-staple") {
		t.Fatalf("expected correct password to verify")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatalf("expected wrong password to fail verification")
	}
}
