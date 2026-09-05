-- +goose Up
CREATE TABLE job_matches (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id                    UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    user_id                   UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    total_score               INTEGER NOT NULL,
    grade                     TEXT NOT NULL,
    component_scores          JSONB NOT NULL DEFAULT '{}',
    matched_skills            TEXT[] NOT NULL DEFAULT '{}',
    transferable_skills       JSONB NOT NULL DEFAULT '[]',
    missing_required_skills   TEXT[] NOT NULL DEFAULT '{}',
    missing_preferred_skills  TEXT[] NOT NULL DEFAULT '{}',
    positive_evidence         TEXT[] NOT NULL DEFAULT '{}',
    concerns                  TEXT[] NOT NULL DEFAULT '{}',
    explanation               TEXT NOT NULL DEFAULT '',
    opportunity_score         INTEGER NOT NULL,
    current_profile_match     INTEGER NOT NULL,
    target_profile_match      INTEGER NOT NULL,
    suggested_target_additions TEXT[] NOT NULL DEFAULT '{}',
    eligible                  BOOLEAN NOT NULL DEFAULT true,
    hard_failures             TEXT[] NOT NULL DEFAULT '{}',
    warnings                  TEXT[] NOT NULL DEFAULT '{}',
    computed_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX job_matches_job_user_idx ON job_matches (job_id, user_id);
CREATE INDEX job_matches_user_id_idx ON job_matches (user_id);
CREATE INDEX job_matches_opportunity_score_idx ON job_matches (opportunity_score DESC);

-- +goose Down
DROP TABLE job_matches;
