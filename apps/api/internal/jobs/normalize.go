package jobs

import (
	"crypto/sha256"
	"encoding/hex"
	"html"
	"regexp"
	"strings"

	xhtml "golang.org/x/net/html"
)

var (
	whitespaceRe   = regexp.MustCompile(`[ \t\f\v\r]+`)
	lineBreakRe    = regexp.MustCompile(`\n{3,}`)
	seniorityWords = []string{
		"senior", "sr.", "sr", "junior", "jr.", "jr", "staff", "principal", "lead",
		"i", "ii", "iii", "iv",
	}
)

// stripTags decodes escaped HTML and structurally renders readable text.
// It preserves headings, lists, paragraphs, and emphasis while removing
// script/style/template/svg content entirely. Using an HTML parser rather
// than regex means malformed/nested ATS markup is handled much more safely.
func stripTags(value string) string {
	for i := 0; i < 3; i++ {
		decoded := html.UnescapeString(value)
		if decoded == value {
			break
		}
		value = decoded
	}

	if !strings.Contains(value, "<") {
		return normalizeRenderedText(value)
	}

	doc, err := xhtml.Parse(strings.NewReader("<html><body>" + value + "</body></html>"))
	if err != nil {
		// Parser failures are unusual because x/net/html is deliberately
		// tolerant. Fall back to normalized plain text rather than returning
		// an empty JD.
		return normalizeRenderedText(value)
	}

	var b strings.Builder
	renderHTMLNode(&b, doc)
	return dedupeRepeatedBlocks(normalizeRenderedText(b.String()))
}

func renderHTMLNode(b *strings.Builder, node *xhtml.Node) {
	if node == nil {
		return
	}

	if node.Type == xhtml.TextNode {
		b.WriteString(node.Data)
		return
	}

	if node.Type != xhtml.ElementNode && node.Type != xhtml.DocumentNode {
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			renderHTMLNode(b, child)
		}
		return
	}

	tag := strings.ToLower(node.Data)
	switch tag {
	case "script", "style", "svg", "noscript", "template":
		return
	case "br":
		b.WriteByte('\n')
		return
	case "h1", "h2", "h3", "h4", "h5", "h6":
		text := strings.Join(strings.Fields(nodeText(node)), " ")
		if text != "" {
			b.WriteString("\n**")
			b.WriteString(text)
			b.WriteString("**\n")
		}
		return
	case "li":
		b.WriteString("\n- ")
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			renderHTMLNode(b, child)
		}
		b.WriteByte('\n')
		return
	case "strong", "b":
		b.WriteString("**")
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			renderHTMLNode(b, child)
		}
		b.WriteString("**")
		return
	}

	block := isHTMLBlock(tag)
	if block {
		b.WriteByte('\n')
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		renderHTMLNode(b, child)
	}
	if block {
		b.WriteByte('\n')
	}
}

func nodeText(node *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		if n.Type == xhtml.ElementNode {
			switch strings.ToLower(n.Data) {
			case "script", "style", "svg", "noscript", "template":
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return b.String()
}

func isHTMLBlock(tag string) bool {
	switch tag {
	case "article", "aside", "blockquote", "div", "footer", "header", "main",
		"ol", "p", "section", "table", "tbody", "td", "th", "thead", "tr", "ul":
		return true
	default:
		return false
	}
}

func normalizeRenderedText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = whitespaceRe.ReplaceAllString(value, " ")

	lines := strings.Split(value, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}
		out = append(out, line)
	}
	text := strings.Join(out, "\n")
	text = lineBreakRe.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

// dedupeRepeatedBlocks removes repeated long ATS blocks (for example the same
// company boilerplate rendered twice by desktop/mobile markup) while keeping
// short repeated bullets/headings intact.
func dedupeRepeatedBlocks(value string) string {
	blocks := strings.Split(value, "\n\n")
	seen := make(map[string]bool)
	out := make([]string, 0, len(blocks))
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		key := strings.ToLower(strings.Join(strings.Fields(block), " "))
		if len(key) >= 80 {
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		out = append(out, block)
	}
	return strings.Join(out, "\n\n")
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

var explicitCountryCodes = map[string]string{
	"india": "IN",
	"in":    "IN",
	"ind":   "IN",
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

	explicitCountry := country != ""
	explicitUSCountry := usCountryTokens[country]

	switch {
	case explicitUSCountry:
		result.CountryCode = "US"
		result.EligibleCountryCodes = []string{"US"}
		result.LocationConfidence = "HIGH"
		if result.WorkplaceType == "REMOTE" {
			result.RemoteScope = "US"
		}
	case explicitCountry:
		// An explicit foreign country is authoritative and must veto later
		// U.S.-state inference. This avoids values such as Country="India",
		// State="IN" being reinterpreted as Indiana / United States.
		if code := explicitCountryCodes[country]; code != "" {
			result.CountryCode = code
			result.EligibleCountryCodes = []string{code}
			result.LocationConfidence = "HIGH"
		}
	case containsUSCountry(location):
		result.CountryCode = "US"
		result.EligibleCountryCodes = []string{"US"}
		result.LocationConfidence = "HIGH"
		if result.WorkplaceType == "REMOTE" {
			result.RemoteScope = "US"
		}
	}

	// Only infer a U.S. state when the source country is absent or explicitly
	// U.S. A provider-supplied foreign country always wins over ambiguous
	// two-letter region codes such as IN, OR, ME, or HI.
	if !explicitCountry || explicitUSCountry {
		if stateCode := normalizeUSState(state); stateCode != "" {
			result.StateCode = stateCode
		} else if stateCode := stateCodeInLocation(location); stateCode != "" {
			result.StateCode = stateCode
		}
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

// buildFingerprint produces a conservative cross-source identity key.
// Unlike normalizeTitle (which intentionally removes seniority for search /
// matching), identityTitle preserves Senior/Junior/Staff distinctions.
// Location and a normalized-description hash keep same-title openings for
// different teams/locations from collapsing into one canonical job.
func buildFingerprint(companyName, title, location, description string) string {
	company := normalizeCompanyName(companyName)
	identityTitle := normalizeIdentityTitle(title)
	locationKey := strings.ToLower(strings.Join(strings.Fields(location), " "))
	descriptionKey := normalizedDescriptionHash(description)
	if company == "" || identityTitle == "" || descriptionKey == "" {
		return ""
	}
	return company + "|" + identityTitle + "|" + locationKey + "|" + descriptionKey
}

func normalizeIdentityTitle(title string) string {
	lower := strings.ToLower(strings.TrimSpace(title))
	lower = strings.NewReplacer(",", " ", "-", " ", "/", " ").Replace(lower)
	return strings.Join(strings.Fields(lower), " ")
}

func normalizedDescriptionHash(description string) string {
	normalized := strings.Join(strings.Fields(strings.ToLower(stripTags(description))), " ")
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// contentHash fingerprints the parts of a job posting that matter for
// change detection (see MASTER_REQUIREMENTS.md §15).
func contentHash(company, title, location, description string) string {
	normalizedDescription := strings.ToLower(stripTags(description))
	sum := sha256.Sum256([]byte(strings.ToLower(company) + "|" + strings.ToLower(title) + "|" + strings.ToLower(location) + "|" + normalizedDescription))
	return hex.EncodeToString(sum[:])
}
