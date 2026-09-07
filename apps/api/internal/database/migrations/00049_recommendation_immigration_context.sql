-- +goose Up
ALTER TABLE job_recommendations
    ADD COLUMN immigration_status TEXT NOT NULL DEFAULT 'UNKNOWN',
    ADD COLUMN immigration_confidence TEXT NOT NULL DEFAULT 'LOW',
    ADD COLUMN immigration_evidence TEXT NOT NULL DEFAULT '',
    ADD COLUMN immigration_priority_score INTEGER NOT NULL DEFAULT 50;

ALTER TABLE job_recommendations
    ADD CONSTRAINT job_recommendations_immigration_priority_check
    CHECK (immigration_priority_score BETWEEN 0 AND 100);

-- +goose Down
ALTER TABLE job_recommendations DROP CONSTRAINT job_recommendations_immigration_priority_check;
ALTER TABLE job_recommendations
    DROP COLUMN immigration_priority_score,
    DROP COLUMN immigration_evidence,
    DROP COLUMN immigration_confidence,
    DROP COLUMN immigration_status;
