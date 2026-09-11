package jobs

import "context"

// SponsorTierCoverage makes source expansion measurable by sponsor tier instead
// of relying on raw job counts alone.
type SponsorTierCoverage struct {
	Tier                  string `json:"tier"`
	Companies             int64  `json:"companies"`
	ResolvedCompanies     int64  `json:"resolved_companies"`
	MonitorableCompanies  int64  `json:"monitorable_companies"`
	EnabledDirectSources  int64  `json:"enabled_direct_sources"`
	FreshUSSoftwareJobs24H int64 `json:"fresh_us_software_jobs_24h"`
	FreshJobCompanies24H  int64  `json:"fresh_job_companies_24h"`
}

// SponsorCoverageHealth summarizes whether broader sponsor discovery is
// actually producing fresh U.S. software jobs from more employers.
type SponsorCoverageHealth struct {
	QuarantinedJobSources     int64                 `json:"quarantined_job_sources"`
	UnlistedFreshJobs24H      int64                 `json:"unlisted_fresh_jobs_24h"`
	UnlistedFreshCompanies24H int64                 `json:"unlisted_fresh_companies_24h"`
	ByTier                    []SponsorTierCoverage `json:"by_tier"`
}

func (r *Repository) GetSponsorCoverageHealth(ctx context.Context) (SponsorCoverageHealth, error) {
	if r.pool == nil {
		return SponsorCoverageHealth{}, ErrSourceHealthUnavailable
	}

	var health SponsorCoverageHealth
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*)::bigint
		FROM job_sources
		WHERE enabled = false
		  AND COALESCE(last_error, '') LIKE $1
	`, permanentSourceErrorPrefix+"%").Scan(&health.QuarantinedJobSources); err != nil {
		return SponsorCoverageHealth{}, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			w.tier,
			count(DISTINCT w.company_id)::bigint AS companies,
			count(DISTINCT w.company_id) FILTER (
				WHERE w.source_discovery_status = 'RESOLVED'
			)::bigint AS resolved_companies,
			count(DISTINCT w.company_id) FILTER (
				WHERE EXISTS (
					SELECT 1
					FROM company_source_registry csr
					WHERE csr.company_id = w.company_id
					  AND csr.monitorable = true
					  AND COALESCE(csr.last_error, '') NOT LIKE $1
				)
			)::bigint AS monitorable_companies,
			count(DISTINCT js.id) FILTER (WHERE js.enabled = true)::bigint AS enabled_direct_sources,
			count(DISTINCT j.id)::bigint AS fresh_us_software_jobs_24h,
			count(DISTINCT j.company_id)::bigint AS fresh_job_companies_24h
		FROM company_sponsor_watchlist w
		LEFT JOIN job_sources js
		  ON js.company_id = w.company_id
		 AND js.source_type IN (
			'GREENHOUSE', 'LEVER', 'ASHBY', 'SMARTRECRUITERS',
			'WORKABLE', 'WORKDAY', 'CAREER_PAGE'
		 )
		LEFT JOIN jobs j
		  ON j.company_id = w.company_id
		 AND j.status = 'ACTIVE'
		 AND j.canonical_job_id IS NULL
		 AND j.country_code = 'US'
		 AND j.role_classification = 'IC_SOFTWARE'
		 AND j.posted_at IS NOT NULL
		 AND j.posted_at >= now() - INTERVAL '24 hours'
		GROUP BY w.tier
		ORDER BY CASE w.tier
			WHEN 'HOT' THEN 0
			WHEN 'WARM' THEN 1
			WHEN 'COOL' THEN 2
			ELSE 3
		END
	`, permanentSourceErrorPrefix+"%")
	if err != nil {
		return SponsorCoverageHealth{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var item SponsorTierCoverage
		if err := rows.Scan(
			&item.Tier,
			&item.Companies,
			&item.ResolvedCompanies,
			&item.MonitorableCompanies,
			&item.EnabledDirectSources,
			&item.FreshUSSoftwareJobs24H,
			&item.FreshJobCompanies24H,
		); err != nil {
			return SponsorCoverageHealth{}, err
		}
		health.ByTier = append(health.ByTier, item)
	}
	if err := rows.Err(); err != nil {
		return SponsorCoverageHealth{}, err
	}

	if err := r.pool.QueryRow(ctx, `
		SELECT
			count(*)::bigint,
			count(DISTINCT j.company_id)::bigint
		FROM jobs j
		WHERE j.status = 'ACTIVE'
		  AND j.canonical_job_id IS NULL
		  AND j.country_code = 'US'
		  AND j.role_classification = 'IC_SOFTWARE'
		  AND j.posted_at IS NOT NULL
		  AND j.posted_at >= now() - INTERVAL '24 hours'
		  AND NOT EXISTS (
			SELECT 1
			FROM company_sponsor_watchlist w
			WHERE w.company_id = j.company_id
		  )
	`).Scan(&health.UnlistedFreshJobs24H, &health.UnlistedFreshCompanies24H); err != nil {
		return SponsorCoverageHealth{}, err
	}

	return health, nil
}
