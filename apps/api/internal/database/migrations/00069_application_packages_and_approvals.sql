-- +goose Up
CREATE TABLE application_packages (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    application_id      UUID NOT NULL REFERENCES applications (id) ON DELETE CASCADE,
    job_id              UUID NOT NULL REFERENCES jobs (id) ON DELETE RESTRICT,
    resume_version_id   UUID NOT NULL REFERENCES resume_versions (id) ON DELETE RESTRICT,
    destination_url     TEXT NOT NULL,
    destination_origin  TEXT NOT NULL,
    adapter             TEXT NOT NULL DEFAULT 'BROWSER_COMPANION',
    form_version        TEXT NOT NULL DEFAULT 'v1',
    answers_json        JSONB NOT NULL DEFAULT '{}'::jsonb,
    required_fields     JSONB NOT NULL DEFAULT '[]'::jsonb,
    evidence_refs       JSONB NOT NULL DEFAULT '[]'::jsonb,
    resume_content_hash TEXT NOT NULL,
    answers_hash        TEXT NOT NULL,
    package_hash        TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT application_packages_destination_check CHECK (destination_url ~ '^https://'),
    CONSTRAINT application_packages_hash_check CHECK (length(package_hash) = 64),
    CONSTRAINT application_packages_resume_hash_check CHECK (length(resume_content_hash) = 64),
    CONSTRAINT application_packages_answers_hash_check CHECK (length(answers_hash) = 64),
    UNIQUE (user_id, application_id, package_hash)
);

CREATE INDEX application_packages_application_idx
    ON application_packages (application_id, created_at DESC);
CREATE INDEX application_packages_user_idx
    ON application_packages (user_id, created_at DESC);

CREATE TABLE application_approvals (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    package_id           UUID NOT NULL REFERENCES application_packages (id) ON DELETE CASCADE,
    user_id              UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    package_hash         TEXT NOT NULL,
    action_scope         TEXT NOT NULL DEFAULT 'SUBMIT_ONCE',
    confirmation_version TEXT NOT NULL,
    confirmation_text    TEXT NOT NULL,
    approved_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at           TIMESTAMPTZ NOT NULL,
    revoked_at           TIMESTAMPTZ,
    CONSTRAINT application_approvals_scope_check CHECK (action_scope IN ('SUBMIT_ONCE')),
    CONSTRAINT application_approvals_hash_check CHECK (length(package_hash) = 64),
    UNIQUE (package_id, user_id, action_scope)
);

CREATE INDEX application_approvals_active_idx
    ON application_approvals (package_id, user_id, expires_at)
    WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE application_approvals;
DROP TABLE application_packages;
