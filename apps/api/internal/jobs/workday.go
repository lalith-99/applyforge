package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	workdayPageSize             = 20
	defaultWorkdayMaxJobs       = 10000
	defaultWorkdayDetailAgeDays = 7
)

type WorkdaySource struct {
	Host         string
	Tenant       string
	Site         string
	BaseURL      string
	MaxJobs      int
	DetailMaxAge time.Duration
	http         *http.Client
	now          func() time.Time
	seenExternal []string
}

type workdayListResponse struct {
	Total       int `json:"total"`
	JobPostings []struct {
		Title         string   `json:"title"`
		ExternalPath  string   `json:"externalPath"`
		LocationsText string   `json:"locationsText"`
		PostedOn      string   `json:"postedOn"`
		BulletFields  []string `json:"bulletFields"`
	} `json:"jobPostings"`
}

type workdayDetailResponse struct {
	JobPostingInfo struct {
		Title                  string   `json:"title"`
		JobReqID               string   `json:"jobReqId"`
		JobPostingID           string   `json:"jobPostingId"`
		JobDescription         string   `json:"jobDescription"`
		StartDate              string   `json:"startDate"`
		Location               string   `json:"location"`
		AdditionalLocations    []string `json:"additionalLocations"`
		TimeType               string   `json:"timeType"`
		RemoteType             string   `json:"remoteType"`
		JobRequisitionLocation struct {
			Country struct {
				Alpha2Code string `json:"alpha2Code"`
				Descriptor string `json:"descriptor"`
			} `json:"country"`
		} `json:"jobRequisitionLocation"`
	} `json:"jobPostingInfo"`
}

func NewWorkdaySource(boardToken string) (*WorkdaySource, error) {
	host, tenant, site, err := parseWorkdayBoardToken(boardToken)
	if err != nil {
		return nil, err
	}

	maxJobs := defaultWorkdayMaxJobs
	if raw := strings.TrimSpace(os.Getenv("WORKDAY_MAX_JOBS_PER_POLL")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 20 || value > 10000 {
			return nil, errors.New("WORKDAY_MAX_JOBS_PER_POLL must be between 20 and 10000")
		}
		maxJobs = value
	}

	detailAgeDays := defaultWorkdayDetailAgeDays
	if raw := strings.TrimSpace(os.Getenv("WORKDAY_DETAIL_MAX_AGE_DAYS")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 90 {
			return nil, errors.New("WORKDAY_DETAIL_MAX_AGE_DAYS must be between 1 and 90")
		}
		detailAgeDays = value
	}

	return &WorkdaySource{
		Host:         host,
		Tenant:       tenant,
		Site:         site,
		BaseURL:      "https://" + host,
		MaxJobs:      maxJobs,
		DetailMaxAge: time.Duration(detailAgeDays) * 24 * time.Hour,
		http:         &http.Client{Timeout: 30 * time.Second},
		now:          time.Now,
	}, nil
}

func (s *WorkdaySource) Name() string {
	return "WORKDAY:" + workdayBoardToken(s.Host, s.Tenant, s.Site)
}

func (s *WorkdaySource) SeenExternalIDs() []string {
	return append([]string(nil), s.seenExternal...)
}

func (s *WorkdaySource) Fetch(ctx context.Context, _ *Cursor) ([]RawJob, *Cursor, error) {
	s.seenExternal = s.seenExternal[:0]

	var out []RawJob
	offset := 0
	total := -1

	for {
		page, err := s.fetchListPage(ctx, offset)
		if err != nil {
			return nil, nil, err
		}
		if total < 0 {
			total = page.Total
			if total > s.MaxJobs {
				return nil, nil, fmt.Errorf(
					"workday board reports %d jobs, above configured safe maximum %d",
					total,
					s.MaxJobs,
				)
			}
		}

		if len(page.JobPostings) == 0 {
			break
		}

		for _, posting := range page.JobPostings {
			externalID := workdayListingExternalID(posting.ExternalPath, posting.BulletFields)
			if externalID == "" {
				continue
			}
			s.seenExternal = append(s.seenExternal, externalID)

			postedAt := parseWorkdayPostedOn(posting.PostedOn, s.now())
			if postedAt != nil && s.now().Sub(*postedAt) > s.DetailMaxAge {
				continue
			}

			raw, err := s.fetchDetail(ctx, posting.ExternalPath, externalID, postedAt, posting.Title, posting.LocationsText)
			if err != nil {
				return nil, nil, err
			}
			out = append(out, raw)
		}

		offset += len(page.JobPostings)
		if offset >= total {
			break
		}
		if offset >= s.MaxJobs {
			return nil, nil, fmt.Errorf(
				"workday pagination reached configured safe maximum %d before reported total %d",
				s.MaxJobs,
				total,
			)
		}
	}

	return out, nil, nil
}

