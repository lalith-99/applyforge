package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
)

const defaultCareerPageMaxBodyBytes int64 = 2 << 20

type CareerPageInspection struct {
	Jobs     []RawJob
	Sources  []DiscoveredCompanySource
	FinalURL string
}

type CareerPageInspector struct {
	http         *http.Client
	maxBodyBytes int64
}

func NewCareerPageInspector() *CareerPageInspector {
	return &CareerPageInspector{
		http:         newPublicHTTPClient(25 * time.Second),
		maxBodyBytes: defaultCareerPageMaxBodyBytes,
	}
}

func (i *CareerPageInspector) Inspect(ctx context.Context, rawURL string) (CareerPageInspection, error) {
	parsed, err := validatePublicHTTPURL(rawURL)
	if err != nil {
		return CareerPageInspection{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return CareerPageInspection{}, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.1")
	req.Header.Set("Accept-Language", "en-US,en;q=0.8")
	req.Header.Set("User-Agent", "ApplyForge/1.0")

	resp, err := i.http.Do(req)
	if err != nil {
		return CareerPageInspection{}, fmt.Errorf("fetch career page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CareerPageInspection{}, fmt.Errorf("career page returned %s", resp.Status)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" &&
		!strings.Contains(contentType, "text/html") &&
		!strings.Contains(contentType, "application/xhtml+xml") {
		return CareerPageInspection{}, fmt.Errorf("career page returned unsupported content type %q", contentType)
	}

	limit := i.maxBodyBytes
	if limit <= 0 {
		limit = defaultCareerPageMaxBodyBytes
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return CareerPageInspection{}, fmt.Errorf("read career page: %w", err)
	}
	if int64(len(body)) > limit {
		return CareerPageInspection{}, fmt.Errorf("career page exceeds %d-byte safety limit", limit)
	}

	finalURL := resp.Request.URL
	inspection, err := parseCareerPageDocument(finalURL, body)
	if err != nil {
		return CareerPageInspection{}, err
	}
	inspection.FinalURL = finalURL.String()
	return inspection, nil
}

type CareerPageSource struct {
	URL       string
	inspector *CareerPageInspector
}

func NewCareerPageSource(rawURL string) (*CareerPageSource, error) {
	parsed, err := validatePublicHTTPURL(rawURL)
	if err != nil {
		return nil, err
	}
	return &CareerPageSource{
		URL:       parsed.String(),
		inspector: NewCareerPageInspector(),
	}, nil
}

func (s *CareerPageSource) Name() string {
	return "CAREER_PAGE:" + brightStableID(s.URL)
}

func (s *CareerPageSource) Fetch(ctx context.Context, _ *Cursor) ([]RawJob, *Cursor, error) {
	inspection, err := s.inspector.Inspect(ctx, s.URL)
	if err != nil {
		return nil, nil, err
	}
	if len(inspection.Jobs) == 0 {
		return nil, nil, errors.New("career page exposes no structured JobPosting data")
	}
	return inspection.Jobs, nil, nil
}

func parseCareerPageDocument(baseURL *url.URL, body []byte) (CareerPageInspection, error) {
	if baseURL == nil {
		return CareerPageInspection{}, errors.New("career page base URL is required")
	}

	doc, err := xhtml.Parse(bytes.NewReader(body))
	if err != nil {
		return CareerPageInspection{}, fmt.Errorf("parse career page HTML: %w", err)
	}

	effectiveBase := baseURL
	var (
		candidateURLs []string
		jsonLDScripts []string
	)

	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode {
			switch strings.ToLower(node.Data) {
			case "base":
				if href := htmlAttr(node, "href"); href != "" {
					if resolved, err := resolveCareerPageURL(baseURL, href); err == nil {
						effectiveBase = resolved
					}
				}
			case "a":
				if href := htmlAttr(node, "href"); href != "" {
					if resolved, err := resolveCareerPageURL(effectiveBase, href); err == nil {
						candidateURLs = append(candidateURLs, resolved.String())
					}
				}
			case "iframe":
				if src := htmlAttr(node, "src"); src != "" {
					if resolved, err := resolveCareerPageURL(effectiveBase, src); err == nil {
						candidateURLs = append(candidateURLs, resolved.String())
					}
				}
			case "form":
				if action := htmlAttr(node, "action"); action != "" {
					if resolved, err := resolveCareerPageURL(effectiveBase, action); err == nil {
						candidateURLs = append(candidateURLs, resolved.String())
					}
				}
			case "script":
				if strings.EqualFold(strings.TrimSpace(htmlAttr(node, "type")), "application/ld+json") {
					if text := nodeText(node); strings.TrimSpace(text) != "" {
						jsonLDScripts = append(jsonLDScripts, text)
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	inspection := CareerPageInspection{
		Sources: DetectCompanySources(candidateURLs...),
	}
	seenJobs := map[string]bool{}
	for _, script := range jsonLDScripts {
		jobs := parseJobPostingJSONLD(effectiveBase, script)
		for _, job := range jobs {
			key := job.ExternalID + "|" + job.Title + "|" + job.ApplyURL
			if seenJobs[key] {
				continue
			}
			seenJobs[key] = true
			inspection.Jobs = append(inspection.Jobs, job)
		}
	}
	return inspection, nil
}

func htmlAttr(node *xhtml.Node, key string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func nodeText(node *xhtml.Node) string {
	var builder strings.Builder
	var walk func(*xhtml.Node)
	walk = func(current *xhtml.Node) {
		if current.Type == xhtml.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return builder.String()
}

func resolveCareerPageURL(base *url.URL, raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty URL")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	resolved := base.ResolveReference(parsed)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return nil, errors.New("unsupported URL scheme")
	}
	if resolved.Hostname() == "" {
		return nil, errors.New("resolved URL has no hostname")
	}
	resolved.Fragment = ""
	return resolved, nil
}

func parseJobPostingJSONLD(base *url.URL, raw string) []RawJob {
	var root any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		if unescaped := stdhtml.UnescapeString(raw); unescaped != raw {
			if err := json.Unmarshal([]byte(unescaped), &root); err != nil {
				return nil
			}
		} else {
			return nil
		}
	}

	var postings []map[string]any
	collectJobPostingObjects(root, &postings)

	out := make([]RawJob, 0, len(postings))
	for _, posting := range postings {
		rawJob, ok := mapJSONLDJobPosting(base, posting)
		if ok {
			out = append(out, rawJob)
		}
	}
	return out
}

func collectJobPostingObjects(value any, out *[]map[string]any) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectJobPostingObjects(item, out)
		}
	case map[string]any:
		if jsonLDTypeContains(typed["@type"], "JobPosting") {
			*out = append(*out, typed)
		}
		for _, item := range typed {
			collectJobPostingObjects(item, out)
		}
	}
}

func jsonLDTypeContains(value any, target string) bool {
	switch typed := value.(type) {
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), target)
	case []any:
		for _, item := range typed {
			if jsonLDTypeContains(item, target) {
				return true
			}
		}
	}
	return false
}

func mapJSONLDJobPosting(base *url.URL, posting map[string]any) (RawJob, bool) {
	title := jsonLDString(posting["title"])
	if title == "" {
		title = jsonLDString(posting["name"])
	}
	if title == "" {
		return RawJob{}, false
	}

	description := stripTags(jsonLDString(posting["description"]))
	locationText, country, state, city := jsonLDLocation(posting["jobLocation"])
	remoteType := "onsite"
	if strings.Contains(strings.ToLower(jsonLDString(posting["jobLocationType"])), "telecommute") ||
		strings.Contains(strings.ToLower(locationText), "remote") {
		remoteType = "remote"
		if locationText == "" {
			locationText = "Remote"
		}
	} else if strings.Contains(strings.ToLower(locationText), "hybrid") {
		remoteType = "hybrid"
	}

	applyURL := jsonLDString(posting["url"])
	if applyURL != "" {
		if resolved, err := resolveCareerPageURL(base, applyURL); err == nil {
			applyURL = resolved.String()
		} else {
			applyURL = ""
		}
	}
	if applyURL == "" {
		applyURL = base.String()
	}

	externalID := jsonLDIdentifier(posting["identifier"])
	if externalID == "" {
		externalID = brightStableID(title, locationText, applyURL, jsonLDString(posting["datePosted"]))
	}

	return RawJob{
		ExternalID:     externalID,
		Title:          title,
		Description:    description,
		LocationText:   locationText,
		Country:        country,
		State:          state,
		City:           city,
		RemoteType:     remoteType,
		EmploymentType: jsonLDStringList(posting["employmentType"]),
		ApplyURL:       applyURL,
		SourceURL:      applyURL,
		PostedAt:       parseJSONLDDate(jsonLDString(posting["datePosted"])),
	}, true
}

func jsonLDIdentifier(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		for _, key := range []string{"value", "@id", "name"} {
			if result := jsonLDString(typed[key]); result != "" {
				return result
			}
		}
	}
	return ""
}

func jsonLDString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return fmt.Sprintf("%.0f", typed)
	case map[string]any:
		for _, key := range []string{"name", "value", "@id"} {
			if result := jsonLDString(typed[key]); result != "" {
				return result
			}
		}
	}
	return ""
}

