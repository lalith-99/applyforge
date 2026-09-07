-- +goose Up
-- Live-verified (curl'd against boards-api.greenhouse.io / api.ashbyhq.com /
-- api.lever.co directly, kept only 200 + non-empty responses) expansion of
-- direct-ATS coverage per user feedback ("850 total, only 8 in last 24h").
-- See also 00013/00026/00027.
INSERT INTO companies (name, normalized_name) VALUES
    ('Discord', 'discord'),
    ('Webflow', 'webflow'),
    ('Brex', 'brex'),
    ('Chime', 'chime'),
    ('Gusto', 'gusto'),
    ('Vercel', 'vercel'),
    ('Netlify', 'netlify'),
    ('Anthropic', 'anthropic'),
    ('Mixpanel', 'mixpanel'),
    ('Cockroach Labs', 'cockroachlabs'),
    ('Airtable', 'airtable'),
    ('Calendly', 'calendly'),
    ('Flexport', 'flexport'),
    ('Faire', 'faire'),
    ('Reddit', 'reddit'),
    ('Twitch', 'twitch'),
    ('Epic Games', 'epicgames'),
    ('Roblox', 'roblox'),
    ('Fastly', 'fastly'),
    ('Fivetran', 'fivetran'),
    ('AssemblyAI', 'assemblyai'),
    ('CoreWeave', 'coreweave'),
    ('Carta', 'carta'),
    ('EarnIn', 'earnin'),
    ('Ripple', 'ripple'),
    ('Gemini', 'gemini'),
    ('BitGo', 'bitgo'),
    ('Cresta', 'cresta'),
    ('Honeycomb', 'honeycomb'),
    ('Grafana Labs', 'grafanalabs'),
    ('Orca Security', 'orca'),
    ('Bugcrowd', 'bugcrowd'),
    ('Netskope', 'netskope'),
    ('Zscaler', 'zscaler'),
    ('Notion', 'notion'),
    ('Cohere', 'cohere'),
    ('Modal', 'modal'),
    ('Fireworks AI', 'fireworks'),
    ('MotherDuck', 'motherduck'),
    ('Warp', 'warp'),
    ('Resend', 'resend'),
    ('Supabase', 'supabase'),
    ('Temporal', 'temporal'),
    ('Hex', 'hex'),
    ('Prefect', 'prefect'),
    ('Pinecone', 'pinecone'),
    ('Weaviate', 'weaviate'),
    ('Browserbase', 'browserbase'),
    ('LlamaIndex', 'llamaindex'),
    ('Crusoe Energy', 'crusoe'),
    ('Cursor', 'cursor'),
    ('Clerk', 'clerk'),
    ('WeRide', 'weride');

-- These four already existed as company rows (created earlier via
-- Arbeitnow's per-job dynamic company upsert) but had no job_sources row.
INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'GREENHOUSE', id, 'scaleai' FROM companies WHERE normalized_name = 'scaleai';
INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'GREENHOUSE', id, 'samsara' FROM companies WHERE normalized_name = 'samsara';
INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'GREENHOUSE', id, 'verkada' FROM companies WHERE normalized_name = 'verkada';
INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'ASHBY', id, 'langchain' FROM companies WHERE normalized_name = 'langchain';

INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'GREENHOUSE', id, normalized_name FROM companies WHERE normalized_name IN (
    'discord', 'webflow', 'brex', 'chime', 'gusto', 'vercel', 'netlify',
    'anthropic', 'mixpanel', 'cockroachlabs', 'airtable', 'calendly',
    'flexport', 'faire', 'reddit', 'twitch', 'epicgames', 'roblox', 'fastly',
    'fivetran', 'assemblyai', 'coreweave', 'carta', 'earnin', 'ripple', 'gemini',
    'bitgo', 'cresta', 'honeycomb', 'grafanalabs', 'orca', 'bugcrowd',
    'netskope', 'zscaler'
);

INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'ASHBY', id, normalized_name FROM companies WHERE normalized_name IN (
    'notion', 'cohere', 'modal', 'fireworks', 'motherduck', 'warp', 'resend',
    'supabase', 'temporal', 'hex', 'prefect', 'pinecone', 'weaviate',
    'browserbase', 'llamaindex', 'crusoe', 'cursor', 'clerk'
);

INSERT INTO job_sources (source_type, company_id, board_token)
SELECT 'LEVER', id, normalized_name FROM companies WHERE normalized_name = 'weride';

-- +goose Down
DELETE FROM job_sources WHERE board_token IN (
    'discord', 'webflow', 'brex', 'chime', 'gusto', 'vercel', 'netlify',
    'anthropic', 'mixpanel', 'cockroachlabs', 'airtable', 'calendly',
    'flexport', 'faire', 'reddit', 'twitch', 'epicgames', 'roblox', 'fastly',
    'fivetran', 'assemblyai', 'coreweave', 'carta', 'earnin', 'ripple', 'gemini',
    'bitgo', 'cresta', 'honeycomb', 'grafanalabs', 'orca', 'bugcrowd',
    'netskope', 'zscaler', 'notion', 'cohere', 'modal', 'fireworks', 'motherduck',
    'warp', 'resend', 'supabase', 'temporal', 'hex', 'prefect', 'pinecone',
    'weaviate', 'browserbase', 'llamaindex', 'crusoe', 'cursor',
    'clerk', 'weride'
) AND company_id IN (SELECT id FROM companies WHERE normalized_name NOT IN ('scaleai', 'samsara', 'verkada', 'langchain'));
DELETE FROM job_sources WHERE (source_type, board_token) IN (
    ('GREENHOUSE', 'scaleai'), ('GREENHOUSE', 'samsara'), ('GREENHOUSE', 'verkada'), ('ASHBY', 'langchain')
);
DELETE FROM companies WHERE normalized_name IN (
    'discord', 'webflow', 'brex', 'chime', 'gusto', 'vercel', 'netlify',
    'anthropic', 'mixpanel', 'cockroachlabs', 'airtable', 'calendly',
    'flexport', 'faire', 'reddit', 'twitch', 'epicgames', 'roblox', 'fastly',
    'fivetran', 'assemblyai', 'coreweave', 'carta', 'earnin', 'ripple', 'gemini',
    'bitgo', 'cresta', 'honeycomb', 'grafanalabs', 'orca', 'bugcrowd',
    'netskope', 'zscaler', 'notion', 'cohere', 'modal', 'fireworks', 'motherduck',
    'warp', 'resend', 'supabase', 'temporal', 'hex', 'prefect', 'pinecone',
    'weaviate', 'browserbase', 'llamaindex', 'crusoe', 'cursor',
    'clerk', 'weride'
);
