package jobs

import (
	"crypto/sha256"
	"encoding/hex"
	"html"
	"regexp"
	"strings"
)

var (
	tagRe          = regexp.MustCompile(`<[^>]*>`)
	headingTagRe   = regexp.MustCompile(`(?i)</?h[1-6](?:\s[^>]*)?>`)
	blockTagRe     = regexp.MustCompile(`(?i)</?(?:article|br|div|h[1-6]|li|ol|p|section|table|tr|ul)(?:\s[^>]*)?>`)
	boldTagRe      = regexp.MustCompile(`(?i)</?(?:b|strong)(?:\s[^>]*)?>`)
	whitespaceRe   = regexp.MustCompile(`[ \t\f\v\r]+`)
	lineBreakRe    = regexp.MustCompile(`\n{3,}`)
	seniorityWords = []string{
		"senior", "sr.", "sr", "junior", "jr.", "jr", "staff", "principal", "lead",
		"i", "ii", "iii", "iv",
	}
)

// stripTags decodes escaped HTML, removes markup, and preserves block breaks.
func stripTags(value string) string {
	for i := 0; i < 3; i++ {
		decoded := html.UnescapeString(value)
		if decoded == value {
			break
		}
		value = decoded
	}
	text := headingTagRe.ReplaceAllStringFunc(value, func(tag string) string {
		if strings.HasPrefix(tag, "</") {
			return "**\n"
		}
		return "\n**"
	})
	text = blockTagRe.ReplaceAllString(text, "\n")
	text = boldTagRe.ReplaceAllStringFunc(text, func(tag string) string {
		if strings.HasPrefix(tag, "</") {
			return "**"
		}
		return "**"
	})
	text = tagRe.ReplaceAllString(text, " ")
	text = whitespaceRe.ReplaceAllString(text, " ")
	text = lineBreakRe.ReplaceAllString(text, "\n\n")
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

// NormalizedLocation is the deterministic location representation used for
// catalog and recommendation hard filters.
type NormalizedLocation struct {
	CountryCode          string
	StateCode            string
	City                 string
	WorkplaceType        string
	RemoteScope          string
	EligibleCountryCodes []string
	LocationConfidence   string
}

var usStateCodes = map[string]string{
	"alabama": "AL", "alaska": "AK", "arizona": "AZ", "arkansas": "AR", "california": "CA", "colorado": "CO",
	"connecticut": "CT", "delaware": "DE", "florida": "FL", "georgia": "GA", "hawaii": "HI", "idaho": "ID",
	"illinois": "IL", "indiana": "IN", "iowa": "IA", "kansas": "KS", "kentucky": "KY", "louisiana": "LA",
	"maine": "ME", "maryland": "MD", "massachusetts": "MA", "michigan": "MI", "minnesota": "MN", "mississippi": "MS",
	"missouri": "MO", "montana": "MT", "nebraska": "NE", "nevada": "NV", "new hampshire": "NH", "new jersey": "NJ",
	"new mexico": "NM", "new york": "NY", "north carolina": "NC", "north dakota": "ND", "ohio": "OH", "oklahoma": "OK",
	"oregon": "OR", "pennsylvania": "PA", "rhode island": "RI", "south carolina": "SC", "south dakota": "SD", "tennessee": "TN",
	"texas": "TX", "utah": "UT", "vermont": "VT", "virginia": "VA", "washington": "WA", "west virginia": "WV",
	"wisconsin": "WI", "wyoming": "WY", "district of columbia": "DC",
}

var usCountryTokens = map[string]bool{
	"united states": true, "united states of america": true, "us": true, "usa": true, "u.s.": true, "u.s": true,
}

// normalizeLocation retains source fields while assigning a canonical country
// only when the source data or location text is unambiguous.
func normalizeLocation(raw RawJob) NormalizedLocation {
	location := strings.TrimSpace(raw.LocationText)
	country := strings.ToLower(strings.TrimSpace(raw.Country))
	state := strings.TrimSpace(raw.State)
	city := strings.TrimSpace(raw.City)
	result := NormalizedLocation{
		City:               city,
		RemoteScope:        "UNKNOWN",
		LocationConfidence: "LOW",
		WorkplaceType:      "ONSITE",
	}

	switch strings.ToLower(strings.TrimSpace(raw.RemoteType)) {
	case "remote":
		result.WorkplaceType = "REMOTE"
	case "hybrid":
		result.WorkplaceType = "HYBRID"
	}

	if usCountryTokens[country] || containsUSCountry(location) {
		result.CountryCode = "US"
		result.EligibleCountryCodes = []string{"US"}
		result.LocationConfidence = "HIGH"
		if result.WorkplaceType == "REMOTE" {
			result.RemoteScope = "US"
		}
	}

	if stateCode := normalizeUSState(state); stateCode != "" {
		result.StateCode = stateCode
	} else if stateCode := stateCodeInLocation(location); stateCode != "" {
		result.StateCode = stateCode
	}
	if result.StateCode != "" {
		result.CountryCode = "US"
		result.EligibleCountryCodes = []string{"US"}
		result.LocationConfidence = "HIGH"
		if result.City == "" && result.WorkplaceType != "REMOTE" {
			result.City = cityInLocation(location)
		}
		if result.WorkplaceType == "REMOTE" {
			result.RemoteScope = "STATE_RESTRICTED"
		}
	}
	return result
}

func containsUSCountry(location string) bool {
	lower := strings.ToLower(location)
	for token := range usCountryTokens {
		if lower == token || strings.Contains(lower, ", "+token) || strings.HasSuffix(lower, " "+token) {
			return true
		}
	}
	return false
}

func normalizeUSState(state string) string {
	lower := strings.ToLower(strings.TrimSpace(state))
	if code, ok := usStateCodes[lower]; ok {
		return code
	}
	if len(lower) == 2 {
		upper := strings.ToUpper(lower)
		for _, code := range usStateCodes {
			if code == upper {
				return code
			}
		}
	}
	return ""
}

var usStateAbbrevRe = regexp.MustCompile(`(?:^|,\s*|\s)(AL|AK|AZ|AR|CA|CO|CT|DE|FL|GA|HI|ID|IL|IN|IA|KS|KY|LA|ME|MD|MA|MI|MN|MS|MO|MT|NE|NV|NH|NJ|NM|NY|NC|ND|OH|OK|OR|PA|RI|SC|SD|TN|TX|UT|VT|VA|WA|WV|WI|WY|DC)(?:$|,\s*(?:United States|United States of America|US|USA|U\.S\.?))`)

func stateCodeInLocation(location string) string {
	lower := strings.ToLower(location)
	for name, code := range usStateCodes {
		if strings.Contains(lower, name) {
			return code
		}
	}

	// State abbreviations are intentionally case-sensitive and must either
	// terminate the location or be followed by an explicit U.S. country
	// marker. This avoids false positives from ordinary words such as
	// "in", "or", "me", and "hi" being interpreted as IN/OR/ME/HI.
	match := usStateAbbrevRe.FindStringSubmatch(location)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func cityInLocation(location string) string {
	parts := strings.Split(location, ",")
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

// normalizeEmploymentType canonicalizes provider-specific employment labels
// so catalog filters and matching use one stable vocabulary.
func normalizeEmploymentType(value string) string {
	original := strings.TrimSpace(value)
	if original == "" {
		return ""
	}

	key := strings.ToLower(original)
	key = strings.NewReplacer(" ", "", "-", "", "_", "", "/", "").Replace(key)

	switch key {
	case "full", "fulltime", "permanent", "regular", "regularfulltime", "employee":
		return "FullTime"
	case "contract", "contractor", "freelance", "consultant":
		return "Contract"
	case "intern", "internship", "studentintern":
		return "Internship"
	case "part", "parttime":
		return "PartTime"
	case "temp", "temporary", "seasonal":
		return "Temporary"
	default:
		return original
	}
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
