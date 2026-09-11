package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	xhtml "golang.org/x/net/html"
)

const (
	defaultICIMSMaxPages        = 200
	defaultICIMSMaxBodyBytes    = 2 << 20
	defaultICIMSDetailLimit     = 100
	defaultICIMSDetailWorkers   = 6
	defaultICIMSVerificationMax = 3
)

// ICIMSSource polls public iCIMS Talent Cloud career sites. iCIMS does not
// expose one stable unauthenticated JSON list API; the public listing page is
// HTML and detail pages expose schema.org JobPosting JSON-LD.
type ICIMSSource struct {
	BoardToken       string
	BaseURL          string
	MaxPages         int
	MaxDetailFetches int
	http             *http.Client
	maxBodyBytes     int64
	seenExternalIDs  []string
}

func NewICIMSSource(boardToken string) (*ICIMSSource, error) {
	boardToken = strings.TrimSpace(boardToken)
	if boardToken == "" {
		return nil, errors.New("iCIMS board token is required")
	}

	baseURL := boardToken
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		if !strings.Contains(baseURL, ".") {
			baseURL = "careers-" + baseURL + ".icims.com"
		}
		baseURL = "https://" + baseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("invalid iCIMS board token %q", boardToken)
	}
	host := strings.ToLower(parsed.Hostname())
	if !strings.HasSuffix(host, ".icims.com") {
		return nil, fmt.Errorf("iCIMS board host %q is not an icims.com tenant", host)
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return &ICIMSSource{
		BoardToken:       host,
		BaseURL:          strings.TrimRight(parsed.String(), "/"),
		MaxPages:         defaultICIMSMaxPages,
		MaxDetailFetches: defaultICIMSDetailLimit,
		http:             newPublicHTTPClient(25 * time.Second),
		maxBodyBytes:     defaultICIMSMaxBodyBytes,
	}, nil
}

func (s *ICIMSSource) Name() string { return "ICIMS:" + s.BoardToken }

func (s *ICIMSSource) SeenExternalIDs() []string {
	out := make([]string, len(s.seenExternalIDs))
	copy(out, s.seenExternalIDs)
	return out
}

func (s *ICIMSSource) Fetch(ctx context.Context, _ *Cursor) ([]RawJob, *Cursor, error) {
	maxPages := s.MaxPages
	if maxPages <= 0 {
		maxPages = defaultICIMSMaxPages
	}

	seen := make(map[string]bool)
	jobs := make([]RawJob, 0, 64)
	s.seenExternalIDs = nil
	completedSnapshot := false

	for page := 0; page < maxPages; page++ {
		body, pageURL, err := s.fetchSearchPage(ctx, page)
		if err != nil {
			return nil, nil, err
		}
		pageJobs, err := parseICIMSSearchPage(pageURL, body)
		if err != nil {
			return nil, nil, err
		}

		newJobs := 0
		for _, job := range pageJobs {
			if job.ExternalID == "" || seen[job.ExternalID] {
				continue
			}
			seen[job.ExternalID] = true
			s.seenExternalIDs = append(s.seenExternalIDs, job.ExternalID)
			jobs = append(jobs, job)
			newJobs++
		}
		if newJobs == 0 {
			completedSnapshot = true
			break
		}
	}
	if !completedSnapshot && len(jobs) > 0 {
		// Never allow closure detection from a capped partial listing. Returning
		// an error leaves the previous catalog intact and makes the source
		// retryable rather than falsely closing jobs beyond the page bound.
		return nil, nil, fmt.Errorf("iCIMS source exceeded %d-page safety bound", maxPages)
	}

	s.enrichRelevantDetails(ctx, jobs)
	return jobs, nil, nil
}

func (s *ICIMSSource) fetchSearchPage(ctx context.Context, page int) ([]byte, *url.URL, error) {
	base, err := url.Parse(strings.TrimRight(s.BaseURL, "/") + "/jobs/search")
	if err != nil {
		return nil, nil, err
	}
	q := base.Query()
	q.Set("ss", "1")
	q.Set("pr", fmt.Sprintf("%d", page))
	q.Set("in_iframe", "1")
	base.RawQuery = q.Encode()
	return s.fetchHTML(ctx, base.String())
}

func (s *ICIMSSource) fetchHTML(ctx context.Context, rawURL string) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.1")
	req.Header.Set("Accept-Language", "en-US,en;q=0.8")
	req.Header.Set("User-Agent", "ApplyForge/1.0")

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch iCIMS page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("iCIMS source returned %s", resp.Status)
	}

	limit := s.maxBodyBytes
	if limit <= 0 {
		limit = defaultICIMSMaxBodyBytes
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read iCIMS page: %w", err)
	}
	if int64(len(body)) > limit {
		return nil, nil, fmt.Errorf("iCIMS page exceeds %d-byte safety limit", limit)
	}
	return body, resp.Request.URL, nil
}

