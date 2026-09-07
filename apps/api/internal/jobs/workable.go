package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WorkableSource fetches postings from a public Workable careers widget.
// See https://workable.com (unauthenticated, public data) - the widget
// endpoint returns every published job for a subdomain in one response.
type WorkableSource struct {
	Subdomain string
	BaseURL   string // overridable for tests; defaults to the real Workable API
	http      *http.Client
}

// NewWorkableSource builds a connector for the given account subdomain.
func NewWorkableSource(subdomain string) *WorkableSource {
	return &WorkableSource{
		Subdomain: subdomain,
		BaseURL:   "https://www.workable.com/api/v1/widget/accounts",
		http:      &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *WorkableSource) Name() string { return "WORKABLE:" + s.Subdomain }

type workableResponse struct {
	Jobs []struct {
		Shortcode      string `json:"shortcode"`
		Title          string `json:"title"`
		Department     string `json:"department"`
		Url            string `json:"url"`
		ApplicationUrl string `json:"application_url"`
		PublishedOn    string `json:"published_on"`
		EmploymentType string `json:"employment_type"`
		Telecommuting  bool   `json:"telecommuting"`
		Description    string `json:"description"`
		Location       struct {
			City        string `json:"city"`
			Region      string `json:"region"`
			Country     string `json:"country"`
			CountryCode string `json:"country_code"`
		} `json:"location"`
	} `json:"jobs"`
}

// Fetch retrieves all current published postings for the account. The
// widget endpoint is not paginated, so cursor is always nil on return.
func (s *WorkableSource) Fetch(ctx context.Context, _ *Cursor) ([]RawJob, *Cursor, error) {
	url := fmt.Sprintf("%s/%s?details=true", s.BaseURL, s.Subdomain)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch workable account %s: %w", s.Subdomain, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("workable account %s returned %s", s.Subdomain, resp.Status)
	}

	var parsed workableResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, nil, fmt.Errorf("decode workable response: %w", err)
	}

	jobs := make([]RawJob, 0, len(parsed.Jobs))
	for _, j := range parsed.Jobs {
		var postedAt *time.Time
		if t, err := time.Parse("2006-01-02", j.PublishedOn); err == nil {
			postedAt = &t
		}
		remoteType := "onsite"
		if j.Telecommuting {
			remoteType = "remote"
		}
		countryName := j.Location.Country
		if countryName == "" {
			countryName = j.Location.CountryCode
		}
		location := firstNonEmpty(j.Location.City, "")
		if j.Location.Region != "" {
			location = firstNonEmpty(location+", "+j.Location.Region, j.Location.Region)
		}
		if countryName != "" {
			location = firstNonEmpty(location+", "+countryName, countryName)
		}
		jobs = append(jobs, RawJob{
			ExternalID:     j.Shortcode,
			Title:          j.Title,
			Description:    stripTags(j.Description),
			LocationText:   location,
			Country:        countryName,
			State:          j.Location.Region,
			City:           j.Location.City,
			RemoteType:     remoteType,
			EmploymentType: j.EmploymentType,
			ApplyURL:       firstNonEmpty(j.ApplicationUrl, j.Url),
			SourceURL:      j.Url,
			PostedAt:       postedAt,
		})
	}
	return jobs, nil, nil
}
