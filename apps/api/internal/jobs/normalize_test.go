package jobs

import "testing"

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
