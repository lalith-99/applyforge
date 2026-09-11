package jobs

import "context"

// SponsorTierCoverage makes source expansion measurable by sponsor tier instead
// of relying on raw job counts alone. The funnel is intentionally company-first:
// a large source count is not useful if only a small number of employers are
// actually producing fresh jobs.
type SponsorTierCoverage struct {
	Tier                     string `json:"tier"`
	Companies                int64  `json:"companies"`
	RegistryCompanies        int64  `json:"registry_companies"`
	ResolvedCompanies        int64  `json:"resolved_companies"`
	FailedDiscoveryCompanies int64  `json:"failed_discovery_companies"`
	MonitorableCompanies     int64  `json:"monitorable_companies"`
	EnabledDirectSources     int64  `json:"enabled_direct_sources"`
	FreshUSSoftwareJobs24H   int64  `json:"fresh_us_software_jobs_24h"`
	FreshJobCompanies24H     int64  `json:"fresh_job_companies_24h"`
}

// SourceTypeCoverage answers the operational question "where is the catalog
// actually coming from?" Registry counts show discovered supply, enabled counts
// show what ApplyForge can poll today, and fresh counts show whether the source
// is producing useful inventory rather than merely existing in configuration.
type SourceTypeCoverage struct {
	SourceType             string `json:"source_type"`
	RegistrySources        int64  `json:"registry_sources"`
	RegistryCompanies      int64  `json:"registry_companies"`
	MonitorableSources     int64  `json:"monitorable_sources"`
	FailedRegistrySources  int64  `json:"failed_registry_sources"`
	EnabledSources         int64  `json:"enabled_sources"`
	EnabledCompanies       int64  `json:"enabled_companies"`
	FreshCanonicalJobs24H  int64  `json:"fresh_canonical_jobs_24h"`
	FreshUSSoftwareJobs24H int64  `json:"fresh_us_software_jobs_24h"`
	FreshJobCompanies24H   int64  `json:"fresh_job_companies_24h"`
}

// SponsorCoverageHealth is the acquisition control-plane view. It measures the
// full source funnel separately from candidate ranking so source discovery,
// connector coverage and ingestion throughput can be debugged independently.
type SponsorCoverageHealth struct {
	WatchlistCompanies        int64                 `json:"watchlist_companies"`
	RegistryCompanies         int64                 `json:"registry_companies"`
	MonitorableCompanies      int64                 `json:"monitorable_companies"`
	EnabledDirectSources      int64                 `json:"enabled_direct_sources"`
	EnabledDirectCompanies    int64                 `json:"enabled_direct_companies"`
	FreshCanonicalJobs24H     int64                 `json:"fresh_canonical_jobs_24h"`
	FreshUSSoftwareJobs24H    int64                 `json:"fresh_us_software_jobs_24h"`
	FreshJobCompanies24H      int64                 `json:"fresh_job_companies_24h"`
	QuarantinedJobSources     int64                 `json:"quarantined_job_sources"`
	StaleEnabledSources       int64                 `json:"stale_enabled_sources"`
	UnlistedFreshJobs24H      int64                 `json:"unlisted_fresh_jobs_24h"`
	UnlistedFreshCompanies24H int64                 `json:"unlisted_fresh_companies_24h"`
	BySourceType              []SourceTypeCoverage  `json:"by_source_type"`
	ByTier                    []SponsorTierCoverage `json:"by_tier"`
}