func jsonLDStringList(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if value := jsonLDString(item); value != "" {
				values = append(values, value)
			}
		}
		return strings.Join(values, ", ")
	default:
		return jsonLDString(value)
	}
}

func jsonLDLocation(value any) (locationText, country, state, city string) {
	var locations []map[string]any
	switch typed := value.(type) {
	case map[string]any:
		locations = append(locations, typed)
	case []any:
		for _, item := range typed {
			if location, ok := item.(map[string]any); ok {
				locations = append(locations, location)
			}
		}
	}
	if len(locations) == 0 {
		return "", "", "", ""
	}

	location := locations[0]
	if name := jsonLDString(location["name"]); name != "" {
		locationText = name
	}
	address, _ := location["address"].(map[string]any)
	if address == nil {
		address = location
	}

	city = jsonLDString(address["addressLocality"])
	state = jsonLDString(address["addressRegion"])
	country = jsonLDString(address["addressCountry"])

	if locationText == "" {
		parts := make([]string, 0, 3)
		for _, part := range []string{city, state, country} {
			if part != "" {
				parts = append(parts, part)
			}
		}
		locationText = strings.Join(parts, ", ")
	}

	return locationText, country, state, city
}

func parseJSONLDDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			result := parsed.UTC()
			return &result
		}
	}
	return nil
}
