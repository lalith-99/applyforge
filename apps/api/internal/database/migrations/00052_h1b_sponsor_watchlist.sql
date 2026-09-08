-- +goose Up
-- Sponsor-first company watchlist and source registry.
--
-- The watchlist is derived from recent DOL H-1B LCA evidence. PERM is stored
-- as a secondary signal only and never contributes to the H-1B priority score.
-- Direct source discovery is tracked separately so companies can move from
-- paid/aggregated discovery to authoritative ATS polling over time.

CREATE TABLE company_sponsor_watchlist (
    company_id                   UUID PRIMARY KEY REFERENCES companies (id) ON DELETE CASCADE,
    employer_normalized_name     TEXT NOT NULL,
    watchlist_rank               INTEGER NOT NULL,
    tier                         TEXT NOT NULL,
    priority_score               DOUBLE PRECISION NOT NULL DEFAULT 0,
    current_fiscal_year          INTEGER NOT NULL,
    current_fy_certified         INTEGER NOT NULL DEFAULT 0,
    previous_fy_certified        INTEGER NOT NULL DEFAULT 0,
    two_years_ago_certified      INTEGER NOT NULL DEFAULT 0,
    recent_h1b_certified         INTEGER NOT NULL DEFAULT 0,
    active_h1b_years             SMALLINT NOT NULL DEFAULT 0,
    recent_perm_certified        INTEGER NOT NULL DEFAULT 0,
    poll_interval_minutes        INTEGER NOT NULL,
    source_discovery_status      TEXT NOT NULL DEFAULT 'PENDING',
    last_source_discovery_at     TIMESTAMPTZ,
    created_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT company_sponsor_watchlist_tier_check
        CHECK (tier IN ('HOT', 'WARM', 'COOL', 'COLD')),
    CONSTRAINT company_sponsor_watchlist_discovery_check
        CHECK (source_discovery_status IN ('PENDING', 'PARTIAL', 'RESOLVED', 'FAILED')),
    CONSTRAINT company_sponsor_watchlist_poll_check
        CHECK (poll_interval_minutes BETWEEN 15 AND 1440)
);

CREATE INDEX company_sponsor_watchlist_rank_idx
    ON company_sponsor_watchlist (watchlist_rank);
CREATE INDEX company_sponsor_watchlist_tier_idx
    ON company_sponsor_watchlist (tier, watchlist_rank);

CREATE TABLE company_source_registry (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id          UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    source_type         TEXT NOT NULL,
    board_token         TEXT NOT NULL DEFAULT '',
    source_url          TEXT NOT NULL,
    discovery_method    TEXT NOT NULL DEFAULT 'JOB_URL',
    confidence          REAL NOT NULL DEFAULT 1.0,
    monitorable         BOOLEAN NOT NULL DEFAULT false,
    first_seen_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_verified_at    TIMESTAMPTZ,
    last_error          TEXT,
    CONSTRAINT company_source_registry_type_check CHECK (
        source_type IN (
            'GREENHOUSE',
            'LEVER',
            'ASHBY',
            'SMARTRECRUITERS',
            'WORKABLE',
            'WORKDAY',
            'ICIMS',
            'ORACLE',
            'CUSTOM'
        )
    ),
    CONSTRAINT company_source_registry_method_check CHECK (
        discovery_method IN ('JOB_URL', 'CAREER_PAGE', 'MANUAL')
    ),
    CONSTRAINT company_source_registry_confidence_check CHECK (
        confidence >= 0 AND confidence <= 1
    )
);

CREATE UNIQUE INDEX company_source_registry_identity_idx
    ON company_source_registry (company_id, source_type, board_token, source_url);
CREATE INDEX company_source_registry_company_idx
    ON company_source_registry (company_id, monitorable, source_type);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION refresh_company_sponsor_watchlist(target_limit INTEGER DEFAULT 10000)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    current_fy INTEGER;
    inserted_count INTEGER;