func (s *WorkdaySource) fetchListPage(ctx context.Context, offset int) (workdayListResponse, error) {
	payload := map[string]any{
		"appliedFacets": map[string]any{},
		"limit":         workdayPageSize,
		"offset":        offset,
		"searchText":    "",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return workdayListResponse{}, err
	}

	endpoint := fmt.Sprintf(
		"%s/wday/cxs/%s/%s/jobs",
		strings.TrimRight(s.BaseURL, "/"),
		url.PathEscape(s.Tenant),
		url.PathEscape(s.Site),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return workdayListResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("User-Agent", "ApplyForge/1.0")

	resp, err := s.http.Do(req)
	if err != nil {
		return workdayListResponse{}, fmt.Errorf("fetch Workday board %s/%s: %w", s.Tenant, s.Site, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return workdayListResponse{}, fmt.Errorf(
			"Workday board %s/%s returned %s",
			s.Tenant,
			s.Site,
			resp.Status,
		)
	}

	var decoded workdayListResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return workdayListResponse{}, fmt.Errorf("decode Workday list response: %w", err)
	}
	return decoded, nil
}

func (s *WorkdaySource) fetchDetail(
	ctx context.Context,
	externalPath string,
	externalID string,
	fallbackPostedAt *time.Time,
	fallbackTitle string,
	fallbackLocation string,
) (RawJob, error) {
	slug := workdayExternalPathSlug(externalPath)
	if slug == "" {
		return RawJob{}, fmt.Errorf("Workday posting %q has no detail slug", externalPath)
	}

	endpoint := fmt.Sprintf(
		"%s/wday/cxs/%s/%s/job/%s",
		strings.TrimRight(s.BaseURL, "/"),
		url.PathEscape(s.Tenant),
		url.PathEscape(s.Site),
		url.PathEscape(slug),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return RawJob{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("User-Agent", "ApplyForge/1.0")

	resp, err := s.http.Do(req)
	if err != nil {
		return RawJob{}, fmt.Errorf("fetch Workday job %s: %w", externalID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return RawJob{}, fmt.Errorf("Workday job %s returned %s", externalID, resp.Status)
	}

	var decoded workdayDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return RawJob{}, fmt.Errorf("decode Workday job %s: %w", externalID, err)
	}

	info := decoded.JobPostingInfo
	title := strings.TrimSpace(info.Title)
	if title == "" {
		title = strings.TrimSpace(fallbackTitle)
	}
	location := strings.TrimSpace(info.Location)
	if location == "" {
		location = strings.TrimSpace(fallbackLocation)
	}

	postedAt := fallbackPostedAt
	if parsed := parseWorkdayDate(info.StartDate); parsed != nil {
		postedAt = parsed
	}

	country := strings.TrimSpace(info.JobRequisitionLocation.Country.Descriptor)
	if country == "" {
		country = strings.TrimSpace(info.JobRequisitionLocation.Country.Alpha2Code)
	}

	publicURL := fmt.Sprintf(
		"https://%s/en-US/%s%s",
		s.Host,
		url.PathEscape(s.Site),
		normalizeWorkdayExternalPath(externalPath),
	)

	remoteType := inferWorkdayRemoteType(info.RemoteType, location)
	return RawJob{
		ExternalID:     externalID,
		Title:          title,
		Description:    stripTags(info.JobDescription),
		LocationText:   location,
		Country:        country,
		RemoteType:     remoteType,
		EmploymentType: info.TimeType,
		ApplyURL:       publicURL,
		SourceURL:      publicURL,
		PostedAt:       postedAt,
	}, nil
}

func workdayBoardToken(host, tenant, site string) string {
	return strings.Join([]string{
		strings.ToLower(strings.TrimSpace(host)),
		strings.TrimSpace(tenant),
		url.PathEscape(strings.TrimSpace(site)),
	}, "|")
}

func parseWorkdayBoardToken(token string) (string, string, string, error) {
	parts := strings.Split(token, "|")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid Workday board token")
	}
	host := strings.ToLower(strings.TrimSpace(parts[0]))
	tenant := strings.TrimSpace(parts[1])
	site, err := url.PathUnescape(strings.TrimSpace(parts[2]))
	if err != nil {
		return "", "", "", fmt.Errorf("decode Workday site token: %w", err)
	}
	if !isWorkdayHost(host) || tenant == "" || site == "" {
		return "", "", "", fmt.Errorf("invalid Workday board token")
	}
	if strings.ToLower(strings.Split(host, ".")[0]) != strings.ToLower(tenant) {
		return "", "", "", fmt.Errorf("Workday tenant does not match host")
	}
	return host, tenant, site, nil
}

func parseWorkdayCareerURL(raw string) (DiscoveredCompanySource, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" {
		return DiscoveredCompanySource{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if !isWorkdayHost(host) {
		return DiscoveredCompanySource{}, false
	}

	parts := splitPath(parsed.Path)
	if len(parts) == 0 {
		return DiscoveredCompanySource{}, false
	}
	if looksLikeLocale(parts[0]) {
		parts = parts[1:]
	}
	if len(parts) == 0 || strings.EqualFold(parts[0], "job") {
		return DiscoveredCompanySource{}, false
	}

	site := parts[0]
	tenant := strings.Split(host, ".")[0]
	if tenant == "" || site == "" {
		return DiscoveredCompanySource{}, false
	}

	return DiscoveredCompanySource{
		SourceType:  "WORKDAY",
		BoardToken:  workdayBoardToken(host, tenant, site),
		SourceURL:   "https://" + host + "/" + url.PathEscape(site),
		Confidence:  1,
		Monitorable: true,
	}, true
}

func isWorkdayHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return strings.HasSuffix(host, ".myworkdayjobs.com") ||
		strings.HasSuffix(host, ".myworkdaysite.com")
}

func looksLikeLocale(value string) bool {
	if len(value) != 5 || value[2] != '-' {
		return false
	}
	isLetter := func(b byte) bool {
		return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
	}
	return isLetter(value[0]) && isLetter(value[1]) &&
		isLetter(value[3]) && isLetter(value[4])
}

func workdayListingExternalID(externalPath string, bulletFields []string) string {
	for _, value := range bulletFields {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return workdayExternalPathSlug(externalPath)
}

func workdayExternalPathSlug(externalPath string) string {
	parts := splitPath(externalPath)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func normalizeWorkdayExternalPath(externalPath string) string {
	externalPath = strings.TrimSpace(externalPath)
	if externalPath == "" {
		return ""
	}
	if strings.HasPrefix(externalPath, "/") {
		return externalPath
	}
	return "/" + externalPath
}

func parseWorkdayDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			result := parsed.UTC()
			return &result
		}
	}
	return nil
}

func parseWorkdayPostedOn(value string, now time.Time) *time.Time {
	text := strings.ToLower(strings.TrimSpace(value))
	text = strings.TrimPrefix(text, "posted ")
	switch text {
	case "today", "just posted":
		result := now.UTC()
		return &result
	case "yesterday":
		result := now.UTC().Add(-24 * time.Hour)
		return &result
	}

	fields := strings.Fields(strings.ReplaceAll(text, "+", ""))
	if len(fields) >= 2 {
		number, err := strconv.Atoi(fields[0])
		if err == nil {
			switch {
			case strings.HasPrefix(fields[1], "hour"):
				result := now.UTC().Add(-time.Duration(number) * time.Hour)
				return &result
			case strings.HasPrefix(fields[1], "day"):
				result := now.UTC().Add(-time.Duration(number) * 24 * time.Hour)
				return &result
			}
		}
	}
	return parseWorkdayDate(value)
}

func inferWorkdayRemoteType(remoteType, location string) string {
	text := strings.ToLower(remoteType + " " + location)
	switch {
	case strings.Contains(text, "hybrid"):
		return "hybrid"
	case strings.Contains(text, "remote"):
		return "remote"
	default:
		return "onsite"
	}
}
