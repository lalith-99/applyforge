-- +goose Up
CREATE TABLE resumes (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    original_filename TEXT NOT NULL,
    mime_type         TEXT NOT NULL,
    size_bytes        BIGINT NOT NULL,
    storage_key       TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'UPLOADED',
    parse_error       TEXT,
    raw_text          TEXT,
    parsed_profile    JSONB,
    parsed_at         TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT resumes_status_check CHECK (status IN ('UPLOADED', 'PARSING', 'PARSED', 'FAILED'))
);

CREATE INDEX resumes_user_id_idx ON resumes (user_id);

-- +goose Down
DROP TABLE resumes;
