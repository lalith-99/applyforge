package jobs

import (
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

const defaultSerpAPIGoogleJobsBaseURL = "https://serpapi.com/search.json"

type SerpAPIGoogleJobsConfig struct {
	APIKey   string
	BaseURL  string
	MaxPages int
}

func SerpAPIGoogleJobsConfigFromEnv() (SerpAPIGoogleJobsConfig, error) {
	cfg := SerpAPIGoogleJobsConfig{
		APIKey:   strings.TrimSpace(os.Getenv("SERPAPI_API_KEY")),
		BaseURL:  strings.TrimSpace(os.Getenv("SERPAPI_GOOGLE_JOBS_BASE_URL")),
		MaxPages: 5,
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultSerpAPIGoogleJobsBaseURL
	}
	if raw := strings.TrimSpace(os.Getenv("SERPAPI_GOOGLE_JOBS_MAX_PAGES")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 && value <= 20 {
			cfg.MaxPages = value
		}
	}
	if cfg.APIKey == "" {
		return SerpAPIGoogleJobsConfig{}, errors.New("SERPAPI_API_KEY is not configured")
	}
	return cfg, nil
}

type SerpAPIGoogleJobsSource struct {
	Shard string
	cfg   SerpAPIGoogleJobsConfig
	http  *http.Client
	now   func() time.Time
}

func NewSerpAPIGoogleJobsSource(shard string, cfg SerpAPIGoogleJobsConfig) *SerpAPIGoogleJobsSource {
	return &SerpAPIGoogleJobsSource{
		Shard: shard,
		cfg:   cfg,
		http:  &http.Client{Timeout: 30 * time.Second},
		now:   time.Now,
	}
}

func (s *SerpAPIGoogleJobsSource) Name() string { return "SERPAPI_GOOGLE_JOBS:" + s.Shard }

type serpAPIGoogleJobsResponse struct {
	JobsResults []struct {
		Title       string `json:"title"`
		CompanyName string `json:"company_name"`
		Location    string `json:"location"`
		Via         string `json:"via"`
		Description string `json:"description"`
		ShareLink   string `json:"share_link"`
		JobID       string `json:"job_id"`
		DetectedExtensions struct {
			PostedAt     string `json:"posted_at"`
			ScheduleType string `json:"schedule_type"`
			WorkFromHome bool   `json:"work_from_home"`
		} `json:"detected_extensions"`
		ApplyOptions []struct {
			Title string `json:"title"`
			Link  string `json:"link"`
		} `json:"apply_options"`
	} `json:"jobs_results"`
	Pagination struct {
		NextPageToken string `json:"next_page_token"`
	} `json:"serpapi_pagination"`
}

func (s *SerpAPIGoogleJobsSource) Fetch(ctx context.Context, _ *Cursor) ([]RawJob, *Cursor, error) {
	var (
		out       []RawJob
		nextToken string
	)
	for page := 0; page < s.cfg.MaxPages; page++ {
		response, err := s.fetchPage(ctx, nextToken)
		if err != nil {
			return nil, nil, err
		}
		for _, record := range response.JobsResults {
			raw, ok := s.mapJob(record)
			if ok {
				out = append(out, raw)
			}
		}
		nextToken = response.Pagination.NextPageToken
		if nextToken == "" || len(response.JobsResults) == 0 {
			break
		}
	}
	return out, nil, nil
}

func (s *SerpAPIGoogleJobsSource) fetchPage(ctx context.Context, nextPageToken string) (serpAPIGoogleJobsResponse, error) {
	parsed, err := url.Parse(s.cfg.BaseURL)
	if err != nil {
		return serpAPIGoogleJobsResponse{}, err
	}
	q := parsed.Query()
	q.Set("engine", "google_jobs")
	q.Set("q", googleJobsQueryForShard(s.Shard))
	q.Set("location", "United States")
	q.Set("gl", "us")
	q.Set("hl", "en")
	q.Set("api_key", s.cfg.APIKey)
	q.Set("output", "json")
	if nextPageToken != "" {
		q.Set("next_page_token", nextPageToken)
	}
	parsed.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return serpAPIGoogleJobsResponse{}, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return serpAPIGoogleJobsResponse{}, fmt.Errorf("fetch Google Jobs via SerpApi: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return serpAPIGoogleJobsResponse{}, fmt.Errorf("Google Jobs via SerpApi returned %s", resp.Status)
	}

	var result serpAPIGoogleJobsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return serpAPIGoogleJobsResponse{}, fmt.Errorf("decode Google Jobs response: %w", err)
	}
	return result, nil
}

