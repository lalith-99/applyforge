-- +goose Up
CREATE TABLE learning_plans (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                  UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    job_id                   UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    skills                   TEXT[] NOT NULL DEFAULT '{}',
    current_readiness        INTEGER NOT NULL DEFAULT 0,
    target_readiness         INTEGER NOT NULL DEFAULT 0,
    topics                   JSONB NOT NULL DEFAULT '[]',
    practice_questions       JSONB NOT NULL DEFAULT '[]',
    projects                 TEXT[] NOT NULL DEFAULT '{}',
    architecture_questions   TEXT[] NOT NULL DEFAULT '{}',
    estimated_effort_category TEXT NOT NULL DEFAULT 'STANDARD_PREP',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX learning_plans_user_job_idx ON learning_plans (user_id, job_id);

-- +goose Down
DROP TABLE learning_plans;
