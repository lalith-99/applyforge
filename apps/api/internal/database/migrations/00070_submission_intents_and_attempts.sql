-- +goose Up
CREATE TABLE submission_intents (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    application_id    UUID NOT NULL REFERENCES applications (id) ON DELETE CASCADE,
    package_id        UUID NOT NULL REFERENCES application_packages (id) ON DELETE RESTRICT,
    approval_id       UUID NOT NULL REFERENCES application_approvals (id) ON DELETE RESTRICT,
    package_hash      TEXT NOT NULL,
    idempotency_key   TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'PENDING',
    lease_generation  BIGINT NOT NULL DEFAULT 0,
    lease_owner       TEXT,
    lease_expires_at  TIMESTAMPTZ,
    attempt_count     INTEGER NOT NULL DEFAULT 0,
    last_error        TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at      TIMESTAMPTZ,
    CONSTRAINT submission_intents_status_check CHECK (
        status IN ('PENDING', 'CLAIMED', 'SUBMITTING', 'CONFIRMED', 'UNCERTAIN', 'FAILED', 'CANCELLED')
    ),
    CONSTRAINT submission_intents_hash_check CHECK (length(package_hash) = 64),
    UNIQUE (package_id),
    UNIQUE (idempotency_key)
);

CREATE INDEX submission_intents_claim_idx
    ON submission_intents (status, created_at)
    WHERE status IN ('PENDING', 'CLAIMED', 'SUBMITTING');

CREATE TABLE submission_attempts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    intent_id        UUID NOT NULL REFERENCES submission_intents (id) ON DELETE CASCADE,
    lease_generation BIGINT NOT NULL,
    worker_id        TEXT NOT NULL,
    state            TEXT NOT NULL DEFAULT 'CLAIMED',
    started_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitting_at    TIMESTAMPTZ,
    finished_at      TIMESTAMPTZ,
    receipt_json     JSONB,
    error_message    TEXT,
    CONSTRAINT submission_attempts_state_check CHECK (
        state IN ('CLAIMED', 'SUBMITTING', 'CONFIRMED', 'UNCERTAIN', 'FAILED')
    ),
    UNIQUE (intent_id, lease_generation)
);

CREATE INDEX submission_attempts_intent_idx
    ON submission_attempts (intent_id, lease_generation DESC);

-- +goose Down
DROP TABLE submission_attempts;
DROP TABLE submission_intents;
