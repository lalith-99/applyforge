-- +goose Up
CREATE TABLE companies (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    domain          TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX companies_normalized_name_idx ON companies (normalized_name);

-- +goose Down
DROP TABLE companies;
