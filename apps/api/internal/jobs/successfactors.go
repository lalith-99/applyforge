package jobs

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const defaultSuccessFactorsMaxBodyBytes = 16 << 20

var (
	successFactorsInvalidEmptyElementRE = regexp.MustCompile(`(?s)<>\s*[^<]*?</>`)
	successFactorsTitleLocationRE       = regexp.MustCompile(`^(.+?)\s*\(([^()]+)\)\s*$`)
)

type successFactorsFeedMode string

const (
	successFactorsRSS    successFactorsFeedMode = "RSS"
	successFactorsLegacy successFactorsFeedMode = "LEGACY_XML"
)

// SuccessFactorsSource polls the two public, credential-free feed families
// exposed by SAP SuccessFactors Recruiting: Recruiting Marketing RSS and the
// legacy Recruiting Management Job-Listing XML endpoint.
type SuccessFactorsSource struct {
	BoardToken      string
	FeedURL         string
	Mode            successFactorsFeedMode
	TenantKey       string
	LegacyCompanyID string
	http            *http.Client
	maxBodyBytes    int64
	seenExternalIDs []string
}

func NewSuccessFactorsSource(boardToken string) (*SuccessFactorsSource, error) {
	boardToken = strings.TrimSpace(boardToken)
	if boardToken == "" {
		return nil, errors.New("SuccessFactors board token is required")
	}

	rawURL := boardToken
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		if !strings.Contains(rawURL, ".") {
			return nil, fmt.Errorf("SuccessFactors board token %q must be a public host or URL", boardToken)
		}
		rawURL = "https://" + rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("invalid SuccessFactors board token %q", boardToken)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported SuccessFactors URL scheme %q", parsed.Scheme)
	}

	host := strings.ToLower(parsed.Hostname())
	companyID := strings.TrimSpace(parsed.Query().Get("company"))
	if companyID != "" && isSuccessFactorsLegacyHost(host) {
		feed := &url.URL{Scheme: "https", Host: host, Path: "/career"}
		q := feed.Query()
		q.Set("company", companyID)
		q.Set("career_ns", "job_listing_summary")
		q.Set("resultType", "XML")
		if locale := strings.TrimSpace(parsed.Query().Get("rcm_site_locale")); locale != "" {
			q.Set("rcm_site_locale", locale)
		}
		feed.RawQuery = q.Encode()
		return &SuccessFactorsSource{
			BoardToken:      boardToken,
			FeedURL:         feed.String(),
			Mode:            successFactorsLegacy,
			TenantKey:       host + "|" + companyID,
			LegacyCompanyID: companyID,
			http:            newPublicHTTPClient(30 * time.Second),
			maxBodyBytes:    defaultSuccessFactorsMaxBodyBytes,
		}, nil
	}

	feed := &url.URL{Scheme: parsed.Scheme, Host: parsed.Host, Path: "/sitemal.xml"}
	if strings.EqualFold(strings.TrimRight(parsed.Path, "/"), "/sitemal.xml") {
		feed = parsed
		feed.RawQuery = ""
		feed.Fragment = ""
	}
	return &SuccessFactorsSource{
		BoardToken:   boardToken,
		FeedURL:      feed.String(),
		Mode:         successFactorsRSS,
		TenantKey:    host,
		http:         newPublicHTTPClient(30 * time.Second),
		maxBodyBytes: defaultSuccessFactorsMaxBodyBytes,
	}, nil
}

func (s *SuccessFactorsSource) Name() string { return "SUCCESSFACTORS:" + s.TenantKey }

func (s *SuccessFactorsSource) SeenExternalIDs() []string {
	out := make([]string, len(s.seenExternalIDs))
	copy(out, s.seenExternalIDs)
	return out
}

type successFactorsFeedResult struct {
	Jobs      []RawJob
	Employers []string
}

func (s *SuccessFactorsSource) Fetch(ctx context.Context, _ *Cursor) ([]RawJob, *Cursor, error) {
	result, err := s.fetchFeed(ctx)
	if err != nil {
		return nil, nil, err
	}
	s.seenExternalIDs = s.seenExternalIDs[:0]
	for _, job := range result.Jobs {
		if job.ExternalID != "" {
			s.seenExternalIDs = append(s.seenExternalIDs, job.ExternalID)
		}
	}
	return result.Jobs, nil, nil
}

