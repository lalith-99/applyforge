package jobs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBrightDataBaseURL = "https://api.brightdata.com"
	defaultBrightDataLimit   = 1500
)

var brightDataTitleShards = map[string][]string{
	"us-software-general-24h": {
		"Software Engineer", "Software Developer", "Software Development Engineer",
		"Application Engineer", "Application Developer",
	},
	"us-java-24h": {
		"Java Developer", "Java Engineer", "Java Backend Developer",
		"Java Full Stack Developer", "Spring Boot Developer", "Spring Boot Engineer",
		"J2EE Developer",
	},
	"us-go-24h": {
		"Golang Developer", "Golang Engineer", "Go Developer", "Go Engineer",
		"Backend Engineer Golang", "Backend Engineer Go",
	},
	"us-fullstack-web-24h": {
		"Full Stack Developer", "Full Stack Engineer", "Frontend Developer",
		"Frontend Engineer", "React Developer", "Angular Developer",
		"Node.js Developer", "TypeScript Developer",
	},
	"us-backend-platform-24h": {
		"Backend Engineer", "Backend Developer", "Platform Engineer",
		"Infrastructure Engineer", "Distributed Systems Engineer",
		"API Engineer", "Microservices Engineer",
	},
	"us-devops-cloud-24h": {
		"DevOps Engineer", "DevSecOps Engineer", "Site Reliability Engineer",
		"Cloud Engineer", "Kubernetes Engineer", "Build Engineer", "Release Engineer",
	},
	"us-data-ai-24h": {
		"Data Engineer", "Machine Learning Engineer", "AI Engineer",
		"MLOps Engineer", "Security Engineer",
	},
	"us-language-developers-24h": {
		"Python Developer", "Python Engineer", ".NET Developer", ".NET Engineer",
		"C# Developer", "C# Engineer", "Scala Developer", "Kotlin Developer",
		"Salesforce Developer", "Integration Developer",
	},
}

// Keep the legacy token useful for existing installations. New deployments
// should use the explicit shards above.
var brightDataLegacySoftwareTitles = []string{
	"Software Engineer", "Backend Engineer", "Frontend Engineer", "Full Stack Engineer",
	"Platform Engineer", "Infrastructure Engineer", "Site Reliability Engineer",
	"DevOps Engineer", "Cloud Engineer", "Data Engineer", "Machine Learning Engineer",
	"AI Engineer", "Security Engineer", "Mobile Engineer", "Embedded Software Engineer",
	"Systems Software Engineer",
}
type BrightDataConfig struct {
	APIKey          string
	DatasetID       string
	BaseURL         string
	RecordsLimit    int
	SnapshotTimeout time.Duration
	PollInterval    time.Duration
	TitleField      string
	PostedDateField string
	CountryField    string
	CountryValue    string
}

