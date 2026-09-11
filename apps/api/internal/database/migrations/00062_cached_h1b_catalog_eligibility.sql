-- +goose Up
-- Cache recent H-1B eligibility at the company level so catalog reads do not
-- repeatedly traverse aliases + raw DOL evidence for every job row.
--
-- The existing company_has_recent_h1b_history(UUID) semantics are preserved:
-- * exact normalized employer matches
-- * reviewed aliases
-- * conservative prefix matches in either direction (minimum 6 chars)
-- * recent certified LCA_H1B evidence from the current fiscal year and prior 2
--
-- The top-10k sponsor watchlist remains a source-acquisition priority list,
-- not the candidate-eligibility universe. Companies outside the watchlist can
-- still be eligible (for example a brand-name catalog company whose DOL record
-- uses a longer legal employer name).

ALTER FUNCTION company_has_recent_h1b_history(UUID)
    RENAME TO company_has_recent_h1b_history_uncached;

CREATE TABLE company_recent_h1b_status (
    company_id              UUID PRIMARY KEY REFERENCES companies (id) ON DELETE CASCADE,
    has_recent_h1b_history  BOOLEAN NOT NULL,
    refreshed_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Refresh all known companies atomically. Watchlist members are known-positive
-- by construction, so only companies outside the acquisition watchlist need
-- the more expensive legacy evidence/alias evaluation.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION refresh_company_recent_h1b_status()
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    refreshed_count INTEGER;
BEGIN
    DELETE FROM company_recent_h1b_status;

    INSERT INTO company_recent_h1b_status (
        company_id,
        has_recent_h1b_history,
        refreshed_at
    )
    SELECT
        c.id,
        CASE
            WHEN w.company_id IS NOT NULL THEN true
            ELSE company_has_recent_h1b_history_uncached(c.id)
        END,
        now()
    FROM companies c
    LEFT JOIN company_sponsor_watchlist w ON w.company_id = c.id;

    GET DIAGNOSTICS refreshed_count = ROW_COUNT;
    RETURN refreshed_count;
END;
$$;
-- +goose StatementEnd

-- Populate the read model from the current database state without forcing a
-- second 10,000-company sponsor-watchlist rebuild during this migration.
SELECT refresh_company_recent_h1b_status();

-- Keep the public predicate name stable for ListJobs, CountJobs, semantic
-- retrieval, and any other existing callers. For all companies known at the
-- last refresh this is one primary-key lookup. The uncached fallback preserves
-- correctness for a brand-new company until the next sponsor refresh runs.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION company_has_recent_h1b_history(target_company_id UUID)
RETURNS BOOLEAN
LANGUAGE SQL
STABLE
AS $$
    SELECT COALESCE(
        (
            SELECT s.has_recent_h1b_history
            FROM company_recent_h1b_status s
            WHERE s.company_id = target_company_id
        ),
        company_has_recent_h1b_history_uncached(target_company_id)
    );
$$;
-- +goose StatementEnd

-- Wrap the existing sponsor refresh so both the API worker and direct local SQL
-- workflow (SELECT refresh_company_sponsor_watchlist(10000)) refresh the cached
-- candidate-eligibility read model automatically.
ALTER FUNCTION refresh_company_sponsor_watchlist(INTEGER)
    RENAME TO refresh_company_sponsor_watchlist_base;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION refresh_company_sponsor_watchlist(target_limit INTEGER DEFAULT 10000)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    sponsor_count INTEGER;
BEGIN
    sponsor_count := refresh_company_sponsor_watchlist_base(target_limit);
    PERFORM refresh_company_recent_h1b_status();
    RETURN sponsor_count;
END;
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS refresh_company_sponsor_watchlist(INTEGER);
ALTER FUNCTION refresh_company_sponsor_watchlist_base(INTEGER)
    RENAME TO refresh_company_sponsor_watchlist;

DROP FUNCTION IF EXISTS company_has_recent_h1b_history(UUID);
DROP FUNCTION IF EXISTS refresh_company_recent_h1b_status();
DROP TABLE company_recent_h1b_status;

ALTER FUNCTION company_has_recent_h1b_history_uncached(UUID)
    RENAME TO company_has_recent_h1b_history;
