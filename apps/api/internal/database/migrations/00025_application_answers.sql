-- +goose Up
CREATE TABLE application_answers (
    user_id               UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    full_name             TEXT,
    phone                 TEXT,
    email                 TEXT,
    location              TEXT,
    desired_location      TEXT,
    work_authorization    TEXT,
    sponsorship           TEXT,
    salary_expectation    TEXT,
    notice_period         TEXT,
    linkedin_url          TEXT,
    github_url            TEXT,
    portfolio_url         TEXT,
    common_answers        JSONB NOT NULL DEFAULT '{}',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE application_answers;