BEGIN
    IF target_limit < 1 OR target_limit > 50000 THEN
        RAISE EXCEPTION 'target_limit must be between 1 and 50000';
    END IF;

    current_fy := EXTRACT(YEAR FROM current_date)::INTEGER
        + CASE WHEN EXTRACT(MONTH FROM current_date)::INTEGER >= 10 THEN 1 ELSE 0 END;

    DROP TABLE IF EXISTS tmp_applyforge_sponsor_watchlist;

    CREATE TEMP TABLE tmp_applyforge_sponsor_watchlist
    ON COMMIT DROP
    AS
    WITH latest_release AS (
        SELECT *
        FROM (
            SELECT
                e.*,
                row_number() OVER (
                    PARTITION BY e.employer_normalized_name, e.program, e.fiscal_year
                    ORDER BY e.latest_decision_date DESC NULLS LAST, e.imported_at DESC
                ) AS release_rank
            FROM company_immigration_evidence e
            WHERE e.fiscal_year BETWEEN current_fy - 2 AND current_fy
        ) ranked_release
        WHERE release_rank = 1
    ),
    h1b AS (
        SELECT
            employer_normalized_name,
            (array_agg(employer_name ORDER BY fiscal_year DESC, certified_count DESC))[1] AS employer_name,
            COALESCE(sum(certified_count) FILTER (WHERE fiscal_year = current_fy), 0)::INTEGER AS current_fy_certified,
            COALESCE(sum(certified_count) FILTER (WHERE fiscal_year = current_fy - 1), 0)::INTEGER AS previous_fy_certified,
            COALESCE(sum(certified_count) FILTER (WHERE fiscal_year = current_fy - 2), 0)::INTEGER AS two_years_ago_certified,
            COALESCE(sum(certified_count), 0)::INTEGER AS recent_h1b_certified,
            count(*) FILTER (WHERE certified_count > 0)::SMALLINT AS active_h1b_years
        FROM latest_release
        WHERE program = 'LCA_H1B'
          AND certified_count > 0
        GROUP BY employer_normalized_name
    ),
    perm AS (
        SELECT
            employer_normalized_name,
            COALESCE(sum(certified_count), 0)::INTEGER AS recent_perm_certified
        FROM latest_release
        WHERE program = 'PERM'
          AND certified_count > 0
        GROUP BY employer_normalized_name
    ),
    scored AS (
        SELECT
            h1b.*,
            COALESCE(perm.recent_perm_certified, 0) AS recent_perm_certified,
            (
                55.0 * ln(1.0 + h1b.current_fy_certified)
                + 30.0 * ln(1.0 + h1b.previous_fy_certified)
                + 15.0 * ln(1.0 + h1b.two_years_ago_certified)
                + 5.0 * h1b.active_h1b_years
            )::DOUBLE PRECISION AS priority_score
        FROM h1b
        LEFT JOIN perm USING (employer_normalized_name)
    ),
    ranked AS (
        SELECT
            scored.*,
            row_number() OVER (
                ORDER BY
                    priority_score DESC,
                    current_fy_certified DESC,
                    recent_h1b_certified DESC,
                    employer_normalized_name
            )::INTEGER AS watchlist_rank
        FROM scored
    )
    SELECT
        employer_normalized_name,
        employer_name,
        watchlist_rank,
        priority_score,
        current_fy_certified,
        previous_fy_certified,
        two_years_ago_certified,
        recent_h1b_certified,
        active_h1b_years,
        recent_perm_certified
    FROM ranked
    WHERE watchlist_rank <= target_limit;

    INSERT INTO companies (name, normalized_name)
    SELECT employer_name, employer_normalized_name
    FROM tmp_applyforge_sponsor_watchlist
    ON CONFLICT (normalized_name) DO NOTHING;

    INSERT INTO company_immigration_aliases (
        company_id,
        evidence_employer_normalized_name,
        match_method,
        confidence
    )
    SELECT
        c.id,
        t.employer_normalized_name,
        'EXACT_NORMALIZED',
        1.0
    FROM tmp_applyforge_sponsor_watchlist t
    JOIN companies c ON c.normalized_name = t.employer_normalized_name
    ON CONFLICT DO NOTHING;

    INSERT INTO company_sponsor_watchlist (
        company_id,
        employer_normalized_name,
        watchlist_rank,
        tier,
        priority_score,
        current_fiscal_year,
        current_fy_certified,
        previous_fy_certified,
        two_years_ago_certified,
        recent_h1b_certified,
        active_h1b_years,
        recent_perm_certified,
        poll_interval_minutes,
        updated_at
    )
    SELECT
        c.id,
        t.employer_normalized_name,
        t.watchlist_rank,
        CASE
            WHEN t.watchlist_rank <= LEAST(500, target_limit) THEN 'HOT'
            WHEN t.watchlist_rank <= LEAST(2500, target_limit) THEN 'WARM'
            WHEN t.watchlist_rank <= LEAST(5500, target_limit) THEN 'COOL'
            ELSE 'COLD'
        END,
        t.priority_score,
        current_fy,
        t.current_fy_certified,
        t.previous_fy_certified,
        t.two_years_ago_certified,
        t.recent_h1b_certified,
        t.active_h1b_years,
        t.recent_perm_certified,
        CASE
            WHEN t.watchlist_rank <= LEAST(500, target_limit) THEN 60
            WHEN t.watchlist_rank <= LEAST(2500, target_limit) THEN 240
            WHEN t.watchlist_rank <= LEAST(5500, target_limit) THEN 480
            ELSE 1440
        END,
        now()
    FROM tmp_applyforge_sponsor_watchlist t
    JOIN companies c ON c.normalized_name = t.employer_normalized_name
    ON CONFLICT (company_id) DO UPDATE SET
        employer_normalized_name = EXCLUDED.employer_normalized_name,
        watchlist_rank = EXCLUDED.watchlist_rank,
        tier = EXCLUDED.tier,
        priority_score = EXCLUDED.priority_score,
        current_fiscal_year = EXCLUDED.current_fiscal_year,
        current_fy_certified = EXCLUDED.current_fy_certified,
        previous_fy_certified = EXCLUDED.previous_fy_certified,
        two_years_ago_certified = EXCLUDED.two_years_ago_certified,
        recent_h1b_certified = EXCLUDED.recent_h1b_certified,
        active_h1b_years = EXCLUDED.active_h1b_years,
        recent_perm_certified = EXCLUDED.recent_perm_certified,
        poll_interval_minutes = EXCLUDED.poll_interval_minutes,
        updated_at = now();

    DELETE FROM company_sponsor_watchlist w
    WHERE NOT EXISTS (
        SELECT 1
        FROM tmp_applyforge_sponsor_watchlist t
        JOIN companies c ON c.normalized_name = t.employer_normalized_name
        WHERE c.id = w.company_id
    );

    SELECT count(*)::INTEGER INTO inserted_count
    FROM company_sponsor_watchlist;

    RETURN inserted_count;
END;
$$;
-- +goose StatementEnd

-- Build the initial list immediately when upgrading an existing database that
-- already contains imported DOL evidence. Fresh databases simply produce zero
-- rows until the importer runs, which refreshes the list again at completion.
SELECT refresh_company_sponsor_watchlist(10000);

-- +goose Down
DROP FUNCTION IF EXISTS refresh_company_sponsor_watchlist(INTEGER);
DROP TABLE company_source_registry;
DROP TABLE company_sponsor_watchlist;
