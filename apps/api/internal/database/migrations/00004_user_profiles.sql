-- +goose Up
CREATE TABLE user_profiles (
    user_id                       UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    first_name                    TEXT,
    last_name                     TEXT,
    city                          TEXT,
    state                         TEXT,
    country                       TEXT,
    primary_target_titles         TEXT[] NOT NULL DEFAULT '{}',
    alternative_target_titles     TEXT[] NOT NULL DEFAULT '{}',
    seniority                     TEXT,
    years_experience              INTEGER,
    preferred_industries          TEXT[] NOT NULL DEFAULT '{}',
    preferred_technologies        TEXT[] NOT NULL DEFAULT '{}',
    desired_compensation_min      INTEGER,
    desired_compensation_max      INTEGER,
    desired_compensation_currency TEXT NOT NULL DEFAULT 'USD',
    onboarding_completed_at       TIMESTAMPTZ,
    created_at                    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE user_profiles;
