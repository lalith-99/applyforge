package jobs

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// DiscoveredCompanySource is a job-publishing endpoint learned from an
// observed job/apply URL. Supported public ATS boards can be promoted into
// job_sources automatically; unsupported ATSs remain in the registry so a
// future connector/crawler can take over without rediscovering the company.
type DiscoveredCompanySource struct {
	SourceType string
	BoardToken string
	SourceURL  string
	Confidence float32
	Monitorable bool
}

// DetectCompanySources recognizes stable ATS URL patterns without making any
// network calls. It intentionally ignores generic URLs rather than guessing.
func DetectCompanySources(rawURLs ...string) []DiscoveredCompanySource {
	seen := map[string]bool{}
	out := make([]DiscoveredCompanySource, 0, len(rawURLs))

	add := func(d DiscoveredCompanySource) {
		if d.SourceType == "" || d.SourceURL == "" {
			return
		}
		key := d.SourceType + "|" + d.BoardToken + "|" + d.SourceURL
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, d)
	}

	for _, raw := range rawURLs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			continue
		}
		host := strings.ToLower(u.Hostname())
		parts := splitPath(u.Path)
		first := ""
		if len(parts) > 0 {
			first = parts[0]
		}

		switch {
		case host == "boards.greenhouse.io" || host == "job-boards.greenhouse.io":
			if first != "" {
				add(DiscoveredCompanySource{
					SourceType: "GREENHOUSE", BoardToken: first,
					SourceURL: "https://" + host + "/" + first,
					Confidence: 1, Monitorable: true,
				})
			}
		case host == "jobs.lever.co":
			if first != "" {
				add(DiscoveredCompanySource{
					SourceType: "LEVER", BoardToken: first,
					SourceURL: "https://" + host + "/" + first,
					Confidence: 1, Monitorable: true,
				})
			}
		case host == "jobs.ashbyhq.com":
			if first != "" {
				add(DiscoveredCompanySource{
					SourceType: "ASHBY", BoardToken: first,
					SourceURL: "https://" + host + "/" + first,
					Confidence: 1, Monitorable: true,
				})
			}
		case host == "jobs.smartrecruiters.com":
			if first != "" {
				add(DiscoveredCompanySource{
					SourceType: "SMARTRECRUITERS", BoardToken: first,
					SourceURL: "https://" + host + "/" + first,
					Confidence: 1, Monitorable: true,
				})
			}
		case host == "apply.workable.com":
			if first != "" {
				add(DiscoveredCompanySource{
					SourceType: "WORKABLE", BoardToken: first,
					SourceURL: "https://" + host + "/" + first,
					Confidence: 1, Monitorable: true,
				})
			}
		case strings.HasSuffix(host, ".myworkdayjobs.com") || host == "myworkdayjobs.com":
			add(DiscoveredCompanySource{
				SourceType: "WORKDAY", BoardToken: host,
				SourceURL: "https://" + host,
				Confidence: 0.95, Monitorable: false,
			})
		case strings.HasSuffix(host, ".icims.com") || host == "icims.com":
			add(DiscoveredCompanySource{
				SourceType: "ICIMS", BoardToken: host,
				SourceURL: "https://" + host,
				Confidence: 0.95, Monitorable: false,
			})
		case strings.HasSuffix(host, ".oraclecloud.com"):
			add(DiscoveredCompanySource{
				SourceType: "ORACLE", BoardToken: host,
				SourceURL: "https://" + host,
				Confidence: 0.95, Monitorable: false,
			})
		}
	}

	return out
}

func splitPath(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	out := raw[:0]
	for _, part := range raw {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// RecordDiscoveredCompanySources persists detected ATS endpoints and enables
// direct polling for connector types ApplyForge can already fetch.
func (r *Repository) RecordDiscoveredCompanySources(ctx context.Context, companyID uuid.UUID, discoveries []DiscoveredCompanySource) error {
	if r.pool == nil || len(discoveries) == 0 {
		return nil
	}

	pollMinutes := 60
	if err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(
			(SELECT poll_interval_minutes FROM company_sponsor_watchlist WHERE company_id = $1),
			60
		)
	`, companyID).Scan(&pollMinutes); err != nil {
		return fmt.Errorf("load sponsor watchlist cadence: %w", err)
	}

	resolved := false
	partial := false
	for _, d := range discoveries {
		if _, err := r.pool.Exec(ctx, `
			INSERT INTO company_source_registry (
				company_id, source_type, board_token, source_url,
				discovery_method, confidence, monitorable, last_seen_at
			) VALUES ($1, $2, $3, $4, 'JOB_URL', $5, $6, now())
			ON CONFLICT (company_id, source_type, board_token, source_url)
			DO UPDATE SET
				confidence = GREATEST(company_source_registry.confidence, EXCLUDED.confidence),
				monitorable = company_source_registry.monitorable OR EXCLUDED.monitorable,
				last_seen_at = now()
		`, companyID, d.SourceType, d.BoardToken, d.SourceURL, d.Confidence, d.Monitorable); err != nil {
			return fmt.Errorf("record discovered %s source: %w", d.SourceType, err)
		}

		if d.Monitorable && d.BoardToken != "" {
			if _, err := r.pool.Exec(ctx, `
				INSERT INTO job_sources (
					source_type, company_id, board_token, enabled, poll_interval_minutes
				) VALUES ($1, $2, $3, true, $4)
				ON CONFLICT (source_type, board_token)
				DO UPDATE SET
					company_id = EXCLUDED.company_id,
					enabled = true,
					poll_interval_minutes = EXCLUDED.poll_interval_minutes
			`, d.SourceType, companyID, d.BoardToken, pollMinutes); err != nil {
				return fmt.Errorf("enable discovered %s source: %w", d.SourceType, err)
			}
			resolved = true
		} else {
			partial = true
		}
	}

	status := "PENDING"
	switch {
	case resolved:
		status = "RESOLVED"
	case partial:
		status = "PARTIAL"
	}
	if _, err := r.pool.Exec(ctx, `
		UPDATE company_sponsor_watchlist
		SET source_discovery_status = $2,
		    last_source_discovery_at = now(),
		    updated_at = now()
		WHERE company_id = $1
	`, companyID, status); err != nil {
		return fmt.Errorf("update source discovery status: %w", err)
	}

	return nil
}
