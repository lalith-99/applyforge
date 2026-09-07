-- +goose Up
-- Intentionally empty. Direct employer board tokens are not seeded.
-- Employer-direct ATS connectors remain available for future on-demand
-- canonical verification, but they are not a discovery mechanism.
SELECT 1;

-- +goose Down
SELECT 1;
