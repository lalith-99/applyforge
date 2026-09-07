-- +goose Up
-- Intentionally empty. Named-company discovery was retired in favor of
-- market-wide job providers and dynamic company creation during ingestion.
SELECT 1;

-- +goose Down
SELECT 1;
