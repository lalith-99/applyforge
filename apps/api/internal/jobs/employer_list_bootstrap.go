package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultEmployerListURL         = "https://raw.githubusercontent.com/Deepakvutla9/employers-list/main/employers.json"
	employerListProvider           = "DEEPAK_EMPLOYERS_LIST"
	maxEmployerListResponseBytes   = 4 << 20
	maxEmployerListRows            = 20000
	employerListGenericCareerToken = "EMPLOYERS_LIST"
)

type employerListDataset struct {
	Updated string            `json:"updated"`
	Rows    []employerListRow `json:"rows"`
}

type employerListRow struct {
	Name         string   `json:"n"`
	CareersURL   string   `json:"u"`
	Headquarters string   `json:"hq"`
	States       []string `json:"st"`
	Industry     string   `json:"ind"`
	Approvals    *int     `json:"a"`
	FiscalYear   *int     `json:"fy"`
	LinkedInURL  string   `json:"li"`
	Tier         string   `json:"tier"`
}

type employerListBootstrapResult struct {
	DirectoryEntries   int
	MatchedCompanies   int
	ExternalLinks      int
	RegistryCandidates int
	EnabledJobSources  int
}

// bootstrapEmployerList uses the public employers-list directory as a source
// discovery accelerator only. ApplyForge's DOL watchlist remains authoritative
// for sponsor ranking. Directory H-1B counts are retained as metadata/provenance
// and never overwrite recent DOL evidence.
func (r *Repository) bootstrapEmployerList(
	ctx context.Context,
	cfg FreeSourceBootstrapConfig,
	companies []freeSourceCompany,
) (employerListBootstrapResult, error) {
	var result employerListBootstrapResult
	if r.pool == nil || len(companies) == 0 {
		return result, nil
	}

	dataset, err := fetchEmployerListDataset(ctx, cfg)
	if err != nil {
		return result, err
	}
	result.DirectoryEntries = len(dataset.Rows)

	index := make(map[string][]employerListRow, len(dataset.Rows))
	for _, row := range dataset.Rows {
		key := sourceCompanyKey(row.Name)
		if key == "" {
			continue
		}
		index[key] = append(index[key], row)
	}

	for _, company := range companies {
		var matches []employerListRow
		seen := map[string]bool{}
		for _, key := range sourceCompanyExactKeys(company.Name, company.EmployerNormalizedName) {
			for _, row := range index[key] {
				identity := row.Name + "|" + row.CareersURL + "|" + row.LinkedInURL
				if seen[identity] {
					continue
				}
				seen[identity] = true
				matches = append(matches, row)
			}
		}
		if len(matches) == 0 {
			continue
		}

		// Prefer a direct careers URL over a search redirect, then prefer the
		// row with stronger historical approval volume when duplicate names exist.
		sort.SliceStable(matches, func(i, j int) bool {
			return employerListRowScore(matches[i]) > employerListRowScore(matches[j])
		})
		row := matches[0]
		result.MatchedCompanies++

		links, err := r.upsertEmployerListExternalLinks(ctx, company.ID, dataset.Updated, row)
		if err != nil {
			return result, fmt.Errorf("store employers-list links for %s: %w", company.Name, err)
		}
		result.ExternalLinks += links

		discoveries := employerListDiscoveries(row)
		if len(discoveries) == 0 {
			continue
		}
		result.RegistryCandidates += len(discoveries)
		for _, discovery := range discoveries {
			if discovery.Monitorable {
				result.EnabledJobSources++
			}
		}
		if err := r.RecordDiscoveredCompanySources(ctx, company.ID, discoveries); err != nil {
			return result, fmt.Errorf("record employers-list source for %s: %w", company.Name, err)
		}
	}

	return result, nil
}

