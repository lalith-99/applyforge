package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	brightDataModeMarketplaceFilter            = "marketplace_filter"
	brightDataModeLinkedInKeyword              = "linkedin_keyword"
	defaultBrightDataLinkedInKeywordDatasetID  = "gd_lpfll7v5hcqtkxl6l"
	defaultBrightDataKeywordLocation           = "United States"
	defaultBrightDataKeywordTimeRange          = "Past 24 hours"
	defaultBrightDataKeywordEmploymentType     = "Full-time"
)

// brightDataKeywordShards deliberately keeps the single-user core shard broad
// but balanced. The provider limit is applied per input, so one generic title
// can no longer consume the entire daily record budget.
var brightDataKeywordShards = map[string][]string{
	"us-single-user-core-24h": {
		"Software Engineer",
		"Java Developer",
		"Java Full Stack Developer",
		"Spring Boot Developer",
		"Backend Engineer",
		"Golang Developer",
		"Full Stack Developer",
		"Frontend Developer",
		"Platform Engineer",
		"DevOps Engineer",
		"Site Reliability Engineer",
		"Cloud Engineer",
	},
	"us-software-general-24h": {
		"Software Engineer", "Software Developer", "Software Development Engineer",
		"Application Engineer", "Application Developer",
	},
	"us-java-24h": {
		"Java Developer", "Java Engineer", "Java Full Stack Developer",
		"Spring Boot Developer", "Java Microservices Developer", "J2EE Developer",
	},
	"us-go-24h": {
		"Golang Developer", "Golang Engineer", "Go Software Engineer", "Go Backend Developer",
	},
	"us-fullstack-web-24h": {
		"Full Stack Developer", "Full Stack Engineer", "Frontend Developer",
		"React Developer", "Angular Developer", "TypeScript Developer",
	},
	"us-backend-platform-24h": {
		"Backend Engineer", "Backend Developer", "Platform Engineer",
		"Distributed Systems Engineer", "API Engineer",
	},
	"us-devops-cloud-24h": {
		"DevOps Engineer", "DevSecOps Engineer", "Site Reliability Engineer",
		"Cloud Engineer", "Kubernetes Engineer",
	},
	"us-data-ai-24h": {
		"Data Engineer", "Machine Learning Engineer", "AI Engineer", "MLOps Engineer", "Security Engineer",
	},
	"us-language-developers-24h": {
		"Python Developer", ".NET Developer", "C# Developer", "Scala Developer",
		"Kotlin Developer", "Integration Developer",
	},
	"us-enterprise-apps-24h": {
		"Application Developer", "Integration Engineer", "Middleware Developer",
		"Microservices Developer", "Enterprise Software Engineer",
	},
	"us-consulting-engineering-24h": {
		"Software Engineering Consultant", "Java Consultant", "Cloud Engineering Consultant", "DevOps Consultant",
	},
}

type brightDataKeywordInput struct {
	Location         string   `json:"location"`
	Keyword          string   `json:"keyword"`
	Country          string   `json:"country"`
	TimeRange        string   `json:"time_range"`
	JobType          string   `json:"job_type"`
	ExperienceLevel  string   `json:"experience_level"`
	Remote           string   `json:"remote"`
	Company          string   `json:"company"`
	SelectiveSearch  bool     `json:"selective_search"`
	JobsToNotInclude []string `json:"jobs_to_not_include,omitempty"`
	LocationRadius   string   `json:"location_radius"`
}

func normalizeBrightDataMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "marketplace", "marketplace_filter", "dataset_filter":
		return brightDataModeMarketplaceFilter
	case "linkedin", "linkedin_keyword", "keyword", "scraper_keyword":
		return brightDataModeLinkedInKeyword
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func brightDataKeywordInputsForShard(shard string, country string) []brightDataKeywordInput {
	keywords := brightDataKeywordShards[shard]
	if len(keywords) == 0 {
		keywords = []string{"Software Engineer", "Java Developer", "Backend Engineer", "Full Stack Developer"}
	}
	if strings.TrimSpace(country) == "" {
		country = "US"
	}

	inputs := make([]brightDataKeywordInput, 0, len(keywords))
	for _, keyword := range keywords {
		inputs = append(inputs, brightDataKeywordInput{
			Location:        defaultBrightDataKeywordLocation,
			Keyword:         keyword,
			Country:         country,
			TimeRange:       defaultBrightDataKeywordTimeRange,
			JobType:         defaultBrightDataKeywordEmploymentType,
			SelectiveSearch: true,
		})
	}
	return inputs
}

func brightDataLimitPerInput(recordsLimit, inputCount int) int {
	if inputCount <= 0 {
		return 1
	}
	if recordsLimit <= 0 {
		recordsLimit = defaultBrightDataLimit
	}
	perInput := recordsLimit / inputCount
	if perInput < 1 {
		return 1
	}
	return perInput
}

