-- +goose Up
-- Intentionally empty.
-- ApplyForge no longer seeds named employers or company-specific ATS boards.
-- Market-wide providers discover employers dynamically from current postings.
SELECT 1;

-- +goose Down
SELECT 1;
