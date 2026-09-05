-- +goose Up
CREATE TABLE quick_prep_modules (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    normalized_skill      TEXT NOT NULL,
    what_it_is            TEXT NOT NULL DEFAULT '',
    why_it_matters        TEXT NOT NULL DEFAULT '',
    transferable_from     TEXT[] NOT NULL DEFAULT '{}',
    core_concepts         TEXT[] NOT NULL DEFAULT '{}',
    screening_points      TEXT[] NOT NULL DEFAULT '{}',
    interview_questions   JSONB NOT NULL DEFAULT '[]',
    common_mistakes       TEXT[] NOT NULL DEFAULT '{}',
    architecture_questions TEXT[] NOT NULL DEFAULT '{}',
    example_code          TEXT,
    generated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX quick_prep_modules_skill_idx ON quick_prep_modules (normalized_skill);

-- +goose Down
DROP TABLE quick_prep_modules;
