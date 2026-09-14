-- +goose Up
-- Cache AI fit judgments by the complete semantic input rather than by job ID
-- alone. A profile, requirements, policy, or model change therefore produces a
-- new key instead of silently reusing a stale judgment.
CREATE TABLE ai_ranking_judgment_cache (
    input_hash      TEXT PRIMARY KEY,
    job_id          UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    cache_version   TEXT NOT NULL,
    judgment        JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ai_ranking_judgment_cache_hash_check
        CHECK (input_hash ~ '^[0-9a-f]{64}$')
);

CREATE INDEX ai_ranking_judgment_cache_job_idx
    ON ai_ranking_judgment_cache (job_id, created_at DESC);
CREATE INDEX ai_ranking_judgment_cache_expiry_idx
    ON ai_ranking_judgment_cache (created_at);

-- Reservations are inserted before an external ranking request. They prevent
-- concurrent workers from all observing the same remaining budget. Failed or
-- heuristic-only requests are deleted; provider-backed requests are consumed.
CREATE TABLE ai_budget_debits (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operation          TEXT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'RESERVED',
    estimated_cost_usd NUMERIC(12, 6) NOT NULL,
    expires_at         TIMESTAMPTZ NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    consumed_at        TIMESTAMPTZ,
    CONSTRAINT ai_budget_debits_status_check
        CHECK (status IN ('RESERVED', 'CONSUMED')),
    CONSTRAINT ai_budget_debits_cost_check
        CHECK (estimated_cost_usd > 0),
    CONSTRAINT ai_budget_debits_consumed_check
        CHECK ((status = 'RESERVED' AND consumed_at IS NULL)
            OR (status = 'CONSUMED' AND consumed_at IS NOT NULL))
);

CREATE INDEX ai_budget_debits_operation_created_idx
    ON ai_budget_debits (operation, created_at DESC);
CREATE INDEX ai_budget_debits_active_reservation_idx
    ON ai_budget_debits (expires_at)
    WHERE status = 'RESERVED';

-- +goose Down
DROP TABLE ai_budget_debits;
DROP TABLE ai_ranking_judgment_cache;
