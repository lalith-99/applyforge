-- +goose Up
-- Phase J/K: async multi-pass tailoring. Tailoring already ran as one
-- blocking HTTP request calling the AI worker once; this adds intermediate
-- status stages (for a polling UI, since generation can now take a couple
-- of AI calls and the user said multi-minute latency is fine) and an AI
-- critic pass with one bounded revision loop.
ALTER TABLE tailoring_runs DROP CONSTRAINT tailoring_runs_status_check;
ALTER TABLE tailoring_runs ADD CONSTRAINT tailoring_runs_status_check
    CHECK (status IN ('PENDING', 'WRITING', 'EVALUATING', 'REVISING', 'COMPLETED', 'FAILED'));

ALTER TABLE tailoring_runs ADD COLUMN critic_result JSONB;
ALTER TABLE tailoring_runs ADD COLUMN revision_count INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE tailoring_runs DROP COLUMN revision_count;
ALTER TABLE tailoring_runs DROP COLUMN critic_result;
ALTER TABLE tailoring_runs DROP CONSTRAINT tailoring_runs_status_check;
ALTER TABLE tailoring_runs ADD CONSTRAINT tailoring_runs_status_check
    CHECK (status IN ('PENDING', 'COMPLETED', 'FAILED'));
