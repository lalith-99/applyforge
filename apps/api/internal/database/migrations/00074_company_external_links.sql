-- +goose Up
-- External company directories are useful for navigation and source discovery,
-- but they are not authoritative job feeds. Keep their links and provenance
-- separate from job_sources/company_source_registry so a LinkedIn search URL,
-- for example, can never accidentally become a pollable source.
CREATE TABLE company_external_links (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id       UUID NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    provider         TEXT NOT NULL,
    link_type        TEXT NOT NULL,
    url              TEXT NOT NULL,
    metadata         JSONB NOT NULL DEFAULT '{}'::jsonb,
    first_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_verified_at TIMESTAMPTZ,
    CONSTRAINT company_external_links_provider_nonempty CHECK (btrim(provider) <> ''),
    CONSTRAINT company_external_links_url_nonempty CHECK (btrim(url) <> ''),
    CONSTRAINT company_external_links_type_check CHECK (
        link_type IN ('CAREERS', 'LINKEDIN_JOBS')
    )
);

CREATE UNIQUE INDEX company_external_links_provider_type_idx
    ON company_external_links (company_id, provider, link_type);
CREATE INDEX company_external_links_company_idx
    ON company_external_links (company_id, link_type);

-- +goose Down
DROP TABLE IF EXISTS company_external_links;