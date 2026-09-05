-- +goose Up
INSERT INTO companies (name, normalized_name) VALUES
    ('Robinhood', 'robinhood'),
    ('Lever', 'lever'),
    ('Ramp', 'ramp');

INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'GREENHOUSE', id, 'robinhood' FROM companies WHERE normalized_name = 'robinhood';

INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'LEVER', id, 'lever' FROM companies WHERE normalized_name = 'lever';

INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'ASHBY', id, 'ramp' FROM companies WHERE normalized_name = 'ramp';

-- +goose Down
DELETE FROM job_sources WHERE board_token IN ('robinhood', 'lever', 'ramp');
DELETE FROM companies WHERE normalized_name IN ('robinhood', 'lever', 'ramp');
