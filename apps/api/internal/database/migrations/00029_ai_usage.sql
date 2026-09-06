-- +goose Up
-- AI cost/latency/reliability tracking (see docs/DECISIONS.md "Phase L" and
-- the Phase A enrichment cost incident that motivated pulling this forward).
CREATE TABLE ai_usage (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operation     TEXT NOT NULL,
    status        TEXT NOT NULL,
    latency_ms    INTEGER NOT NULL,
    cache_hit     BOOLEAN NOT NULL DEFAULT false,
    error_message TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ai_usage_status_check CHECK (status IN ('SUCCESS', 'ERROR'))
);

CREATE INDEX ai_usage_operation_created_idx ON ai_usage (operation, created_at);

-- +goose Down
DROP TABLE ai_usage;
