-- +goose Up
-- Retire the hand-maintained company-board registry from production discovery.
-- Historical source rows are retained (disabled) so old poll-run history and
-- debugging provenance remain intact. Market-wide providers now own discovery.
UPDATE job_sources
SET enabled = false
WHERE source_type IN ('GREENHOUSE', 'LEVER', 'ASHBY', 'SMARTRECRUITERS', 'WORKABLE');

-- +goose Down
-- Intentionally do not blindly re-enable legacy company seeds. Some rows may
-- have been explicitly disabled before this migration because their board
-- tokens were invalid or intentionally retired.
SELECT 1;
