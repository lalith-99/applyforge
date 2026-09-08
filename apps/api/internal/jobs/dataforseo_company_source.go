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

const defaultDataForSEOOrganicLiveURL = "https://api.dataforseo.com/v3/serp/google/organic/live/advanced"

type DataForSEOCompanySourceConfig struct {
	Login        string
	Password     string
	Endpoint     string
	LocationCode int
	LanguageCode string
	Depth        int
}

func DataForSEOCompanySourceConfigFromEnv() (DataForSEOCompanySourceConfig, error) {
	cfg := DataForSEOCompanySourceConfig{
		Login:        strings.TrimSpace(os.Getenv("DATAFORSEO_API_LOGIN")),
		Password:     strings.TrimSpace(os.Getenv("DATAFORSEO_API_PASSWORD")),
		Endpoint:     strings.TrimSpace(os.Getenv("DATAFORSEO_ORGANIC_LIVE_URL")),
		LocationCode: 2840,
		LanguageCode: "en",
		Depth:        10,
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultDataForSEOOrganicLiveURL
	}
	if raw := strings.TrimSpace(os.Getenv("DATAFORSEO_SOURCE_DISCOVERY_DEPTH")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 200 {
			return DataForSEOCompanySourceConfig{}, errors.New("DATAFORSEO_SOURCE_DISCOVERY_DEPTH must be between 1 and 200")
		}
		cfg.Depth = value
	}
	if cfg.Login == "" {
		return DataForSEOCompanySourceConfig{}, errors.New("DATAFORSEO_API_LOGIN is not configured")
	}
	if cfg.Password == "" {
		return DataForSEOCompanySourceConfig{}, errors.New("DATAFORSEO_API_PASSWORD is not configured")
	}
	return cfg, nil
}

type DataForSEOCompanySourceResolver struct {
	cfg  DataForSEOCompanySourceConfig
	http *http.Client
}

func NewDataForSEOCompanySourceResolver(cfg DataForSEOCompanySourceConfig) *DataForSEOCompanySourceResolver {
	return &DataForSEOCompanySourceResolver{
		cfg:  cfg,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (r *DataForSEOCompanySourceResolver) Provider() string  { return "DATAFORSEO" }
func (r *DataForSEOCompanySourceResolver) Operation() string { return "company_source_discovery" }

type dataForSEOOrganicResponse struct {
	StatusCode    int    `json:"status_code"`
	StatusMessage string `json:"status_message"`
	Tasks         []struct {
		ID            string `json:"id"`
		StatusCode    int    `json:"status_code"`
		StatusMessage string `json:"status_message"`
		Result        []struct {
			Items []struct {
				Type        string `json:"type"`
				Domain      string `json:"domain"`
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"items"`
		} `json:"result"`
	} `json:"tasks"`
}

func (r *DataForSEOCompanySourceResolver) Resolve(
	ctx context.Context,
	companyName string,
) (CompanySourceResolution, error) {
	companyName = strings.TrimSpace(companyName)
	if companyName == "" {
		return CompanySourceResolution{}, errors.New("company name is required")
	}

	payload := []map[string]any{{
		"keyword":       companyName + " careers jobs",
		"location_code": r.cfg.LocationCode,
		"language_code": r.cfg.LanguageCode,
		"depth":         r.cfg.Depth,
		"device":        "desktop",
		"os":            "windows",
	}}
	body, err := json.Marshal(payload)
	if err != nil {
		return CompanySourceResolution{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return CompanySourceResolution{}, err
	}
	req.SetBasicAuth(r.cfg.Login, r.cfg.Password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ApplyForge/1.0")

	resp, err := r.http.Do(req)
	if err != nil {
		return CompanySourceResolution{}, fmt.Errorf("query DataForSEO company source discovery: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return CompanySourceResolution{}, fmt.Errorf("DataForSEO company source discovery returned %s", resp.Status)
	}

	var decoded dataForSEOOrganicResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return CompanySourceResolution{}, fmt.Errorf("decode DataForSEO company source discovery: %w", err)
	}
	if decoded.StatusCode != 20000 {
		return CompanySourceResolution{}, fmt.Errorf(
			"DataForSEO request failed with status %d: %s",
			decoded.StatusCode,
			decoded.StatusMessage,
		)
	}
	if len(decoded.Tasks) == 0 {
		return CompanySourceResolution{}, errors.New("DataForSEO returned no task")
	}

	task := decoded.Tasks[0]
	resolution := CompanySourceResolution{ProviderRequestID: task.ID}
	if task.StatusCode != 20000 {
		return resolution, fmt.Errorf(
			"DataForSEO task failed with status %d: %s",
			task.StatusCode,
			task.StatusMessage,
		)
	}

	type organicResult struct {
		url         string
		title       string
		description string
	}
	organic := make([]organicResult, 0)
	urls := make([]string, 0)
	for _, result := range task.Result {
		for _, item := range result.Items {
			if item.Type != "organic" || strings.TrimSpace(item.URL) == "" {
				continue
			}
			candidateURL := strings.TrimSpace(item.URL)
			urls = append(urls, candidateURL)
			organic = append(organic, organicResult{
				url:         candidateURL,
				title:       item.Title,
				description: item.Description,
			})
		}
	}

	if direct := DetectCompanySources(urls...); len(direct) > 0 {
		resolution.Sources = direct
		return resolution, nil
	}

	for _, item := range organic {
		if !looksLikeCompanyCareerResult(item.url, item.title, item.description) {
			continue
		}
		parsed, err := url.Parse(item.url)
		if err != nil || parsed.Hostname() == "" {
			continue
		}
		resolution.Sources = []DiscoveredCompanySource{{
			SourceType:  "CUSTOM",
			BoardToken:  strings.ToLower(parsed.Hostname()),
			SourceURL:   item.url,
			Confidence:  0.6,
			Monitorable: false,
		}}
		break
	}

	return resolution, nil
}

var sourceDiscoveryAggregatorHosts = []string{
	"linkedin.com",
	"indeed.com",
	"glassdoor.com",
	"ziprecruiter.com",
	"dice.com",
	"monster.com",
	"careerbuilder.com",
	"jobright.ai",
	"simplify.jobs",
}

func looksLikeCompanyCareerResult(rawURL, title, description string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" {
		return false
	}

	host := strings.ToLower(parsed.Hostname())
	for _, blocked := range sourceDiscoveryAggregatorHosts {
		if host == blocked || strings.HasSuffix(host, "."+blocked) {
			return false
		}
	}

	text := strings.ToLower(parsed.Path + " " + title + " " + description)
	for _, token := range []string{"career", "careers", "jobs", "job openings", "open positions"} {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}
