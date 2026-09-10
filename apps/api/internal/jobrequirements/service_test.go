package jobrequirements

import "testing"

func TestVersionedRequirementsHashChangesParserCacheKey(t *testing.T) {
	got := versionedRequirementsHash("abc123")
	want := requirementsParserCacheVersion + ":abc123"
	if got != want {
		t.Fatalf("versionedRequirementsHash() = %q, want %q", got, want)
	}
	if got == "abc123" {
		t.Fatal("parser cache key must differ from the raw job content hash")
	}
}