func (r *Repository) GetSponsorCoverageHealth(ctx context.Context) (SponsorCoverageHealth, error) {
	if r.pool == nil {
		return SponsorCoverageHealth{}, ErrSourceHealthUnavailable
	}

	const directSourceTypes = `(
		'GREENHOUSE', 'LEVER', 'ASHBY', 'SMARTRECRUITERS',
		'WORKABLE', 'WORKDAY', 'ICIMS', 'CAREER_PAGE'
	)`

	var health SponsorCoverageHealth
	if err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*)::bigint FROM company_sponsor_watchlist),
			(SELECT count(DISTINCT csr.company_id)::bigint FROM company_source_registry csr),
			(SELECT count(DISTINCT csr.company_id)::bigint
			 FROM company_source_registry csr
			 WHERE csr.monitorable = true
			   AND COALESCE(csr.last_error, '') NOT LIKE $1),
			(SELECT count(*)::bigint
			 FROM job_sources js
			 WHERE js.enabled = true
			   AND js.source_type IN `+directSourceTypes+`),
			(SELECT count(DISTINCT js.company_id)::bigint
			 FROM job_sources js
			 WHERE js.enabled = true
			   AND js.source_type IN `+directSourceTypes+`),
			(SELECT count(*)::bigint
			 FROM jobs j
			 WHERE j.status = 'ACTIVE'
			   AND j.canonical_job_id IS NULL
			   AND j.posted_at IS NOT NULL
			   AND j.posted_at >= now() - INTERVAL '24 hours'),
			(SELECT count(*)::bigint
			 FROM jobs j
			 WHERE j.status = 'ACTIVE'
			   AND j.canonical_job_id IS NULL
			   AND j.country_code = 'US'
			   AND j.role_classification = 'IC_SOFTWARE'
			   AND j.posted_at IS NOT NULL
			   AND j.posted_at >= now() - INTERVAL '24 hours'),
			(SELECT count(DISTINCT j.company_id)::bigint
			 FROM jobs j
			 WHERE j.status = 'ACTIVE'
			   AND j.canonical_job_id IS NULL
			   AND j.country_code = 'US'
			   AND j.role_classification = 'IC_SOFTWARE'
			   AND j.posted_at IS NOT NULL
			   AND j.posted_at >= now() - INTERVAL '24 hours'),
			(SELECT count(*)::bigint
			 FROM job_sources js
			 WHERE js.enabled = false
			   AND COALESCE(js.last_error, '') LIKE $1),
			(SELECT count(*)::bigint
			 FROM job_sources js
			 WHERE js.enabled = true
			   AND js.last_polled_at IS NOT NULL
			   AND js.last_polled_at < now() - make_interval(mins => GREATEST(js.poll_interval_minutes * 3, 180)))
	`, permanentSourceErrorPrefix+"%").Scan(
		&health.WatchlistCompanies,
		&health.RegistryCompanies,
		&health.MonitorableCompanies,
		&health.EnabledDirectSources,
		&health.EnabledDirectCompanies,
		&health.FreshCanonicalJobs24H,
		&health.FreshUSSoftwareJobs24H,
		&health.FreshJobCompanies24H,
		&health.QuarantinedJobSources,
		&health.StaleEnabledSources,
	); err != nil {
		return SponsorCoverageHealth{}, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			w.tier,
			count(*)::bigint AS companies,
			count(*) FILTER (
				WHERE EXISTS (
					SELECT 1 FROM company_source_registry csr
					WHERE csr.company_id = w.company_id
				)
			)::bigint AS registry_companies,
			count(*) FILTER (
				WHERE w.source_discovery_status = 'RESOLVED'
			)::bigint AS resolved_companies,
			count(*) FILTER (
				WHERE w.source_discovery_status = 'FAILED'
			)::bigint AS failed_discovery_companies,
			count(*) FILTER (
				WHERE EXISTS (
					SELECT 1
					FROM company_source_registry csr
					WHERE csr.company_id = w.company_id
					  AND csr.monitorable = true
					  AND COALESCE(csr.last_error, '') NOT LIKE $1
				)
			)::bigint AS monitorable_companies,
			COALESCE(sum((
				SELECT count(*)
				FROM job_sources js
				WHERE js.company_id = w.company_id
				  AND js.enabled = true
				  AND js.source_type IN `+directSourceTypes+`
			)), 0)::bigint AS enabled_direct_sources,
			COALESCE(sum((
				SELECT count(*)
				FROM jobs j
				WHERE j.company_id = w.company_id
				  AND j.status = 'ACTIVE'
				  AND j.canonical_job_id IS NULL
				  AND j.country_code = 'US'
				  AND j.role_classification = 'IC_SOFTWARE'
				  AND j.posted_at IS NOT NULL
				  AND j.posted_at >= now() - INTERVAL '24 hours'
			)), 0)::bigint AS fresh_us_software_jobs_24h,
			count(*) FILTER (
				WHERE EXISTS (
					SELECT 1
					FROM jobs j
					WHERE j.company_id = w.company_id
					  AND j.status = 'ACTIVE'
					  AND j.canonical_job_id IS NULL
					  AND j.country_code = 'US'
					  AND j.role_classification = 'IC_SOFTWARE'
					  AND j.posted_at IS NOT NULL
					  AND j.posted_at >= now() - INTERVAL '24 hours'
				)
			)::bigint AS fresh_job_companies_24h
		FROM company_sponsor_watchlist w
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
			&item.RegistryCompanies,
			&item.ResolvedCompanies,
			&item.FailedDiscoveryCompanies,
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

	sourceRows, err := r.pool.Query(ctx, `
		WITH source_types AS (
			SELECT DISTINCT source_type FROM company_source_registry
			UNION
			SELECT DISTINCT source_type FROM job_sources
			UNION
			SELECT DISTINCT source FROM jobs WHERE first_seen_at >= now() - INTERVAL '7 days'
		)
		SELECT
			st.source_type,
			(SELECT count(*)::bigint FROM company_source_registry csr WHERE csr.source_type = st.source_type),
			(SELECT count(DISTINCT csr.company_id)::bigint FROM company_source_registry csr WHERE csr.source_type = st.source_type),
			(SELECT count(*)::bigint FROM company_source_registry csr
			 WHERE csr.source_type = st.source_type
			   AND csr.monitorable = true
			   AND COALESCE(csr.last_error, '') NOT LIKE $1),
			(SELECT count(*)::bigint FROM company_source_registry csr
			 WHERE csr.source_type = st.source_type
			   AND (
				csr.inspection_status IN ('FAILED', 'UNSUPPORTED')
				OR COALESCE(csr.last_error, '') LIKE $1
			   )),
			(SELECT count(*)::bigint FROM job_sources js
			 WHERE js.source_type = st.source_type AND js.enabled = true),
			(SELECT count(DISTINCT js.company_id)::bigint FROM job_sources js
			 WHERE js.source_type = st.source_type AND js.enabled = true),
			(SELECT count(*)::bigint FROM jobs j
			 WHERE j.source = st.source_type
			   AND j.status = 'ACTIVE'
			   AND j.canonical_job_id IS NULL
			   AND j.posted_at IS NOT NULL
			   AND j.posted_at >= now() - INTERVAL '24 hours'),
			(SELECT count(*)::bigint FROM jobs j
			 WHERE j.source = st.source_type
			   AND j.status = 'ACTIVE'
			   AND j.canonical_job_id IS NULL
			   AND j.country_code = 'US'
			   AND j.role_classification = 'IC_SOFTWARE'
			   AND j.posted_at IS NOT NULL
			   AND j.posted_at >= now() - INTERVAL '24 hours'),
			(SELECT count(DISTINCT j.company_id)::bigint FROM jobs j
			 WHERE j.source = st.source_type
			   AND j.status = 'ACTIVE'
			   AND j.canonical_job_id IS NULL
			   AND j.country_code = 'US'
			   AND j.role_classification = 'IC_SOFTWARE'
			   AND j.posted_at IS NOT NULL
			   AND j.posted_at >= now() - INTERVAL '24 hours')
		FROM source_types st
		WHERE st.source_type <> ''
		ORDER BY st.source_type
	`, permanentSourceErrorPrefix+"%")
	if err != nil {
		return SponsorCoverageHealth{}, err
	}
	defer sourceRows.Close()

	for sourceRows.Next() {
		var item SourceTypeCoverage
		if err := sourceRows.Scan(
			&item.SourceType,
			&item.RegistrySources,
			&item.RegistryCompanies,
			&item.MonitorableSources,
			&item.FailedRegistrySources,
			&item.EnabledSources,
			&item.EnabledCompanies,
			&item.FreshCanonicalJobs24H,
			&item.FreshUSSoftwareJobs24H,
			&item.FreshJobCompanies24H,
		); err != nil {
			return SponsorCoverageHealth{}, err
		}
		health.BySourceType = append(health.BySourceType, item)
	}
	if err := sourceRows.Err(); err != nil {
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
