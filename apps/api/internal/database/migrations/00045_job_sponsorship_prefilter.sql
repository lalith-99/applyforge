-- +goose Up
-- Cheap role-level immigration prefilter. This flag is intentionally narrow:
-- true means the posting itself contains explicit negative sponsorship text.
-- Ambiguous authorization language remains false and is evaluated later by
-- the richer matching immigration assessment.
ALTER TABLE jobs
    ADD COLUMN explicit_sponsorship_denied BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX jobs_active_sponsorship_compatible_idx
    ON jobs (country_code, role_classification, posted_at DESC)
    WHERE status = 'ACTIVE'
      AND canonical_job_id IS NULL
      AND explicit_sponsorship_denied = false;

UPDATE jobs
SET explicit_sponsorship_denied = true
WHERE lower(description) LIKE ANY (ARRAY[
    '%will not sponsor%',
    '%do not sponsor%',
    '%does not sponsor%',
    '%cannot sponsor%',
    '%can''t sponsor%',
    '%unable to sponsor%',
    '%not able to sponsor%',
    '%no sponsorship%',
    '%without sponsorship now or in the future%',
    '%without visa sponsorship%',
    '%not provide visa sponsorship%',
    '%not provide sponsorship%',
    '%not eligible for visa sponsorship%',
    '%no visa sponsorship available%',
    '%does not offer sponsorship%',
    '%must not require sponsorship%',
    '%cannot provide sponsorship%'
]);

-- +goose Down
DROP INDEX jobs_active_sponsorship_compatible_idx;
ALTER TABLE jobs DROP COLUMN explicit_sponsorship_denied;
