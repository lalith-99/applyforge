package jobs

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeLocation_ClassifiesExplicitUSAndState(t *testing.T) {
	location := normalizeLocation(RawJob{LocationText: "San Francisco, CA, United States", RemoteType: "hybrid"})
	if location.CountryCode != "US" || location.StateCode != "CA" || location.City != "San Francisco" {
		t.Fatalf("unexpected normalized location: %+v", location)
	}
	if location.WorkplaceType != "HYBRID" || location.LocationConfidence != "HIGH" {
		t.Fatalf("expected high-confidence hybrid US location, got %+v", location)
	}
}

func TestNormalizeLocation_DoesNotTreatAustraliaAsUS(t *testing.T) {
	location := normalizeLocation(RawJob{LocationText: "Sydney, Australia"})
	if location.CountryCode != "" || location.RemoteScope != "UNKNOWN" {
		t.Fatalf("Australia must not be classified as US: %+v", location)
	}
}

func TestNormalizeLocation_ExplicitIndiaVetoesIndianaStateCode(t *testing.T) {
	location := normalizeLocation(RawJob{
		LocationText: "Bengaluru, IN, India",
		Country:      "India",
		State:        "IN",
		City:         "Bengaluru",
	})
	if location.CountryCode != "IN" {
		t.Fatalf("India must normalize to IN, got %+v", location)
	}
	if location.StateCode != "" {
		t.Fatalf("foreign state code IN must not be treated as Indiana: %+v", location)
	}
	if len(location.EligibleCountryCodes) != 1 || location.EligibleCountryCodes[0] != "IN" {
		t.Fatalf("expected India eligibility only, got %+v", location)
	}
}

func TestNormalizeLocation_ExplicitForeignCountryVetoesUSStateInference(t *testing.T) {
	location := normalizeLocation(RawJob{
		LocationText: "Vancouver, BC, Canada",
		Country:      "Canada",
		State:        "CA",
		City:         "Vancouver",
	})
	if location.CountryCode == "US" || location.StateCode != "" {
		t.Fatalf("explicit foreign country must veto US state inference: %+v", location)
	}
}

func TestNormalizeLocation_ExplicitUSStillAllowsIndiana(t *testing.T) {
	location := normalizeLocation(RawJob{
		LocationText: "Indianapolis, IN, United States",
		Country:      "United States",
		State:        "IN",
		City:         "Indianapolis",
	})
	if location.CountryCode != "US" || location.StateCode != "IN" {
		t.Fatalf("explicit US country should still allow Indiana: %+v", location)
	}
}

func TestNormalizeLocation_LeavesUnscopedRemoteUnknown(t *testing.T) {
	location := normalizeLocation(RawJob{LocationText: "Remote", RemoteType: "remote"})
	if location.CountryCode != "" || location.RemoteScope != "UNKNOWN" || location.WorkplaceType != "REMOTE" {
		t.Fatalf("unscoped remote job must remain geographically unknown: %+v", location)
	}
}