func parseICIMSSearchPage(baseURL *url.URL, body []byte) ([]RawJob, error) {
	if baseURL == nil {
		return nil, errors.New("iCIMS page URL is required")
	}
	doc, err := xhtml.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse iCIMS listing HTML: %w", err)
	}

	jobs := make([]RawJob, 0, 32)
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode && strings.EqualFold(node.Data, "li") && nodeHasClass(node, "iCIMS_JobCardItem") {
			if job, ok := parseICIMSJobCard(baseURL, node); ok {
				jobs = append(jobs, job)
			}
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return jobs, nil
}

func parseICIMSJobCard(baseURL *url.URL, card *xhtml.Node) (RawJob, bool) {
	anchor := firstDescendant(card, func(node *xhtml.Node) bool {
		return node.Type == xhtml.ElementNode && strings.EqualFold(node.Data, "a") &&
			nodeHasClass(node, "iCIMS_Anchor") && strings.Contains(htmlAttr(node, "href"), "/jobs/")
	})
	if anchor == nil {
		return RawJob{}, false
	}

	href := htmlAttr(anchor, "href")
	resolved, err := resolveCareerPageURL(baseURL, href)
	if err != nil {
		return RawJob{}, false
	}
	externalID := icimsExternalID(resolved.Path)
	if externalID == "" {
		return RawJob{}, false
	}

	titleNode := firstDescendant(anchor, func(node *xhtml.Node) bool {
		return node.Type == xhtml.ElementNode && strings.EqualFold(node.Data, "h3")
	})
	title := strings.TrimSpace(strings.Join(strings.Fields(rawNodeText(titleNode)), " "))
	if title == "" {
		return RawJob{}, false
	}

	locationRaw := icimsFieldValue(card, "Job Locations")
	locationText, country, state, city := normalizeICIMSListingLocation(locationRaw)
	description := ""
	if descriptionNode := firstDescendant(card, func(node *xhtml.Node) bool {
		return node.Type == xhtml.ElementNode && nodeHasClass(node, "description")
	}); descriptionNode != nil {
		description = strings.TrimSpace(strings.Join(strings.Fields(rawNodeText(descriptionNode)), " "))
	}

	return RawJob{
		ExternalID:   externalID,
		Title:        title,
		Description:  description,
		LocationText: locationText,
		Country:      country,
		State:        state,
		City:         city,
		ApplyURL:     resolved.String(),
		SourceURL:    resolved.String(),
	}, true
}

func (s *ICIMSSource) enrichRelevantDetails(ctx context.Context, jobs []RawJob) {
	limit := s.MaxDetailFetches
	if limit <= 0 {
		limit = defaultICIMSDetailLimit
	}

	indexes := make([]int, 0, limit)
	for index := range jobs {
		if len(indexes) >= limit {
			break
		}
		location := normalizeLocation(jobs[index])
		if location.CountryCode != "US" || classifyTitle(jobs[index].Title).Classification != "IC_SOFTWARE" {
			continue
		}
		indexes = append(indexes, index)
	}
	if len(indexes) == 0 {
		return
	}

	workers := defaultICIMSDetailWorkers
	if workers > len(indexes) {
		workers = len(indexes)
	}
	queue := make(chan int)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for index := range queue {
				detail, err := s.fetchDetail(ctx, jobs[index].SourceURL)
				if err != nil {
					continue
				}
				mergeICIMSDetail(&jobs[index], detail)
			}
		}()
	}
	for _, index := range indexes {
		select {
		case <-ctx.Done():
			close(queue)
			wg.Wait()
			return
		case queue <- index:
		}
	}
	close(queue)
	wg.Wait()
}

type icimsDetail struct {
	Job         RawJob
	CompanyName string
}

func (s *ICIMSSource) fetchDetail(ctx context.Context, rawURL string) (icimsDetail, error) {
	body, finalURL, err := s.fetchHTML(ctx, rawURL)
	if err != nil {
		return icimsDetail{}, err
	}
	inspection, err := parseCareerPageDocument(finalURL, body)
	if err != nil {
		return icimsDetail{}, err
	}
	if len(inspection.Jobs) == 0 {
		return icimsDetail{}, errors.New("iCIMS detail page exposes no JobPosting JSON-LD")
	}

	return icimsDetail{
		Job:         inspection.Jobs[0],
		CompanyName: icimsHiringOrganizationName(body),
	}, nil
}

func mergeICIMSDetail(job *RawJob, detail icimsDetail) {
	if job == nil {
		return
	}
	if detail.CompanyName != "" {
		job.CompanyName = detail.CompanyName
	}
	if detail.Job.Description != "" {
		job.Description = detail.Job.Description
	}
	if detail.Job.LocationText != "" {
		job.LocationText = detail.Job.LocationText
		job.Country = detail.Job.Country
		job.State = detail.Job.State
		job.City = detail.Job.City
	}
	if detail.Job.RemoteType != "" {
		job.RemoteType = detail.Job.RemoteType
	}
	if detail.Job.EmploymentType != "" {
		job.EmploymentType = detail.Job.EmploymentType
	}
	if detail.Job.ApplyURL != "" {
		job.ApplyURL = detail.Job.ApplyURL
	}
	if detail.Job.SourceURL != "" {
		job.SourceURL = detail.Job.SourceURL
	}
	if detail.Job.PostedAt != nil {
		job.PostedAt = detail.Job.PostedAt
	}
}

