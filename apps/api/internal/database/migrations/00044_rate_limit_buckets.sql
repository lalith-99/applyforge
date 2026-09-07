-- +goose Up
CREATE TABLE rate_limit_buckets (
    scope        TEXT NOT NULL,
    limit_key    TEXT NOT NULL,
    bucket_start TIMESTAMPTZ NOT NULL,
    count        INTEGER NOT NULL DEFAULT 0,
    expires_at   TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (scope, limit_key, bucket_start),
    CONSTRAINT rate_limit_buckets_count_check CHECK (count >= 0)
);

CREATE INDEX rate_limit_buckets_expiry_idx ON rate_limit_buckets (expires_at);

-- +goose Down
DROP TABLE rate_limit_buckets;