func (s *SuccessFactorsSource) fetchFeed(ctx context.Context) (successFactorsFeedResult, error) {
	body, err := s.fetchXML(ctx)
	if err != nil {
		return successFactorsFeedResult{}, err
	}
	if s.Mode == successFactorsLegacy {
		return s.parseLegacyFeed(body)
	}
	return s.parseRSSFeed(body)
}

func (s *SuccessFactorsSource) fetchXML(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.FeedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml;q=0.9, */*;q=0.1")
	req.Header.Set("User-Agent", "ApplyForge/1.0")

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch SuccessFactors feed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SuccessFactors feed returned %s", resp.Status)
	}
	limit := s.maxBodyBytes
	if limit <= 0 {
		limit = defaultSuccessFactorsMaxBodyBytes
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read SuccessFactors feed: %w", err)
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("SuccessFactors feed exceeds %d-byte safety limit", limit)
	}
	return body, nil
}

type successFactorsRSSDocument struct {
	XMLName xml.Name                 `xml:"rss"`
	Channel successFactorsRSSChannel `xml:"channel"`
}

type successFactorsRSSChannel struct {
	Title string                  `xml:"title"`
	Items []successFactorsRSSItem `xml:"item"`
}

type successFactorsRSSItem struct {
	Title              string `xml:"title"`
	Link               string `xml:"link"`
	GUID               string `xml:"guid"`
	Description        string `xml:"description"`
	PubDate            string `xml:"pubDate"`
	Date               string `xml:"date"`
	PostDate           string `xml:"postDate"`
	PublishedDate      string `xml:"publishedDate"`
	GoogleID           string `xml:"http://base.google.com/ns/1.0 id"`
	GoogleEmployer     string `xml:"http://base.google.com/ns/1.0 employer"`
	GoogleLocation     string `xml:"http://base.google.com/ns/1.0 location"`
	GoogleJobType      string `xml:"http://base.google.com/ns/1.0 job_type"`
	GoogleEmployment   string `xml:"http://base.google.com/ns/1.0 employment_type"`
	GoogleJobFunction  string `xml:"http://base.google.com/ns/1.0 job_function"`
	GoogleExpirationAt string `xml:"http://base.google.com/ns/1.0 expiration_date"`
}

func (s *SuccessFactorsSource) parseRSSFeed(body []byte) (successFactorsFeedResult, error) {
	var document successFactorsRSSDocument
	if err := xml.Unmarshal(body, &document); err != nil {
		return successFactorsFeedResult{}, fmt.Errorf("decode SuccessFactors RSS feed: %w", err)
	}
	if !strings.EqualFold(document.XMLName.Local, "rss") {
		return successFactorsFeedResult{}, fmt.Errorf("SuccessFactors returned non-RSS XML root %q", document.XMLName.Local)
	}

	channelEmployer := normalizeSuccessFactorsFeedEmployer(document.Channel.Title)
	employers := make([]string, 0, 4)
	if channelEmployer != "" {
		employers = append(employers, channelEmployer)
	}
	seenEmployers := map[string]bool{}
	for _, employer := range employers {
		seenEmployers[strings.ToLower(employer)] = true
	}

	seenIDs := make(map[string]bool)
	jobs := make([]RawJob, 0, len(document.Channel.Items))
	for _, item := range document.Channel.Items {
		link := strings.TrimSpace(item.Link)
		if link == "" {
			continue
		}
		rawID := firstNonEmpty(strings.TrimSpace(item.GoogleID), strings.TrimSpace(item.GUID))
		if rawID == "" {
			rawID = successFactorsIDFromURL(link)
		}
		if rawID == "" {
			continue
		}
		externalID := s.canonicalExternalID(rawID)
		if seenIDs[externalID] {
			continue
		}
		seenIDs[externalID] = true

		title, titleLocation := splitSuccessFactorsTitleLocation(strings.TrimSpace(item.Title))
		if title == "" {
			continue
		}
		location := strings.TrimSpace(item.GoogleLocation)
		if location == "" {
			location = titleLocation
		}
		jobType := firstNonEmpty(strings.TrimSpace(item.GoogleJobType), strings.TrimSpace(item.GoogleEmployment))
		remoteType := ""
		if strings.Contains(strings.ToLower(location), "remote") {
			remoteType = "remote"
		}
		postedAt := parseSuccessFactorsDate(firstNonEmpty(
			strings.TrimSpace(item.PubDate),
			strings.TrimSpace(item.PostDate),
			strings.TrimSpace(item.PublishedDate),
			strings.TrimSpace(item.Date),
		))

		jobs = append(jobs, RawJob{
			ExternalID:     externalID,
			Title:          title,
			Description:    stripTags(item.Description),
			LocationText:   location,
			RemoteType:     remoteType,
			EmploymentType: jobType,
			ApplyURL:       link,
			SourceURL:      link,
			PostedAt:       postedAt,
		})

		if employer := normalizeSuccessFactorsFeedEmployer(item.GoogleEmployer); employer != "" {
			key := strings.ToLower(employer)
			if !seenEmployers[key] {
				seenEmployers[key] = true
				employers = append(employers, employer)
			}
		}
	}
	return successFactorsFeedResult{Jobs: jobs, Employers: employers}, nil
}

type successFactorsLegacyDocument struct {
	XMLName xml.Name                  `xml:"Job-Listing"`
	Jobs    []successFactorsLegacyJob `xml:"Job"`
}

type successFactorsLegacyJob struct {
	ReqID          string `xml:"ReqId"`
	JobTitle       string `xml:"JobTitle"`
	JobDescription string `xml:"Job-Description"`
	Location       string `xml:"Location"`
	City           string `xml:"City"`
	State          string `xml:"State"`
	Country        string `xml:"Country"`
	PostedDate     string `xml:"Posted-Date"`
	Department     string `xml:"Department"`
	JobType        string `xml:"JobType"`
	EmploymentType string `xml:"EmploymentType"`
	CompanyName    string `xml:"CompanyName"`
	Employer       string `xml:"Employer"`
}

func (s *SuccessFactorsSource) parseLegacyFeed(body []byte) (successFactorsFeedResult, error) {
	body = successFactorsInvalidEmptyElementRE.ReplaceAll(body, nil)
	var document successFactorsLegacyDocument
	if err := xml.Unmarshal(body, &document); err != nil {
		return successFactorsFeedResult{}, fmt.Errorf("decode SuccessFactors legacy XML: %w", err)
	}
	if document.XMLName.Local != "Job-Listing" {
		return successFactorsFeedResult{}, fmt.Errorf("SuccessFactors returned non-Job-Listing XML root %q", document.XMLName.Local)
	}

	seenIDs := make(map[string]bool)
	seenEmployers := make(map[string]bool)
	jobs := make([]RawJob, 0, len(document.Jobs))
	employers := make([]string, 0, 2)
	for _, item := range document.Jobs {
		rawID := strings.TrimSpace(item.ReqID)
		title := strings.TrimSpace(html.UnescapeString(item.JobTitle))
		if rawID == "" || title == "" {
			continue
		}
		externalID := s.canonicalExternalID(rawID)
		if seenIDs[externalID] {
			continue
		}
		seenIDs[externalID] = true

		location := strings.TrimSpace(html.UnescapeString(item.Location))
		if location == "" {
			parts := make([]string, 0, 3)
			for _, value := range []string{item.City, item.State, item.Country} {
				value = strings.TrimSpace(html.UnescapeString(value))
				if value != "" && !isEmptySuccessFactorsLegacyValue(value) {
					parts = append(parts, value)
				}
			}
			location = strings.Join(parts, ", ")
		}

		jobURL := &url.URL{Scheme: "https", Host: successFactorsFeedHost(s.FeedURL), Path: "/sfcareer/jobreqcareer"}
		q := jobURL.Query()
		q.Set("jobId", rawID)
		q.Set("company", s.LegacyCompanyID)
		jobURL.RawQuery = q.Encode()

		jobs = append(jobs, RawJob{
			ExternalID:     externalID,
			Title:          title,
			Description:    stripTags(item.JobDescription),
			LocationText:   location,
			EmploymentType: firstNonEmpty(strings.TrimSpace(item.EmploymentType), strings.TrimSpace(item.JobType)),
			ApplyURL:       jobURL.String(),
			SourceURL:      jobURL.String(),
			PostedAt:       parseSuccessFactorsDate(strings.TrimSpace(item.PostedDate)),
		})

		for _, observed := range []string{item.CompanyName, item.Employer} {
			observed = normalizeSuccessFactorsFeedEmployer(observed)
			if observed == "" || seenEmployers[strings.ToLower(observed)] {
				continue
			}
			seenEmployers[strings.ToLower(observed)] = true
			employers = append(employers, observed)
		}
	}
	return successFactorsFeedResult{Jobs: jobs, Employers: employers}, nil
}

func (s *SuccessFactorsSource) canonicalExternalID(rawID string) string {
	rawID = strings.TrimSpace(rawID)
	if rawID == "" {
		return ""
	}
	return strings.ToLower(s.TenantKey) + ":" + rawID
}

type SuccessFactorsTenantVerification struct {
	Verified     bool
	ObservedName string
}

// VerifyCompanyOwnership requires employer identity from the public feed
// itself. A legacy feed that exposes jobs but no employer/company field stays
// registry-only rather than relying on the discovery directory alone.
func (s *SuccessFactorsSource) VerifyCompanyOwnership(ctx context.Context, expectedCompanyName string) (SuccessFactorsTenantVerification, error) {
	expectedCompanyName = strings.TrimSpace(expectedCompanyName)
	if expectedCompanyName == "" {
		return SuccessFactorsTenantVerification{}, errors.New("expected company name is required")
	}
	result, err := s.fetchFeed(ctx)
	if err != nil {
		return SuccessFactorsTenantVerification{}, err
	}
	if len(result.Jobs) == 0 {
		return SuccessFactorsTenantVerification{}, errors.New("SuccessFactors feed has no active postings to verify")
	}
	if len(result.Employers) == 0 {
		return SuccessFactorsTenantVerification{}, errors.New("SuccessFactors feed did not expose employer identity")
	}

	expected := make(map[string]struct{})
	for _, key := range sourceCompanyExactKeys(expectedCompanyName) {
		expected[key] = struct{}{}
	}
	for _, employer := range result.Employers {
		for _, observedKey := range sourceCompanyExactKeys(employer) {
			if _, ok := expected[observedKey]; ok {
				return SuccessFactorsTenantVerification{Verified: true, ObservedName: employer}, nil
			}
		}
	}
	return SuccessFactorsTenantVerification{ObservedName: result.Employers[0]}, nil
}

func isSuccessFactorsLegacyHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "successfactors.com" || strings.HasSuffix(host, ".successfactors.com")
}

func successFactorsFeedHost(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

func successFactorsIDFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	for _, key := range []string{"jobId", "jobid", "jobID", "job"} {
		if value := strings.TrimSpace(parsed.Query().Get(key)); value != "" {
			return value
		}
	}
	parts := splitPath(parsed.Path)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return strings.TrimSpace(raw)
}

func splitSuccessFactorsTitleLocation(raw string) (string, string) {
	raw = strings.TrimSpace(html.UnescapeString(raw))
	match := successFactorsTitleLocationRE.FindStringSubmatch(raw)
	if len(match) != 3 {
		return raw, ""
	}
	candidate := strings.TrimSpace(match[2])
	if !strings.Contains(candidate, ",") && !regexp.MustCompile(`\b[A-Z]{2}\b`).MatchString(candidate) {
		return raw, ""
	}
	return strings.TrimSpace(match[1]), candidate
}

func normalizeSuccessFactorsFeedEmployer(raw string) string {
	value := strings.TrimSpace(html.UnescapeString(raw))
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	for _, prefix := range []string{"jobs at ", "careers at ", "jobs - ", "careers - "} {
		if strings.HasPrefix(lower, prefix) {
			value = strings.TrimSpace(value[len(prefix):])
			lower = strings.ToLower(value)
			break
		}
	}
	for _, suffix := range []string{" careers", " jobs", " career opportunities"} {
		if strings.HasSuffix(lower, suffix) {
			value = strings.TrimSpace(value[:len(value)-len(suffix)])
			break
		}
	}
	return value
}

func isEmptySuccessFactorsLegacyValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "n/a", "na", "none", "not applicable":
		return true
	default:
		return false
	}
}

func parseSuccessFactorsDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{
		time.RFC3339,
		time.RFC3339Nano,
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"2006-01-02",
		"01/02/2006",
		"02/01/2006",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			parsed = parsed.UTC()
			return &parsed
		}
	}
	return nil
}