// ICIMSTenantVerification captures provider-backed evidence used before a
// registry-only iCIMS tenant is promoted to direct polling.
type ICIMSTenantVerification struct {
	Verified     bool
	ObservedName string
}

func (s *ICIMSSource) VerifyCompanyOwnership(ctx context.Context, expectedCompanyName string) (ICIMSTenantVerification, error) {
	expectedCompanyName = strings.TrimSpace(expectedCompanyName)
	if expectedCompanyName == "" {
		return ICIMSTenantVerification{}, errors.New("expected company name is required")
	}
	body, pageURL, err := s.fetchSearchPage(ctx, 0)
	if err != nil {
		return ICIMSTenantVerification{}, err
	}
	jobs, err := parseICIMSSearchPage(pageURL, body)
	if err != nil {
		return ICIMSTenantVerification{}, err
	}
	if len(jobs) == 0 {
		return ICIMSTenantVerification{}, errors.New("iCIMS tenant has no active postings to verify")
	}

	expectedKeys := make(map[string]struct{})
	for _, key := range sourceCompanyExactKeys(expectedCompanyName) {
		expectedKeys[key] = struct{}{}
	}

	max := defaultICIMSVerificationMax
	if max > len(jobs) {
		max = len(jobs)
	}
	observedName := ""
	for _, job := range jobs[:max] {
		detail, detailErr := s.fetchDetail(ctx, job.SourceURL)
		if detailErr != nil || strings.TrimSpace(detail.CompanyName) == "" {
			continue
		}
		if observedName == "" {
			observedName = strings.TrimSpace(detail.CompanyName)
		}
		for _, observedKey := range sourceCompanyExactKeys(detail.CompanyName) {
			if _, ok := expectedKeys[observedKey]; ok {
				return ICIMSTenantVerification{Verified: true, ObservedName: detail.CompanyName}, nil
			}
		}
	}
	if observedName == "" {
		return ICIMSTenantVerification{}, errors.New("iCIMS JobPosting data did not expose hiringOrganization identity")
	}
	return ICIMSTenantVerification{ObservedName: observedName}, nil
}

func icimsHiringOrganizationName(body []byte) string {
	doc, err := xhtml.Parse(bytes.NewReader(body))
	if err != nil {
		return ""
	}

	var observed string
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if observed != "" {
			return
		}
		if node.Type == xhtml.ElementNode && strings.EqualFold(node.Data, "script") &&
			strings.EqualFold(strings.TrimSpace(htmlAttr(node, "type")), "application/ld+json") {
			var root any
			if err := json.Unmarshal([]byte(rawNodeText(node)), &root); err == nil {
				var postings []map[string]any
				collectJobPostingObjects(root, &postings)
				for _, posting := range postings {
					if name := jsonLDString(posting["hiringOrganization"]); name != "" {
						observed = name
						return
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return observed
}

func nodeHasClass(node *xhtml.Node, target string) bool {
	if node == nil {
		return false
	}
	for _, className := range strings.Fields(htmlAttr(node, "class")) {
		if className == target {
			return true
		}
	}
	return false
}

func firstDescendant(root *xhtml.Node, predicate func(*xhtml.Node) bool) *xhtml.Node {
	if root == nil {
		return nil
	}
	if predicate(root) {
		return root
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if match := firstDescendant(child, predicate); match != nil {
			return match
		}
	}
	return nil
}

func icimsFieldValue(card *xhtml.Node, label string) string {
	labelNode := firstDescendant(card, func(node *xhtml.Node) bool {
		return node.Type == xhtml.ElementNode && strings.EqualFold(node.Data, "span") &&
			nodeHasClass(node, "field-label") && strings.EqualFold(strings.TrimSpace(rawNodeText(node)), label)
	})
	if labelNode == nil {
		return ""
	}
	for sibling := labelNode.NextSibling; sibling != nil; sibling = sibling.NextSibling {
		if sibling.Type == xhtml.ElementNode && strings.EqualFold(sibling.Data, "span") {
			return strings.TrimSpace(strings.Join(strings.Fields(rawNodeText(sibling)), " "))
		}
	}
	return ""
}

func icimsExternalID(path string) string {
	parts := splitPath(path)
	for index, part := range parts {
		if strings.EqualFold(part, "jobs") && index+1 < len(parts) {
			return strings.TrimSpace(parts[index+1])
		}
	}
	return ""
}

func normalizeICIMSListingLocation(raw string) (locationText, country, state, city string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", "", ""
	}
	parts := strings.SplitN(raw, "-", 3)
	if len(parts) == 3 && len(strings.TrimSpace(parts[0])) == 2 && len(strings.TrimSpace(parts[1])) == 2 {
		country = strings.ToUpper(strings.TrimSpace(parts[0]))
		state = strings.ToUpper(strings.TrimSpace(parts[1]))
		city = strings.TrimSpace(parts[2])
		locationText = strings.Join([]string{city, state, country}, ", ")
		return locationText, country, state, city
	}
	return raw, "", "", ""
}
