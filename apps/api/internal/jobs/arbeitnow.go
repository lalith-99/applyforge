package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ArbeitnowSource fetches postings from the public Arbeitnow job board API
// (https://www.arbeitnow.com/api/job-board-api). Unlike Greenhouse/Lever/
// Ashby, this is a multi-company aggregator: a single feed returns postings
// from many different companies, each carrying its own company_name field
// (see RawJob.CompanyName, resolved dynamically by Ingest instead of using
// a fixed per-source company).
type ArbeitnowSource struct {
	BaseURL  string // overridable for tests; defaults to the real Arbeitnow API
	MaxPages int    // safety cap so a single poll can't fetch the entire feed
	http     *http.Client
}

// NewArbeitnowSource builds a connector for the public Arbeitnow feed.
func NewArbeitnowSource() *ArbeitnowSource {
	return &ArbeitnowSource{
		BaseURL:  "https://www.arbeitnow.com/api/job-board-api",
		MaxPages: 5, // ~250 jobs/page; the feed refreshes hourly per its own docs
		http:     &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *ArbeitnowSource) Name() string { return "ARBEITNOW" }

type arbeitnowResponse struct {
	Data []struct {
		Slug        string   `json:"slug"`
		CompanyName string   `json:"company_name"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Remote      bool     `json:"remote"`
		URL         string   `json:"url"`
		Tags        []string `json:"tags"`
		JobTypes    []string `json:"job_types"`
		Location    string   `json:"location"`
		CreatedAt   int64    `json:"created_at"`
	} `json:"data"`
	Links struct {
		Next *string `json:"next"`
	} `json:"links"`
}

// Fetch retrieves postings across up to MaxPages of the feed. The API is not
// keyed on a company board token, so cursor is unused and always nil on
// return (each poll re-fetches from page 1).
func (s *ArbeitnowSource) Fetch(ctx context.Context, _ *Cursor) ([]RawJob, *Cursor, error) {
	var jobs []RawJob
	url := s.BaseURL

	for page := 0; page < s.MaxPages && url != ""; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, nil, err
		}

		resp, err := s.http.Do(req)
		if err != nil {
			return nil, nil, fmt.Errorf("fetch arbeitnow feed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, nil, fmt.Errorf("arbeitnow feed returned %s", resp.Status)
		}

		var parsed arbeitnowResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&parsed)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, nil, fmt.Errorf("decode arbeitnow response: %w", decodeErr)
		}

		for _, j := range parsed.Data {
			remoteType := "onsite"
			if j.Remote {
				remoteType = "remote"
			}
			var postedAt *time.Time
			if j.CreatedAt > 0 {
				t := time.Unix(j.CreatedAt, 0).UTC()
				postedAt = &t
			}
			jobs = append(jobs, RawJob{
				ExternalID:   j.Slug,
				Title:        j.Title,
				CompanyName:  j.CompanyName,
				Description:  stripTags(j.Description),
				LocationText: j.Location,
				RemoteType:   remoteType,
				ApplyURL:     j.URL,
				SourceURL:    j.URL,
				PostedAt:     postedAt,
			})
		}

		if parsed.Links.Next == nil {
			break
		}
		url = *parsed.Links.Next
	}

	return jobs, nil, nil
}
