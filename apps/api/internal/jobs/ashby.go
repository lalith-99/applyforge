package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AshbySource fetches postings from a public Ashby job board.
// See https://developers.ashbyhq.com/docs/public-job-posting-api (unauthenticated, public data).
type AshbySource struct {
	BoardToken string
	BaseURL    string // overridable for tests; defaults to the real Ashby API
	http       *http.Client
}

// NewAshbySource builds a connector for the given board token (company slug).
func NewAshbySource(boardToken string) *AshbySource {
	return &AshbySource{
		BoardToken: boardToken,
		BaseURL:    "https://api.ashbyhq.com/posting-api/job-board",
		http:       &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *AshbySource) Name() string { return "ASHBY:" + s.BoardToken }

type ashbyResponse struct {
	Jobs []struct {
		ID              string `json:"id"`
		Title           string `json:"title"`
		Location        string `json:"location"`
		EmploymentType  string `json:"employmentType"`
		WorkplaceType   string `json:"workplaceType"`
		PublishedAt     string `json:"publishedAt"`
		JobURL          string `json:"jobUrl"`
		ApplyURL        string `json:"applyUrl"`
		DescriptionHTML string `json:"descriptionHtml"`
	} `json:"jobs"`
}

// Fetch retrieves all current postings for the board. Ashby's public job
// board endpoint is not paginated, so cursor is always nil on return.
func (s *AshbySource) Fetch(ctx context.Context, _ *Cursor) ([]RawJob, *Cursor, error) {
	url := fmt.Sprintf("%s/%s", s.BaseURL, s.BoardToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch ashby board %s: %w", s.BoardToken, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("ashby board %s returned %s", s.BoardToken, resp.Status)
	}

	var parsed ashbyResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, nil, fmt.Errorf("decode ashby response: %w", err)
	}

	jobs := make([]RawJob, 0, len(parsed.Jobs))
	for _, j := range parsed.Jobs {
		var postedAt *time.Time
		if t, err := time.Parse(time.RFC3339, j.PublishedAt); err == nil {
			postedAt = &t
		}
		remoteType := "onsite"
		if j.WorkplaceType == "Remote" {
			remoteType = "remote"
		} else if j.WorkplaceType == "Hybrid" {
			remoteType = "hybrid"
		}
		jobs = append(jobs, RawJob{
			ExternalID:     j.ID,
			Title:          j.Title,
			CompanyName:    s.BoardToken,
			Description:    stripTags(j.DescriptionHTML),
			LocationText:   j.Location,
			RemoteType:     remoteType,
			EmploymentType: j.EmploymentType,
			ApplyURL:       firstNonEmpty(j.ApplyURL, j.JobURL),
			SourceURL:      j.JobURL,
			PostedAt:       postedAt,
		})
	}
	return jobs, nil, nil
}
