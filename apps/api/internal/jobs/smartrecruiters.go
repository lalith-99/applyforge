package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const smartRecruitersPageSize = 100

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

type smartRecruitersPosting struct {
	ID           string `json:"id"`
	UUID         string `json:"uuid"`
	Name         string `json:"name"`
	RefNumber    string `json:"refNumber"`
	ReleasedDate string `json:"releasedDate"`
	PostingURL   string `json:"postingUrl"`
	ApplyURL     string `json:"applyUrl"`
	Location     struct {
		City    string `json:"city"`
		Region  string `json:"region"`
		Country string `json:"country"`
		Remote  bool   `json:"remote"`
	} `json:"location"`
	TypeOfEmployment struct {
		Label string `json:"label"`
	} `json:"typeOfEmployment"`
}

type smartRecruitersResponse struct {
	Limit      int                      `json:"limit"`
	Offset     int                      `json:"offset"`
	TotalFound int                      `json:"totalFound"`
	Content    []smartRecruitersPosting `json:"content"`
}

type smartRecruitersPostingDetails struct {
	smartRecruitersPosting
	JobAd struct {
		Sections struct {
			CompanyDescription struct {
				Title string `json:"title"`
				Text  string `json:"text"`
			} `json:"companyDescription"`
			JobDescription struct {
				Title string `json:"title"`
				Text  string `json:"text"`
			} `json:"jobDescription"`
			Qualifications struct {
				Title string `json:"title"`
				Text  string `json:"text"`
			} `json:"qualifications"`
			AdditionalInformation struct {
				Title string `json:"title"`
				Text  string `json:"text"`
			} `json:"additionalInformation"`
		} `json:"sections"`
	} `json:"jobAd"`
}

func (s *SmartRecruitersSource) Fetch(ctx context.Context, _ *Cursor) ([]RawJob, *Cursor, error) {
	var all []smartRecruitersPosting
	for offset := 0; ; offset += smartRecruitersPageSize {
		page, err := s.fetchPage(ctx, offset)
		if err != nil {
			return nil, nil, err
		}
		all = append(all, page.Content...)

		if len(page.Content) == 0 || offset+len(page.Content) >= page.TotalFound {
			break
		}
	}

	jobs := make([]RawJob, 0, len(all))
	for _, posting := range all {
		raw := smartRecruitersRaw(posting)

		// The list endpoint is enough for closure detection and cheap
		// metadata filtering. Only pay the additional detail request for
		// U.S. software jobs, which are the rows ApplyForge may enrich,
		// embed, recommend, or show to the user.
		if normalizeLocation(raw).CountryCode == "US" && classifyTitle(raw.Title).Classification == "IC_SOFTWARE" {
			detail, err := s.fetchDetails(ctx, firstNonEmpty(posting.ID, posting.UUID))
			if err != nil {
				return nil, nil, err
			}
			raw = smartRecruitersRaw(detail.smartRecruitersPosting)
			raw.Description = smartRecruitersDescription(detail)
		}
		jobs = append(jobs, raw)
	}
	return jobs, nil, nil
}

func (s *SmartRecruitersSource) fetchPage(ctx context.Context, offset int) (smartRecruitersResponse, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/postings", s.BaseURL, s.CompanyIdentifier))
	if err != nil {
		return smartRecruitersResponse{}, err
	}
	q := u.Query()
	q.Set("offset", strconv.Itoa(offset))
	q.Set("limit", strconv.Itoa(smartRecruitersPageSize))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return smartRecruitersResponse{}, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return smartRecruitersResponse{}, fmt.Errorf("fetch SmartRecruiters company %s: %w", s.CompanyIdentifier, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return smartRecruitersResponse{}, fmt.Errorf("SmartRecruiters company %s returned %s", s.CompanyIdentifier, resp.Status)
	}

	var parsed smartRecruitersResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return smartRecruitersResponse{}, fmt.Errorf("decode SmartRecruiters response: %w", err)
	}
	return parsed, nil
}

func (s *SmartRecruitersSource) fetchDetails(ctx context.Context, postingID string) (smartRecruitersPostingDetails, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s/postings/%s", s.BaseURL, s.CompanyIdentifier, postingID), nil)
	if err != nil {
		return smartRecruitersPostingDetails{}, err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return smartRecruitersPostingDetails{}, fmt.Errorf("fetch SmartRecruiters posting %s: %w", postingID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return smartRecruitersPostingDetails{}, fmt.Errorf("SmartRecruiters posting %s returned %s", postingID, resp.Status)
	}

	var detail smartRecruitersPostingDetails
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return smartRecruitersPostingDetails{}, fmt.Errorf("decode SmartRecruiters posting %s: %w", postingID, err)
	}
	return detail, nil
}

func smartRecruitersRaw(posting smartRecruitersPosting) RawJob {
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
	remoteType := "onsite"
	if posting.Location.Remote {
		remoteType = "remote"
	}

	return RawJob{
		ExternalID:     firstNonEmpty(posting.ID, firstNonEmpty(posting.UUID, posting.RefNumber)),
		Title:          posting.Name,
		LocationText:   location,
		Country:        posting.Location.Country,
		State:          posting.Location.Region,
		City:           posting.Location.City,
		RemoteType:     remoteType,
		EmploymentType: posting.TypeOfEmployment.Label,
		ApplyURL:       posting.ApplyURL,
		SourceURL:      posting.PostingURL,
		PostedAt:       postedAt,
	}
}

func smartRecruitersDescription(detail smartRecruitersPostingDetails) string {
	sections := detail.JobAd.Sections
	var b strings.Builder
	for _, section := range []struct{ title, text string }{
		{sections.JobDescription.Title, sections.JobDescription.Text},
		{sections.Qualifications.Title, sections.Qualifications.Text},
		{sections.AdditionalInformation.Title, sections.AdditionalInformation.Text},
		{sections.CompanyDescription.Title, sections.CompanyDescription.Text},
	} {
		text := stripTags(section.text)
		if text == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		if section.title != "" {
			b.WriteString("**")
			b.WriteString(strings.TrimSpace(section.title))
			b.WriteString("**\n")
		}
		b.WriteString(text)
	}
	return strings.TrimSpace(b.String())
}
