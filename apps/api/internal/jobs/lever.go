package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// LeverSource fetches postings from a public Lever job board.
// See https://github.com/lever/postings-api (unauthenticated, public data).
type LeverSource struct {
	BoardToken string
	BaseURL    string // overridable for tests; defaults to the real Lever API
	http       *http.Client
}

// NewLeverSource builds a connector for the given board token (company slug).
func NewLeverSource(boardToken string) *LeverSource {
	return &LeverSource{
		BoardToken: boardToken,
		BaseURL:    "https://api.lever.co/v0/postings",
		http:       &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *LeverSource) Name() string { return "LEVER:" + s.BoardToken }

type leverPosting struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	CreatedAt  int64  `json:"createdAt"`
	HostedURL  string `json:"hostedUrl"`
	ApplyURL   string `json:"applyUrl"`
	Categories struct {
		Location   string `json:"location"`
		Commitment string `json:"commitment"`
	} `json:"categories"`
	DescriptionPlain string `json:"descriptionPlain"`
	Description      string `json:"description"`
	Lists            []struct {
		Text    string `json:"text"`
		Content string `json:"content"`
	} `json:"lists"`
	AdditionalPlain string `json:"additionalPlain"`
	Additional      string `json:"additional"`
	Country         string `json:"country"`
	WorkplaceType   string `json:"workplaceType"`
}

// Fetch retrieves all current postings for the board. Lever's public
// postings endpoint is not paginated, so cursor is always nil on return.
func (s *LeverSource) Fetch(ctx context.Context, _ *Cursor) ([]RawJob, *Cursor, error) {
	url := fmt.Sprintf("%s/%s?mode=json", s.BaseURL, s.BoardToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch lever board %s: %w", s.BoardToken, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("lever board %s returned %s", s.BoardToken, resp.Status)
	}

	var postings []leverPosting
	if err := json.NewDecoder(resp.Body).Decode(&postings); err != nil {
		return nil, nil, fmt.Errorf("decode lever response: %w", err)
	}

	jobs := make([]RawJob, 0, len(postings))
	for _, p := range postings {
		var postedAt *time.Time
		if p.CreatedAt > 0 {
			t := time.UnixMilli(p.CreatedAt)
			postedAt = &t
		}
		jobs = append(jobs, RawJob{
			ExternalID:     p.ID,
			Title:          p.Text,
			CompanyName:    s.BoardToken,
			Description:    leverDescription(p),
			LocationText:   p.Categories.Location,
			Country:        p.Country,
			RemoteType:     leverRemoteType(p.WorkplaceType),
			EmploymentType: p.Categories.Commitment,
			ApplyURL:       firstNonEmpty(p.ApplyURL, p.HostedURL),
			SourceURL:      p.HostedURL,
			PostedAt:       postedAt,
		})
	}
	return jobs, nil, nil
}

// Lever keeps qualifications/responsibilities and closing disclosures outside
// descriptionPlain. Preserve them before matching and sponsorship filtering.
func leverDescription(p leverPosting) string {
	parts := make([]string, 0, len(p.Lists)+2)
	if body := strings.TrimSpace(firstNonEmpty(p.DescriptionPlain, stripTags(p.Description))); body != "" {
		parts = append(parts, body)
	}
	for _, list := range p.Lists {
		content := strings.TrimSpace(stripTags(list.Content))
		if content == "" {
			continue
		}
		if title := strings.TrimSpace(stripTags(list.Text)); title != "" {
			content = title + "\n" + content
		}
		parts = append(parts, content)
	}
	if closing := strings.TrimSpace(firstNonEmpty(p.AdditionalPlain, stripTags(p.Additional))); closing != "" {
		parts = append(parts, closing)
	}
	return strings.Join(parts, "\n\n")
}

func leverRemoteType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "remote":
		return "remote"
	case "hybrid":
		return "hybrid"
	case "on-site", "onsite":
		return "onsite"
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
