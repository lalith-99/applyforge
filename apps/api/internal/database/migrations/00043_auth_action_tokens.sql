-- +goose Up
CREATE TABLE auth_action_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose     TEXT NOT NULL,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT auth_action_tokens_purpose_check
        CHECK (purpose IN ('EMAIL_VERIFY', 'PASSWORD_RESET'))
);

CREATE INDEX auth_action_tokens_user_purpose_idx
    ON auth_action_tokens (user_id, purpose, created_at DESC);
CREATE INDEX auth_action_tokens_expiry_idx
    ON auth_action_tokens (expires_at)
    WHERE used_at IS NULL;

-- +goose Down
DROP TABLE auth_action_tokens;
