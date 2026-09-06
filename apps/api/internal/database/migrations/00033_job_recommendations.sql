-- +goose Up
-- Phase I: precompute recommendations asynchronously instead of computing
-- the full funnel (hard filter -> semantic retrieval -> deterministic
-- score -> AI rerank) on every /jobs request. A background worker runs the
-- funnel once per profile change and materializes the result here; reads
-- become a simple indexed SELECT.
CREATE TABLE job_recommendations (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                  UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    job_id                   UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    deterministic_score      INTEGER NOT NULL,
    ai_fit_score             INTEGER,
    ai_recommendation        TEXT,
    ai_reason                TEXT NOT NULL DEFAULT '',
    final_score              INTEGER NOT NULL,
    candidate_profile_version INTEGER,
    computed_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, job_id)
);

CREATE INDEX job_recommendations_user_score_idx ON job_recommendations (user_id, final_score DESC);

-- +goose Down
DROP TABLE job_recommendations;