func googleJobsQueryForShard(shard string) string {
	titles := brightDataTitleShards[shard]
	if len(titles) == 0 {
		titles = brightDataLegacySoftwareTitles
	}
	quoted := make([]string, 0, len(titles))
	for _, title := range titles {
		quoted = append(quoted, `"`+title+`"`)
	}
	return strings.Join(quoted, " OR ")
}

func (s *SerpAPIGoogleJobsSource) mapJob(record struct {
	Title       string `json:"title"`
	CompanyName string `json:"company_name"`
	Location    string `json:"location"`
	Via         string `json:"via"`
	Description string `json:"description"`
	ShareLink   string `json:"share_link"`
	JobID       string `json:"job_id"`
	DetectedExtensions struct {
		PostedAt     string `json:"posted_at"`
		ScheduleType string `json:"schedule_type"`
		WorkFromHome bool   `json:"work_from_home"`
	} `json:"detected_extensions"`
	ApplyOptions []struct {
		Title string `json:"title"`
		Link  string `json:"link"`
	} `json:"apply_options"`
}) (RawJob, bool) {
	if strings.TrimSpace(record.Title) == "" || strings.TrimSpace(record.CompanyName) == "" {
		return RawJob{}, false
	}

	applyURL := preferredGoogleJobsApplyURL(record.ApplyOptions)
	sourceURL := record.ShareLink
	if applyURL == "" {
		applyURL = sourceURL
	}
	externalID := strings.TrimSpace(record.JobID)
	if externalID == "" {
		externalID = brightStableID(record.CompanyName, record.Title, record.Location, applyURL)
	}

	remoteType := "onsite"
	locationLower := strings.ToLower(record.Location)
	if record.DetectedExtensions.WorkFromHome ||
		strings.Contains(locationLower, "remote") ||
		strings.EqualFold(strings.TrimSpace(record.Location), "Anywhere") {
		remoteType = "remote"
	} else if strings.Contains(locationLower, "hybrid") {
		remoteType = "hybrid"
	}

	return RawJob{
		ExternalID:     externalID,
		Title:          strings.TrimSpace(record.Title),
		CompanyName:    strings.TrimSpace(record.CompanyName),
		Description:    stripTags(record.Description),
		LocationText:   strings.TrimSpace(record.Location),
		Country:        "United States",
		RemoteType:     remoteType,
		EmploymentType: record.DetectedExtensions.ScheduleType,
		ApplyURL:       applyURL,
		SourceURL:      sourceURL,
		PostedAt:       parseRelativeGoogleJobsTime(record.DetectedExtensions.PostedAt, s.now()),
	}, true
}

func preferredGoogleJobsApplyURL(options []struct {
	Title string `json:"title"`
	Link  string `json:"link"`
}) string {
	if len(options) == 0 {
		return ""
	}
	// Prefer employer/ATS links over another aggregator when Google provides
	// multiple application destinations.
	preferred := []string{"workday", "greenhouse", "lever", "ashby", "smartrecruiters", "workable", "careers"}
	for _, token := range preferred {
		for _, option := range options {
			if strings.Contains(strings.ToLower(option.Title+" "+option.Link), token) {
				return strings.TrimSpace(option.Link)
			}
		}
	}
	return strings.TrimSpace(options[0].Link)
}

func parseRelativeGoogleJobsTime(value string, now time.Time) *time.Time {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return nil
	}
	if value == "today" || value == "just posted" {
		result := now.UTC()
		return &result
	}

	fields := strings.Fields(strings.TrimSuffix(value, "+"))
	if len(fields) < 2 {
		return nil
	}
	number, err := strconv.Atoi(strings.TrimSuffix(fields[0], "+"))
	if err != nil {
		return nil
	}

	var age time.Duration
	switch {
	case strings.HasPrefix(fields[1], "minute"):
		age = time.Duration(number) * time.Minute
	case strings.HasPrefix(fields[1], "hour"):
		age = time.Duration(number) * time.Hour
	case strings.HasPrefix(fields[1], "day"):
		age = time.Duration(number) * 24 * time.Hour
	default:
		return nil
	}
	result := now.UTC().Add(-age)
	return &result
}