func BrightDataConfigFromEnv() (BrightDataConfig, error) {
	cfg := BrightDataConfig{
		APIKey:          strings.TrimSpace(os.Getenv("BRIGHTDATA_API_KEY")),
		DatasetID:       strings.TrimSpace(os.Getenv("BRIGHTDATA_JOBS_DATASET_ID")),
		BaseURL:         strings.TrimRight(strings.TrimSpace(os.Getenv("BRIGHTDATA_API_BASE_URL")), "/"),
		RecordsLimit:    defaultBrightDataLimit,
		SnapshotTimeout: 5 * time.Minute,
		PollInterval:    5 * time.Second,
		TitleField:      envOr("BRIGHTDATA_JOBS_TITLE_FIELD", "job_title"),
		PostedDateField: envOr("BRIGHTDATA_JOBS_POSTED_DATE_FIELD", "posted_date"),
		CountryField:    envOr("BRIGHTDATA_JOBS_COUNTRY_FIELD", "country_code"),
		CountryValue:    envOr("BRIGHTDATA_JOBS_COUNTRY_VALUE", "US"),
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBrightDataBaseURL
	}
	if raw := os.Getenv("BRIGHTDATA_JOBS_RECORDS_LIMIT"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			cfg.RecordsLimit = n
		}
	}
	if cfg.APIKey == "" {
		return BrightDataConfig{}, errors.New("BRIGHTDATA_API_KEY is not configured")
	}
	if cfg.DatasetID == "" {
		return BrightDataConfig{}, errors.New("BRIGHTDATA_JOBS_DATASET_ID is not configured")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// BrightDataSource uses Bright Data's asynchronous Marketplace Dataset filter
// API. Each configured source row represents one shared role/technology shard;
// ApplyForge still owns canonical US validation, role classification, dedupe,
// sponsorship filtering, and strict posted_at freshness after download.
type BrightDataSource struct {
	Shard string
	cfg   BrightDataConfig
	http  *http.Client
	now   func() time.Time
}

func NewBrightDataSource(shard string, cfg BrightDataConfig) *BrightDataSource {
	return &BrightDataSource{
		Shard: shard,
		cfg:   cfg,
		http:  &http.Client{Timeout: 30 * time.Second},
		now:   time.Now,
	}
}

func (s *BrightDataSource) Name() string { return "BRIGHTDATA:" + s.Shard }

func (s *BrightDataSource) Fetch(ctx context.Context, _ *Cursor) ([]RawJob, *Cursor, error) {
	snapshotID, err := s.triggerSnapshot(ctx)
	if err != nil {
		return nil, nil, err
	}
	if err := s.waitForSnapshot(ctx, snapshotID); err != nil {
		return nil, nil, err
	}
	records, err := s.downloadSnapshot(ctx, snapshotID)
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

func (s *BrightDataSource) triggerSnapshot(ctx context.Context) (string, error) {
	filters := []any{
		map[string]any{
			"operator": "or",
			"filters":  brightDataTitleFilters(s.cfg.TitleField, s.Shard),
		},
		map[string]any{
			"name":     s.cfg.PostedDateField,
			"operator": ">=",
			"value":    s.now().UTC().Add(-24 * time.Hour).Format("2006-01-02"),
		},
	}
	if s.cfg.CountryField != "" {
		filters = append(filters, map[string]any{
			"name":     s.cfg.CountryField,
			"operator": "=",
			"value":    s.cfg.CountryValue,
		})
	}

	payload := map[string]any{
		"dataset_id":    s.cfg.DatasetID,
		"records_limit": s.cfg.RecordsLimit,
		"filter": map[string]any{
			"operator": "and",
			"filters":  filters,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.BaseURL+"/datasets/filter", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	s.authorize(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("trigger Bright Data jobs snapshot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("trigger Bright Data jobs snapshot returned %s", resp.Status)
	}

	var out struct {
		SnapshotID string `json:"snapshot_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode Bright Data snapshot trigger: %w", err)
	}
	if out.SnapshotID == "" {
		return "", errors.New("Bright Data snapshot trigger returned no snapshot_id")
	}
	return out.SnapshotID, nil
}

func brightDataTitleFilters(field, shard string) []any {
	titles := brightDataTitleShards[shard]
	if len(titles) == 0 {
		titles = brightDataLegacySoftwareTitles
	}
	out := make([]any, 0, len(titles))
	for _, title := range titles {
		out = append(out, map[string]any{
			"name":     field,
			"operator": "includes",
			"value":    title,
		})
	}
	return out
}

func (s *BrightDataSource) waitForSnapshot(ctx context.Context, snapshotID string) error {
	timeout := time.NewTimer(s.cfg.SnapshotTimeout)
	defer timeout.Stop()
	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()

	for {
		status, providerErr, err := s.snapshotStatus(ctx, snapshotID)
		if err != nil {
			return err
		}
		switch status {
		case "ready":
			return nil
		case "failed":
			if providerErr == "" {
				providerErr = "snapshot failed"
			}
			return fmt.Errorf("Bright Data snapshot %s failed: %s", snapshotID, providerErr)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return fmt.Errorf("Bright Data snapshot %s did not become ready within %s", snapshotID, s.cfg.SnapshotTimeout)
		case <-ticker.C:
		}
	}
}

func (s *BrightDataSource) snapshotStatus(ctx context.Context, snapshotID string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.BaseURL+"/datasets/snapshots/"+snapshotID, nil)
	if err != nil {
		return "", "", err
	}
	s.authorize(req)

	resp, err := s.http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("get Bright Data snapshot status: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("get Bright Data snapshot status returned %s", resp.Status)
	}

	var out struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("decode Bright Data snapshot status: %w", err)
	}
	return strings.ToLower(out.Status), out.Error, nil
}

func (s *BrightDataSource) downloadSnapshot(ctx context.Context, snapshotID string) ([]map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.BaseURL+"/datasets/snapshots/"+snapshotID+"/download?format=json", nil)
	if err != nil {
		return nil, err
	}
	s.authorize(req)

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download Bright Data snapshot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download Bright Data snapshot returned %s", resp.Status)
	}

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode Bright Data snapshot: %w", err)
	}

	var records []map[string]any
	if err := json.Unmarshal(raw, &records); err == nil {
		return records, nil
	}

	var wrapped struct {
		Data    []map[string]any `json:"data"`
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, fmt.Errorf("Bright Data snapshot has unsupported JSON shape: %w", err)
	}
	if len(wrapped.Data) > 0 {
		return wrapped.Data, nil
	}
	return wrapped.Records, nil
}

func (s *BrightDataSource) authorize(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
}

func brightDataRawJob(record map[string]any) (RawJob, bool) {
	title := brightString(record, "job_title", "title", "position", "name")
	company := brightString(record, "company_name", "company", "employer_name", "organization")
	if title == "" || company == "" {
		return RawJob{}, false
	}

	description := brightString(record, "job_description", "description", "job_summary", "description_text")
	location := brightString(record, "location", "job_location", "location_name")
	country := brightString(record, "country", "country_name", "country_code")
	state := brightString(record, "state", "region", "province")
	city := brightString(record, "city")
	applyURL := brightString(record, "apply_url", "application_url", "job_apply_url")
	sourceURL := brightString(record, "job_url", "url", "source_url", "link")
	if applyURL == "" {
		applyURL = sourceURL
	}
	postedAt := brightTime(record, "posted_date", "date_posted", "posted_at", "created_at", "publication_date")
	externalID := brightString(record, "job_id", "id", "job_posting_id", "external_id")
	if externalID == "" {
		externalID = brightStableID(company, title, location, applyURL)
	}

	remoteType := "onsite"
	if brightBool(record, "remote", "is_remote") || strings.Contains(strings.ToLower(location), "remote") {
		remoteType = "remote"
	} else if strings.Contains(strings.ToLower(location), "hybrid") {
		remoteType = "hybrid"
	}

	return RawJob{
		ExternalID:     externalID,
		Title:          title,
		CompanyName:    company,
		Description:    description,
		LocationText:   location,
		Country:        country,
		State:          state,
		City:           city,
		RemoteType:     remoteType,
		EmploymentType: brightString(record, "employment_type", "job_type", "employment"),
		ApplyURL:       applyURL,
		SourceURL:      sourceURL,
		PostedAt:       postedAt,
	}, true
}

func brightString(record map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := record[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case map[string]any:
			for _, nestedKey := range []string{"name", "value", "label", "title"} {
				if nested, ok := typed[nestedKey].(string); ok && strings.TrimSpace(nested) != "" {
					return strings.TrimSpace(nested)
				}
			}
		}
	}
	return ""
}

func brightBool(record map[string]any, keys ...string) bool {
	for _, key := range keys {
		value, ok := record[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
			if err == nil {
				return parsed
			}
		}
	}
	return false
}

func brightTime(record map[string]any, keys ...string) *time.Time {
	value := brightString(record, keys...)
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
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func brightStableID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return "bd-" + hex.EncodeToString(sum[:16])
}