func fetchEmployerListDataset(ctx context.Context, cfg FreeSourceBootstrapConfig) (employerListDataset, error) {
	endpoint := defaultEmployerListURL
	if _, err := validatePublicHTTPURL(endpoint); err != nil {
		return employerListDataset{}, fmt.Errorf("invalid employers-list endpoint: %w", err)
	}

	client := cfg.HTTPClient
	if client == nil {
		client = newPublicHTTPClient(25 * time.Second)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return employerListDataset{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ApplyForge/1.0 (+sponsor source bootstrap)")

	resp, err := client.Do(req)
	if err != nil {
		return employerListDataset{}, fmt.Errorf("fetch employers-list directory: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return employerListDataset{}, fmt.Errorf("fetch employers-list directory returned %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxEmployerListResponseBytes+1))
	if err != nil {
		return employerListDataset{}, fmt.Errorf("read employers-list directory: %w", err)
	}
	if len(body) > maxEmployerListResponseBytes {
		return employerListDataset{}, errors.New("employers-list directory exceeds response size limit")
	}
	return decodeEmployerListDataset(body)
}

func decodeEmployerListDataset(body []byte) (employerListDataset, error) {
	var dataset employerListDataset
	if err := json.Unmarshal(body, &dataset); err != nil {
		return employerListDataset{}, fmt.Errorf("decode employers-list directory: %w", err)
	}
	if len(dataset.Rows) > maxEmployerListRows {
		return employerListDataset{}, fmt.Errorf("employers-list directory has %d rows; maximum is %d", len(dataset.Rows), maxEmployerListRows)
	}
	return dataset, nil
}

func employerListRowScore(row employerListRow) int {
	score := 0
	if strings.TrimSpace(row.CareersURL) != "" {
		score += 20
		if !isDuckDuckGoDuckyURL(row.CareersURL) {
			score += 100
		}
	}
	if strings.TrimSpace(row.LinkedInURL) != "" {
		score += 10
	}
	if row.Approvals != nil {
		approvals := *row.Approvals
		if approvals > 1000 {
			approvals = 1000
		}
		if approvals > 0 {
			score += approvals / 100
		}
	}
	return score
}

func employerListDiscoveries(row employerListRow) []DiscoveredCompanySource {
	careerURL := strings.TrimSpace(row.CareersURL)
	if careerURL == "" || isDuckDuckGoDuckyURL(careerURL) {
		return nil
	}
	if _, err := validatePublicHTTPURL(careerURL); err != nil {
		return nil
	}

	detected := DetectCompanySources(careerURL)
	if len(detected) > 0 {
		for i := range detected {
			detected[i].DiscoveryMethod = "CAREER_PAGE"
			detected[i].Confidence = minFloat32(detected[i].Confidence, 0.92)
		}
		return detected
	}

	return []DiscoveredCompanySource{{
		SourceType:      "CUSTOM",
		BoardToken:      employerListGenericCareerToken,
		SourceURL:       careerURL,
		DiscoveryMethod: "CAREER_PAGE",
		Confidence:      0.72,
		Monitorable:     false,
	}}
}

func isDuckDuckGoDuckyURL(raw string) bool {
	parsed, err := validatePublicHTTPURL(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "duckduckgo.com" && host != "www.duckduckgo.com" {
		return false
	}
	query := strings.ToLower(parsed.RawQuery)
	return strings.Contains(query, "!ducky") || strings.Contains(query, "%21ducky")
}

func (r *Repository) upsertEmployerListExternalLinks(
	ctx context.Context,
	companyID uuid.UUID,
	datasetUpdated string,
	row employerListRow,
) (int, error) {
	metadata, err := json.Marshal(map[string]any{
		"provider_updated": datasetUpdated,
		"employer_name":    strings.TrimSpace(row.Name),
		"headquarters":     strings.TrimSpace(row.Headquarters),
		"states":           row.States,
		"industry":         strings.TrimSpace(row.Industry),
		"h1b_approvals":    row.Approvals,
		"fiscal_year":      row.FiscalYear,
		"volume_tier":      strings.TrimSpace(row.Tier),
	})
	if err != nil {
		return 0, err
	}

	links := []struct {
		kind string
		url  string
	}{
		{kind: "CAREERS", url: strings.TrimSpace(row.CareersURL)},
		{kind: "LINKEDIN_JOBS", url: strings.TrimSpace(row.LinkedInURL)},
	}

	stored := 0
	for _, link := range links {
		if link.url == "" {
			continue
		}
		if _, err := validatePublicHTTPURL(link.url); err != nil {
			continue
		}
		if _, err := r.pool.Exec(ctx, `
			INSERT INTO company_external_links (
				company_id, provider, link_type, url, metadata, last_seen_at
			) VALUES ($1, $2, $3, $4, $5::jsonb, now())
			ON CONFLICT (company_id, provider, link_type)
			DO UPDATE SET
				url = EXCLUDED.url,
				metadata = EXCLUDED.metadata,
				last_seen_at = now()
		`, companyID, employerListProvider, link.kind, link.url, string(metadata)); err != nil {
			return stored, err
		}
		stored++
	}
	return stored, nil
}
