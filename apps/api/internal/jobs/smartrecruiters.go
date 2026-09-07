package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SmartRecruitersSource struct {
	CompanyIdentifier string
	BaseURL           string
	http              *http.Client
}

func NewSmartRecruitersSource(companyIdentifier string) *SmartRecruitersSource {
	return &SmartRecruitersSource{
		CompanyIdentifier: companyIdentifier,
		BaseURL:           "https://api.smartrecruiters.com/v1/companies",
		http:              &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *SmartRecruitersSource) Name() string { return "SMARTRECRUITERS:" + s.CompanyIdentifier }

type smartRecruitersResponse struct {
	Content []struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		RefNumber    string `json:"refNumber"`
		ReleasedDate string `json:"releasedDate"`
		Location     struct {
			City    string `json:"city"`
			Region  string `json:"region"`
			Country string `json:"country"`
		} `json:"location"`
		TypeOfEmployment struct {
			Label string `json:"label"`
		} `json:"typeOfEmployment"`
	} `json:"content"`
}

func (s *SmartRecruitersSource) Fetch(ctx context.Context, _ *Cursor) ([]RawJob, *Cursor, error) {
	url := fmt.Sprintf("%s/%s/postings", s.BaseURL, s.CompanyIdentifier)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch SmartRecruiters company %s: %w", s.CompanyIdentifier, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("SmartRecruiters company %s returned %s", s.CompanyIdentifier, resp.Status)
	}
	var parsed smartRecruitersResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, nil, fmt.Errorf("decode SmartRecruiters response: %w", err)
	}
	jobs := make([]RawJob, 0, len(parsed.Content))
	for _, posting := range parsed.Content {
		var postedAt *time.Time
		if parsedTime, err := time.Parse(time.RFC3339, posting.ReleasedDate); err == nil {
			postedAt = &parsedTime
		}
		location := posting.Location.City
		if posting.Location.Region != "" {
			location = firstNonEmpty(location+", "+posting.Location.Region, posting.Location.Region)
		}
		if posting.Location.Country != "" {
			location = firstNonEmpty(location+", "+posting.Location.Country, posting.Location.Country)
		}
		jobs = append(jobs, RawJob{ExternalID: firstNonEmpty(posting.ID, posting.RefNumber), Title: posting.Name, CompanyName: s.CompanyIdentifier, LocationText: location, Country: posting.Location.Country, State: posting.Location.Region, City: posting.Location.City, EmploymentType: posting.TypeOfEmployment.Label, SourceURL: fmt.Sprintf("%s/%s/postings/%s", s.BaseURL, s.CompanyIdentifier, posting.ID), PostedAt: postedAt})
	}
	return jobs, nil, nil
}
