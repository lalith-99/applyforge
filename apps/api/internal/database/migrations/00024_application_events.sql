-- +goose Up
CREATE TABLE application_events (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications (id) ON DELETE CASCADE,
    event_type     TEXT NOT NULL,
    from_status    TEXT,
    to_status      TEXT,
    notes          TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX application_events_application_id_idx ON application_events (application_id);

-- +goose Down
DROP TABLE application_events;
