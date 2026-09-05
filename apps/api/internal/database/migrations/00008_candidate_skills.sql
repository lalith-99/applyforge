-- +goose Up
CREATE TABLE candidate_skills (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    normalized_name TEXT NOT NULL,
    display_name    TEXT NOT NULL,
    category        TEXT,
    proficiency     TEXT,
    source          TEXT NOT NULL,
    status          TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT candidate_skills_source_check CHECK (
        source IN ('MASTER_RESUME', 'USER_PROFILE', 'AI_RECOMMENDATION', 'JOB_TARGETING', 'PROJECT', 'MANUAL_ENTRY')
    ),
    CONSTRAINT candidate_skills_status_check CHECK (
        status IN (
            'VERIFIED_PROFESSIONAL', 'VERIFIED_PROJECT', 'FAMILIAR', 'LEARNING',
            'TARGET_SKILL', 'USER_APPROVED', 'UNKNOWN'
        )
    )
);

CREATE UNIQUE INDEX candidate_skills_user_skill_idx ON candidate_skills (user_id, normalized_name);
CREATE INDEX candidate_skills_user_id_idx ON candidate_skills (user_id);

-- +goose Down
DROP TABLE candidate_skills;
