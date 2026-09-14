-- +goose Up
CREATE TABLE submission_companion_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    intent_id   UUID NOT NULL REFERENCES submission_intents(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT submission_companion_tokens_hash_check CHECK (length(token_hash) = 64)
);

CREATE INDEX submission_companion_tokens_intent_idx
    ON submission_companion_tokens (intent_id, expires_at DESC);

CREATE INDEX submission_companion_tokens_active_idx
    ON submission_companion_tokens (token_hash, expires_at)
    WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE submission_companion_tokens;