func (s *BrightDataSource) fetchLinkedInKeyword(ctx context.Context) ([]RawJob, *Cursor, error) {
	inputs := brightDataKeywordInputsForShard(s.Shard, s.cfg.CountryValue)
	if len(inputs) == 0 {
		return nil, nil, errors.New("Bright Data keyword discovery has no configured inputs")
	}

	limitPerInput := brightDataLimitPerInput(s.cfg.RecordsLimit, len(inputs))
	snapshotID, err := s.triggerLinkedInKeywordSnapshot(ctx, inputs, limitPerInput)
	if err != nil {
		return nil, nil, err
	}
	if err := s.waitForV3Snapshot(ctx, snapshotID); err != nil {
		return nil, nil, err
	}
	records, err := s.downloadV3Snapshot(ctx, snapshotID)
	if err != nil {
		return nil, nil, err
	}

	jobs := make([]RawJob, 0, len(records))
	for _, record := range records {
		raw, ok := brightDataRawJob(record)
		if !ok {
			continue
		}
		jobs = append(jobs, raw)
	}
	return jobs, nil, nil
}

func (s *BrightDataSource) triggerLinkedInKeywordSnapshot(
	ctx context.Context,
	inputs []brightDataKeywordInput,
	limitPerInput int,
) (string, error) {
	body, err := json.Marshal(inputs)
	if err != nil {
		return "", err
	}

	query := url.Values{}
	query.Set("dataset_id", s.cfg.DatasetID)
	query.Set("include_errors", "true")
	query.Set("type", "discover_new")
	query.Set("discover_by", "keyword")
	query.Set("limit_per_input", fmt.Sprintf("%d", limitPerInput))

	endpoint := s.cfg.BaseURL + "/datasets/v3/trigger?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	s.authorize(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("trigger Bright Data LinkedIn keyword snapshot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("trigger Bright Data LinkedIn keyword snapshot returned %s", resp.Status)
	}

	var out struct {
		SnapshotID string `json:"snapshot_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode Bright Data LinkedIn keyword trigger: %w", err)
	}
	if strings.TrimSpace(out.SnapshotID) == "" {
		return "", errors.New("Bright Data LinkedIn keyword trigger returned no snapshot_id")
	}
	return out.SnapshotID, nil
}

func (s *BrightDataSource) waitForV3Snapshot(ctx context.Context, snapshotID string) error {
	timeout := time.NewTimer(s.cfg.SnapshotTimeout)
	defer timeout.Stop()
	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()

	for {
		status, providerErr, err := s.v3SnapshotProgress(ctx, snapshotID)
		if err != nil {
			return err
		}
		switch status {
		case "ready", "completed", "success":
			return nil
		case "failed", "error":
			if providerErr == "" {
				providerErr = "snapshot failed"
			}
			return fmt.Errorf("Bright Data LinkedIn keyword snapshot %s failed: %s", snapshotID, providerErr)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return fmt.Errorf("Bright Data LinkedIn keyword snapshot %s did not become ready within %s", snapshotID, s.cfg.SnapshotTimeout)
		case <-ticker.C:
		}
	}
}

func (s *BrightDataSource) v3SnapshotProgress(ctx context.Context, snapshotID string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.BaseURL+"/datasets/v3/progress/"+snapshotID, nil)
	if err != nil {
		return "", "", err
	}
	s.authorize(req)

	resp, err := s.http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("get Bright Data LinkedIn keyword snapshot progress: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return "", "", fmt.Errorf("get Bright Data LinkedIn keyword snapshot progress returned %s", resp.Status)
	}

	var out struct {
		Status       string `json:"status"`
		Error        string `json:"error"`
		ErrorMessage string `json:"error_message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("decode Bright Data LinkedIn keyword snapshot progress: %w", err)
	}
	providerErr := strings.TrimSpace(out.ErrorMessage)
	if providerErr == "" {
		providerErr = strings.TrimSpace(out.Error)
	}
	return strings.ToLower(strings.TrimSpace(out.Status)), providerErr, nil
}

func (s *BrightDataSource) downloadV3Snapshot(ctx context.Context, snapshotID string) ([]map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.BaseURL+"/datasets/v3/snapshot/"+snapshotID+"?format=json", nil)
	if err != nil {
		return nil, err
	}
	s.authorize(req)

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download Bright Data LinkedIn keyword snapshot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download Bright Data LinkedIn keyword snapshot returned %s", resp.Status)
	}

	var records []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		return nil, fmt.Errorf("decode Bright Data LinkedIn keyword snapshot: %w", err)
	}
	return records, nil
}
