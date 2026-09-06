package jobs

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var (
	tagRe          = regexp.MustCompile(`<[^>]*>`)
	whitespaceRe   = regexp.MustCompile(`\s+`)
	seniorityWords = []string{
		"senior", "sr.", "sr", "junior", "jr.", "jr", "staff", "principal", "lead",
		"i", "ii", "iii", "iv",
	}
)

// stripTags removes HTML markup and collapses whitespace, producing plain text.
func stripTags(html string) string {
	text := tagRe.ReplaceAllString(html, " ")
	text = whitespaceRe.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// normalizeTitle produces a coarse, comparison-friendly title: lowercased,
// punctuation-trimmed, with common seniority qualifiers removed so
// "Sr. Backend Engineer" and "Backend Engineer II" both normalize similarly.
func normalizeTitle(title string) string {
	lower := strings.ToLower(strings.TrimSpace(title))
	lower = strings.ReplaceAll(lower, ",", " ")
	lower = strings.ReplaceAll(lower, "-", " ")
	words := strings.Fields(lower)

	kept := make([]string, 0, len(words))
	for _, w := range words {
		skip := false
		for _, sw := range seniorityWords {
			if w == sw {
				skip = true
				break
			}
		}
		if !skip {
			kept = append(kept, w)
		}
	}
	return strings.Join(kept, " ")
}

// normalizeCompanyName produces a comparison-friendly company name key.
func normalizeCompanyName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	lower = strings.TrimSuffix(lower, ", inc.")
	lower = strings.TrimSuffix(lower, " inc.")
	lower = strings.TrimSuffix(lower, " inc")
	lower = strings.TrimSuffix(lower, " llc")
	return strings.TrimSpace(lower)
}

// buildFingerprint produces a coarse cross-source dedupe key: the same real
// posting from two different sources (e.g. a company's own Greenhouse board
// and an aggregator like Arbeitnow) should normally produce the same
// fingerprint even though their (source, external_id) differ. Deliberately
// uses remote_type rather than raw location text, since free-text location
// formatting varies far more across sources than a normalized title/company
// pair does - remote_type is already normalized identically by every
// connector (see source.go's RawJob.RemoteType).
func buildFingerprint(companyName, title, remoteType string) string {
	company := normalizeCompanyName(companyName)
	normTitle := normalizeTitle(title)
	if company == "" || normTitle == "" {
		return ""
	}
	return company + "|" + normTitle + "|" + strings.ToLower(strings.TrimSpace(remoteType))
}

// contentHash fingerprints the parts of a job posting that matter for
// change detection (see MASTER_REQUIREMENTS.md §15).
func contentHash(company, title, location, description string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(company) + "|" + strings.ToLower(title) + "|" + strings.ToLower(location) + "|" + description))
	return hex.EncodeToString(sum[:])
}
