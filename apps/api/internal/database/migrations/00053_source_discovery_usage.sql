-- +goose Up
-- Provider-independent source-discovery scheduling and usage accounting.
-- The request ledger lets ApplyForge enforce hard API request caps before
-- calling paid discovery providers.

ALTER TABLE company_sponsor_watchlist
    ADD COLUMN source_discovery_attempt_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN next_source_discovery_at TIMESTAMPTZ,
    ADD COLUMN source_discovery_last_error TEXT;

CREATE INDEX company_sponsor_watchlist_source_discovery_due_idx
    ON company_sponsor_watchlist (
        source_discovery_status,
        next_source_discovery_at,
        watchlist_rank
    );

CREATE TABLE provider_usage (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider            TEXT NOT NULL,
    operation           TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'RESERVED',
    units               INTEGER NOT NULL DEFAULT 1,
    estimated_cost_usd  NUMERIC(12, 6),
    external_request_id TEXT,
    metadata            JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_message       TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at        TIMESTAMPTZ,
    CONSTRAINT provider_usage_status_check
        CHECK (status IN ('RESERVED', 'SUCCESS', 'ERROR', 'CANCELLED')),
    CONSTRAINT provider_usage_units_check CHECK (units > 0),
    CONSTRAINT provider_usage_estimated_cost_check
        CHECK (estimated_cost_usd IS NULL OR estimated_cost_usd >= 0)
);

CREATE INDEX provider_usage_provider_operation_created_idx
    ON provider_usage (provider, operation, created_at DESC);
CREATE INDEX provider_usage_status_created_idx
    ON provider_usage (status, created_at DESC);

-- Existing unresolved sponsor companies are immediately eligible for source
-- resolution once a resolver provider is enabled.
UPDATE company_sponsor_watchlist
SET next_source_discovery_at = now()
WHERE source_discovery_status IN ('PENDING', 'PARTIAL', 'FAILED')
  AND next_source_discovery_at IS NULL;

-- +goose Down
DROP TABLE provider_usage;
DROP INDEX IF EXISTS company_sponsor_watchlist_source_discovery_due_idx;

ALTER TABLE company_sponsor_watchlist
    DROP COLUMN source_discovery_last_error,
    DROP COLUMN next_source_discovery_at,
    DROP COLUMN source_discovery_attempt_count;
