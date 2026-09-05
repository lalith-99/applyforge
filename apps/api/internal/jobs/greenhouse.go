package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GreenhouseSource fetches postings from a public Greenhouse job board.
// See https://developers.greenhouse.io/job-board.html (unauthenticated, public data).
type GreenhouseSource struct {
	BoardToken string
	BaseURL    string // overridable for tests; defaults to the real Greenhouse API
	http       *http.Client
}

// NewGreenhouseSource builds a connector for the given board token (company slug).
func NewGreenhouseSource(boardToken string) *GreenhouseSource {
	return &GreenhouseSource{
		BoardToken: boardToken,
		BaseURL:    "https://boards-api.greenhouse.io/v1/boards",
		http:       &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *GreenhouseSource) Name() string { return "GREENHOUSE:" + s.BoardToken }

type greenhouseResponse struct {
	Jobs []struct {
		ID        int64  `json:"id"`
		Title     string `json:"title"`
		UpdatedAt string `json:"updated_at"`
		Location  struct {
			Name string `json:"name"`
		} `json:"location"`
		AbsoluteURL string `json:"absolute_url"`
		Content     string `json:"content"`
	} `json:"jobs"`
}

// Fetch retrieves all current postings for the board. Greenhouse's public
// jobs endpoint is not paginated, so cursor is always nil on return.
func (s *GreenhouseSource) Fetch(ctx context.Context, _ *Cursor) ([]RawJob, *Cursor, error) {
	url := fmt.Sprintf("%s/%s/jobs?content=true", s.BaseURL, s.BoardToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch greenhouse board %s: %w", s.BoardToken, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("greenhouse board %s returned %s", s.BoardToken, resp.Status)
	}

	var parsed greenhouseResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, nil, fmt.Errorf("decode greenhouse response: %w", err)
	}

	jobs := make([]RawJob, 0, len(parsed.Jobs))
	for _, j := range parsed.Jobs {
		var postedAt *time.Time
		if t, err := time.Parse(time.RFC3339, j.UpdatedAt); err == nil {
			postedAt = &t
		}
		jobs = append(jobs, RawJob{
			ExternalID:   fmt.Sprintf("%d", j.ID),
			Title:        j.Title,
			CompanyName:  s.BoardToken,
			Description:  stripTags(j.Content),
			LocationText: j.Location.Name,
			ApplyURL:     j.AbsoluteURL,
			SourceURL:    j.AbsoluteURL,
			PostedAt:     postedAt,
		})
	}
	return jobs, nil, nil
}
