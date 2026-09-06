-- +goose Up
-- Phase F: CandidateIntelligenceProfile - materializes a single AI-synthesized
-- summary of a candidate (currently spread across profile, preferences,
-- candidate_skills, resume/resume_experiences) so matching/ranking reads one
-- compact object instead of re-deriving it from scattered tables every time.
-- Append-only (one row per generation); "current" is simply the highest
-- version per user.
CREATE TABLE candidate_profile_versions (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    version                INTEGER NOT NULL,
    target_roles           TEXT[] NOT NULL DEFAULT '{}',
    seniority              TEXT,
    years_experience       INTEGER,
    core_skills            TEXT[] NOT NULL DEFAULT '{}',
    secondary_skills       TEXT[] NOT NULL DEFAULT '{}',
    transferable_skills    JSONB NOT NULL DEFAULT '[]',
    domains                TEXT[] NOT NULL DEFAULT '{}',
    architecture_strengths TEXT[] NOT NULL DEFAULT '{}',
    leadership_signals     TEXT[] NOT NULL DEFAULT '{}',
    experience_evidence    TEXT[] NOT NULL DEFAULT '{}',
    summary                TEXT NOT NULL DEFAULT '',
    source_content_hash    TEXT NOT NULL,
    embedding              vector(1536),
    embedding_model        TEXT,
    embedded_at            TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, version)
);

CREATE INDEX candidate_profile_versions_user_latest_idx ON candidate_profile_versions (user_id, version DESC);

-- +goose Down
DROP TABLE candidate_profile_versions;
