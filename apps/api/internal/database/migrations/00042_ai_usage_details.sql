-- +goose Up
ALTER TABLE ai_usage
    ADD COLUMN provider TEXT,
    ADD COLUMN model TEXT,
    ADD COLUMN prompt_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN completion_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN total_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN estimated_cost_usd NUMERIC(16, 10);

CREATE INDEX ai_usage_model_created_idx ON ai_usage (model, created_at)
WHERE model IS NOT NULL;

-- +goose Down
DROP INDEX ai_usage_model_created_idx;
ALTER TABLE ai_usage
    DROP COLUMN estimated_cost_usd,
    DROP COLUMN total_tokens,
    DROP COLUMN completion_tokens,
    DROP COLUMN prompt_tokens,
    DROP COLUMN model,
    DROP COLUMN provider;
