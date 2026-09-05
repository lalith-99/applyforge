-- +goose Up
-- The original seed's only LEVER source (board_token 'lever') isn't a real
-- company board and always fetches 0 jobs, which was the root cause of jobs
-- only ever coming from 2 companies (Robinhood + Ramp). Disable it rather
-- than delete, and seed real, verified-live board tokens across all three
-- connectors so ingestion actually pulls from more companies.
UPDATE job_sources SET enabled = false WHERE source_type = 'LEVER' AND board_token = 'lever';

INSERT INTO companies (name, normalized_name) VALUES
    ('Stripe', 'stripe'),
    ('Airbnb', 'airbnb'),
    ('Coinbase', 'coinbase'),
    ('Affirm', 'affirm'),
    ('GitLab', 'gitlab'),
    ('Figma', 'figma'),
    ('Tala', 'tala'),
    ('Wealthfront', 'wealthfront'),
    ('Linear', 'linear'),
    ('Watershed', 'watershed'),
    ('Vanta', 'vanta');

INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'GREENHOUSE', id, normalized_name FROM companies
WHERE normalized_name IN ('stripe', 'airbnb', 'coinbase', 'affirm', 'gitlab', 'figma');

INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'LEVER', id, normalized_name FROM companies
WHERE normalized_name IN ('tala', 'wealthfront');

INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'ASHBY', id, normalized_name FROM companies
WHERE normalized_name IN ('linear', 'watershed', 'vanta');

-- +goose Down
DELETE FROM job_sources WHERE board_token IN (
    'stripe', 'airbnb', 'coinbase', 'affirm', 'gitlab', 'figma',
    'tala', 'wealthfront', 'linear', 'watershed', 'vanta'
);
DELETE FROM companies WHERE normalized_name IN (
    'stripe', 'airbnb', 'coinbase', 'affirm', 'gitlab', 'figma',
    'tala', 'wealthfront', 'linear', 'watershed', 'vanta'
);
UPDATE job_sources SET enabled = true WHERE source_type = 'LEVER' AND board_token = 'lever';