func TestNormalizeTitle(t *testing.T) {
	cases := map[string]string{
		"Sr. Backend Engineer":    "backend engineer",
		"Backend Engineer II":     "backend engineer",
		"Staff Software Engineer": "software engineer",
		"Software Engineer":       "software engineer",
	}
	for input, want := range cases {
		if got := normalizeTitle(input); got != want {
			t.Errorf("normalizeTitle(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeCompanyName(t *testing.T) {
	cases := map[string]string{
		"Acme, Inc.": "acme",
		"Acme Inc":   "acme",
		"Acme LLC":   "acme",
		"Acme":       "acme",
	}
	for input, want := range cases {
		if got := normalizeCompanyName(input); got != want {
			t.Errorf("normalizeCompanyName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestContentHash_StableAndSensitive(t *testing.T) {
	a := contentHash("Acme", "Backend Engineer", "Remote", "Build things")
	b := contentHash("Acme", "Backend Engineer", "Remote", "Build things")
	if a != b {
		t.Fatalf("expected identical inputs to hash identically")
	}

	c := contentHash("Acme", "Backend Engineer", "Remote", "Build other things")
	if a == c {
		t.Fatalf("expected different descriptions to hash differently")
	}
}

func TestStripTags(t *testing.T) {
	got := stripTags("<p>Hello <b>world</b></p>")
	if got != "Hello **world**" {
		t.Fatalf("expected %q, got %q", "Hello **world**", got)
	}
}

func TestStripTags_DecodesEscapedHTMLAndPreservesBlocks(t *testing.T) {
	input := `&lt;div class="content-intro"&gt;&lt;p&gt;First paragraph&lt;/p&gt;&lt;h2&gt;Your opportunity&lt;/h2&gt;&lt;p&gt;&lt;strong&gt;Second &amp;amp; final&lt;/strong&gt;&lt;/p&gt;&lt;/div&gt;`
	want := "First paragraph\n\n**Your opportunity**\n\n**Second & final**"
	if got := stripTags(input); got != want {
		t.Fatalf("stripTags(%q) = %q, want %q", input, got, want)
	}
}

func TestNormalizeLocation_DoesNotTreatEnglishWordsAsStateCodes(t *testing.T) {
	for _, input := range []string{
		"Remote in Europe",
		"Portland or Vancouver",
		"Tell me more",
		"Say hi remotely",
	} {
		location := normalizeLocation(RawJob{LocationText: input, RemoteType: "remote"})
		if location.CountryCode != "" {
			t.Fatalf("%q must not be classified as US: %+v", input, location)
		}
	}
}

func TestNormalizeLocation_RecognizesUppercaseUSStateSuffix(t *testing.T) {
	location := normalizeLocation(RawJob{LocationText: "Austin, TX"})
	if location.CountryCode != "US" || location.StateCode != "TX" {
		t.Fatalf("expected Austin, TX to normalize to US/TX: %+v", location)
	}
}

func TestNormalizeEmploymentType(t *testing.T) {
	cases := map[string]string{
		"Full-time":  "FullTime",
		"full":       "FullTime",
		"Permanent":  "FullTime",
		"Contractor": "Contract",
		"intern":     "Internship",
		"part_time":  "PartTime",
	}
	for input, want := range cases {
		if got := normalizeEmploymentType(input); got != want {
			t.Errorf("normalizeEmploymentType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestIsFreshForEagerAI(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-24 * time.Hour)
	old := now.Add(-31 * 24 * time.Hour)
	futureBad := now.Add(48 * time.Hour)

	if !isFreshForEagerAI(&recent, now) {
		t.Fatal("recent job should be eligible for eager AI")
	}
	if isFreshForEagerAI(&old, now) {
		t.Fatal("old job must not be eligible for eager AI")
	}
	if isFreshForEagerAI(nil, now) {
		t.Fatal("unknown posting time must not be eligible for eager AI")
	}
	if isFreshForEagerAI(&futureBad, now) {
		t.Fatal("wildly future timestamp must not be eligible for eager AI")
	}
}

func TestBuildFingerprint_PreservesSeniorityAndLocation(t *testing.T) {
	senior := buildFingerprint("Acme", "Senior Backend Engineer", "Austin, TX", "Build APIs with Go.")
	junior := buildFingerprint("Acme", "Junior Backend Engineer", "Austin, TX", "Build APIs with Go.")
	otherLocation := buildFingerprint("Acme", "Senior Backend Engineer", "Seattle, WA", "Build APIs with Go.")
	if senior == junior {
		t.Fatal("senior and junior openings must not share a dedupe fingerprint")
	}
	if senior == otherLocation {
		t.Fatal("same-title openings in different locations must not share a dedupe fingerprint")
	}
}

func TestBuildFingerprint_NormalizesDescriptionFormatting(t *testing.T) {
	a := buildFingerprint("Acme", "Senior Backend Engineer", "Austin, TX", "<p>Build   APIs with Go.</p>")
	b := buildFingerprint("Acme, Inc.", "Senior Backend Engineer", "Austin, TX", "Build APIs with Go.")
	if a == "" || a != b {
		t.Fatalf("expected equivalent normalized postings to dedupe: %q vs %q", a, b)
	}
}

func TestBuildFingerprint_RequiresDescription(t *testing.T) {
	if got := buildFingerprint("Acme", "Backend Engineer", "Remote", ""); got != "" {
		t.Fatalf("description-less jobs are too ambiguous for cross-source dedupe: %q", got)
	}
}

func TestStripTags_RemovesUnsafeBlocksAndPreservesLists(t *testing.T) {
	input := `<section><h2>Responsibilities</h2><ul><li>Build APIs</li><li>Operate Kafka</li></ul><script>alert("x")</script><style>.x{}</style></section>`
	got := stripTags(input)
	want := "**Responsibilities**\n\n- Build APIs\n\n- Operate Kafka"
	if got != want {
		t.Fatalf("stripTags() = %q, want %q", got, want)
	}
}

func TestStripTags_DedupesRepeatedLongATSBlocks(t *testing.T) {
	block := "This is a sufficiently long company boilerplate paragraph that is duplicated by two responsive ATS containers."
	input := "<div><p>" + block + "</p></div><div><p>" + block + "</p></div>"
	got := stripTags(input)
	if strings.Count(got, block) != 1 {
		t.Fatalf("expected duplicate long ATS block to be removed: %q", got)
	}
}

func TestContentHash_IgnoresEquivalentHTMLWrappers(t *testing.T) {
	a := contentHash("Acme", "Backend Engineer", "Austin, TX", "<div><p>Build APIs with Go.</p></div>")
	b := contentHash("Acme", "Backend Engineer", "Austin, TX", "<section class=\"mobile\"><p>Build APIs with Go.</p></section>")
	if a != b {
		t.Fatalf("equivalent rendered descriptions should hash identically: %q vs %q", a, b)
	}
}
