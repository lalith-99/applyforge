-- +goose Up
CREATE TABLE job_preferences (
    user_id                                 UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    remote                                  BOOLEAN NOT NULL DEFAULT false,
    hybrid                                  BOOLEAN NOT NULL DEFAULT false,
    onsite                                  BOOLEAN NOT NULL DEFAULT false,
    preferred_locations                     TEXT[] NOT NULL DEFAULT '{}',
    willingness_to_relocate                 BOOLEAN NOT NULL DEFAULT false,
    employment_types                        TEXT[] NOT NULL DEFAULT '{}',
    minimum_salary                          INTEGER,
    excluded_companies                      TEXT[] NOT NULL DEFAULT '{}',
    excluded_locations                      TEXT[] NOT NULL DEFAULT '{}',
    excluded_industries                     TEXT[] NOT NULL DEFAULT '{}',
    clearance_constraints                   TEXT,
    work_authorization                      TEXT,
    -- Immigration preferences are intentionally granular (see MASTER_REQUIREMENTS.md,
    -- Immigration-Aware Job Matching): never collapse into a single sponsorship boolean.
    immigration_status                      TEXT,
    requires_h1b_transfer                   BOOLEAN NOT NULL DEFAULT false,
    requires_new_h1b_cap_sponsorship        BOOLEAN NOT NULL DEFAULT false,
    requires_future_employment_sponsorship  BOOLEAN NOT NULL DEFAULT false,
    green_card_support_preferred            BOOLEAN NOT NULL DEFAULT false,
    green_card_support_required             BOOLEAN NOT NULL DEFAULT false,
    perm_support_preferred                  BOOLEAN NOT NULL DEFAULT false,
    immigration_support_min_confidence      TEXT,
    created_at                              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE job_preferences;
