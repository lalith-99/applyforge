-- +goose Up
CREATE TABLE tailoring_suggestions (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tailoring_run_id      UUID NOT NULL REFERENCES tailoring_runs (id) ON DELETE CASCADE,
    section               TEXT NOT NULL,
    original_text         TEXT,
    suggested_text        TEXT NOT NULL,
    requirements_addressed TEXT[] NOT NULL DEFAULT '{}',
    skills_added          TEXT[] NOT NULL DEFAULT '{}',
    keywords_added        TEXT[] NOT NULL DEFAULT '{}',
    source                TEXT NOT NULL,
    reason                TEXT NOT NULL DEFAULT '',
    confidence            DOUBLE PRECISION NOT NULL DEFAULT 0.6,
    risk_level            TEXT NOT NULL DEFAULT 'LOW',
    user_status           TEXT NOT NULL DEFAULT 'PENDING',
    edited_text           TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tailoring_suggestions_status_check CHECK (
        user_status IN ('PENDING', 'APPROVED', 'EDITED', 'REJECTED')
    ),
    CONSTRAINT tailoring_suggestions_risk_check CHECK (risk_level IN ('LOW', 'MEDIUM', 'HIGH'))
);

CREATE INDEX tailoring_suggestions_run_id_idx ON tailoring_suggestions (tailoring_run_id);

-- +goose Down
DROP TABLE tailoring_suggestions;
