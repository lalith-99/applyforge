-- +goose Up
-- Further broadens company coverage per user feedback ("why only 11
-- companies?"). Still the same connector architecture (Greenhouse/Lever/
-- Ashby require a known per-company board_token - there is no "search all
-- companies" API for any of them), but with many more real, verified-live
-- board tokens seeded. See also 00013/00026.
INSERT INTO companies (name, normalized_name) VALUES
    ('Datadog', 'datadog'),
    ('Cloudflare', 'cloudflare'),
    ('Twilio', 'twilio'),
    ('Okta', 'okta'),
    ('Asana', 'asana'),
    ('Dropbox', 'dropbox'),
    ('Squarespace', 'squarespace'),
    ('Elastic', 'elastic'),
    ('MongoDB', 'mongodb'),
    ('PagerDuty', 'pagerduty'),
    ('New Relic', 'newrelic'),
    ('Pinterest', 'pinterest'),
    ('Lyft', 'lyft'),
    ('Instacart', 'instacart'),
    ('Databricks', 'databricks'),
    ('Intercom', 'intercom'),
    ('Amplitude', 'amplitude'),
    ('Braze', 'braze'),
    ('Ro', 'ro'),
    ('Vevo', 'vevo'),
    ('Imprint', 'imprint'),
    ('Speak', 'speak'),
    ('Multiverse', 'multiverse');

INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'GREENHOUSE', id, normalized_name FROM companies
WHERE normalized_name IN (
    'datadog', 'cloudflare', 'twilio', 'okta', 'asana', 'dropbox', 'squarespace',
    'elastic', 'mongodb', 'pagerduty', 'newrelic', 'pinterest', 'lyft', 'instacart',
    'databricks', 'intercom', 'amplitude', 'braze'
);

INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'LEVER', id, normalized_name FROM companies WHERE normalized_name IN ('ro', 'vevo');

INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'ASHBY', id, normalized_name FROM companies
WHERE normalized_name IN ('imprint', 'speak', 'multiverse');

-- +goose Down
DELETE FROM job_sources WHERE board_token IN (
    'datadog', 'cloudflare', 'twilio', 'okta', 'asana', 'dropbox', 'squarespace',
    'elastic', 'mongodb', 'pagerduty', 'newrelic', 'pinterest', 'lyft', 'instacart',
    'databricks', 'intercom', 'amplitude', 'braze', 'ro', 'vevo', 'imprint', 'speak',
    'multiverse'
);
DELETE FROM companies WHERE normalized_name IN (
    'datadog', 'cloudflare', 'twilio', 'okta', 'asana', 'dropbox', 'squarespace',
    'elastic', 'mongodb', 'pagerduty', 'newrelic', 'pinterest', 'lyft', 'instacart',
    'databricks', 'intercom', 'amplitude', 'braze', 'ro', 'vevo', 'imprint', 'speak',
    'multiverse'
);
